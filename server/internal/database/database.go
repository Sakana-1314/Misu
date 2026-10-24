package database

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"misu/internal/config"
	"misu/internal/model"
)

func Open(cfg *config.Config, log *slog.Logger) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)

	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= 30; attempt++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err == nil {
			break
		}
		log.Warn("database not ready", "attempt", attempt, "error", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := db.AutoMigrate(
		&model.PlatformState{},
		&model.Authorizer{},
		&model.CodeTemplate{},
		&model.AuditAsset{},
		&model.MPWebSession{},
		&model.AuditRecord{},
		&model.ReleaseRecord{},
		&model.Job{},
		&model.JobItem{},
		&model.APICallLog{},
		&model.OperationLog{},
		&model.CallbackEvent{},
		&model.UndoQuotaUsage{},
	); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	state := model.PlatformState{ID: 1}
	if err := db.Where("id = ?", 1).FirstOrCreate(&state).Error; err != nil {
		return nil, fmt.Errorf("init platform state: %w", err)
	}
	return db, nil
}
