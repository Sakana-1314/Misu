package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	"misu/internal/model"
	"misu/internal/mpweb"
	"misu/internal/secret"
)

// mpWebKeepAlive is how often an idle console session is touched so the
// console does not drop it for inactivity.
const mpWebKeepAlive = 20 * time.Minute

// MPWebService manages the scanned login to the official MP console and the
// console-side operations that do not need the third-party platform flow.
type MPWebService struct {
	db   *gorm.DB
	box  *secret.Box
	wx   *mpweb.Client
	logs *LogService
	log  *slog.Logger

	mu       sync.Mutex
	accounts *mpweb.AccountList
	expired  bool
}

func NewMPWebService(db *gorm.DB, box *secret.Box, logs *LogService, log *slog.Logger) *MPWebService {
	return &MPWebService{db: db, box: box, wx: mpweb.New(), logs: logs, log: log}
}

// MPWebAccount is a console account annotated with its platform authorization.
type MPWebAccount struct {
	mpweb.Account
	// AuthorizerAppID is set when the account is also authorized to the
	// third-party platform, so the UI can link to its detail page.
	AuthorizerAppID string `json:"authorizer_appid,omitempty"`
}

type MPWebStatus struct {
	Connected bool `json:"connected"`
	// Expired means a previous login existed but the console dropped it.
	Expired         bool           `json:"expired"`
	NickName        string         `json:"nick_name"`
	Avatar          string         `json:"avatar"`
	UserName        string         `json:"user_name"`
	LastLoginAt     *time.Time     `json:"last_login_at"`
	Accounts        []MPWebAccount `json:"accounts"`
	AccountCount    int            `json:"account_count"`
	SwitchPermitted bool           `json:"switch_permitted"`
}

// Restore loads the persisted console session at startup and keeps it alive.
func (s *MPWebService) Restore(ctx context.Context) {
	s.restore(ctx)
	go s.keepAlive(ctx)
}

func (s *MPWebService) restore(ctx context.Context) {
	var row model.MPWebSession
	if err := s.db.WithContext(ctx).Order("id DESC").First(&row).Error; err != nil {
		return
	}
	if row.CookiesEnc == "" || row.TokenEnc == "" {
		s.setExpired(true)
		return
	}
	saved, err := s.decodeRow(&row)
	if err != nil {
		s.log.Warn("failed to decode mp web session", "error", err)
		s.setExpired(true)
		return
	}
	if err := s.wx.Restore(saved); err != nil {
		s.setExpired(true)
		return
	}
	if _, err := s.wx.Check(ctx); err != nil {
		s.handleErr(ctx, err)
		s.log.Info("mp web session could not be restored", "error", err)
		return
	}
	if _, err := s.RefreshAccounts(ctx); err != nil {
		s.log.Info("mp web session restored without account list", "error", err)
	}
	// The console may have rotated the token or the identity may now parse;
	// keep the stored copy current.
	if err := s.persist(ctx); err != nil {
		s.log.Warn("failed to persist mp web session", "error", err)
	}
}

func (s *MPWebService) keepAlive(ctx context.Context) {
	ticker := time.NewTicker(mpWebKeepAlive)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !s.wx.Connected() {
				continue
			}
			checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			if _, err := s.wx.Check(checkCtx); err != nil {
				s.handleErr(checkCtx, err)
			}
			cancel()
		}
	}
}

func (s *MPWebService) decodeRow(row *model.MPWebSession) (*mpweb.SavedSession, error) {
	raw, err := s.box.Decrypt(row.CookiesEnc)
	if err != nil {
		return nil, err
	}
	token, err := s.box.Decrypt(row.TokenEnc)
	if err != nil {
		return nil, err
	}
	saved := &mpweb.SavedSession{}
	if err := json.Unmarshal([]byte(raw), saved); err != nil || len(saved.Cookies) == 0 {
		// Rows written before the fingerprint was stored hold a bare cookie list.
		var cookies []mpweb.SessionCookie
		if err := json.Unmarshal([]byte(raw), &cookies); err != nil {
			return nil, err
		}
		saved = &mpweb.SavedSession{Cookies: cookies}
	}
	saved.Token = token
	if saved.Profile.NickName == "" {
		saved.Profile.NickName = row.NickName
	}
	if saved.Profile.Avatar == "" {
		saved.Profile.Avatar = row.Avatar
	}
	return saved, nil
}

func (s *MPWebService) setExpired(expired bool) {
	s.mu.Lock()
	s.expired = expired
	if expired {
		s.accounts = nil
	}
	s.mu.Unlock()
}

// handleErr drops the in-memory session when the console reports it gone, so
// the UI asks for a fresh scan instead of failing every call.
func (s *MPWebService) handleErr(ctx context.Context, err error) {
	if !errors.Is(err, mpweb.ErrNotLoggedIn) {
		return
	}
	s.wx.Logout()
	s.setExpired(true)
	if dbErr := s.db.WithContext(ctx).Model(&model.MPWebSession{}).Where("1 = 1").Updates(map[string]any{
		"cookies_enc": "",
		"token_enc":   "",
		"updated_at":  time.Now(),
	}).Error; dbErr != nil {
		s.log.Warn("failed to clear expired mp web session", "error", dbErr)
	}
}

func (s *MPWebService) Status(ctx context.Context) *MPWebStatus {
	status := &MPWebStatus{Accounts: []MPWebAccount{}}
	var row model.MPWebSession
	hasRow := s.db.WithContext(ctx).Order("id DESC").First(&row).Error == nil
	if hasRow {
		if !row.LastLoginAt.IsZero() {
			t := row.LastLoginAt
			status.LastLoginAt = &t
		}
		status.NickName = row.NickName
		status.Avatar = row.Avatar
	}
	if !s.wx.Connected() {
		s.mu.Lock()
		status.Expired = s.expired || hasRow
		s.mu.Unlock()
		return status
	}
	status.Connected = true
	if profile, err := s.wx.Profile(); err == nil {
		status.NickName = firstNonEmpty(profile.NickName, status.NickName)
		status.Avatar = firstNonEmpty(profile.Avatar, status.Avatar)
		status.UserName = profile.UserName
	}
	s.mu.Lock()
	list := s.accounts
	s.mu.Unlock()
	if list != nil {
		status.Accounts = s.annotate(ctx, list.Accounts)
		status.SwitchPermitted = list.SwitchPermitted
	}
	status.AccountCount = len(status.Accounts)
	return status
}

// annotate links console accounts to authorized mini programs by appid or
// original id (gh_...), whichever the console reported.
func (s *MPWebService) annotate(ctx context.Context, accounts []mpweb.Account) []MPWebAccount {
	out := make([]MPWebAccount, 0, len(accounts))
	appids := make([]string, 0, len(accounts))
	names := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if account.AppID != "" {
			appids = append(appids, account.AppID)
		}
		if account.UserName != "" {
			names = append(names, account.UserName)
		}
	}
	linked := map[string]string{}
	if len(appids) > 0 || len(names) > 0 {
		var rows []model.Authorizer
		if err := s.db.WithContext(ctx).Select("app_id", "user_name").
			Where("(app_id IN ? OR user_name IN ?) AND status = ?", append(appids, ""), append(names, ""), "active").
			Find(&rows).Error; err == nil {
			for _, row := range rows {
				linked[row.AppID] = row.AppID
				if row.UserName != "" {
					linked[row.UserName] = row.AppID
				}
			}
		}
	}
	for _, account := range accounts {
		appid := linked[account.AppID]
		if appid == "" {
			appid = linked[account.UserName]
		}
		out = append(out, MPWebAccount{Account: account, AuthorizerAppID: appid})
	}
	return out
}

func (s *MPWebService) StartLogin(ctx context.Context, actor, ip string) (*mpweb.ScanState, error) {
	state, _, err := s.wx.StartLogin(ctx)
	if err != nil {
		return nil, err
	}
	s.logs.Operation(actor, "mpweb_login_start", "", nil, ip)
	return state, nil
}

func (s *MPWebService) CancelLogin() {
	s.wx.CancelLogin()
}

func (s *MPWebService) PollLogin(ctx context.Context, actor, ip string) (*mpweb.ScanState, error) {
	state, err := s.wx.PollLogin(ctx)
	if err != nil {
		return nil, err
	}
	if state.Status != mpweb.StatusConfirmed {
		return state, nil
	}
	s.setExpired(false)
	list, err := s.RefreshAccounts(ctx)
	if err != nil {
		s.log.Warn("failed to list console accounts", "error", err)
	} else {
		state.AccountCnt = len(list)
	}
	if err := s.persist(ctx); err != nil {
		s.log.Warn("failed to persist mp web session", "error", err)
	}
	s.logs.Operation(actor, "mpweb_login_success", state.NickName, nil, ip)
	return state, nil
}

func (s *MPWebService) persist(ctx context.Context) error {
	saved, err := s.wx.Export()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	cookiesEnc, err := s.box.Encrypt(string(raw))
	if err != nil {
		return err
	}
	tokenEnc, err := s.box.Encrypt(saved.Token)
	if err != nil {
		return err
	}
	s.mu.Lock()
	accountCount := 0
	if s.accounts != nil {
		accountCount = len(s.accounts.Accounts)
	}
	s.mu.Unlock()

	now := time.Now()
	var row model.MPWebSession
	findErr := s.db.WithContext(ctx).Order("id DESC").First(&row).Error
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return findErr
	}
	if errors.Is(findErr, gorm.ErrRecordNotFound) {
		row = model.MPWebSession{
			CookiesEnc:   cookiesEnc,
			TokenEnc:     tokenEnc,
			NickName:     saved.Profile.NickName,
			Avatar:       saved.Profile.Avatar,
			AccountCount: accountCount,
			LastLoginAt:  now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		return s.db.WithContext(ctx).Create(&row).Error
	}
	return s.db.WithContext(ctx).Model(&model.MPWebSession{}).Where("id = ?", row.ID).Updates(map[string]any{
		"cookies_enc":   cookiesEnc,
		"token_enc":     tokenEnc,
		"nick_name":     saved.Profile.NickName,
		"avatar":        saved.Profile.Avatar,
		"account_count": accountCount,
		"last_login_at": now,
		"updated_at":    now,
	}).Error
}

func (s *MPWebService) Logout(ctx context.Context, actor, ip string) error {
	s.wx.Logout()
	s.mu.Lock()
	s.accounts = nil
	s.expired = false
	s.mu.Unlock()
	if err := s.db.WithContext(ctx).Where("1 = 1").Delete(&model.MPWebSession{}).Error; err != nil {
		return err
	}
	s.logs.Operation(actor, "mpweb_logout", "", nil, ip)
	return nil
}

func (s *MPWebService) RefreshAccounts(ctx context.Context) ([]MPWebAccount, error) {
	list, err := s.wx.Accounts(ctx)
	if err != nil {
		s.handleErr(ctx, err)
		return nil, err
	}
	s.mu.Lock()
	s.accounts = list
	s.mu.Unlock()
	return s.annotate(ctx, list.Accounts), nil
}

func (s *MPWebService) Version(ctx context.Context, appid string) (*mpweb.VersionState, error) {
	state, err := s.wx.Version(ctx, appid)
	if err != nil {
		s.handleErr(ctx, err)
		return nil, err
	}
	// Switching accounts rotates the console token; keep the stored copy fresh.
	if err := s.persist(ctx); err != nil {
		s.log.Warn("failed to persist mp web session", "error", err)
	}
	return state, nil
}

func (s *MPWebService) QRCode() ([]byte, error) {
	return s.wx.QRCode()
}

func (s *MPWebService) Connected() bool {
	return s.wx.Connected()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
