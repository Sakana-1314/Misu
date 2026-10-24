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

// MPWebService manages the scanned login to the official MP console and the
// console-side operations that do not need the third-party platform flow.
type MPWebService struct {
	db   *gorm.DB
	box  *secret.Box
	wx   *mpweb.Client
	logs *LogService
	log  *slog.Logger

	mu       sync.Mutex
	accounts []mpweb.Account
}

func NewMPWebService(db *gorm.DB, box *secret.Box, logs *LogService, log *slog.Logger) *MPWebService {
	return &MPWebService{db: db, box: box, wx: mpweb.New(), logs: logs, log: log}
}

type MPWebStatus struct {
	Connected    bool           `json:"connected"`
	NickName     string         `json:"nick_name"`
	Avatar       string         `json:"avatar"`
	LastLoginAt  *time.Time     `json:"last_login_at"`
	Accounts     []mpweb.Account `json:"accounts"`
	AccountCount int            `json:"account_count"`
	CurrentAppID string         `json:"current_appid"`
}

// Restore loads the persisted console session at startup.
func (s *MPWebService) Restore(ctx context.Context) {
	var row model.MPWebSession
	if err := s.db.WithContext(ctx).Order("id DESC").First(&row).Error; err != nil {
		return
	}
	cookieJSON, err := s.box.Decrypt(row.CookiesEnc)
	if err != nil {
		s.log.Warn("failed to decrypt mp session cookies", "error", err)
		return
	}
	token, err := s.box.Decrypt(row.TokenEnc)
	if err != nil {
		s.log.Warn("failed to decrypt mp session token", "error", err)
		return
	}
	var cookies []mpweb.SessionCookie
	if err := json.Unmarshal([]byte(cookieJSON), &cookies); err != nil {
		return
	}
	session, err := mpweb.Restore(cookies, token, row.NickName, row.Avatar)
	if err != nil {
		return
	}
	s.wx.SetSession(session)
	accounts, err := s.wx.Accounts(ctx)
	if err != nil {
		s.log.Info("mp session restore could not list accounts", "error", err)
		return
	}
	s.mu.Lock()
	s.accounts = accounts
	s.mu.Unlock()
}

func (s *MPWebService) Status(ctx context.Context) *MPWebStatus {
	status := &MPWebStatus{Accounts: []mpweb.Account{}}
	if !s.wx.Connected() {
		return status
	}
	status.Connected = true
	s.mu.Lock()
	status.Accounts = append([]mpweb.Account{}, s.accounts...)
	s.mu.Unlock()
	status.AccountCount = len(status.Accounts)
	if profile, err := s.wx.Profile(ctx); err == nil {
		status.NickName = profile.NickName
		status.Avatar = profile.Avatar
	}
	var row model.MPWebSession
	if err := s.db.WithContext(ctx).Order("id DESC").First(&row).Error; err == nil {
		t := row.LastLoginAt
		if !t.IsZero() {
			status.LastLoginAt = &t
		}
		status.NickName = firstNonEmpty(status.NickName, row.NickName)
		status.Avatar = firstNonEmpty(status.Avatar, row.Avatar)
	}
	return status
}

func (s *MPWebService) StartLogin(ctx context.Context, actor, ip string) (*mpweb.ScanState, []byte, error) {
	state, qr, err := s.wx.StartLogin(ctx)
	if err != nil {
		return nil, nil, err
	}
	s.logs.Operation(actor, "mpweb_login_start", "", nil, ip)
	return state, qr, nil
}

func (s *MPWebService) PollLogin(ctx context.Context) (*mpweb.ScanState, error) {
	state, err := s.wx.PollLogin(ctx)
	if err != nil {
		return nil, err
	}
	if state.Status == "confirmed" {
		if err := s.persist(ctx, state); err != nil {
			s.log.Warn("failed to persist mp web session", "error", err)
		}
		accounts, err := s.wx.Accounts(ctx)
		if err != nil {
			s.log.Warn("failed to list console accounts", "error", err)
		} else {
			s.mu.Lock()
			s.accounts = accounts
			s.mu.Unlock()
			state.AccountCnt = len(accounts)
		}
	}
	return state, nil
}

func (s *MPWebService) persist(ctx context.Context, state *mpweb.ScanState) error {
	session := s.wx.CurrentSession()
	if session == nil {
		return errors.New("没有可保存的网页会话")
	}
	rawCookies, err := json.Marshal(session.Cookies())
	if err != nil {
		return err
	}
	cookiesEnc, err := s.box.Encrypt(string(rawCookies))
	if err != nil {
		return err
	}
	tokenEnc, err := s.box.Encrypt(session.Token())
	if err != nil {
		return err
	}
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
			NickName:     state.NickName,
			Avatar:       state.Avatar,
			AccountCount: state.AccountCnt,
			LastLoginAt:  now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		return s.db.WithContext(ctx).Create(&row).Error
	}
	return s.db.WithContext(ctx).Model(&model.MPWebSession{}).Where("id = ?", row.ID).Updates(map[string]any{
		"cookies_enc":   cookiesEnc,
		"token_enc":     tokenEnc,
		"nick_name":     state.NickName,
		"avatar":        state.Avatar,
		"account_count": state.AccountCnt,
		"last_login_at": now,
		"updated_at":    now,
	}).Error
}

func (s *MPWebService) Logout(ctx context.Context, actor, ip string) error {
	s.wx.Logout()
	s.mu.Lock()
	s.accounts = nil
	s.mu.Unlock()
	if err := s.db.WithContext(ctx).Where("1 = 1").Delete(&model.MPWebSession{}).Error; err != nil {
		return err
	}
	s.logs.Operation(actor, "mpweb_logout", "", nil, ip)
	return nil
}

func (s *MPWebService) RefreshAccounts(ctx context.Context) ([]mpweb.Account, error) {
	accounts, err := s.wx.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.accounts = accounts
	s.mu.Unlock()
	return accounts, nil
}

// Accounts returns the console-visible mini programs, refreshing when empty.
func (s *MPWebService) Accounts(ctx context.Context) ([]mpweb.Account, error) {
	s.mu.Lock()
	cached := append([]mpweb.Account{}, s.accounts...)
	s.mu.Unlock()
	if len(cached) > 0 {
		return cached, nil
	}
	return s.RefreshAccounts(ctx)
}

func (s *MPWebService) AccountsByAppID(ctx context.Context) (map[string]mpweb.Account, error) {
	accounts, err := s.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]mpweb.Account, len(accounts))
	for _, account := range accounts {
		out[account.AppID] = account
	}
	return out, nil
}

func (s *MPWebService) Version(ctx context.Context, appid string) (*mpweb.VersionState, error) {
	return s.wx.Version(ctx, appid)
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
