package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) mpWebStatus(c *gin.Context) {
	ok(c, s.mpWeb.Status(c.Request.Context()))
}

func (s *Server) mpWebLoginStart(c *gin.Context) {
	state, _, err := s.mpWeb.StartLogin(c.Request.Context(), adminName(c), clientIP(c))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, state)
}

func (s *Server) mpWebLoginQRCode(c *gin.Context) {
	image, err := s.mpWeb.QRCode()
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/jpeg", image)
}

func (s *Server) mpWebLoginStatus(c *gin.Context) {
	state, err := s.mpWeb.PollLogin(c.Request.Context())
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, state)
}

func (s *Server) mpWebLogout(c *gin.Context) {
	if err := s.mpWeb.Logout(c.Request.Context(), adminName(c), clientIP(c)); err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}

func (s *Server) mpWebRefreshAccounts(c *gin.Context) {
	accounts, err := s.mpWeb.RefreshAccounts(c.Request.Context())
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"items": accounts, "total": len(accounts)})
}

func (s *Server) mpWebVersion(c *gin.Context) {
	version, err := s.mpWeb.Version(c.Request.Context(), c.Param("appid"))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, version)
}
