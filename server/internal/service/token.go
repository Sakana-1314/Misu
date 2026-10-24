package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"misu/internal/config"
	"misu/internal/model"
	"misu/internal/secret"
	"misu/internal/wechat"
)

var (
	ErrUnconfigured = errors.New("第三方平台尚未配置，请在 .env 中填写 AppID/AppSecret/Token/AESKey")
	ErrNoTicket     = errors.New("尚未收到 component_verify_ticket，请确认回调地址并启动票据推送")
	ErrNeedReauth   = errors.New("小程序授权已失效，需要管理员重新授权")
)

type cachedToken struct {
	token   string
	expires time.Time
}

func (c cachedToken) valid(now time.Time) bool {
	return c.token != "" && now.Add(5*time.Minute).Before(c.expires)
}

type TokenManager struct {
	cfg *config.Config
	db  *gorm.DB
	box *secret.Box
	wx  *wechat.Client
	log *slog.Logger

	mu      sync.Mutex
	comp    cachedToken
	auth    map[string]cachedToken
	sf      singleflight.Group
	pushMu  sync.Mutex
	lastPush time.Time
}

func NewTokenManager(cfg *config.Config, db *gorm.DB, box *secret.Box, wx *wechat.Client, log *slog.Logger) *TokenManager {
	return &TokenManager{
		cfg:  cfg,
		db:   db,
		box:  box,
		wx:   wx,
		log:  log,
		auth: make(map[string]cachedToken),
	}
}

func (m *TokenManager) state() (*model.PlatformState, error) {
	var st model.PlatformState
	if err := m.db.Where("id = ?", 1).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			st = model.PlatformState{ID: 1}
			if err := m.db.Create(&st).Error; err != nil {
				return nil, err
			}
			return &st, nil
		}
		return nil, err
	}
	return &st, nil
}

func (m *TokenManager) GetComponent(ctx context.Context) (string, error) {
	if !m.cfg.WeChatConfigured() {
		return "", ErrUnconfigured
	}
	now := time.Now()
	m.mu.Lock()
	cached := m.comp
	m.mu.Unlock()
	if cached.valid(now) {
		return cached.token, nil
	}
	v, err, _ := m.sf.Do("component", func() (any, error) {
		m.mu.Lock()
		cached = m.comp
		m.mu.Unlock()
		if cached.valid(time.Now()) {
			return cached.token, nil
		}
		st, err := m.state()
		if err != nil {
			return "", err
		}
		if st.ComponentTokenEnc != "" && st.ComponentTokenExpiresAt != nil && now.Add(5*time.Minute).Before(*st.ComponentTokenExpiresAt) {
			token, err := m.box.Decrypt(st.ComponentTokenEnc)
			if err != nil {
				return "", fmt.Errorf("decrypt component token: %w", err)
			}
			m.setComponent(token, *st.ComponentTokenExpiresAt)
			return token, nil
		}
		ticket, err := m.box.Decrypt(st.VerifyTicketEnc)
		if err != nil {
			return "", fmt.Errorf("decrypt verify ticket: %w", err)
		}
		if ticket == "" {
			return "", ErrNoTicket
		}
		resp, err := m.wx.GetComponentToken(ctx, m.cfg.ComponentAppID, m.cfg.ComponentAppSecret, ticket)
		if err != nil {
			m.recordTokenError(err)
			return "", err
		}
		if resp.ComponentAccessToken == "" {
			err := errors.New("微信未返回 component_access_token")
			m.recordTokenError(err)
			return "", err
		}
		expiresIn := resp.ExpiresIn
		if expiresIn <= 0 {
			expiresIn = 7200
		}
		expires := time.Now().Add(time.Duration(expiresIn) * time.Second)
		enc, err := m.box.Encrypt(resp.ComponentAccessToken)
		if err != nil {
			return "", err
		}
		updates := map[string]any{
			"component_app_id":          m.cfg.ComponentAppID,
			"component_token_enc":       enc,
			"component_token_expires_at": expires,
			"last_token_error":          "",
			"updated_at":                 time.Now(),
		}
		if err := m.db.Model(&model.PlatformState{}).Where("id = ?", 1).Updates(updates).Error; err != nil {
			m.log.Warn("failed to persist component token", "error", err)
		}
		m.setComponent(resp.ComponentAccessToken, expires)
		return resp.ComponentAccessToken, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (m *TokenManager) setComponent(token string, expires time.Time) {
	m.mu.Lock()
	m.comp = cachedToken{token: token, expires: expires}
	m.mu.Unlock()
}

func (m *TokenManager) GetAuthorizer(ctx context.Context, appid string) (string, error) {
	now := time.Now()
	m.mu.Lock()
	cached := m.auth[appid]
	m.mu.Unlock()
	if cached.valid(now) {
		return cached.token, nil
	}
	v, err, _ := m.sf.Do("auth:"+appid, func() (any, error) {
		var a model.Authorizer
		if err := m.db.Where("app_id = ?", appid).First(&a).Error; err != nil {
			return "", err
		}
		if a.Status != model.AuthorizerActive {
			return "", ErrNeedReauth
		}
		m.mu.Lock()
		cached = m.auth[appid]
		m.mu.Unlock()
		if cached.valid(time.Now()) {
			return cached.token, nil
		}
		if a.AccessTokenEnc != "" && a.AccessTokenExpiresAt != nil && time.Now().Add(5*time.Minute).Before(*a.AccessTokenExpiresAt) {
			token, err := m.box.Decrypt(a.AccessTokenEnc)
			if err != nil {
				return "", fmt.Errorf("decrypt authorizer token: %w", err)
			}
			m.setAuthorizer(appid, token, *a.AccessTokenExpiresAt)
			return token, nil
		}
		refresh, err := m.box.Decrypt(a.RefreshTokenEnc)
		if err != nil {
			return "", fmt.Errorf("decrypt refresh token: %w", err)
		}
		if refresh == "" {
			return "", ErrNeedReauth
		}
		componentToken, err := m.GetComponent(ctx)
		if err != nil {
			return "", err
		}
		resp, err := m.wx.RefreshAuthorizerToken(ctx, componentToken, m.cfg.ComponentAppID, appid, refresh)
		if err != nil {
			return "", err
		}
		if resp.AuthorizerAccessToken == "" {
			return "", errors.New("微信未返回 authorizer_access_token")
		}
		expiresIn := resp.ExpiresIn
		if expiresIn <= 0 {
			expiresIn = 7200
		}
		newRefresh := resp.AuthorizerRefreshToken
		if newRefresh == "" {
			newRefresh = refresh
		}
		expires := time.Now().Add(time.Duration(expiresIn) * time.Second)
		accessEnc, err := m.box.Encrypt(resp.AuthorizerAccessToken)
		if err != nil {
			return "", err
		}
		refreshEnc, err := m.box.Encrypt(newRefresh)
		if err != nil {
			return "", err
		}
		if err := m.db.Model(&model.Authorizer{}).Where("app_id = ?", appid).Updates(map[string]any{
			"access_token_enc":         accessEnc,
			"access_token_expires_at":  expires,
			"refresh_token_enc":        refreshEnc,
			"updated_at":               time.Now(),
		}).Error; err != nil {
			m.log.Warn("failed to persist authorizer token", "appid", appid, "error", err)
		}
		m.setAuthorizer(appid, resp.AuthorizerAccessToken, expires)
		return resp.AuthorizerAccessToken, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (m *TokenManager) setAuthorizer(appid, token string, expires time.Time) {
	m.mu.Lock()
	m.auth[appid] = cachedToken{token: token, expires: expires}
	m.mu.Unlock()
}

func (m *TokenManager) InvalidateAuthorizer(appid string) {
	m.mu.Lock()
	delete(m.auth, appid)
	m.mu.Unlock()
	_ = m.db.Model(&model.Authorizer{}).Where("app_id = ?", appid).Updates(map[string]any{
		"access_token_enc":        "",
		"access_token_expires_at": nil,
	}).Error
}

func (m *TokenManager) InvalidateComponent() {
	m.mu.Lock()
	m.comp = cachedToken{}
	m.mu.Unlock()
	_ = m.db.Model(&model.PlatformState{}).Where("id = ?", 1).Updates(map[string]any{
		"component_token_enc":        "",
		"component_token_expires_at": nil,
	}).Error
}

func (m *TokenManager) SaveTicket(ticket string, createTime int64) error {
	if ticket == "" {
		return nil
	}
	st, err := m.state()
	if err != nil {
		return err
	}
	var created *time.Time
	if createTime > 0 {
		t := time.Unix(createTime, 0)
		created = &t
		if st.TicketCreatedAt != nil && t.Before(*st.TicketCreatedAt) {
			// Out-of-order push: keep the newer ticket.
			return nil
		}
	}
	enc, err := m.box.Encrypt(ticket)
	if err != nil {
		return err
	}
	now := time.Now()
	return m.db.Model(&model.PlatformState{}).Where("id = ?", 1).Updates(map[string]any{
		"component_app_id":   m.cfg.ComponentAppID,
		"verify_ticket_enc":  enc,
		"ticket_created_at":  created,
		"ticket_received_at": now,
		"updated_at":         now,
	}).Error
}

func (m *TokenManager) StartPushTicket(ctx context.Context) error {
	if !m.cfg.WeChatConfigured() {
		return ErrUnconfigured
	}
	m.pushMu.Lock()
	if time.Since(m.lastPush) < time.Minute {
		m.pushMu.Unlock()
		return errors.New("票据推送启动过于频繁，请稍后再试")
	}
	m.lastPush = time.Now()
	m.pushMu.Unlock()

	err := m.wx.StartPushTicket(ctx, m.cfg.ComponentAppID, m.cfg.ComponentAppSecret)
	updates := map[string]any{"updated_at": time.Now()}
	if err != nil {
		updates["last_push_error"] = truncate(err.Error(), 500)
	} else {
		now := time.Now()
		updates["last_push_ok_at"] = now
		updates["last_push_error"] = ""
	}
	_ = m.db.Model(&model.PlatformState{}).Where("id = ?", 1).Updates(updates).Error
	return err
}

func (m *TokenManager) recordTokenError(err error) {
	_ = m.db.Model(&model.PlatformState{}).Where("id = ?", 1).Updates(map[string]any{
		"last_token_error": truncate(err.Error(), 500),
		"updated_at":       time.Now(),
	}).Error
}

type TokenStats struct {
	Configured              bool       `json:"configured"`
	ComponentAppID          string     `json:"component_appid"`
	TicketReceivedAt        *time.Time `json:"ticket_received_at"`
	TicketCreatedAt         *time.Time `json:"ticket_created_at"`
	TicketAgeSeconds        int64      `json:"ticket_age_seconds"`
	ComponentTokenExpiresAt *time.Time `json:"component_token_expires_at"`
	ComponentTokenValid     bool       `json:"component_token_valid"`
	LastPushOKAt            *time.Time `json:"last_push_ok_at"`
	LastPushError           string     `json:"last_push_error"`
	LastTokenError          string     `json:"last_token_error"`
}

func (m *TokenManager) Stats() TokenStats {
	stats := TokenStats{Configured: m.cfg.WeChatConfigured(), ComponentAppID: m.cfg.ComponentAppID}
	st, err := m.state()
	if err != nil {
		return stats
	}
	stats.TicketReceivedAt = st.TicketReceivedAt
	stats.TicketCreatedAt = st.TicketCreatedAt
	stats.ComponentTokenExpiresAt = st.ComponentTokenExpiresAt
	stats.ComponentTokenValid = st.ComponentTokenExpiresAt != nil && st.ComponentTokenExpiresAt.After(time.Now())
	stats.LastPushOKAt = st.LastPushOKAt
	stats.LastPushError = st.LastPushError
	stats.LastTokenError = st.LastTokenError
	if st.TicketReceivedAt != nil {
		stats.TicketAgeSeconds = int64(time.Since(*st.TicketReceivedAt).Seconds())
	}
	return stats
}
