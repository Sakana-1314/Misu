package httpapi

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"misu/internal/model"
	"misu/internal/service"
	"misu/internal/wechat"
)

var appIDPattern = regexp.MustCompile(`^wx[0-9a-f]{16}$`)

func (s *Server) listAuthorizers(c *gin.Context) {
	page, size := pageParams(c)
	items, total, err := s.auths.List(c.Request.Context(), service.ListQuery{
		Keyword: c.Query("keyword"), Status: c.Query("status"), Page: page, PageSize: size,
	})
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, listData(items, total, page, size))
}

func (s *Server) syncAuthorizers(c *gin.Context) {
	result, err := s.auths.SyncAll(c.Request.Context(), adminName(c), clientIP(c))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, result)
}

func (s *Server) preauth(c *gin.Context) {
	if !s.cfg.WeChatConfigured() {
		s.respondServiceError(c, service.ErrUnconfigured)
		return
	}
	var req struct {
		AppID string `json:"appid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "请填写小程序 AppID")
		return
	}
	appid := strings.ToLower(strings.TrimSpace(req.AppID))
	if !appIDPattern.MatchString(appid) {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "AppID 格式不正确")
		return
	}
	if err := s.openplatform.AddTestAppID(c.Request.Context(), appid); err != nil {
		s.respondServiceError(c, err)
		return
	}
	componentToken, err := s.tokens.GetComponent(c.Request.Context())
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	resp, err := s.wx.GetPreAuthCode(c.Request.Context(), componentToken, s.cfg.ComponentAppID)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":     "misu",
		"sub":     adminName(c),
		"purpose": "wechat_auth",
		"appid":   appid,
		"nonce":   uuid.NewString(),
		"iat":     now.Unix(),
		"exp":     now.Add(15 * time.Minute).Unix(),
	}
	state, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	redirectURI := s.cfg.PublicBaseURL + "/auth/callback?state=" + url.QueryEscape(state)
	q := url.Values{}
	q.Set("component_appid", s.cfg.ComponentAppID)
	q.Set("pre_auth_code", resp.PreAuthCode)
	q.Set("redirect_uri", redirectURI)
	q.Set("auth_type", "2")
	authURL := "https://mp.weixin.qq.com/cgi-bin/componentloginpage?" + q.Encode()
	expiresIn := resp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 600
	}
	s.logs.Operation(adminName(c), "preauth_create", appid, map[string]any{"appid": appid}, clientIP(c))
	ok(c, gin.H{
		"auth_url":     authURL,
		"redirect_uri": redirectURI,
		"expires_in":   expiresIn,
		"test_mode":    !s.cfg.PlatformPublished,
	})
}

func (s *Server) exchangeAuthCode(c *gin.Context) {
	if !s.exchangeLimit.allow(clientIP(c)) {
		fail(c, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "请求过于频繁")
		return
	}
	var req struct {
		AuthCode string `json:"auth_code"`
		State    string `json:"state"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AuthCode == "" {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "缺少授权码")
		return
	}
	if err := s.validateAuthState(req.State); err != nil {
		fail(c, http.StatusBadRequest, "INVALID_STATE", "授权状态校验失败，请重新发起授权")
		return
	}
	a, err := s.auths.ExchangeAuthCode(c.Request.Context(), req.AuthCode)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	s.logs.Operation(adminName(c), "authorizer_exchange", a.AppID, map[string]any{"appid": a.AppID}, clientIP(c))
	ok(c, a)
}

func (s *Server) getAuthorizer(c *gin.Context) {
	appid := c.Param("appid")
	if c.Query("refresh") == "1" {
		a, err := s.auths.SyncInfo(c.Request.Context(), appid)
		if err != nil {
			s.respondServiceError(c, err)
			return
		}
		ok(c, a)
		return
	}
	a, err := s.auths.Get(c.Request.Context(), appid)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, a)
}

func (s *Server) refreshAuthorizer(c *gin.Context) {
	appid := c.Param("appid")
	a, err := s.auths.SyncInfo(c.Request.Context(), appid)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	s.logs.Operation(adminName(c), "authorizer_refresh", appid, nil, clientIP(c))
	ok(c, a)
}

func (s *Server) authorizerVersion(c *gin.Context) {
	resp, err := s.auths.Version(c.Request.Context(), c.Param("appid"))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, resp)
}

func (s *Server) authorizerLatestAudit(c *gin.Context) {
	appid := c.Param("appid")
	resp, err := s.auths.LatestAudit(c.Request.Context(), appid)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	_, _ = s.audits.RefreshLatest(c.Request.Context(), appid)
	ok(c, resp)
}

func (s *Server) authorizerAudits(c *gin.Context) {
	page, size := pageParams(c)
	items, total, err := s.audits.ListByApp(c.Request.Context(), c.Param("appid"), page, size)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, listData(items, total, page, size))
}

func (s *Server) refreshAudit(c *gin.Context) {
	appid := c.Param("appid")
	auditID, err := strconv.ParseInt(c.Param("audit_id"), 10, 64)
	if err != nil || auditID <= 0 {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "审核单号不合法")
		return
	}
	record, err := s.audits.RefreshByAuditID(c.Request.Context(), appid, auditID)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, record)
}

func (s *Server) authorizerReleases(c *gin.Context) {
	appid := c.Param("appid")
	page, size := pageParams(c)
	base := s.db.Model(&model.ReleaseRecord{}).Where("authorizer_app_id = ?", appid)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		s.respondServiceError(c, err)
		return
	}
	var items []model.ReleaseRecord
	if err := base.Order("released_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, listData(items, total, page, size))
}

func (s *Server) authorizerPages(c *gin.Context) {
	pages, err := s.auths.Pages(c.Request.Context(), c.Param("appid"))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"items": pages})
}

func (s *Server) authorizerCategories(c *gin.Context) {
	resp, err := s.auths.SettingCategories(c.Request.Context(), c.Param("appid"))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, resp)
}

func (s *Server) authorizerAllCategories(c *gin.Context) {
	resp, err := s.auths.AllCategories(c.Request.Context(), c.Param("appid"))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, resp)
}

func (s *Server) authorizerDomains(c *gin.Context) {
	resp, err := s.auths.Domains(c.Request.Context(), c.Param("appid"))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, resp)
}

func (s *Server) authorizerTesters(c *gin.Context) {
	items, err := s.auths.Testers(c.Request.Context(), c.Param("appid"))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"items": items})
}

func (s *Server) bindTester(c *gin.Context) {
	appid := c.Param("appid")
	var req struct {
		WeChatID string `json:"wechatid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.WeChatID) == "" {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "请填写体验者微信号")
		return
	}
	userstr, err := s.auths.BindTester(c.Request.Context(), appid, req.WeChatID)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	s.logs.Operation(adminName(c), "tester_bind", appid, map[string]any{"wechatid": req.WeChatID}, clientIP(c))
	ok(c, gin.H{"userstr": userstr})
}

func (s *Server) unbindTester(c *gin.Context) {
	appid := c.Param("appid")
	var req struct {
		UserStr string `json:"userstr"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.UserStr == "" {
		req.UserStr = c.Query("userstr")
	}
	if strings.TrimSpace(req.UserStr) == "" {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "缺少体验者 userstr")
		return
	}
	if err := s.auths.UnbindTester(c.Request.Context(), appid, req.UserStr); err != nil {
		s.respondServiceError(c, err)
		return
	}
	s.logs.Operation(adminName(c), "tester_unbind", appid, map[string]any{"userstr": req.UserStr}, clientIP(c))
	ok(c, gin.H{"ok": true})
}

func (s *Server) authorizerQRCode(c *gin.Context) {
	appid := c.Param("appid")
	qrType := c.DefaultQuery("type", "trial")
	var (
		data        []byte
		contentType string
		err         error
	)
	switch qrType {
	case "unlimited":
		scene := strings.TrimSpace(c.Query("scene"))
		if scene == "" {
			fail(c, http.StatusBadRequest, "BAD_REQUEST", "生成小程序码需要 scene 参数")
			return
		}
		width, _ := strconv.Atoi(c.DefaultQuery("width", "430"))
		if width < 280 || width > 1280 {
			width = 430
		}
		req := wechat.UnlimitedQRCodeRequest{
			Scene:      scene,
			Page:       c.Query("path"),
			Width:      width,
			EnvVersion: c.Query("env_version"),
		}
		data, contentType, err = s.auths.UnlimitedQRCode(c.Request.Context(), appid, req)
	default:
		data, contentType, err = s.auths.TrialQRCode(c.Request.Context(), appid, c.Query("path"))
	}
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", "inline; filename=\"qrcode.jpg\"")
	c.Data(http.StatusOK, contentType, data)
}

func (s *Server) authorizerHistoryVersions(c *gin.Context) {
	resp, err := s.auths.HistoryVersions(c.Request.Context(), c.Param("appid"))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, resp)
}

func (s *Server) authorizerGrayPlan(c *gin.Context) {
	resp, err := s.auths.GrayPlan(c.Request.Context(), c.Param("appid"))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, resp)
}

func (s *Server) authorizerVisitStatus(c *gin.Context) {
	resp, err := s.auths.VisitStatus(c.Request.Context(), c.Param("appid"))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, resp)
}

func (s *Server) authorizerPrivacyCheck(c *gin.Context) {
	resp, err := s.auths.PrivacyCheck(c.Request.Context(), c.Param("appid"))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, resp)
}

func (s *Server) authorizerMedia(c *gin.Context) {
	mediaID := c.Query("media_id")
	if mediaID == "" {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "缺少 media_id")
		return
	}
	data, contentType, err := s.auths.Media(c.Request.Context(), c.Param("appid"), mediaID)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, contentType, data)
}

func (s *Server) uploadAuditAsset(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "请上传图片或视频文件")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 10<<20))
	if err != nil || len(data) == 0 {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "读取上传文件失败")
		return
	}
	filename := header.Filename
	if filename == "" {
		filename = "material"
	}
	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = http.DetectContentType(data)
	}
	asset, err := s.assets.Create(c.Request.Context(), filename, mime, data)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"id": asset.ID, "filename": asset.Filename, "mime": asset.Mime, "size": asset.Size})
}

func (s *Server) getAuditAsset(c *gin.Context) {
	id, valid := parseUintParam(c, "id")
	if !valid {
		return
	}
	asset, err := s.assets.Get(c.Request.Context(), id)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	contentType := asset.Mime
	if contentType == "" {
		contentType = http.DetectContentType(asset.Data)
	}
	c.Header("Cache-Control", "private, max-age=600")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", asset.Filename))
	c.Data(http.StatusOK, contentType, asset.Data)
}

func (s *Server) qrcodeZip(c *gin.Context) {
	var req struct {
		AppIDs []string `json:"appids"`
		Path   string   `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.AppIDs) == 0 {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "请选择小程序")
		return
	}
	if len(req.AppIDs) > 50 {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "单次最多生成 50 个小程序码")
		return
	}
	active, _, err := s.auths.ActiveAppIDs(c.Request.Context(), req.AppIDs)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	if len(active) != len(req.AppIDs) {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "部分小程序不存在或已取消授权")
		return
	}
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	failures := make([]string, 0)
	successes := 0
	for i, appid := range active {
		data, contentType, err := s.auths.TrialQRCode(c.Request.Context(), appid, req.Path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", appid, err))
			continue
		}
		ext := ".jpg"
		if strings.Contains(strings.ToLower(contentType), "png") {
			ext = ".png"
		}
		writer, err := zw.Create(fmt.Sprintf("%02d_%s%s", i+1, appid, ext))
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: 写入 ZIP 失败", appid))
			continue
		}
		if _, err := writer.Write(data); err != nil {
			failures = append(failures, fmt.Sprintf("%s: 写入 ZIP 失败", appid))
			continue
		}
		successes++
	}
	if len(failures) > 0 {
		if writer, err := zw.Create("errors.txt"); err == nil {
			_, _ = writer.Write([]byte(strings.Join(failures, "\n")))
		}
	}
	if err := zw.Close(); err != nil {
		s.respondServiceError(c, err)
		return
	}
	if successes == 0 {
		fail(c, http.StatusBadGateway, "QRCODE_FAILED", strings.Join(failures, "; "))
		return
	}
	s.logs.Operation(adminName(c), "qrcode_zip", "", map[string]any{"count": successes, "failed": len(failures)}, clientIP(c))
	c.Header("Content-Disposition", "attachment; filename=\"misu_qrcodes.zip\"")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}
