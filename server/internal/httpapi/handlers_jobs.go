package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"misu/internal/service"
)

func (s *Server) createJob(c *gin.Context) {
	var req service.CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "任务参数格式错误")
		return
	}
	job, err := s.jobs.Create(c.Request.Context(), adminName(c), clientIP(c), req)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, job)
}

func (s *Server) listJobs(c *gin.Context) {
	page, size := pageParams(c)
	items, total, err := s.jobs.List(c.Request.Context(), service.ListJobsQuery{
		Type: c.Query("type"), Status: c.Query("status"), Page: page, PageSize: size,
	})
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, listData(items, total, page, size))
}

func (s *Server) getJob(c *gin.Context) {
	id, okID := parseUintParam(c, "id")
	if !okID {
		return
	}
	job, err := s.jobs.Get(c.Request.Context(), id)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, job)
}

func (s *Server) jobItems(c *gin.Context) {
	id, okID := parseUintParam(c, "id")
	if !okID {
		return
	}
	page, size := pageParams(c)
	items, total, err := s.jobs.Items(c.Request.Context(), id, c.Query("status"), page, size)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, listData(items, total, page, size))
}

func (s *Server) retryJob(c *gin.Context) {
	id, okID := parseUintParam(c, "id")
	if !okID {
		return
	}
	job, err := s.jobs.RetryFailed(c.Request.Context(), adminName(c), clientIP(c), id)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, job)
}

func (s *Server) cancelJob(c *gin.Context) {
	id, okID := parseUintParam(c, "id")
	if !okID {
		return
	}
	job, err := s.jobs.Cancel(c.Request.Context(), adminName(c), clientIP(c), id)
	if err != nil {
		s.respondServiceError(c, err)
		return
	}
	ok(c, job)
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || value == 0 {
		fail(c, http.StatusBadRequest, "BAD_REQUEST", "参数 "+name+" 不合法")
		return 0, false
	}
	return uint(value), true
}

