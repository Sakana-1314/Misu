package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"misu/internal/model"
	"misu/internal/wechat"
)

type ProfileService struct {
	db   *gorm.DB
	logs *LogService
	log  *slog.Logger
}

type ProfileInput struct {
	Name          string             `json:"name"`
	ItemList      []wechat.AuditItem `json:"item_list"`
	VersionDesc   string             `json:"version_desc"`
	FeedbackInfo  string             `json:"feedback_info"`
	FeedbackStuff string             `json:"feedback_stuff"`
	UGCDeclare    *wechat.UGCDeclare `json:"ugc_declare"`
	PreviewInfo   *wechat.PreviewInfo `json:"preview_info"`
	IsDefault     bool               `json:"is_default"`
}

type ProfileSnapshot struct {
	ID            uint               `json:"id"`
	Name          string             `json:"name"`
	ItemList      []wechat.AuditItem `json:"item_list"`
	VersionDesc   string             `json:"version_desc"`
	FeedbackInfo  string             `json:"feedback_info"`
	FeedbackStuff string             `json:"feedback_stuff"`
	UGCDeclare    *wechat.UGCDeclare `json:"ugc_declare,omitempty"`
	PreviewInfo   *wechat.PreviewInfo `json:"preview_info,omitempty"`
}

func NewProfileService(db *gorm.DB, logs *LogService, log *slog.Logger) *ProfileService {
	return &ProfileService{db: db, logs: logs, log: log}
}

func (s *ProfileService) List(ctx context.Context) ([]model.AuditProfile, error) {
	var items []model.AuditProfile
	err := s.db.WithContext(ctx).Order("is_default DESC, updated_at DESC").Find(&items).Error
	return items, err
}

func (s *ProfileService) Get(ctx context.Context, id uint) (*model.AuditProfile, error) {
	var p model.AuditProfile
	if err := s.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *ProfileService) Create(ctx context.Context, actor, ip string, input ProfileInput) (*model.AuditProfile, error) {
	items, ugc, preview, err := validateProfile(input)
	if err != nil {
		return nil, err
	}
	p := model.AuditProfile{
		Name:          strings.TrimSpace(input.Name),
		ItemList:      items,
		VersionDesc:   strings.TrimSpace(input.VersionDesc),
		FeedbackInfo:  input.FeedbackInfo,
		FeedbackStuff: input.FeedbackStuff,
		UGCDeclare:    ugc,
		PreviewInfo:   preview,
		IsDefault:     input.IsDefault,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if p.IsDefault {
			if err := tx.Model(&model.AuditProfile{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&p).Error
	})
	if err != nil {
		return nil, err
	}
	s.logs.Operation(actor, "audit_profile_create", fmt.Sprint(p.ID), map[string]any{"name": p.Name}, ip)
	return &p, nil
}

func (s *ProfileService) Update(ctx context.Context, actor, ip string, id uint, input ProfileInput) (*model.AuditProfile, error) {
	var existing model.AuditProfile
	if err := s.db.WithContext(ctx).First(&existing, id).Error; err != nil {
		return nil, err
	}
	items, ugc, preview, err := validateProfile(input)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"name":           strings.TrimSpace(input.Name),
		"item_list":      items,
		"version_desc":   strings.TrimSpace(input.VersionDesc),
		"feedback_info":  input.FeedbackInfo,
		"feedback_stuff": input.FeedbackStuff,
		"ugc_declare":    ugc,
		"preview_info":   preview,
		"is_default":     input.IsDefault,
		"updated_at":     time.Now(),
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if input.IsDefault {
			if err := tx.Model(&model.AuditProfile{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.AuditProfile{}).Where("id = ?", id).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}
	s.logs.Operation(actor, "audit_profile_update", fmt.Sprint(id), map[string]any{"name": input.Name}, ip)
	return s.Get(ctx, id)
}

func (s *ProfileService) Delete(ctx context.Context, actor, ip string, id uint) error {
	var p model.AuditProfile
	if err := s.db.WithContext(ctx).First(&p, id).Error; err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Delete(&p).Error; err != nil {
		return err
	}
	s.logs.Operation(actor, "audit_profile_delete", fmt.Sprint(id), map[string]any{"name": p.Name}, ip)
	return nil
}

func (s *ProfileService) Snapshot(ctx context.Context, id uint) (*ProfileSnapshot, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return snapshotProfile(p)
}

func snapshotProfile(p *model.AuditProfile) (*ProfileSnapshot, error) {
	var items []wechat.AuditItem
	if len(p.ItemList) > 0 {
		if err := json.Unmarshal(p.ItemList, &items); err != nil {
			return nil, fmt.Errorf("解析审核项失败: %w", err)
		}
	}
	snap := &ProfileSnapshot{
		ID:            p.ID,
		Name:          p.Name,
		ItemList:      items,
		VersionDesc:   p.VersionDesc,
		FeedbackInfo:  p.FeedbackInfo,
		FeedbackStuff: p.FeedbackStuff,
	}
	if len(p.UGCDeclare) > 0 && string(p.UGCDeclare) != "null" {
		var ugc wechat.UGCDeclare
		if err := json.Unmarshal(p.UGCDeclare, &ugc); err != nil {
			return nil, fmt.Errorf("解析 UGC 声明失败: %w", err)
		}
		snap.UGCDeclare = &ugc
	}
	if len(p.PreviewInfo) > 0 && string(p.PreviewInfo) != "null" {
		var preview wechat.PreviewInfo
		if err := json.Unmarshal(p.PreviewInfo, &preview); err != nil {
			return nil, fmt.Errorf("解析预览素材失败: %w", err)
		}
		snap.PreviewInfo = &preview
	}
	return snap, nil
}

func validateProfile(input ProfileInput) (datatypes.JSON, datatypes.JSON, datatypes.JSON, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, nil, nil, errors.New("配置名称不能为空")
	}
	if utf8.RuneCountInString(name) > 64 {
		return nil, nil, nil, errors.New("配置名称不能超过 64 个字")
	}
	if len(input.ItemList) < 1 || len(input.ItemList) > 5 {
		return nil, nil, nil, errors.New("审核项数量必须在 1-5 之间")
	}
	for i, item := range input.ItemList {
		if strings.TrimSpace(item.FirstClass) == "" || item.FirstID == 0 {
			return nil, nil, nil, fmt.Errorf("第 %d 个审核项缺少一级类目", i+1)
		}
		if strings.TrimSpace(item.SecondClass) == "" || item.SecondID == 0 {
			return nil, nil, nil, fmt.Errorf("第 %d 个审核项缺少二级类目", i+1)
		}
		if (item.ThirdClass == "") != (item.ThirdID == 0) {
			return nil, nil, nil, fmt.Errorf("第 %d 个审核项的二级/三级类目不匹配", i+1)
		}
		if utf8.RuneCountInString(item.Title) > 32 {
			return nil, nil, nil, fmt.Errorf("第 %d 个审核项标题不能超过 32 个字", i+1)
		}
	}
	items, err := json.Marshal(input.ItemList)
	if err != nil {
		return nil, nil, nil, err
	}
	ugc := datatypes.JSON([]byte("null"))
	if input.UGCDeclare != nil {
		raw, err := json.Marshal(input.UGCDeclare)
		if err != nil {
			return nil, nil, nil, err
		}
		ugc = datatypes.JSON(raw)
	}
	preview := datatypes.JSON([]byte("null"))
	if input.PreviewInfo != nil {
		raw, err := json.Marshal(input.PreviewInfo)
		if err != nil {
			return nil, nil, nil, err
		}
		preview = datatypes.JSON(raw)
	}
	return datatypes.JSON(items), ugc, preview, nil
}
