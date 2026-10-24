package httpapi

import (
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
	"gorm.io/gorm"

	"misu/internal/config"
	"misu/internal/service"
	"misu/internal/wechat"
)

type Server struct {
	cfg       *config.Config
	log       *slog.Logger
	db        *gorm.DB
	logs      *service.LogService
	tokens    *service.TokenManager
	auths     *service.AuthorizerService
	templates *service.TemplateService
	profiles  *service.ProfileService
	audits    *service.AuditService
	jobs      *service.JobService
	callback  *service.CallbackService
	dashboard *service.DashboardService
	settings  *service.SettingsService
	wx        *wechat.Client

	allowedHost   string
	adminHash     []byte
	loginLimit    *ipLimiter
	exchangeLimit *ipLimiter
}

func New(
	cfg *config.Config,
	log *slog.Logger,
	db *gorm.DB,
	logs *service.LogService,
	tokens *service.TokenManager,
	auths *service.AuthorizerService,
	templates *service.TemplateService,
	profiles *service.ProfileService,
	audits *service.AuditService,
	jobs *service.JobService,
	callback *service.CallbackService,
	dashboard *service.DashboardService,
	settings *service.SettingsService,
	wx *wechat.Client,
) (*Server, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(cfg.PublicBaseURL)
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:           cfg,
		log:           log,
		db:            db,
		logs:          logs,
		tokens:        tokens,
		auths:         auths,
		templates:     templates,
		profiles:      profiles,
		audits:        audits,
		jobs:          jobs,
		callback:      callback,
		dashboard:     dashboard,
		settings:      settings,
		wx:            wx,
		allowedHost:   parsed.Host,
		adminHash:     hash,
		loginLimit:    newIPLimiter(rate.Every(3*time.Minute), 5),
		exchangeLimit: newIPLimiter(rate.Every(3*time.Second), 20),
	}, nil
}

func (s *Server) Engine() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(s.requestID(), s.requestLogger(), s.recovery(), s.securityHeaders(), gzip.Gzip(gzip.DefaultCompression))

	api := r.Group("/api")
	api.GET("/health", s.health)
	api.GET("/version", s.version)
	api.POST("/auth/login", s.login)
	api.POST("/authorizers/exchange", s.exchangeAuthCode)

	api.GET("/wechat/callback", s.callbackGET)
	api.POST("/wechat/callback", s.callbackPOST)
	api.GET("/wechat/events/:appid", s.eventGET)
	api.POST("/wechat/events/:appid", s.eventPOST)

	auth := api.Group("")
	auth.Use(s.authRequired(), s.csrfProtect())
	{
		auth.GET("/auth/me", s.me)
		auth.POST("/auth/logout", s.logout)

		auth.GET("/dashboard", s.getDashboard)

		auth.GET("/authorizers", s.listAuthorizers)
		auth.POST("/authorizers/sync", s.syncAuthorizers)
		auth.POST("/authorizers/preauth", s.preauth)
		auth.POST("/authorizers/qrcodes", s.qrcodeZip)
		auth.GET("/apps/:appid", s.getAuthorizer)
		auth.POST("/apps/:appid/refresh", s.refreshAuthorizer)
		auth.GET("/apps/:appid/version", s.authorizerVersion)
		auth.GET("/apps/:appid/latest-audit", s.authorizerLatestAudit)
		auth.GET("/apps/:appid/audits", s.authorizerAudits)
		auth.POST("/apps/:appid/audits/:audit_id/refresh", s.refreshAudit)
		auth.GET("/apps/:appid/releases", s.authorizerReleases)
		auth.GET("/apps/:appid/pages", s.authorizerPages)
		auth.GET("/apps/:appid/categories", s.authorizerCategories)
		auth.GET("/apps/:appid/all-categories", s.authorizerAllCategories)
		auth.GET("/apps/:appid/domains", s.authorizerDomains)
		auth.GET("/apps/:appid/testers", s.authorizerTesters)
		auth.POST("/apps/:appid/testers", s.bindTester)
		auth.DELETE("/apps/:appid/testers", s.unbindTester)
		auth.GET("/apps/:appid/qrcode", s.authorizerQRCode)
		auth.GET("/apps/:appid/history-versions", s.authorizerHistoryVersions)
		auth.GET("/apps/:appid/gray-plan", s.authorizerGrayPlan)
		auth.GET("/apps/:appid/visit-status", s.authorizerVisitStatus)
		auth.GET("/apps/:appid/privacy-check", s.authorizerPrivacyCheck)
		auth.GET("/apps/:appid/media", s.authorizerMedia)
		auth.POST("/uploads/audit-media", s.uploadAuditMedia)

		auth.GET("/templates", s.listTemplates)
		auth.POST("/templates/sync", s.syncTemplates)
		auth.GET("/templates/drafts", s.draftTemplates)
		auth.POST("/templates/from-draft", s.addDraft)
		auth.DELETE("/templates/:template_id", s.deleteTemplate)

		auth.GET("/audit-profiles", s.listProfiles)
		auth.POST("/audit-profiles", s.createProfile)
		auth.GET("/audit-profiles/:id", s.getProfile)
		auth.PUT("/audit-profiles/:id", s.updateProfile)
		auth.DELETE("/audit-profiles/:id", s.deleteProfile)

		auth.POST("/jobs", s.createJob)
		auth.GET("/jobs", s.listJobs)
		auth.GET("/jobs/:id", s.getJob)
		auth.GET("/jobs/:id/items", s.jobItems)
		auth.POST("/jobs/:id/retry", s.retryJob)
		auth.POST("/jobs/:id/cancel", s.cancelJob)

		auth.GET("/logs", s.operationLogs)
		auth.GET("/api-call-logs", s.apiCallLogs)
		auth.GET("/callback-events", s.callbackEvents)

		auth.GET("/settings", s.getSettings)
		auth.POST("/settings/refresh-token", s.refreshComponentToken)
		auth.POST("/settings/start-push-ticket", s.startPushTicket)
	}

	r.NoRoute(func(c *gin.Context) {
		fail(c, http.StatusNotFound, "NOT_FOUND", "接口不存在")
	})
	return r
}

func (s *Server) health(c *gin.Context) {
	health := s.settings.Health(c.Request.Context())
	status := http.StatusOK
	if !health.DBOK {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, health)
}

func (s *Server) version(c *gin.Context) {
	ok(c, gin.H{"version": s.cfg.Version, "commit": s.cfg.Commit, "build_time": s.cfg.BuildTime})
}

func (s *Server) callbackGET(c *gin.Context) {
	echostr, valid := s.callback.VerifyURL(c.Request.URL.Query())
	if !valid {
		c.String(http.StatusForbidden, "invalid signature")
		return
	}
	c.String(http.StatusOK, echostr)
}

func (s *Server) callbackPOST(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	body, err := c.GetRawData()
	if err != nil {
		c.String(http.StatusBadRequest, "invalid body")
		return
	}
	if err := s.callback.HandleAuthEvent(c.Request.Context(), c.Request.URL.Query(), body); err != nil {
		s.log.Warn("wechat auth callback rejected", "error", err)
		c.String(http.StatusForbidden, "invalid message")
		return
	}
	c.String(http.StatusOK, "success")
}

func (s *Server) eventGET(c *gin.Context) {
	echostr, valid := s.callback.VerifyURL(c.Request.URL.Query())
	if !valid {
		c.String(http.StatusForbidden, "invalid signature")
		return
	}
	c.String(http.StatusOK, echostr)
}

func (s *Server) eventPOST(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	body, err := c.GetRawData()
	if err != nil {
		c.String(http.StatusBadRequest, "invalid body")
		return
	}
	appid := c.Param("appid")
	if err := s.callback.HandleMessageEvent(c.Request.Context(), appid, c.Request.URL.Query(), body); err != nil {
		s.log.Warn("wechat message event rejected", "appid", appid, "error", err)
		c.String(http.StatusForbidden, "invalid message")
		return
	}
	c.String(http.StatusOK, "success")
}
