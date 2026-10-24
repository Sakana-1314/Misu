package httpapi

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

const (
	sessionCookie = "misu_session"
	ctxRequestID  = "request_id"
	ctxAdminName  = "admin_name"
	ctxAdminExp   = "admin_exp"
)

type adminClaims struct {
	Name string `json:"name"`
	jwt.RegisteredClaims
}

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipLimiter struct {
	mu      sync.Mutex
	entries map[string]*ipEntry
	limit   rate.Limit
	burst   int
	lastGC  time.Time
}

func newIPLimiter(limit rate.Limit, burst int) *ipLimiter {
	return &ipLimiter{entries: make(map[string]*ipEntry), limit: limit, burst: burst, lastGC: time.Now()}
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.lastGC) > 10*time.Minute {
		for key, entry := range l.entries {
			if now.Sub(entry.lastSeen) > 30*time.Minute {
				delete(l.entries, key)
			}
		}
		l.lastGC = now
	}
	entry, ok := l.entries[ip]
	if !ok {
		entry = &ipEntry{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.entries[ip] = entry
	}
	entry.lastSeen = now
	return entry.limiter.Allow()
}

func (s *Server) requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := uuid.NewString()
		c.Set(ctxRequestID, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func requestID(c *gin.Context) string {
	if v, ok := c.Get(ctxRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (s *Server) requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.Request.URL.Path
		if path == "/api/health" || !strings.HasPrefix(path, "/api/") {
			return
		}
		s.log.Info("http request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", clientIP(c),
			"request_id", requestID(c),
		)
	}
}

func (s *Server) recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.log.Error("panic recovered", "panic", recovered, "path", c.Request.URL.Path, "request_id", requestID(c))
				fail(c, http.StatusInternalServerError, "INTERNAL", "服务器内部错误")
				c.Abort()
			}
		}()
		c.Next()
	}
}

func (s *Server) securityHeaders() gin.HandlerFunc {
	csp := "default-src 'self'; img-src 'self' data: blob: https:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; font-src 'self' data:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", csp)
		if strings.HasPrefix(s.cfg.PublicBaseURL, "https://") {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

func (s *Server) authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := c.Cookie(sessionCookie)
		if err != nil || raw == "" {
			fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
			c.Abort()
			return
		}
		claims := &adminClaims{}
		token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
			return []byte(s.cfg.JWTSecret), nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("misu"))
		if err != nil || !token.Valid {
			fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "登录已失效，请重新登录")
			c.Abort()
			return
		}
		c.Set(ctxAdminName, claims.Name)
		if claims.ExpiresAt != nil {
			c.Set(ctxAdminExp, claims.ExpiresAt.Time)
		}
		c.Next()
	}
}

func (s *Server) csrfProtect() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if c.GetHeader("X-Requested-With") != "XMLHttpRequest" {
			fail(c, http.StatusForbidden, "CSRF_REJECTED", "缺少请求头")
			c.Abort()
			return
		}
		if !s.originAllowed(c) {
			fail(c, http.StatusForbidden, "ORIGIN_REJECTED", "请求来源不合法")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *Server) originAllowed(c *gin.Context) bool {
	origin := c.GetHeader("Origin")
	if origin == "" {
		ref := c.GetHeader("Referer")
		if ref == "" {
			return true
		}
		origin = ref
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, s.allowedHost)
}

func (s *Server) issueToken() (string, time.Time, error) {
	now := time.Now()
	expires := now.Add(12 * time.Hour)
	claims := adminClaims{
		Name: s.cfg.AdminUsername,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "misu",
			Subject:   "admin",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWTSecret))
	return signed, expires, err
}

func (s *Server) validateAuthState(state string) error {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(state, claims, func(t *jwt.Token) (any, error) {
		return []byte(s.cfg.JWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("misu"))
	if err != nil || !token.Valid {
		return err
	}
	purpose, _ := claims["purpose"].(string)
	if purpose != "wechat_auth" {
		return jwt.ErrTokenInvalidClaims
	}
	return nil
}

func adminName(c *gin.Context) string {
	if v, ok := c.Get(ctxAdminName); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "anonymous"
}

func clientIP(c *gin.Context) string {
	if ip := strings.TrimSpace(c.GetHeader("CF-Connecting-IP")); ip != "" {
		return ip
	}
	return c.ClientIP()
}

