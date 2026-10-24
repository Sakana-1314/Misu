package service

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"misu/internal/config"
)

type SettingsService struct {
	cfg       *config.Config
	db        *gorm.DB
	tokens    *TokenManager
	logs      *LogService
	log       *slog.Logger
	startedAt time.Time
}

type ConfigView struct {
	ComponentAppID     string `json:"component_appid"`
	AppSecretSet       bool   `json:"app_secret_set"`
	VerifyToken        string `json:"verify_token"`
	AESKey             string `json:"aes_key"`
	AESKeyPreviousSet  bool   `json:"aes_key_previous_set"`
	AdminUsername      string `json:"admin_username"`
	CookieSecure       bool   `json:"cookie_secure"`
}

type SettingsData struct {
	PublicBaseURL     string     `json:"public_base_url"`
	AuthRedirectURL   string     `json:"auth_redirect_url"`
	CallbackURL       string     `json:"callback_url"`
	EventURL          string     `json:"event_url"`
	EgressIP          string     `json:"egress_ip"`
	PlatformPublished bool       `json:"platform_published"`
	WeChatConfigured  bool       `json:"wechat_configured"`
	Health            TokenStats `json:"health"`
	Version           string     `json:"version"`
	Commit            string     `json:"commit"`
	BuildTime         string     `json:"build_time"`
	UptimeSeconds     int64      `json:"uptime_seconds"`
	DBOK              bool       `json:"db_ok"`
	JobConcurrency    int        `json:"job_concurrency"`
	WXMaxQPS          int        `json:"wx_max_qps"`
	LogRetentionDays  int        `json:"log_retention_days"`
	Config            ConfigView `json:"config"`
	Warnings          []string   `json:"warnings"`
}

func NewSettingsService(cfg *config.Config, db *gorm.DB, tokens *TokenManager, logs *LogService, log *slog.Logger) *SettingsService {
	return &SettingsService{cfg: cfg, db: db, tokens: tokens, logs: logs, log: log, startedAt: time.Now()}
}

func (s *SettingsService) Get(ctx context.Context, reveal bool, actor, ip string) *SettingsData {
	stats := s.tokens.Stats()
	dbOK := true
	if sqlDB, err := s.db.DB(); err != nil {
		dbOK = false
	} else {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if err := sqlDB.PingContext(pingCtx); err != nil {
			dbOK = false
		}
		cancel()
	}
	view := ConfigView{
		ComponentAppID:    s.cfg.ComponentAppID,
		AppSecretSet:      s.cfg.ComponentAppSecret != "",
		AESKeyPreviousSet: s.cfg.ComponentAESKeyPrev != "",
		AdminUsername:     s.cfg.AdminUsername,
		CookieSecure:      s.cfg.CookieSecure,
	}
	if reveal {
		view.VerifyToken = s.cfg.ComponentVerifyToken
		view.AESKey = s.cfg.ComponentAESKey
		s.logs.Operation(actor, "settings_reveal", "", map[string]any{"fields": []string{"verify_token", "aes_key"}}, ip)
	} else {
		view.VerifyToken = maskSecret(s.cfg.ComponentVerifyToken)
		view.AESKey = maskSecret(s.cfg.ComponentAESKey)
	}
	warnings := make([]string, 0)
	if !s.cfg.WeChatConfigured() {
		warnings = append(warnings, "第三方平台配置不完整，请填写 WX_COMPONENT_APPID / APPSECRET / VERIFY_TOKEN / ENCODING_AES_KEY")
	}
	if stats.TicketReceivedAt == nil {
		warnings = append(warnings, "尚未收到 component_verify_ticket，请确认回调地址并在开放平台启动票据推送")
	}
	if stats.ComponentTokenExpiresAt == nil {
		warnings = append(warnings, "尚未获取 component_access_token")
	} else if !stats.ComponentTokenValid {
		warnings = append(warnings, "component_access_token 已过期，请点击刷新或等待自动刷新")
	}
	if !s.cfg.PlatformPublished {
		warnings = append(warnings, "第三方平台尚未全网发布，只有授权测试列表内的小程序可以授权")
	}
	if stats.LastPushError != "" {
		warnings = append(warnings, "最近一次启动票据推送失败: "+stats.LastPushError)
	}
	if stats.LastTokenError != "" {
		warnings = append(warnings, "最近一次 token 获取失败: "+stats.LastTokenError)
	}
	if s.cfg.EgressIP == "" {
		warnings = append(warnings, "未配置出口 IP 展示值，请确认开放平台白名单已加入服务器出口 IP")
	}
	return &SettingsData{
		PublicBaseURL:     s.cfg.PublicBaseURL,
		AuthRedirectURL:   s.cfg.PublicBaseURL + "/auth/callback",
		CallbackURL:       s.cfg.PublicBaseURL + "/api/wechat/callback",
		EventURL:          s.cfg.PublicBaseURL + "/api/wechat/events/$APPID$",
		EgressIP:          s.cfg.EgressIP,
		PlatformPublished: s.cfg.PlatformPublished,
		WeChatConfigured:  s.cfg.WeChatConfigured(),
		Health:            stats,
		Version:           s.cfg.Version,
		Commit:            s.cfg.Commit,
		BuildTime:         s.cfg.BuildTime,
		UptimeSeconds:     int64(time.Since(s.startedAt).Seconds()),
		DBOK:              dbOK,
		JobConcurrency:    s.cfg.JobConcurrency,
		WXMaxQPS:          s.cfg.WXMaxQPS,
		LogRetentionDays:  s.cfg.LogRetentionDays,
		Config:            view,
		Warnings:          warnings,
	}
}

func (s *SettingsService) RefreshToken(ctx context.Context, actor, ip string) (time.Time, error) {
	s.tokens.InvalidateComponent()
	if _, err := s.tokens.GetComponent(ctx); err != nil {
		s.logs.Operation(actor, "component_token_refresh_failed", "", map[string]any{"error": err.Error()}, ip)
		return time.Time{}, err
	}
	stats := s.tokens.Stats()
	s.logs.Operation(actor, "component_token_refresh", "", nil, ip)
	if stats.ComponentTokenExpiresAt != nil {
		return *stats.ComponentTokenExpiresAt, nil
	}
	return time.Time{}, nil
}

func (s *SettingsService) StartPushTicket(ctx context.Context, actor, ip string) error {
	err := s.tokens.StartPushTicket(ctx)
	if err != nil {
		s.logs.Operation(actor, "ticket_push_failed", "", map[string]any{"error": err.Error()}, ip)
		return err
	}
	s.logs.Operation(actor, "ticket_push", "", nil, ip)
	return nil
}

type HealthData struct {
	Status    string `json:"status"`
	DBOK      bool   `json:"db_ok"`
	Version   string `json:"version"`
	Uptime    int64  `json:"uptime_seconds"`
	ServerTime string `json:"server_time"`
}

func (s *SettingsService) Health(ctx context.Context) HealthData {
	health := HealthData{Status: "ok", DBOK: true, Version: s.cfg.Version, Uptime: int64(time.Since(s.startedAt).Seconds()), ServerTime: time.Now().Format(time.RFC3339)}
	if sqlDB, err := s.db.DB(); err != nil {
		health.DBOK = false
		health.Status = "degraded"
	} else {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if err := sqlDB.PingContext(pingCtx); err != nil {
			health.DBOK = false
			health.Status = "degraded"
		}
		cancel()
	}
	return health
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 8 {
		return "********"
	}
	return string(runes[:4]) + "****" + string(runes[len(runes)-4:])
}
