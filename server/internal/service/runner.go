package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"misu/internal/config"
	"misu/internal/model"
)

type ItemResult struct {
	Request      any
	Response     any
	ErrorCode    string
	ErrorMessage string
	AuditID      int64
	Skipped      bool
}

type Executor interface {
	Type() string
	Execute(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult
}

type JobRunner struct {
	cfg       *config.Config
	db        *gorm.DB
	logs      *LogService
	log       *slog.Logger
	executors map[string]Executor

	queue chan uint
	mu    sync.Mutex
	active map[uint]bool
}

func NewJobRunner(cfg *config.Config, db *gorm.DB, logs *LogService, log *slog.Logger) *JobRunner {
	return &JobRunner{
		cfg:       cfg,
		db:        db,
		logs:      logs,
		log:       log,
		executors: make(map[string]Executor),
		queue:     make(chan uint, 1000),
		active:    make(map[uint]bool),
	}
}

func (r *JobRunner) Register(executor Executor) {
	r.executors[executor.Type()] = executor
}

func (r *JobRunner) Start(ctx context.Context) {
	r.markStale()
	go r.loop(ctx)
	go r.sweeper(ctx)
}

func (r *JobRunner) Enqueue(jobID uint) {
	select {
	case r.queue <- jobID:
	default:
		r.log.Warn("job queue full, relying on sweeper", "job_id", jobID)
	}
}

func (r *JobRunner) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case jobID := <-r.queue:
			r.runJob(ctx, jobID)
		}
	}
}

func (r *JobRunner) sweeper(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var ids []uint
			if err := r.db.Model(&model.Job{}).Where("status = ?", model.JobQueued).Order("id ASC").Limit(100).Pluck("id", &ids).Error; err != nil {
				r.log.Warn("failed to sweep queued jobs", "error", err)
				continue
			}
			for _, id := range ids {
				r.Enqueue(id)
			}
		}
	}
}

func (r *JobRunner) markStale() {
	now := time.Now()
	if err := r.db.Model(&model.JobItem{}).Where("status = ?", model.ItemRunning).Updates(map[string]any{
		"status":        model.ItemFailed,
		"error_code":    "INTERRUPTED",
		"error_message": "服务重启，执行结果未知，请核实后重试",
		"finished_at":   now,
		"updated_at":    now,
	}).Error; err != nil {
		r.log.Warn("failed to mark stale job items", "error", err)
	}
	if err := r.db.Model(&model.Job{}).Where("status = ?", model.JobRunning).Updates(map[string]any{
		"status":     model.JobQueued,
		"finished_at": nil,
		"updated_at": now,
	}).Error; err != nil {
		r.log.Warn("failed to requeue stale jobs", "error", err)
	}
	var ids []uint
	if err := r.db.Model(&model.Job{}).Where("status = ?", model.JobQueued).Pluck("id", &ids).Error; err == nil {
		for _, id := range ids {
			r.Enqueue(id)
		}
	}
}

func (r *JobRunner) runJob(ctx context.Context, jobID uint) {
	if !r.markActive(jobID) {
		return
	}
	defer r.unmarkActive(jobID)

	var job model.Job
	if err := r.db.First(&job, jobID).Error; err != nil {
		return
	}
	if job.Status != model.JobQueued && job.Status != model.JobRunning {
		return
	}
	if job.CancelRequested {
		r.finalizeCanceled(jobID)
		return
	}
	now := time.Now()
	startedAt := job.StartedAt
	if startedAt == nil {
		startedAt = &now
	}
	if err := r.db.Model(&model.Job{}).Where("id = ?", jobID).Updates(map[string]any{
		"status":     model.JobRunning,
		"started_at": startedAt,
		"updated_at": now,
	}).Error; err != nil {
		r.log.Warn("failed to start job", "job_id", jobID, "error", err)
		return
	}
	job.Status = model.JobRunning
	job.StartedAt = startedAt

	var items []model.JobItem
	if err := r.db.Where("job_id = ? AND status = ?", jobID, model.ItemPending).Order("id ASC").Find(&items).Error; err != nil {
		r.failJob(jobID, err.Error())
		return
	}
	if len(items) == 0 {
		r.finalize(job)
		return
	}

	sem := make(chan struct{}, r.cfg.JobConcurrency)
	var wg sync.WaitGroup
	for i := range items {
		item := items[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			r.runItem(ctx, &job, &item)
		}()
	}
	wg.Wait()

	var current model.Job
	if err := r.db.Select("cancel_requested").First(&current, jobID).Error; err == nil && current.CancelRequested {
		r.db.Model(&model.JobItem{}).Where("job_id = ? AND status = ?", jobID, model.ItemPending).Updates(map[string]any{
			"status":     model.ItemCanceled,
			"updated_at": time.Now(),
		})
	}
	r.finalize(job)
}

func (r *JobRunner) runItem(ctx context.Context, job *model.Job, item *model.JobItem) {
	var current model.Job
	if err := r.db.Select("cancel_requested").First(&current, job.ID).Error; err == nil && current.CancelRequested {
		r.db.Model(&model.JobItem{}).Where("id = ?", item.ID).Updates(map[string]any{
			"status":     model.ItemCanceled,
			"updated_at": time.Now(),
		})
		return
	}
	executor, ok := r.executors[job.Type]
	if !ok {
		r.finishItem(job, item, ItemResult{ErrorCode: "NO_EXECUTOR", ErrorMessage: "没有找到任务执行器"})
		return
	}
	now := time.Now()
	attempts := item.Attempts + 1
	if err := r.db.Model(&model.JobItem{}).Where("id = ?", item.ID).Updates(map[string]any{
		"status":     model.ItemRunning,
		"attempts":   attempts,
		"started_at": now,
		"updated_at": now,
	}).Error; err != nil {
		return
	}
	result := r.executeSafe(ctx, executor, job, item)
	r.finishItem(job, item, result)
}

func (r *JobRunner) executeSafe(ctx context.Context, executor Executor, job *model.Job, item *model.JobItem) (result ItemResult) {
	execCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			r.log.Error("job item panic", "job_id", job.ID, "appid", item.AuthorizerAppID, "panic", recovered, "stack", string(debug.Stack()))
			result = ItemResult{ErrorCode: "PANIC", ErrorMessage: fmt.Sprintf("执行器异常: %v", recovered)}
		}
	}()
	return executor.Execute(execCtx, job, item)
}

func (r *JobRunner) finishItem(job *model.Job, item *model.JobItem, result ItemResult) {
	status := model.ItemSuccess
	if result.Skipped {
		status = model.ItemSkipped
	}
	if result.ErrorMessage != "" && !result.Skipped {
		status = model.ItemFailed
	}
	request := RedactJSON(result.Request)
	response := RedactJSON(result.Response)
	if result.Request == nil {
		request = datatypes.JSON([]byte("null"))
	}
	if result.Response == nil {
		response = datatypes.JSON([]byte("null"))
	}
	now := time.Now()
	updates := map[string]any{
		"status":        status,
		"request":       request,
		"response":      response,
		"error_code":    result.ErrorCode,
		"error_message": truncate(result.ErrorMessage, 2000),
		"audit_id":      result.AuditID,
		"finished_at":   now,
		"updated_at":    now,
	}
	if err := r.db.Model(&model.JobItem{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
		r.log.Warn("failed to update job item", "item_id", item.ID, "error", err)
		return
	}
	var field string
	switch status {
	case model.ItemSuccess:
		field = "success_count"
	case model.ItemFailed:
		field = "failed_count"
	case model.ItemSkipped:
		field = "skipped_count"
	}
	if field != "" {
		if err := r.db.Model(&model.Job{}).Where("id = ?", job.ID).
			UpdateColumn(field, gorm.Expr(field+" + ?", 1)).Error; err != nil {
			r.log.Warn("failed to update job counters", "job_id", job.ID, "error", err)
		}
	}
}

func (r *JobRunner) finalize(job model.Job) {
	var counts []struct {
		Status string
		Count  int
	}
	if err := r.db.Model(&model.JobItem{}).
		Select("status, count(*) as count").
		Where("job_id = ?", job.ID).Group("status").Scan(&counts).Error; err != nil {
		r.failJob(job.ID, err.Error())
		return
	}
	success, failed, skipped, canceled, pending := 0, 0, 0, 0, 0
	for _, c := range counts {
		switch c.Status {
		case model.ItemSuccess:
			success = c.Count
		case model.ItemFailed:
			failed = c.Count
		case model.ItemSkipped:
			skipped = c.Count
		case model.ItemCanceled:
			canceled = c.Count
		case model.ItemPending, model.ItemRunning:
			pending += c.Count
		}
	}
	var current model.Job
	_ = r.db.Select("cancel_requested").First(&current, job.ID).Error
	status := model.JobSuccess
	switch {
	case current.CancelRequested:
		status = model.JobCanceled
	case failed > 0 && success == 0 && skipped == 0:
		status = model.JobFailed
	case failed > 0 || skipped > 0 || canceled > 0 || pending > 0:
		status = model.JobPartial
	}
	now := time.Now()
	if err := r.db.Model(&model.Job{}).Where("id = ?", job.ID).Updates(map[string]any{
		"status":        status,
		"success_count": success,
		"failed_count":  failed,
		"skipped_count": skipped,
		"finished_at":   now,
		"updated_at":    now,
	}).Error; err != nil {
		r.log.Warn("failed to finalize job", "job_id", job.ID, "error", err)
	}
	r.logs.Operation("system", "job_finished", fmt.Sprint(job.ID), map[string]any{
		"status": status, "success": success, "failed": failed, "skipped": skipped, "canceled": canceled,
	}, "")
}

func (r *JobRunner) finalizeCanceled(jobID uint) {
	now := time.Now()
	_ = r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Job{}).Where("id = ?", jobID).Updates(map[string]any{
			"status":      model.JobCanceled,
			"finished_at": now,
			"updated_at":  now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.JobItem{}).Where("job_id = ? AND status = ?", jobID, model.ItemPending).Updates(map[string]any{
			"status":     model.ItemCanceled,
			"updated_at": now,
		}).Error
	})
}

func (r *JobRunner) failJob(jobID uint, message string) {
	now := time.Now()
	_ = r.db.Model(&model.Job{}).Where("id = ?", jobID).Updates(map[string]any{
		"status":      model.JobFailed,
		"error":       truncate(message, 2000),
		"finished_at": now,
		"updated_at":  now,
	}).Error
	r.log.Error("job failed", "job_id", jobID, "error", message)
}

func (r *JobRunner) markActive(jobID uint) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active[jobID] {
		return false
	}
	r.active[jobID] = true
	return true
}

func (r *JobRunner) unmarkActive(jobID uint) {
	r.mu.Lock()
	delete(r.active, jobID)
	r.mu.Unlock()
}

// DecodeJobPayload is a small helper for executors.
func DecodeJobPayload(job *model.Job, out any) error {
	if len(job.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(job.Payload, out)
}
