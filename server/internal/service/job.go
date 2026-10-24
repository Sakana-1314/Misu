package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"misu/internal/config"
	"misu/internal/model"
	"misu/internal/wechat"
)

const (
	JobCommit            = "commit"
	JobSubmitAudit       = "submit_audit"
	JobRelease           = "release"
	JobUndoAudit         = "undo_audit"
	JobRollback          = "rollback"
	JobGrayRelease       = "gray_release"
	JobCancelGrayRelease = "cancel_gray_release"
	JobModifyDomain      = "modify_domain"
	JobBindTester        = "bind_tester"
	JobUnbindTester      = "unbind_tester"
	JobSetVisitStatus    = "set_visit_status"
	JobSetSupportVersion = "set_support_version"
	JobPreflight         = "preflight"
)

var jobTitles = map[string]string{
	JobCommit:            "批量上传代码",
	JobSubmitAudit:       "批量提交审核",
	JobRelease:           "批量发布",
	JobUndoAudit:         "批量撤回审核",
	JobRollback:          "批量版本回退",
	JobGrayRelease:       "批量分阶段发布",
	JobCancelGrayRelease: "批量取消分阶段发布",
	JobModifyDomain:      "批量配置服务器域名",
	JobBindTester:        "批量绑定体验者",
	JobUnbindTester:      "批量解绑体验者",
	JobSetVisitStatus:    "批量设置服务状态",
	JobSetSupportVersion: "批量设置最低基础库",
	JobPreflight:         "批量提审预检",
}

func JobTitle(jobType string) string {
	if t, ok := jobTitles[jobType]; ok {
		return t
	}
	return jobType
}

type JobService struct {
	cfg      *config.Config
	db       *gorm.DB
	runner   *JobRunner
	logs     *LogService
	profiles *ProfileService
	auths    *AuthorizerService
	log      *slog.Logger
}

func NewJobService(cfg *config.Config, db *gorm.DB, runner *JobRunner, logs *LogService, profiles *ProfileService, auths *AuthorizerService, log *slog.Logger) *JobService {
	return &JobService{cfg: cfg, db: db, runner: runner, logs: logs, profiles: profiles, auths: auths, log: log}
}

type CreateJobRequest struct {
	Type    string          `json:"type"`
	AppIDs  []string        `json:"appids"`
	Payload json.RawMessage `json:"payload"`
}

type ListJobsQuery struct {
	Type     string
	Status   string
	Page     int
	PageSize int
}

func (s *JobService) Create(ctx context.Context, actor, ip string, req CreateJobRequest) (*model.Job, error) {
	if _, ok := jobTitles[req.Type]; !ok {
		return nil, errors.New("不支持的任务类型")
	}
	appids := dedupeStrings(req.AppIDs)
	if len(appids) == 0 {
		return nil, errors.New("请选择至少一个小程序")
	}
	if len(appids) > 200 {
		return nil, errors.New("单次批量操作最多 200 个小程序")
	}
	active, _, err := s.auths.ActiveAppIDs(ctx, appids)
	if err != nil {
		return nil, err
	}
	if len(active) != len(appids) {
		return nil, errors.New("部分小程序不存在或已取消授权，请刷新列表后重试")
	}
	payload, err := s.normalizePayload(ctx, req.Type, req.Payload)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	job := model.Job{
		Type:      req.Type,
		Title:     fmt.Sprintf("%s（%d 个小程序）", JobTitle(req.Type), len(active)),
		Status:    model.JobQueued,
		Payload:   payload,
		Total:     len(active),
		CreatedBy: actor,
		CreatedAt: now,
		UpdatedAt: now,
	}
	items := make([]model.JobItem, 0, len(active))
	for _, appid := range active {
		items = append(items, model.JobItem{
			AuthorizerAppID: appid,
			Status:          model.ItemPending,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&job).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].JobID = job.ID
		}
		return tx.Create(&items).Error
	})
	if err != nil {
		return nil, err
	}
	s.runner.Enqueue(job.ID)
	s.logs.Operation(actor, "job_create", fmt.Sprint(job.ID), map[string]any{
		"type": job.Type, "appids": active, "payload": json.RawMessage(payload),
	}, ip)
	return &job, nil
}

func (s *JobService) CreateAutoRelease(ctx context.Context, appid string, auditID int64) (*model.Job, error) {
	var existing model.Job
	err := s.db.WithContext(ctx).Where("type = ? AND source_audit_id = ?", JobRelease, auditID).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	payload := datatypes.JSON([]byte(`{"auto":true}`))
	now := time.Now()
	job := model.Job{
		Type:          JobRelease,
		Title:         "审核通过自动发布（1 个小程序）",
		Status:        model.JobQueued,
		Payload:       payload,
		Total:         1,
		CreatedBy:     "system",
		SourceAuditID: &auditID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	item := model.JobItem{AuthorizerAppID: appid, Status: model.ItemPending, CreatedAt: now, UpdatedAt: now}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&job).Error; err != nil {
			return err
		}
		item.JobID = job.ID
		return tx.Create(&item).Error
	})
	if err != nil {
		return nil, err
	}
	s.runner.Enqueue(job.ID)
	s.logs.Operation("system", "job_auto_release", fmt.Sprint(job.ID), map[string]any{"appid": appid, "audit_id": auditID}, "")
	return &job, nil
}

func (s *JobService) List(ctx context.Context, q ListJobsQuery) ([]model.Job, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}
	base := s.db.WithContext(ctx).Model(&model.Job{})
	if q.Type != "" {
		base = base.Where("type = ?", q.Type)
	}
	if q.Status != "" {
		base = base.Where("status = ?", q.Status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Job
	err := base.Order("created_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error
	return items, total, err
}

func (s *JobService) Get(ctx context.Context, id uint) (*model.Job, error) {
	var job model.Job
	if err := s.db.WithContext(ctx).First(&job, id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *JobService) Items(ctx context.Context, jobID uint, status string, page, pageSize int) ([]model.JobItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	base := s.db.WithContext(ctx).Model(&model.JobItem{}).Where("job_id = ?", jobID)
	if status != "" {
		base = base.Where("status = ?", status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.JobItem
	err := base.Order("id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

func (s *JobService) RetryFailed(ctx context.Context, actor, ip string, jobID uint) (*model.Job, error) {
	var job model.Job
	if err := s.db.WithContext(ctx).First(&job, jobID).Error; err != nil {
		return nil, err
	}
	if job.Status != model.JobFailed && job.Status != model.JobPartial && job.Status != model.JobCanceled {
		return nil, errors.New("只有失败、部分成功或已取消的任务可以重试")
	}
	var failed []model.JobItem
	if err := s.db.WithContext(ctx).Where("job_id = ? AND status = ?", jobID, model.ItemFailed).Find(&failed).Error; err != nil {
		return nil, err
	}
	if len(failed) == 0 {
		return nil, errors.New("没有可重试的失败项")
	}
	for _, item := range failed {
		if item.Attempts >= 5 {
			return nil, fmt.Errorf("小程序 %s 已重试 5 次，请核实原因后再操作", item.AuthorizerAppID)
		}
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Job{}).Where("id = ?", jobID).Updates(map[string]any{
			"status":           model.JobQueued,
			"failed_count":     0,
			"finished_at":      nil,
			"error":            "",
			"cancel_requested": false,
			"updated_at":       time.Now(),
		}).Error; err != nil {
			return err
		}
		return tx.Model(&model.JobItem{}).Where("job_id = ? AND status = ?", jobID, model.ItemFailed).Updates(map[string]any{
			"status":        model.ItemPending,
			"request":       nil,
			"response":      nil,
			"error_code":    "",
			"error_message": "",
			"audit_id":      0,
			"started_at":    nil,
			"finished_at":   nil,
			"updated_at":    time.Now(),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	s.runner.Enqueue(jobID)
	s.logs.Operation(actor, "job_retry", fmt.Sprint(jobID), map[string]any{"failed": len(failed)}, ip)
	return s.Get(ctx, jobID)
}

func (s *JobService) Cancel(ctx context.Context, actor, ip string, jobID uint) (*model.Job, error) {
	var job model.Job
	if err := s.db.WithContext(ctx).First(&job, jobID).Error; err != nil {
		return nil, err
	}
	switch job.Status {
	case model.JobQueued:
		now := time.Now()
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		if err != nil {
			return nil, err
		}
	case model.JobRunning:
		if err := s.db.WithContext(ctx).Model(&model.Job{}).Where("id = ?", jobID).Updates(map[string]any{
			"cancel_requested": true,
			"updated_at":       time.Now(),
		}).Error; err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("该任务已结束，无法取消")
	}
	s.logs.Operation(actor, "job_cancel", fmt.Sprint(jobID), nil, ip)
	return s.Get(ctx, jobID)
}

func (s *JobService) normalizePayload(ctx context.Context, jobType string, raw json.RawMessage) (datatypes.JSON, error) {
	switch jobType {
	case JobCommit:
		var p struct {
			TemplateID  int64  `json:"template_id"`
			UserVersion string `json:"user_version"`
			UserDesc    string `json:"user_desc"`
			ExtJSON     string `json:"ext_json"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, errors.New("上传代码参数格式错误")
			}
		}
		if p.TemplateID <= 0 {
			return nil, errors.New("请选择代码模板")
		}
		p.UserVersion = strings.TrimSpace(p.UserVersion)
		p.UserDesc = strings.TrimSpace(p.UserDesc)
		if p.UserVersion == "" || len([]rune(p.UserVersion)) > 64 {
			return nil, errors.New("版本号不能为空且不能超过 64 个字符")
		}
		if strings.TrimSpace(p.ExtJSON) == "" {
			p.ExtJSON = "{}"
		}
		if !json.Valid([]byte(renderPlaceholders(p.ExtJSON, "wx0000000000000000", "示例小程序", p.UserVersion))) {
			return nil, errors.New("ext_json 不是合法的 JSON")
		}
		return marshalJSON(p)
	case JobSubmitAudit:
		var p struct {
			ProfileID   uint `json:"profile_id"`
			AutoRelease bool `json:"auto_release"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, errors.New("提审参数格式错误")
			}
		}
		if p.ProfileID == 0 {
			return nil, errors.New("请选择审核配置")
		}
		snapshot, err := s.profiles.Snapshot(ctx, p.ProfileID)
		if err != nil {
			return nil, fmt.Errorf("加载审核配置失败: %w", err)
		}
		return marshalJSON(AuditSubmitPayload{Profile: *snapshot, AutoRelease: p.AutoRelease})
	case JobRelease, JobUndoAudit, JobCancelGrayRelease, JobPreflight:
		return datatypes.JSON([]byte("{}")), nil
	case JobRollback:
		var p struct {
			AppVersion string `json:"app_version"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, errors.New("版本回退参数格式错误")
			}
		}
		return marshalJSON(p)
	case JobGrayRelease:
		var p wechat.GrayReleaseRequest
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, errors.New("灰度发布参数格式错误")
			}
		}
		if p.GrayPercentage < 0 || p.GrayPercentage > 100 {
			return nil, errors.New("灰度比例必须在 0-100 之间")
		}
		if p.GrayPercentage == 0 && !p.SupportDebugerFirst && !p.SupportExperiencerFirst {
			return nil, errors.New("灰度比例为 0 时，至少选择项目成员或体验成员优先")
		}
		return marshalJSON(p)
	case JobModifyDomain:
		var p wechat.DomainRequest
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, errors.New("域名参数格式错误")
			}
		}
		p.Action = strings.TrimSpace(p.Action)
		if p.Action != "add" && p.Action != "set" && p.Action != "delete" {
			return nil, errors.New("域名操作必须是 add、set 或 delete")
		}
		p.RequestDomain = cleanDomains(p.RequestDomain)
		p.WSRequestDomain = cleanDomains(p.WSRequestDomain)
		p.UploadDomain = cleanDomains(p.UploadDomain)
		p.DownloadDomain = cleanDomains(p.DownloadDomain)
		p.UDPDomain = cleanDomains(p.UDPDomain)
		p.TCPDomain = cleanDomains(p.TCPDomain)
		if err := validateDomains(p); err != nil {
			return nil, err
		}
		return marshalJSON(p)
	case JobBindTester:
		var p struct {
			WeChatID string `json:"wechatid"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, errors.New("体验者参数格式错误")
			}
		}
		p.WeChatID = strings.TrimSpace(p.WeChatID)
		if p.WeChatID == "" {
			return nil, errors.New("请填写体验者微信号")
		}
		return marshalJSON(p)
	case JobUnbindTester:
		var p struct {
			UserStr string `json:"userstr"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, errors.New("体验者参数格式错误")
			}
		}
		if strings.TrimSpace(p.UserStr) == "" {
			return nil, errors.New("缺少体验者 userstr")
		}
		return marshalJSON(p)
	case JobSetVisitStatus:
		var p struct {
			Action string `json:"action"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, errors.New("服务状态参数格式错误")
			}
		}
		if p.Action != "open" && p.Action != "close" {
			return nil, errors.New("服务状态必须是 open 或 close")
		}
		return marshalJSON(p)
	case JobSetSupportVersion:
		var p struct {
			Version string `json:"version"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, errors.New("基础库版本参数格式错误")
			}
		}
		p.Version = strings.TrimSpace(p.Version)
		if p.Version == "" {
			return nil, errors.New("请填写基础库版本号")
		}
		return marshalJSON(p)
	}
	return nil, errors.New("不支持的任务类型")
}

func marshalJSON(v any) (datatypes.JSON, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(b), nil
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func cleanDomains(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func validateDomains(req wechat.DomainRequest) error {
	lists := []struct {
		name   string
		values []string
	}{
		{"request 合法域名", req.RequestDomain},
		{"socket 合法域名", req.WSRequestDomain},
		{"uploadFile 合法域名", req.UploadDomain},
		{"downloadFile 合法域名", req.DownloadDomain},
		{"UDP 合法域名", req.UDPDomain},
		{"TCP 合法域名", req.TCPDomain},
	}
	total := 0
	for _, list := range lists {
		if len(list.values) > 200 {
			return fmt.Errorf("%s最多 200 个", list.name)
		}
		for _, d := range list.values {
			if !validDomain(d) {
				return fmt.Errorf("%s不合法: %s", list.name, d)
			}
			total++
		}
	}
	if total == 0 {
		return errors.New("请至少填写一个域名")
	}
	return nil
}

func validDomain(value string) bool {
	if strings.ContainsAny(value, " /\\") || strings.Contains(value, "://") {
		return false
	}
	host := value
	if strings.Contains(value, ":") {
		h, p, err := net.SplitHostPort(value)
		if err != nil {
			return false
		}
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
		host = h
	}
	if net.ParseIP(host) != nil {
		return false
	}
	return strings.Contains(host, ".")
}

func renderPlaceholders(input, appid, nickname, version string) string {
	replacer := strings.NewReplacer(
		"{{appid}}", appid,
		"{{authorizer_appid}}", appid,
		"{{nick_name}}", nickname,
		"{{user_version}}", version,
		"{{now}}", time.Now().Format("20060102150405"),
	)
	return replacer.Replace(input)
}
