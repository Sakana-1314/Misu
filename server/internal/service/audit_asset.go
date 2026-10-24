package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"misu/internal/model"
)

const maxAuditAssetSize = 10 << 20

type AuditAssetService struct {
	db  *gorm.DB
	log *slog.Logger
}

func NewAuditAssetService(db *gorm.DB, log *slog.Logger) *AuditAssetService {
	return &AuditAssetService{db: db, log: log}
}

func (s *AuditAssetService) Create(ctx context.Context, filename, mime string, data []byte) (*model.AuditAsset, error) {
	if len(data) == 0 {
		return nil, errors.New("文件内容为空")
	}
	if len(data) > maxAuditAssetSize {
		return nil, errors.New("文件不能超过 10MB")
	}
	asset := model.AuditAsset{
		Filename:  filename,
		Mime:      mime,
		Size:      int64(len(data)),
		Data:      data,
		CreatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(&asset).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

func (s *AuditAssetService) Get(ctx context.Context, id uint) (*model.AuditAsset, error) {
	var asset model.AuditAsset
	if err := s.db.WithContext(ctx).Select("id", "filename", "mime", "size", "data", "created_at").First(&asset, id).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

func (s *AuditAssetService) EnsureExists(ctx context.Context, ids []uint) error {
	unique := uniqueIDs(ids)
	if len(unique) == 0 {
		return nil
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.AuditAsset{}).Where("id IN ?", unique).Count(&count).Error; err != nil {
		return err
	}
	if int(count) != len(unique) {
		return errors.New("部分提审素材不存在或已过期，请重新上传")
	}
	return nil
}

func uniqueIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
