package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"misu/internal/model"
	"misu/internal/wechat"
)

var ErrAlreadyAuditing = errors.New("该小程序已有版本正在审核中")

type AuditService struct {
	db    *gorm.DB
	wx    *wechat.Client
	auths *AuthorizerService
	log   *slog.Logger
}

type AuditSubmitPayload struct {
	Profile     ProfileSnapshot `json:"profile"`
	AutoRelease bool            `json:"auto_release"`
}

func NewAuditService(db *gorm.DB, wx *wechat.Client, auths *AuthorizerService, log *slog.Logger) *AuditService {
	return &AuditService{db: db, wx: wx, auths: auths, log: log}
}

func (s *AuditService) Submit(ctx context.Context, appid string, profile ProfileSnapshot, autoRelease bool, profileID *uint) (*model.AuditRecord, error) {
	latestBefore, latestErr := s.auths.LatestAudit(ctx, appid)
	if latestErr != nil {
		// A first-time audit has no previous audit record. Some WeChat
		// deployments return an error in that case, so do not block submission.
		s.log.Warn("failed to query latest audit before submit", "appid", appid, "error", latestErr)
		latestBefore = &wechat.LatestAuditStatus{}
	}
	if latestBefore.Status == 2 {
		return nil, ErrAlreadyAuditing
	}
	req := wechat.SubmitAuditRequest{
		ItemList:      profile.ItemList,
		VersionDesc:   profile.VersionDesc,
		FeedbackInfo:  profile.FeedbackInfo,
		FeedbackStuff: profile.FeedbackStuff,
		UGCDeclare:    profile.UGCDeclare,
		PreviewInfo:   profile.PreviewInfo,
	}
	var submitResp *wechat.SubmitAuditResponse
	err := s.auths.CallWithToken(ctx, appid, func(token string) error {
		resp, err := s.wx.SubmitAudit(ctx, appid, token, req)
		if err == nil {
			submitResp = resp
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if submitResp == nil || submitResp.AuditID == 0 {
		return nil, errors.New("微信未返回审核编号")
	}

	version := latestBefore.UserVersion
	desc := latestBefore.UserDesc
	// Best effort: get the version associated with the newly created audit.
	if latestAfter, err := s.auths.LatestAudit(ctx, appid); err == nil && latestAfter.AuditID == submitResp.AuditID {
		version = latestAfter.UserVersion
		desc = latestAfter.UserDesc
	}

	now := time.Now()
	var record model.AuditRecord
	findErr := s.db.WithContext(ctx).Where("authorizer_app_id = ? AND audit_id = ?", appid, submitResp.AuditID).First(&record).Error
	if findErr == gorm.ErrRecordNotFound {
		record = model.AuditRecord{
			AuthorizerAppID: appid,
			AuditID:         submitResp.AuditID,
			Status:          2,
			UserVersion:     version,
			UserDesc:        desc,
			SubmittedAt:     &now,
			ProfileID:       profileID,
			AutoRelease:     autoRelease,
		}
		if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
			return nil, err
		}
	} else if findErr != nil {
		return nil, findErr
	} else {
		updates := map[string]any{
			"status":       2,
			"user_version": version,
			"user_desc":    desc,
			"submitted_at": now,
			"profile_id":   profileID,
			"auto_release": autoRelease,
			"updated_at":   now,
		}
		if err := s.db.WithContext(ctx).Model(&model.AuditRecord{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		_ = s.db.WithContext(ctx).First(&record, record.ID).Error
	}
	return &record, nil
}

func (s *AuditService) Refresh(ctx context.Context, recordID uint) (*model.AuditRecord, error) {
	var record model.AuditRecord
	if err := s.db.WithContext(ctx).First(&record, recordID).Error; err != nil {
		return nil, err
	}
	status, err := s.auths.AuditStatus(ctx, record.AuthorizerAppID, record.AuditID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.db.WithContext(ctx).Model(&model.AuditRecord{}).Where("id = ?", record.ID).Updates(map[string]any{
		"status":          status.Status,
		"reason":          status.Reason,
		"screenshot":      status.Screenshot,
		"last_checked_at": now,
		"updated_at":      now,
	}).Error; err != nil {
		return nil, err
	}
	return s.GetByID(ctx, record.ID)
}

func (s *AuditService) RefreshLatest(ctx context.Context, appid string) (*model.AuditRecord, error) {
	latest, err := s.auths.LatestAudit(ctx, appid)
	if err != nil {
		return nil, err
	}
	if latest.AuditID == 0 {
		return nil, nil
	}
	now := time.Now()
	var submitTime *time.Time
	if latest.SubmitAuditTime > 0 {
		t := time.Unix(latest.SubmitAuditTime, 0)
		submitTime = &t
	}
	var record model.AuditRecord
	findErr := s.db.WithContext(ctx).Where("authorizer_app_id = ? AND audit_id = ?", appid, latest.AuditID).First(&record).Error
	if findErr == gorm.ErrRecordNotFound {
		record = model.AuditRecord{
			AuthorizerAppID: appid,
			AuditID:         latest.AuditID,
			Status:          latest.Status,
			Reason:          latest.Reason,
			Screenshot:      latest.Screenshot,
			UserVersion:     latest.UserVersion,
			UserDesc:        latest.UserDesc,
			SubmitAuditTime: submitTime,
			LastCheckedAt:   &now,
			SubmittedAt:     submitTime,
		}
		if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
			return nil, err
		}
		return &record, nil
	}
	if findErr != nil {
		return nil, findErr
	}
	if err := s.db.WithContext(ctx).Model(&model.AuditRecord{}).Where("id = ?", record.ID).Updates(map[string]any{
		"status":            latest.Status,
		"reason":            latest.Reason,
		"screenshot":        latest.Screenshot,
		"user_version":      latest.UserVersion,
		"user_desc":         latest.UserDesc,
		"submit_audit_time": submitTime,
		"last_checked_at":   now,
		"updated_at":        now,
	}).Error; err != nil {
		return nil, err
	}
	return s.GetByID(ctx, record.ID)
}

func (s *AuditService) MarkUndone(ctx context.Context, appid string) error {
	var record model.AuditRecord
	err := s.db.WithContext(ctx).Where("authorizer_app_id = ? AND status = ?", appid, 2).
		Order("submitted_at DESC NULLS LAST, id DESC").First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	now := time.Now()
	return s.db.WithContext(ctx).Model(&model.AuditRecord{}).Where("id = ?", record.ID).Updates(map[string]any{
		"status":          3,
		"last_checked_at": now,
		"updated_at":      now,
	}).Error
}

func (s *AuditService) RefreshByAuditID(ctx context.Context, appid string, auditID int64) (*model.AuditRecord, error) {
	var record model.AuditRecord
	if err := s.db.WithContext(ctx).Where("authorizer_app_id = ? AND audit_id = ?", appid, auditID).First(&record).Error; err != nil {
		return nil, err
	}
	return s.Refresh(ctx, record.ID)
}

func (s *AuditService) GetByID(ctx context.Context, id uint) (*model.AuditRecord, error) {
	var record model.AuditRecord
	if err := s.db.WithContext(ctx).First(&record, id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *AuditService) ListByApp(ctx context.Context, appid string, page, pageSize int) ([]model.AuditRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	q := s.db.WithContext(ctx).Model(&model.AuditRecord{}).Where("authorizer_app_id = ?", appid)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.AuditRecord
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

func (s *AuditService) ListRecent(ctx context.Context, limit int) ([]model.AuditRecord, error) {
	var items []model.AuditRecord
	err := s.db.WithContext(ctx).Order("updated_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

func (s *AuditService) Pending(ctx context.Context, limit int) ([]model.AuditRecord, error) {
	var items []model.AuditRecord
	cutoff := time.Now().Add(-2 * time.Minute)
	err := s.db.WithContext(ctx).
		Where("status = ? AND (last_checked_at IS NULL OR last_checked_at < ?)", 2, cutoff).
		Order("last_checked_at ASC NULLS FIRST").Limit(limit).Find(&items).Error
	return items, err
}

func (s *AuditService) AutoReleaseCandidates(ctx context.Context, limit int) ([]model.AuditRecord, error) {
	var items []model.AuditRecord
	err := s.db.WithContext(ctx).
		Where("status = 0 AND auto_release = ? AND auto_release_job_id IS NULL AND updated_at > ?", true, time.Now().Add(-24*time.Hour)).
		Order("updated_at ASC").Limit(limit).Find(&items).Error
	return items, err
}

func (s *AuditService) SetAutoReleaseJob(ctx context.Context, recordID, jobID uint) error {
	return s.db.WithContext(ctx).Model(&model.AuditRecord{}).Where("id = ?", recordID).Updates(map[string]any{
		"auto_release_job_id": jobID,
		"updated_at":          time.Now(),
	}).Error
}

func (s *AuditService) CountAuditing(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.AuditRecord{}).
		Where("status = ?", 2).Distinct("authorizer_app_id").Count(&count).Error
	return count, err
}
