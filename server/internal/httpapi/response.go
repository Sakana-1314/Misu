package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"misu/internal/service"
	"misu/internal/wechat"
)

type envelope struct {
	Code      any    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", Data: data, RequestID: requestID(c)})
}

func fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, envelope{Code: code, Message: message, RequestID: requestID(c)})
}

func failData(c *gin.Context, status int, code, message string, data any) {
	c.JSON(status, envelope{Code: code, Message: message, Data: data, RequestID: requestID(c)})
}

func (s *Server) respondError(c *gin.Context, err error) {
	var apiErr *wechat.APIError
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		fail(c, http.StatusNotFound, "NOT_FOUND", "记录不存在")
	case errors.As(err, &apiErr):
		failData(c, http.StatusBadGateway, "WECHAT_ERROR", apiErr.Msg, gin.H{
			"errcode":  apiErr.Code,
			"endpoint": apiErr.Endpoint,
		})
	case errors.Is(err, service.ErrUnconfigured):
		fail(c, http.StatusConflict, "WECHAT_UNCONFIGURED", err.Error())
	case errors.Is(err, service.ErrNoTicket):
		fail(c, http.StatusConflict, "NO_VERIFY_TICKET", err.Error())
	case errors.Is(err, service.ErrNeedReauth):
		fail(c, http.StatusConflict, "NEED_REAUTH", err.Error())
	default:
		s.log.Error("request failed", "path", c.Request.URL.Path, "error", err, "request_id", requestID(c))
		fail(c, http.StatusInternalServerError, "INTERNAL", "服务器内部错误，请查看服务端日志")
	}
}

func (s *Server) respondServiceError(c *gin.Context, err error) {
	s.respondError(c, err)
}

func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}

func listData(items any, total int64, page, size int) gin.H {
	return gin.H{"items": items, "total": total, "page": page, "page_size": size}
}
