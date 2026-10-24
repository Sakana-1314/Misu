package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"misu/internal/model"
)

func (s *Server) getDashboard(c *gin.Context) {
	data, err := s.dashboard.Get(c.Request.Context())
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, data)
}

func (s *Server) listTemplates(c *gin.Context) {
	items, err := s.templates.List(c.Request.Context())
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"items": items, "total": len(items)})
}

func (s *Server) syncTemplates(c *gin.Context) {
	count, err := s.templates.Sync(c.Request.Context(), adminName(c), clientIP(c))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"count": count})
}

func (s *Server) draftTemplates(c *gin.Context) {
	items, err := s.templates.Drafts(c.Request.Context())
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"items": items, "total": len(items)})
}

func (s *Server) addDraft(c *gin.Context) {
	var req struct {
		DraftID      int64 `json:"draft_id"`
		TemplateType int   `json:"template_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.DraftID <= 0 {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "请选择有效的草稿")
		return
	}
	count, err := s.templates.AddDraft(c.Request.Context(), adminName(c), clientIP(c), req.DraftID, req.TemplateType)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"count": count})
}

func (s *Server) deleteTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("template_id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "模板 ID 不合法")
		return
	}
	if err := s.templates.Delete(c.Request.Context(), adminName(c), clientIP(c), id); err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}

func (s *Server) operationLogs(c *gin.Context) {
	page, size := pageParams(c)
	base := s.db.Model(&model.OperationLog{})
	if v := c.Query("action"); v != "" {
		base = base.Where("action = ?", v)
	}
	if v := c.Query("actor"); v != "" {
		base = base.Where("actor = ?", v)
	}
	if v := c.Query("target"); v != "" {
		base = base.Where("target = ?", v)
	}
	from, to := parseTimeRange(c)
	if from != nil {
		base = base.Where("created_at >= ?", *from)
	}
	if to != nil {
		base = base.Where("created_at < ?", *to)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		s.respondServiceError(c, err)
		return
	}
	var items []model.OperationLog
	if err := base.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, listData(items, total, page, size))
}

func (s *Server) apiCallLogs(c *gin.Context) {
	page, size := pageParams(c)
	base := s.db.Model(&model.APICallLog{})
	if v := c.Query("appid"); v != "" {
		base = base.Where("app_id = ?", v)
	}
	if v := c.Query("endpoint"); v != "" {
		base = base.Where("endpoint ILIKE ?", "%"+v+"%")
	}
	if v := c.Query("errcode"); v != "" {
		if code, err := strconv.Atoi(v); err == nil {
			base = base.Where("errcode = ?", code)
		}
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		s.respondServiceError(c, err)
		return
	}
	var items []model.APICallLog
	if err := base.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, listData(items, total, page, size))
}

func (s *Server) callbackEvents(c *gin.Context) {
	page, size := pageParams(c)
	base := s.db.Model(&model.CallbackEvent{})
	if v := c.Query("info_type"); v != "" {
		base = base.Where("info_type = ?", v)
	}
	if v := c.Query("appid"); v != "" {
		base = base.Where("authorizer_app_id = ?", v)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		s.respondServiceError(c, err)
		return
	}
	var items []model.CallbackEvent
	if err := base.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, listData(items, total, page, size))
}

func parseTimeRange(c *gin.Context) (*time.Time, *time.Time) {
	var fromPtr, toPtr *time.Time
	if v := strings.TrimSpace(c.Query("from")); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			fromPtr = &t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			fromPtr = &t
		}
	}
	if v := strings.TrimSpace(c.Query("to")); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			end := t.Add(24 * time.Hour)
			toPtr = &end
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			toPtr = &t
		}
	}
	return fromPtr, toPtr
}

func (s *Server) getSettings(c *gin.Context) {
	reveal := c.Query("reveal") == "1"
	data := s.settings.Get(c.Request.Context(), reveal, adminName(c), clientIP(c))
	ok(c, data)
}

func (s *Server) refreshComponentToken(c *gin.Context) {
	expires, err := s.settings.RefreshToken(c.Request.Context(), adminName(c), clientIP(c))
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"expires_at": expires})
}

func (s *Server) startPushTicket(c *gin.Context) {
	if err := s.settings.StartPushTicket(c.Request.Context(), adminName(c), clientIP(c)); err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, gin.H{"ok": true})
}
