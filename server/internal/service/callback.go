package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"misu/internal/config"
	"misu/internal/model"
	"misu/internal/wechat"
)

type CallbackService struct {
	cfg    *config.Config
	db     *gorm.DB
	tokens *TokenManager
	auths  *AuthorizerService
	audits *AuditService
	logs   *LogService
	log    *slog.Logger
	tasks  chan callbackTask
}

type callbackTask struct {
	kind  string
	appid string
	code  string
}

func NewCallbackService(cfg *config.Config, db *gorm.DB, tokens *TokenManager, auths *AuthorizerService, audits *AuditService, logs *LogService, log *slog.Logger) *CallbackService {
	return &CallbackService{
		cfg:    cfg,
		db:     db,
		tokens: tokens,
		auths:  auths,
		audits: audits,
		logs:   logs,
		log:    log,
		tasks:  make(chan callbackTask, 256),
	}
}

func (s *CallbackService) Start(ctx context.Context) {
	for i := 0; i < 2; i++ {
		go s.worker(ctx)
	}
}

func (s *CallbackService) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-s.tasks:
			taskCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			switch task.kind {
			case "exchange":
				if _, err := s.auths.ExchangeAuthCode(taskCtx, task.code); err != nil {
					s.log.Warn("failed to exchange authorization code", "appid", task.appid, "error", err)
					if task.appid != "" {
						if _, syncErr := s.auths.SyncInfo(taskCtx, task.appid); syncErr != nil {
							s.log.Warn("failed to sync authorizer after exchange failure", "appid", task.appid, "error", syncErr)
						}
					}
				}
			case "unauthorized":
				if err := s.auths.MarkUnauthorized(taskCtx, task.appid); err != nil {
					s.log.Warn("failed to mark authorizer unauthorized", "appid", task.appid, "error", err)
				}
			case "audit_refresh":
				if _, err := s.audits.RefreshLatest(taskCtx, task.appid); err != nil {
					s.log.Warn("failed to refresh audit status from event", "appid", task.appid, "error", err)
				}
			}
			cancel()
		}
	}
}

func (s *CallbackService) VerifyURL(query url.Values) (string, bool) {
	echostr := query.Get("echostr")
	if !wechat.VerifyURL(s.cfg.ComponentVerifyToken, query.Get("signature"), query.Get("timestamp"), query.Get("nonce")) {
		return "", false
	}
	return echostr, true
}

func (s *CallbackService) HandleAuthEvent(ctx context.Context, query url.Values, body []byte) error {
	plain, err := s.decrypt(query, body)
	if err != nil {
		return err
	}
	msg, err := wechat.ParseEvent(plain)
	if err != nil {
		return err
	}
	if msg.InfoType == "component_verify_ticket" {
		if err := s.tokens.SaveTicket(msg.ComponentVerifyTicket, msg.CreateTime); err != nil {
			return fmt.Errorf("save verify ticket: %w", err)
		}
		_, _ = s.storeEvent(msg, plain)
		return nil
	}
	inserted, err := s.storeEvent(msg, plain)
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}
	switch msg.InfoType {
	case "authorized", "updateauthorized":
		if msg.AuthorizationCode != "" {
			s.enqueue(callbackTask{kind: "exchange", appid: msg.AuthorizerAppID, code: msg.AuthorizationCode})
		}
	case "unauthorized":
		if msg.AuthorizerAppID != "" {
			s.enqueue(callbackTask{kind: "unauthorized", appid: msg.AuthorizerAppID})
		}
	}
	return nil
}

func (s *CallbackService) HandleMessageEvent(ctx context.Context, appid string, query url.Values, body []byte) error {
	plain, err := s.decrypt(query, body)
	if err != nil {
		return err
	}
	msg, err := wechat.ParseEvent(plain)
	if err != nil {
		return err
	}
	if msg.AuthorizerAppID != "" {
		appid = msg.AuthorizerAppID
	}
	inserted, err := s.storeEvent(msg, plain)
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}
	isAudit := msg.Event == "weapp_audit_success" || msg.Event == "weapp_audit_fail" || msg.Event == "weapp_audit_delay" ||
		msg.InfoType == "weapp_audit_success" || msg.InfoType == "weapp_audit_fail" || msg.InfoType == "weapp_audit_delay"
	if isAudit && appid != "" {
		s.enqueue(callbackTask{kind: "audit_refresh", appid: appid})
	}
	return nil
}

func (s *CallbackService) decrypt(query url.Values, body []byte) ([]byte, error) {
	if s.cfg.ComponentVerifyToken == "" || s.cfg.ComponentAESKey == "" {
		return nil, fmt.Errorf("回调配置不完整，请设置 Token 和 EncodingAESKey")
	}
	env, err := wechat.ParseEnvelope(body)
	if err != nil {
		return nil, err
	}
	timestamp := query.Get("timestamp")
	nonce := query.Get("nonce")
	if env.Encrypt != "" {
		msgSignature := query.Get("msg_signature")
		if msgSignature == "" {
			return nil, fmt.Errorf("缺少 msg_signature")
		}
		if !wechat.VerifyMessage(s.cfg.ComponentVerifyToken, msgSignature, timestamp, nonce, env.Encrypt) {
			return nil, fmt.Errorf("消息签名校验失败")
		}
		plain, appID, err := wechat.DecryptMessage(s.cfg.ComponentAESKey, env.Encrypt)
		if err != nil && s.cfg.ComponentAESKeyPrev != "" {
			plain, appID, err = wechat.DecryptMessage(s.cfg.ComponentAESKeyPrev, env.Encrypt)
		}
		if err != nil {
			return nil, fmt.Errorf("解密消息失败: %w", err)
		}
		if s.cfg.ComponentAppID != "" && appID != "" && appID != s.cfg.ComponentAppID {
			return nil, fmt.Errorf("消息 AppID 不匹配")
		}
		return plain, nil
	}
	if !wechat.VerifyURL(s.cfg.ComponentVerifyToken, query.Get("signature"), timestamp, nonce) {
		return nil, fmt.Errorf("消息签名校验失败")
	}
	return body, nil
}

func (s *CallbackService) storeEvent(msg *wechat.EventMessage, plain []byte) (bool, error) {
	infoType := msg.InfoType
	if infoType == "" {
		infoType = msg.Event
	}
	if infoType == "" {
		infoType = msg.MsgType
	}
	hash := sha256.Sum256(plain)
	keyInput := fmt.Sprintf("%s|%s|%d|%s", infoType, msg.AuthorizerAppID, msg.CreateTime, hex.EncodeToString(hash[:]))
	dedup := sha256.Sum256([]byte(keyInput))
	payload := map[string]any{
		"info_type":        infoType,
		"appid":            msg.AppID,
		"authorizer_appid": msg.AuthorizerAppID,
		"create_time":      msg.CreateTime,
		"msg_type":         msg.MsgType,
		"event":            msg.Event,
	}
	var eventTime *time.Time
	if msg.CreateTime > 0 {
		t := time.Unix(msg.CreateTime, 0)
		eventTime = &t
	}
	entry := model.CallbackEvent{
		InfoType:        truncate(infoType, 60),
		AuthorizerAppID: msg.AuthorizerAppID,
		AppID:           msg.AppID,
		EventTime:       eventTime,
		Payload:         RedactJSON(payload),
		DedupKey:        hex.EncodeToString(dedup[:]),
		CreatedAt:       time.Now(),
	}
	res := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (s *CallbackService) enqueue(task callbackTask) {
	select {
	case s.tasks <- task:
	default:
		s.log.Warn("callback task queue full", "kind", task.kind, "appid", task.appid)
	}
}
