package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"misu/internal/model"
	"misu/internal/wechat"
)

type LogService struct {
	db   *gorm.DB
	log  *slog.Logger
	ch   chan model.APICallLog
	stop chan struct{}
	once sync.Once
}

func NewLogService(db *gorm.DB, log *slog.Logger) *LogService {
	return &LogService{
		db:   db,
		log:  log,
		ch:   make(chan model.APICallLog, 2000),
		stop: make(chan struct{}),
	}
}

func (s *LogService) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		batch := make([]model.APICallLog, 0, 200)
		for {
			select {
			case entry := <-s.ch:
				batch = append(batch, entry)
				if len(batch) >= 200 {
					s.flush(batch)
					batch = batch[:0]
				}
			case <-ticker.C:
				if len(batch) > 0 {
					s.flush(batch)
					batch = batch[:0]
				}
			case <-ctx.Done():
				for len(s.ch) > 0 {
					batch = append(batch, <-s.ch)
				}
				if len(batch) > 0 {
					s.flush(batch)
				}
				return
			}
		}
	}()
}

func (s *LogService) RecordCall(info wechat.CallInfo) {
	msg := truncate(info.Msg, 500)
	entry := model.APICallLog{
		Endpoint:  truncate(info.Endpoint, 200),
		AppID:     info.AppID,
		Method:    info.Method,
		Errcode:   info.Code,
		Errmsg:    msg,
		LatencyMS: int(info.Latency.Milliseconds()),
		CreatedAt: time.Now(),
	}
	select {
	case s.ch <- entry:
	default:
		// Avoid blocking WeChat calls if the log pipeline is saturated.
	}
}

func (s *LogService) flush(batch []model.APICallLog) {
	if len(batch) == 0 {
		return
	}
	if err := s.db.Create(&batch).Error; err != nil {
		s.log.Warn("failed to write api call logs", "error", err)
	}
}

func (s *LogService) Operation(actor, action, target string, detail any, ip string) {
	raw := RedactJSON(detail)
	entry := model.OperationLog{
		Actor:     actor,
		Action:    action,
		Target:    truncate(target, 120),
		Detail:    raw,
		IP:        ip,
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(&entry).Error; err != nil {
		s.log.Warn("failed to write operation log", "error", err)
	}
}

func RedactJSON(v any) datatypes.JSON {
	if v == nil {
		return datatypes.JSON([]byte("null"))
	}
	b, err := json.Marshal(v)
	if err != nil {
		return datatypes.JSON([]byte(`{"error":"unserializable"}`))
	}
	var parsed any
	if err := json.Unmarshal(b, &parsed); err != nil {
		return datatypes.JSON(b)
	}
	redacted := redactValue(parsed)
	out, err := json.Marshal(redacted)
	if err != nil {
		return datatypes.JSON(b)
	}
	return datatypes.JSON(out)
}

func redactValue(v any) any {
	switch value := v.(type) {
	case map[string]any:
		for key, item := range value {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "secret") ||
				strings.Contains(lower, "password") || strings.Contains(lower, "aeskey") ||
				strings.Contains(lower, "verify_ticket") || lower == "authorization_code" {
				value[key] = "***"
				continue
			}
			value[key] = redactValue(item)
		}
		return value
	case []any:
		for i, item := range value {
			value[i] = redactValue(item)
		}
		return value
	default:
		return v
	}
}

func truncate(s string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit]) + "..."
}
