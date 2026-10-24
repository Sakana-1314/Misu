package httpapi

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(c *gin.Context) {
	if !s.originAllowed(c) {
		fail(c, http.StatusForbidden, "ORIGIN_REJECTED", "请求来源不合法")
		return
	}
	ip := clientIP(c)
	if !s.loginLimit.allow(ip) {
		fail(c, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "登录尝试过于频繁，请稍后再试")
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "请输入用户名和密码")
		return
	}
	userOK := subtle.ConstantTimeCompare([]byte(req.Username), []byte(s.cfg.AdminUsername)) == 1
	passOK := bcrypt.CompareHashAndPassword(s.adminHash, []byte(req.Password)) == nil
	if !userOK || !passOK {
		s.logs.Operation(req.Username, "login_failed", "", nil, ip)
		fail(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
		return
	}
	token, expires, err := s.issueToken()
	if err != nil {
		s.respondError(c, err)
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookie, token, int(12*time.Hour/time.Second), "/", "", s.cfg.CookieSecure, true)
	s.logs.Operation(req.Username, "login_success", "", nil, ip)
	ok(c, gin.H{"username": s.cfg.AdminUsername, "expires_at": expires})
}

func (s *Server) me(c *gin.Context) {
	exp, _ := c.Get(ctxAdminExp)
	ok(c, gin.H{"username": adminName(c), "expires_at": exp})
}

func (s *Server) logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookie, "", -1, "/", "", s.cfg.CookieSecure, true)
	s.logs.Operation(adminName(c), "logout", "", nil, clientIP(c))
	ok(c, gin.H{"ok": true})
}
