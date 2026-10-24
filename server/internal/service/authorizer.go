package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"misu/internal/config"
	"misu/internal/model"
	"misu/internal/secret"
	"misu/internal/wechat"
)

type AuthorizerService struct {
	cfg    *config.Config
	db     *gorm.DB
	box    *secret.Box
	wx     *wechat.Client
	tokens *TokenManager
	logs   *LogService
	log    *slog.Logger
	syncMu sync.Mutex
}

func NewAuthorizerService(cfg *config.Config, db *gorm.DB, box *secret.Box, wx *wechat.Client, tokens *TokenManager, logs *LogService, log *slog.Logger) *AuthorizerService {
	return &AuthorizerService{cfg: cfg, db: db, box: box, wx: wx, tokens: tokens, logs: logs, log: log}
}

type ListQuery struct {
	Keyword  string
	Status   string
	Page     int
	PageSize int
}

func (s *AuthorizerService) List(ctx context.Context, q ListQuery) ([]model.Authorizer, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}
	base := s.db.WithContext(ctx).Model(&model.Authorizer{})
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("app_id ILIKE ? OR nick_name ILIKE ? OR principal_name ILIKE ?", like, like, like)
	}
	if q.Status == model.AuthorizerActive || q.Status == model.AuthorizerUnauthorized {
		base = base.Where("status = ?", q.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Authorizer
	err := base.Order("status ASC, updated_at DESC").
		Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error
	return items, total, err
}

func (s *AuthorizerService) Get(ctx context.Context, appid string) (*model.Authorizer, error) {
	var a model.Authorizer
	if err := s.db.WithContext(ctx).Where("app_id = ?", appid).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

type SyncResult struct {
	Total        int      `json:"total"`
	Added        int      `json:"added"`
	Updated      int      `json:"updated"`
	Unauthorized int      `json:"unauthorized"`
	InfoFailed   int      `json:"info_failed"`
	Errors       []string `json:"errors"`
}

func (s *AuthorizerService) SyncAll(ctx context.Context, actor, ip string) (*SyncResult, error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	comp, err := s.tokens.GetComponent(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]wechat.AuthorizerListEntry, 0)
	seen := make(map[string]bool)
	const pageSize = 500
	offset := 0
	for page := 0; page < 10; page++ {
		resp, err := s.wx.GetAuthorizerList(ctx, comp, s.cfg.ComponentAppID, offset, pageSize)
		if err != nil {
			return nil, err
		}
		for _, entry := range resp.List {
			if seen[entry.AuthorizerAppID] {
				continue
			}
			seen[entry.AuthorizerAppID] = true
			entries = append(entries, entry)
		}
		offset += len(resp.List)
		if len(resp.List) < pageSize || offset >= resp.TotalCount || len(resp.List) == 0 {
			break
		}
	}

	result := &SyncResult{Total: len(entries)}
	allAppIDs := make([]string, 0, len(entries))
	newAppIDs := make([]string, 0)
	for _, entry := range entries {
		if entry.AuthorizerAppID == "" {
			continue
		}
		allAppIDs = append(allAppIDs, entry.AuthorizerAppID)
		var a model.Authorizer
		err := s.db.WithContext(ctx).Where("app_id = ?", entry.AuthorizerAppID).First(&a).Error
		isNew := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !isNew {
			result.Errors = appendError(result.Errors, fmt.Sprintf("%s: %v", entry.AuthorizerAppID, err))
			continue
		}
		var authTime *time.Time
		if entry.AuthTime > 0 {
			t := time.Unix(entry.AuthTime, 0)
			authTime = &t
		}
		if isNew {
			a = model.Authorizer{
				AppID:        entry.AuthorizerAppID,
				Status:       model.AuthorizerActive,
				AuthTime:     authTime,
				LastSyncedAt: ptrTime(time.Now()),
			}
			if entry.RefreshToken != "" {
				enc, encErr := s.box.Encrypt(entry.RefreshToken)
				if encErr != nil {
					result.Errors = appendError(result.Errors, fmt.Sprintf("%s: 加密 refresh_token 失败", entry.AuthorizerAppID))
					continue
				}
				a.RefreshTokenEnc = enc
			}
			if err := s.db.WithContext(ctx).Create(&a).Error; err != nil {
				result.Errors = appendError(result.Errors, fmt.Sprintf("%s: %v", entry.AuthorizerAppID, err))
				continue
			}
			result.Added++
			newAppIDs = append(newAppIDs, entry.AuthorizerAppID)
			continue
		}
		updates := map[string]any{
			"status":         model.AuthorizerActive,
			"last_synced_at": time.Now(),
			"updated_at":     time.Now(),
		}
		if entry.RefreshToken != "" {
			enc, encErr := s.box.Encrypt(entry.RefreshToken)
			if encErr != nil {
				result.Errors = appendError(result.Errors, fmt.Sprintf("%s: 加密 refresh_token 失败", entry.AuthorizerAppID))
			} else {
				updates["refresh_token_enc"] = enc
			}
		}
		if a.AuthTime == nil && authTime != nil {
			updates["auth_time"] = authTime
		}
		if err := s.db.WithContext(ctx).Model(&model.Authorizer{}).Where("app_id = ?", entry.AuthorizerAppID).Updates(updates).Error; err != nil {
			result.Errors = appendError(result.Errors, fmt.Sprintf("%s: %v", entry.AuthorizerAppID, err))
		}
		if a.NickName == "" {
			newAppIDs = append(newAppIDs, entry.AuthorizerAppID)
		}
		result.Updated++
	}

	if len(newAppIDs) > 0 {
		sem := make(chan struct{}, 3)
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, appid := range newAppIDs {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if _, err := s.SyncInfo(ctx, id); err != nil {
					mu.Lock()
					result.InfoFailed++
					result.Errors = appendError(result.Errors, fmt.Sprintf("%s: %v", id, err))
					mu.Unlock()
				}
			}(appid)
		}
		wg.Wait()
	}

	// The list API only returns currently authorized accounts. Anything else
	// that we previously considered active is no longer valid.
	q := s.db.WithContext(ctx).Model(&model.Authorizer{}).Where("status = ?", model.AuthorizerActive)
	if len(allAppIDs) > 0 {
		q = q.Where("app_id NOT IN ?", allAppIDs)
	}
	res := q.Updates(map[string]any{
		"status":                  model.AuthorizerUnauthorized,
		"access_token_enc":        "",
		"access_token_expires_at": nil,
		"refresh_token_enc":       "",
		"updated_at":              time.Now(),
	})
	if res.Error != nil {
		result.Errors = appendError(result.Errors, res.Error.Error())
	} else {
		result.Unauthorized = int(res.RowsAffected)
	}

	s.logs.Operation(actor, "authorizer_sync", "", result, ip)
	return result, nil
}

func (s *AuthorizerService) SyncInfo(ctx context.Context, appid string) (*model.Authorizer, error) {
	comp, err := s.tokens.GetComponent(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := s.wx.GetAuthorizerInfo(ctx, comp, s.cfg.ComponentAppID, appid)
	if err != nil {
		return nil, err
	}
	if resp.AuthorizerInfo.NickName == "" && resp.AuthorizationInfo.AuthorizerAppID == "" {
		return nil, errors.New("微信未返回授权账号信息，请确认该账号已授权")
	}
	return s.upsertAuthorizer(ctx, appid, &resp.AuthorizerInfo, &resp.AuthorizationInfo)
}

func (s *AuthorizerService) ExchangeAuthCode(ctx context.Context, code string) (*model.Authorizer, error) {
	comp, err := s.tokens.GetComponent(ctx)
	if err != nil {
		return nil, err
	}
	authResp, err := s.wx.QueryAuth(ctx, comp, s.cfg.ComponentAppID, code)
	if err != nil {
		return nil, err
	}
	info := authResp.AuthorizationInfo
	if info.AuthorizerAppID == "" {
		return nil, errors.New("授权码无效或已过期")
	}
	infoResp, infoErr := s.wx.GetAuthorizerInfo(ctx, comp, s.cfg.ComponentAppID, info.AuthorizerAppID)
	var ai *wechat.AuthorizerInfo
	if infoErr == nil {
		ai = &infoResp.AuthorizerInfo
		if infoResp.AuthorizationInfo.AuthorizerRefreshToken != "" {
			info.AuthorizerRefreshToken = infoResp.AuthorizationInfo.AuthorizerRefreshToken
		}
	} else {
		s.log.Warn("failed to get authorizer info after exchange", "appid", info.AuthorizerAppID, "error", infoErr)
	}
	a, upsertErr := s.upsertAuthorizer(ctx, info.AuthorizerAppID, ai, &info)
	if upsertErr != nil {
		return nil, upsertErr
	}
	s.logs.Operation("wechat_callback", "authorizer_authorized", info.AuthorizerAppID, map[string]any{
		"appid": info.AuthorizerAppID,
	}, "")
	return a, nil
}

func (s *AuthorizerService) upsertAuthorizer(ctx context.Context, appid string, info *wechat.AuthorizerInfo, auth *wechat.AuthorizationInfo) (*model.Authorizer, error) {
	var a model.Authorizer
	err := s.db.WithContext(ctx).Where("app_id = ?", appid).First(&a).Error
	isNew := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !isNew {
		return nil, err
	}
	now := time.Now()
	if isNew {
		a = model.Authorizer{AppID: appid, Status: model.AuthorizerActive, AuthTime: &now}
		if err := s.db.WithContext(ctx).Create(&a).Error; err != nil {
			return nil, err
		}
	}
	updates := map[string]any{
		"status":         model.AuthorizerActive,
		"last_synced_at": now,
		"updated_at":     now,
	}
	if info != nil {
		funcJSON := json.RawMessage("null")
		if auth != nil && len(auth.FuncInfo) > 0 {
			if raw, err := json.Marshal(auth.FuncInfo); err == nil {
				funcJSON = raw
			}
		}
		mpJSON := json.RawMessage("null")
		if len(info.MiniProgramInfo) > 0 && string(info.MiniProgramInfo) != "null" {
			mpJSON = info.MiniProgramInfo
		}
		updates["nick_name"] = info.NickName
		updates["head_img"] = info.HeadImg
		updates["principal_name"] = info.PrincipalName
		updates["user_name"] = info.UserName
		updates["alias"] = info.Alias
		updates["qr_code_url"] = info.QRCodeURL
		updates["signature"] = info.Signature
		updates["service_type_id"] = info.ServiceTypeInfo.ID
		updates["service_type_name"] = info.ServiceTypeInfo.Name
		updates["verify_type_id"] = info.VerifyTypeInfo.ID
		updates["verify_type_name"] = info.VerifyTypeInfo.Name
		updates["account_status"] = info.AccountStatus
		updates["register_type"] = info.RegisterType
		updates["mini_program_info"] = datatypes.JSON(mpJSON)
		updates["func_info"] = datatypes.JSON(funcJSON)
	}
	if auth != nil {
		if auth.AuthorizerRefreshToken != "" {
			enc, encErr := s.box.Encrypt(auth.AuthorizerRefreshToken)
			if encErr != nil {
				return nil, encErr
			}
			updates["refresh_token_enc"] = enc
		}
		if auth.AuthorizerAccessToken != "" {
			enc, encErr := s.box.Encrypt(auth.AuthorizerAccessToken)
			if encErr != nil {
				return nil, encErr
			}
			expiresIn := auth.ExpiresIn
			if expiresIn <= 0 {
				expiresIn = 7200
			}
			expires := now.Add(time.Duration(expiresIn) * time.Second)
			updates["access_token_enc"] = enc
			updates["access_token_expires_at"] = expires
		}
	}
	if err := s.db.WithContext(ctx).Model(&model.Authorizer{}).Where("app_id = ?", appid).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Where("app_id = ?", appid).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *AuthorizerService) MarkUnauthorized(ctx context.Context, appid string) error {
	err := s.db.WithContext(ctx).Model(&model.Authorizer{}).Where("app_id = ?", appid).Updates(map[string]any{
		"status":                  model.AuthorizerUnauthorized,
		"access_token_enc":        "",
		"access_token_expires_at": nil,
		"refresh_token_enc":       "",
		"updated_at":              time.Now(),
	}).Error
	s.tokens.InvalidateAuthorizer(appid)
	return err
}

// CallWithToken runs fn with a valid authorizer_access_token, refreshing it
// once if WeChat reports that the token is invalid.
func (s *AuthorizerService) CallWithToken(ctx context.Context, appid string, fn func(token string) error) error {
	return s.withToken(ctx, appid, fn)
}

func (s *AuthorizerService) withToken(ctx context.Context, appid string, fn func(token string) error) error {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		token, err := s.tokens.GetAuthorizer(ctx, appid)
		if err != nil {
			return err
		}
		err = fn(token)
		if err == nil {
			return nil
		}
		lastErr = err
		if wechat.IsTokenInvalid(err) {
			s.tokens.InvalidateAuthorizer(appid)
			continue
		}
		return err
	}
	return lastErr
}

func (s *AuthorizerService) Version(ctx context.Context, appid string) (*wechat.VersionInfoResponse, error) {
	var out *wechat.VersionInfoResponse
	err := s.withToken(ctx, appid, func(token string) error {
		resp, err := s.wx.GetVersionInfo(ctx, appid, token)
		if err == nil {
			out = resp
		}
		return err
	})
	return out, err
}

func (s *AuthorizerService) LatestAudit(ctx context.Context, appid string) (*wechat.LatestAuditStatus, error) {
	var out *wechat.LatestAuditStatus
	err := s.withToken(ctx, appid, func(token string) error {
		resp, err := s.wx.GetLatestAuditStatus(ctx, appid, token)
		if err == nil {
			out = resp
		}
		return err
	})
	if err != nil {
		var apiErr *wechat.APIError
		if errors.As(err, &apiErr) && apiErr.Code == 85058 {
			return &wechat.LatestAuditStatus{AuditID: 0, Status: -1, Reason: "暂无审核记录"}, nil
		}
	}
	return out, err
}

func (s *AuthorizerService) AuditStatus(ctx context.Context, appid string, auditID int64) (*wechat.AuditStatus, error) {
	var out *wechat.AuditStatus
	err := s.withToken(ctx, appid, func(token string) error {
		resp, err := s.wx.GetAuditStatus(ctx, appid, token, auditID)
		if err == nil {
			out = resp
		}
		return err
	})
	return out, err
}

func (s *AuthorizerService) Pages(ctx context.Context, appid string) ([]string, error) {
	var out []string
	err := s.withToken(ctx, appid, func(token string) error {
		pages, err := s.wx.GetPages(ctx, appid, token)
		if err == nil {
			out = pages
		}
		return err
	})
	return out, err
}

func (s *AuthorizerService) SettingCategories(ctx context.Context, appid string) (*wechat.SettingCategoriesResponse, error) {
	var out *wechat.SettingCategoriesResponse
	err := s.withToken(ctx, appid, func(token string) error {
		resp, err := s.wx.GetSettingCategories(ctx, appid, token)
		if err == nil {
			out = resp
		}
		return err
	})
	return out, err
}

func (s *AuthorizerService) AllCategories(ctx context.Context, appid string) (*wechat.AllCategoriesResponse, error) {
	var out *wechat.AllCategoriesResponse
	err := s.withToken(ctx, appid, func(token string) error {
		resp, err := s.wx.GetAllCategories(ctx, appid, token)
		if err == nil {
			out = resp
		}
		return err
	})
	return out, err
}

func (s *AuthorizerService) Domains(ctx context.Context, appid string) (*wechat.DomainResponse, error) {
	var out *wechat.DomainResponse
	err := s.withToken(ctx, appid, func(token string) error {
		resp, err := s.wx.ModifyDomain(ctx, appid, token, wechat.DomainRequest{Action: "get"})
		if err == nil {
			out = resp
		}
		return err
	})
	return out, err
}

func (s *AuthorizerService) Testers(ctx context.Context, appid string) ([]string, error) {
	var out []string
	err := s.withToken(ctx, appid, func(token string) error {
		list, err := s.wx.GetTesters(ctx, appid, token)
		if err == nil {
			out = list
		}
		return err
	})
	return out, err
}

func (s *AuthorizerService) BindTester(ctx context.Context, appid, wechatID string) (string, error) {
	var userstr string
	err := s.withToken(ctx, appid, func(token string) error {
		resp, err := s.wx.BindTester(ctx, appid, token, wechatID)
		if err == nil {
			userstr = resp.UserStr
		}
		return err
	})
	return userstr, err
}

func (s *AuthorizerService) UnbindTester(ctx context.Context, appid, userstr string) error {
	return s.withToken(ctx, appid, func(token string) error {
		return s.wx.UnbindTester(ctx, appid, token, userstr)
	})
}

func (s *AuthorizerService) TrialQRCode(ctx context.Context, appid, path string) ([]byte, string, error) {
	var data []byte
	var contentType string
	err := s.withToken(ctx, appid, func(token string) error {
		b, ct, err := s.wx.GetTrialQRCode(ctx, appid, token, path)
		if err == nil {
			data, contentType = b, ct
		}
		return err
	})
	return data, contentType, err
}

func (s *AuthorizerService) UnlimitedQRCode(ctx context.Context, appid string, req wechat.UnlimitedQRCodeRequest) ([]byte, string, error) {
	var data []byte
	var contentType string
	err := s.withToken(ctx, appid, func(token string) error {
		b, ct, err := s.wx.GetUnlimitedQRCode(ctx, appid, token, req)
		if err == nil {
			data, contentType = b, ct
		}
		return err
	})
	return data, contentType, err
}

func (s *AuthorizerService) HistoryVersions(ctx context.Context, appid string) (*wechat.HistoryVersionResponse, error) {
	var out *wechat.HistoryVersionResponse
	err := s.withToken(ctx, appid, func(token string) error {
		resp, err := s.wx.GetHistoryVersions(ctx, appid, token)
		if err == nil {
			out = resp
		}
		return err
	})
	return out, err
}

func (s *AuthorizerService) GrayPlan(ctx context.Context, appid string) (*wechat.GrayReleasePlanResponse, error) {
	var out *wechat.GrayReleasePlanResponse
	err := s.withToken(ctx, appid, func(token string) error {
		resp, err := s.wx.GetGrayReleasePlan(ctx, appid, token)
		if err == nil {
			out = resp
		}
		return err
	})
	return out, err
}

func (s *AuthorizerService) VisitStatus(ctx context.Context, appid string) (*wechat.VisitStatusResponse, error) {
	var out *wechat.VisitStatusResponse
	err := s.withToken(ctx, appid, func(token string) error {
		resp, err := s.wx.GetVisitStatus(ctx, appid, token)
		if err == nil {
			out = resp
		}
		return err
	})
	return out, err
}

func (s *AuthorizerService) PrivacyCheck(ctx context.Context, appid string) (*wechat.PrivacyCheckResponse, error) {
	var out *wechat.PrivacyCheckResponse
	err := s.withToken(ctx, appid, func(token string) error {
		resp, err := s.wx.GetPrivacyCheck(ctx, appid, token)
		if err == nil {
			out = resp
		}
		return err
	})
	return out, err
}

func (s *AuthorizerService) Media(ctx context.Context, appid, mediaID string) ([]byte, string, error) {
	var data []byte
	var contentType string
	err := s.withToken(ctx, appid, func(token string) error {
		b, ct, err := s.wx.GetMedia(ctx, appid, token, mediaID)
		if err == nil {
			data, contentType = b, ct
		}
		return err
	})
	return data, contentType, err
}

func (s *AuthorizerService) ActiveAppIDs(ctx context.Context, appids []string) ([]string, []string, error) {
	var rows []model.Authorizer
	err := s.db.WithContext(ctx).Where("app_id IN ?", appids).Find(&rows).Error
	if err != nil {
		return nil, nil, err
	}
	found := make(map[string]bool)
	active := make([]string, 0, len(rows))
	missing := make([]string, 0)
	for _, r := range rows {
		found[r.AppID] = true
		if r.Status == model.AuthorizerActive {
			active = append(active, r.AppID)
		}
	}
	for _, id := range appids {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	return active, missing, nil
}

func ptrTime(t time.Time) *time.Time { return &t }

func appendError(list []string, msg string) []string {
	if len(list) >= 20 {
		return list
	}
	return append(list, msg)
}
