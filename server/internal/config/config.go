package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is loaded from environment variables. WeChat component credentials
// live only here and are never persisted by the application.
type Config struct {
	PublicBaseURL string
	ServerPort    int
	CookieSecure  bool

	AdminUsername string
	AdminPassword string
	JWTSecret     string
	SecretEncKey  string

	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	DBSSLMode  string

	ComponentAppID      string
	ComponentAppSecret  string
	ComponentVerifyToken string
	ComponentAESKey     string
	ComponentAESKeyPrev string

	PlatformPublished bool
	EgressIP          string

	JobConcurrency    int
	WXMaxQPS          int
	WXTimeout         time.Duration
	AuditPollInterval time.Duration
	LogRetentionDays  int
	LogLevel          string

	Version   string
	Commit    string
	BuildTime string
}

func Load() (*Config, error) {
	c := &Config{
		PublicBaseURL:         strings.TrimRight(env("PUBLIC_BASE_URL", "http://127.0.0.1:8080"), "/"),
		ServerPort:            envInt("SERVER_PORT", 8080),
		CookieSecure:          envBool("COOKIE_SECURE", true),
		AdminUsername:         env("ADMIN_USERNAME", "admin"),
		AdminPassword:         env("ADMIN_PASSWORD", ""),
		JWTSecret:             env("JWT_SECRET", ""),
		SecretEncKey:          env("SECRET_ENC_KEY", ""),
		DBHost:                env("DB_HOST", "127.0.0.1"),
		DBPort:                env("DB_PORT", "5432"),
		DBName:                env("DB_NAME", "misu"),
		DBUser:                env("DB_USER", "misu"),
		DBPassword:            env("DB_PASSWORD", ""),
		DBSSLMode:             env("DB_SSLMODE", "disable"),
		ComponentAppID:        strings.TrimSpace(os.Getenv("WX_COMPONENT_APPID")),
		ComponentAppSecret:    strings.TrimSpace(os.Getenv("WX_COMPONENT_APPSECRET")),
		ComponentVerifyToken:  strings.TrimSpace(os.Getenv("WX_COMPONENT_VERIFY_TOKEN")),
		ComponentAESKey:       strings.TrimSpace(os.Getenv("WX_COMPONENT_ENCODING_AES_KEY")),
		ComponentAESKeyPrev:   strings.TrimSpace(os.Getenv("WX_COMPONENT_ENCODING_AES_KEY_PREVIOUS")),
		PlatformPublished:     envBool("PLATFORM_PUBLISHED", false),
		EgressIP:              env("EGRESS_IP", ""),
		JobConcurrency:        envInt("JOB_CONCURRENCY", 3),
		WXMaxQPS:              envInt("WX_MAX_QPS", 8),
		WXTimeout:             time.Duration(envInt("WX_REQUEST_TIMEOUT_SECONDS", 20)) * time.Second,
		AuditPollInterval:     time.Duration(envInt("AUDIT_POLL_INTERVAL_SECONDS", 300)) * time.Second,
		LogRetentionDays:      envInt("LOG_RETENTION_DAYS", 30),
		LogLevel:              env("LOG_LEVEL", "info"),
	}

	if c.AdminPassword == "" {
		return nil, fmt.Errorf("ADMIN_PASSWORD 不能为空")
	}
	if len(c.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET 至少需要 32 个字符")
	}
	key, err := base64.StdEncoding.DecodeString(c.SecretEncKey)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("SECRET_ENC_KEY 必须是 32 字节的 base64 字符串")
	}
	if c.DBPassword == "" {
		return nil, fmt.Errorf("DB_PASSWORD 不能为空")
	}
	if c.JobConcurrency < 1 || c.JobConcurrency > 20 {
		c.JobConcurrency = 3
	}
	if c.WXMaxQPS < 1 || c.WXMaxQPS > 50 {
		c.WXMaxQPS = 8
	}
	if c.WXTimeout < 5*time.Second {
		c.WXTimeout = 20 * time.Second
	}
	if c.AuditPollInterval < 30*time.Second {
		c.AuditPollInterval = 5 * time.Minute
	}
	if c.ComponentAESKey != "" && len(c.ComponentAESKey) != 43 {
		return nil, fmt.Errorf("WX_COMPONENT_ENCODING_AES_KEY 必须是 43 位")
	}
	if c.ComponentAESKeyPrev != "" && len(c.ComponentAESKeyPrev) != 43 {
		return nil, fmt.Errorf("WX_COMPONENT_ENCODING_AES_KEY_PREVIOUS 必须是 43 位")
	}
	return c, nil
}

func (c *Config) WeChatConfigured() bool {
	return c.ComponentAppID != "" && c.ComponentAppSecret != "" && c.ComponentAESKey != "" && c.ComponentVerifyToken != ""
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
