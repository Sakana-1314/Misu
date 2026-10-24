package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"misu/internal/config"
	"misu/internal/model"
)

func StartBackgroundWorkers(ctx context.Context, cfg *config.Config, audits *AuditService, jobs *JobService, log *slog.Logger) {
	go auditPoller(ctx, cfg, audits, jobs, log)
	go cleanupWorker(ctx, cfg, audits, log)
}

func auditPoller(ctx context.Context, cfg *config.Config, audits *AuditService, jobs *JobService, log *slog.Logger) {
	ticker := time.NewTicker(cfg.AuditPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pollOnce(ctx, audits, jobs, log)
		}
	}
}

func pollOnce(ctx context.Context, audits *AuditService, jobs *JobService, log *slog.Logger) {
	pending, err := audits.Pending(ctx, 50)
	if err != nil {
		log.Warn("failed to load pending audits", "error", err)
	} else if len(pending) > 0 {
		sem := make(chan struct{}, 3)
		var wg sync.WaitGroup
		for i := range pending {
			record := pending[i]
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				refreshCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
				defer cancel()
				if _, err := audits.Refresh(refreshCtx, record.ID); err != nil {
					log.Warn("failed to refresh audit", "audit_id", record.AuditID, "appid", record.AuthorizerAppID, "error", err)
				}
			}()
		}
		wg.Wait()
	}

	candidates, err := audits.AutoReleaseCandidates(ctx, 20)
	if err != nil {
		log.Warn("failed to load auto release candidates", "error", err)
		return
	}
	for _, record := range candidates {
		job, err := jobs.CreateAutoRelease(ctx, record.AuthorizerAppID, record.AuditID)
		if err != nil {
			log.Warn("failed to create auto release job", "appid", record.AuthorizerAppID, "audit_id", record.AuditID, "error", err)
			continue
		}
		if err := audits.SetAutoReleaseJob(ctx, record.ID, job.ID); err != nil {
			log.Warn("failed to mark auto release job", "record", record.ID, "error", err)
		}
	}
}

func cleanupWorker(ctx context.Context, cfg *config.Config, audits *AuditService, log *slog.Logger) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			retention := time.Duration(cfg.LogRetentionDays) * 24 * time.Hour
			if retention < 24*time.Hour {
				retention = 30 * 24 * time.Hour
			}
			cutoff := time.Now().Add(-retention)
			db := audits.db
			if res := db.Where("created_at < ?", cutoff).Delete(&model.APICallLog{}); res.Error != nil {
				log.Warn("failed to clean api logs", "error", res.Error)
			}
			if res := db.Where("created_at < ?", cutoff).Delete(&model.CallbackEvent{}); res.Error != nil {
				log.Warn("failed to clean callback events", "error", res.Error)
			}
			opCutoff := time.Now().Add(-90 * 24 * time.Hour)
			if res := db.Where("created_at < ?", opCutoff).Delete(&model.OperationLog{}); res.Error != nil {
				log.Warn("failed to clean operation logs", "error", res.Error)
			}
			quotaCutoff := time.Now().AddDate(0, 0, -60).Format("2006-01-02")
			if res := db.Where("usage_date < ?", quotaCutoff).Delete(&model.UndoQuotaUsage{}); res.Error != nil {
				log.Warn("failed to clean undo quota", "error", res.Error)
			}
		}
	}
}
