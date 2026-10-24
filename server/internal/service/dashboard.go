package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	"misu/internal/config"
	"misu/internal/model"
)

type DashboardService struct {
	db     *gorm.DB
	tokens *TokenManager
	cfg    *config.Config
}

func NewDashboardService(db *gorm.DB, tokens *TokenManager, cfg *config.Config) *DashboardService {
	return &DashboardService{db: db, tokens: tokens, cfg: cfg}
}

type DashboardStats struct {
	Authorizers  int64 `json:"authorizers"`
	Auditing     int64 `json:"auditing"`
	Released7d   int64 `json:"released_7d"`
	FailedJobs7d int64 `json:"failed_jobs_7d"`
	PendingJobs  int64 `json:"pending_jobs"`
}

type AuditSummary struct {
	model.AuditRecord
	NickName string `json:"nick_name"`
	HeadImg  string `json:"head_img"`
}

type DashboardData struct {
	Stats             DashboardStats       `json:"stats"`
	Health            TokenStats           `json:"health"`
	PlatformPublished bool                 `json:"platform_published"`
	WeChatConfigured  bool                 `json:"wechat_configured"`
	RecentJobs        []model.Job          `json:"recent_jobs"`
	RecentAudits      []AuditSummary       `json:"recent_audits"`
	RecentEvents      []model.CallbackEvent `json:"recent_events"`
}

func (s *DashboardService) Get(ctx context.Context) (*DashboardData, error) {
	data := &DashboardData{
		Health:            s.tokens.Stats(),
		PlatformPublished: s.cfg.PlatformPublished,
		WeChatConfigured:  s.cfg.WeChatConfigured(),
	}
	_ = s.db.WithContext(ctx).Model(&model.Authorizer{}).Where("status = ?", model.AuthorizerActive).Count(&data.Stats.Authorizers).Error
	_ = s.db.WithContext(ctx).Model(&model.AuditRecord{}).Where("status = ?", 2).Distinct("authorizer_app_id").Count(&data.Stats.Auditing).Error
	weekAgo := time.Now().Add(-7 * 24 * time.Hour)
	_ = s.db.WithContext(ctx).Model(&model.ReleaseRecord{}).Where("released_at > ?", weekAgo).Distinct("authorizer_app_id").Count(&data.Stats.Released7d).Error
	_ = s.db.WithContext(ctx).Model(&model.Job{}).Where("status IN ? AND created_at > ?", []string{model.JobFailed, model.JobPartial}, weekAgo).Count(&data.Stats.FailedJobs7d).Error
	_ = s.db.WithContext(ctx).Model(&model.Job{}).Where("status IN ?", []string{model.JobQueued, model.JobRunning}).Count(&data.Stats.PendingJobs).Error

	if err := s.db.WithContext(ctx).Order("created_at DESC").Limit(5).Find(&data.RecentJobs).Error; err != nil {
		return nil, err
	}
	var audits []model.AuditRecord
	if err := s.db.WithContext(ctx).Order("updated_at DESC").Limit(5).Find(&audits).Error; err != nil {
		return nil, err
	}
	appids := make([]string, 0, len(audits))
	for _, a := range audits {
		appids = append(appids, a.AuthorizerAppID)
	}
	names := make(map[string]model.Authorizer)
	if len(appids) > 0 {
		var rows []model.Authorizer
		if err := s.db.WithContext(ctx).Where("app_id IN ?", appids).Find(&rows).Error; err == nil {
			for _, row := range rows {
				names[row.AppID] = row
			}
		}
	}
	data.RecentAudits = make([]AuditSummary, 0, len(audits))
	for _, a := range audits {
		summary := AuditSummary{AuditRecord: a}
		if row, ok := names[a.AuthorizerAppID]; ok {
			summary.NickName = row.NickName
			summary.HeadImg = row.HeadImg
		}
		data.RecentAudits = append(data.RecentAudits, summary)
	}
	if err := s.db.WithContext(ctx).Order("created_at DESC").Limit(5).Find(&data.RecentEvents).Error; err != nil {
		return nil, err
	}
	return data, nil
}
