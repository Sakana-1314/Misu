package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"misu/internal/model"
	"misu/internal/wechat"
)

type TemplateService struct {
	db     *gorm.DB
	wx     *wechat.Client
	tokens *TokenManager
	logs   *LogService
	log    *slog.Logger
	mu     sync.Mutex
}

func NewTemplateService(db *gorm.DB, wx *wechat.Client, tokens *TokenManager, logs *LogService, log *slog.Logger) *TemplateService {
	return &TemplateService{db: db, wx: wx, tokens: tokens, logs: logs, log: log}
}

func (s *TemplateService) withComponent(ctx context.Context, fn func(token string) error) error {
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		token, err := s.tokens.GetComponent(ctx)
		if err != nil {
			return err
		}
		err = fn(token)
		if err == nil {
			return nil
		}
		last = err
		if wechat.IsTokenInvalid(err) {
			s.tokens.InvalidateComponent()
			continue
		}
		return err
	}
	return last
}

func (s *TemplateService) Sync(ctx context.Context, actor, ip string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count, err := s.syncLocked(ctx)
	if err != nil {
		return 0, err
	}
	s.logs.Operation(actor, "template_sync", "", map[string]any{"count": count}, ip)
	return count, nil
}

func (s *TemplateService) syncLocked(ctx context.Context) (int, error) {
	var resp *wechat.TemplateListResponse
	err := s.withComponent(ctx, func(token string) error {
		r, err := s.wx.GetTemplateList(ctx, token, nil)
		if err == nil {
			resp = r
		}
		return err
	})
	if err != nil {
		return 0, err
	}
	now := time.Now()
	ids := make([]int64, 0, len(resp.TemplateList))
	for _, t := range resp.TemplateList {
		ids = append(ids, t.TemplateID)
		category, _ := json.Marshal(t.CategoryList)
		if category == nil {
			category = []byte("[]")
		}
		var row model.CodeTemplate
		findErr := s.db.WithContext(ctx).Where("template_id = ?", t.TemplateID).First(&row).Error
		if findErr == gorm.ErrRecordNotFound {
			row = model.CodeTemplate{
				TemplateID:             t.TemplateID,
				DraftID:                t.DraftID,
				UserVersion:            t.UserVersion,
				UserDesc:               t.UserDesc,
				CreateTime:             t.CreateTime,
				SourceMiniprogramAppID: t.SourceMiniprogramAppID,
				SourceMiniprogram:      t.SourceMiniprogram,
				TemplateType:           t.TemplateType,
				CategoryList:           datatypes.JSON(category),
				AuditScene:             t.AuditScene,
				AuditStatus:            t.AuditStatus,
				Reason:                 t.Reason,
				SyncedAt:               now,
			}
			if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
				return 0, err
			}
			continue
		}
		if findErr != nil {
			return 0, findErr
		}
		if err := s.db.WithContext(ctx).Model(&model.CodeTemplate{}).Where("template_id = ?", t.TemplateID).Updates(map[string]any{
			"draft_id":                  t.DraftID,
			"user_version":              t.UserVersion,
			"user_desc":                 t.UserDesc,
			"create_time":               t.CreateTime,
			"source_miniprogram_app_id":  t.SourceMiniprogramAppID,
			"source_miniprogram":        t.SourceMiniprogram,
			"template_type":             t.TemplateType,
			"category_list":             datatypes.JSON(category),
			"audit_scene":               t.AuditScene,
			"audit_status":              t.AuditStatus,
			"reason":                    t.Reason,
			"synced_at":                 now,
			"updated_at":                now,
		}).Error; err != nil {
			return 0, err
		}
	}
	q := s.db.WithContext(ctx).Where("1 = 1")
	if len(ids) > 0 {
		q = q.Where("template_id NOT IN ?", ids)
	}
	if err := q.Delete(&model.CodeTemplate{}).Error; err != nil {
		return 0, err
	}
	return len(resp.TemplateList), nil
}

func (s *TemplateService) List(ctx context.Context) ([]model.CodeTemplate, error) {
	var items []model.CodeTemplate
	err := s.db.WithContext(ctx).Order("create_time DESC").Find(&items).Error
	return items, err
}

func (s *TemplateService) Drafts(ctx context.Context) ([]wechat.DraftEntry, error) {
	var out []wechat.DraftEntry
	err := s.withComponent(ctx, func(token string) error {
		resp, err := s.wx.GetDraftList(ctx, token)
		if err == nil {
			out = resp.DraftList
		}
		return err
	})
	return out, err
}

func (s *TemplateService) AddDraft(ctx context.Context, actor, ip string, draftID int64, templateType int) (int, error) {
	if draftID <= 0 {
		return 0, gorm.ErrRecordNotFound
	}
	if templateType != 1 {
		templateType = 0
	}
	err := s.withComponent(ctx, func(token string) error {
		return s.wx.AddToTemplate(ctx, token, draftID, templateType)
	})
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	count, syncErr := s.syncLocked(ctx)
	s.mu.Unlock()
	s.logs.Operation(actor, "template_add", "", map[string]any{"draft_id": draftID, "template_type": templateType}, ip)
	return count, syncErr
}

func (s *TemplateService) Delete(ctx context.Context, actor, ip string, templateID int64) error {
	err := s.withComponent(ctx, func(token string) error {
		return s.wx.DeleteTemplate(ctx, token, templateID)
	})
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Where("template_id = ?", templateID).Delete(&model.CodeTemplate{}).Error; err != nil {
		return err
	}
	s.logs.Operation(actor, "template_delete", "", map[string]any{"template_id": templateID}, ip)
	return nil
}
