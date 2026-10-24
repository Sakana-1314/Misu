package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"misu/internal/model"
	"misu/internal/wechat"
)

type ExecutorDeps struct {
	db     *gorm.DB
	wx     *wechat.Client
	auths  *AuthorizerService
	audits *AuditService
	log    *slog.Logger
}

type funcExecutor struct {
	typ string
	fn  func(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult
}

func (f *funcExecutor) Type() string { return f.typ }
func (f *funcExecutor) Execute(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	return f.fn(ctx, job, item)
}

func NewExecutors(db *gorm.DB, wx *wechat.Client, auths *AuthorizerService, audits *AuditService, log *slog.Logger) []Executor {
	d := &ExecutorDeps{db: db, wx: wx, auths: auths, audits: audits, log: log}
	return []Executor{
		&funcExecutor{typ: JobCommit, fn: d.commit},
		&funcExecutor{typ: JobSubmitAudit, fn: d.submitAudit},
		&funcExecutor{typ: JobRelease, fn: d.release},
		&funcExecutor{typ: JobUndoAudit, fn: d.undoAudit},
		&funcExecutor{typ: JobRollback, fn: d.rollback},
		&funcExecutor{typ: JobGrayRelease, fn: d.grayRelease},
		&funcExecutor{typ: JobCancelGrayRelease, fn: d.cancelGrayRelease},
		&funcExecutor{typ: JobModifyDomain, fn: d.modifyDomain},
		&funcExecutor{typ: JobBindTester, fn: d.bindTester},
		&funcExecutor{typ: JobUnbindTester, fn: d.unbindTester},
		&funcExecutor{typ: JobSetVisitStatus, fn: d.setVisitStatus},
		&funcExecutor{typ: JobSetSupportVersion, fn: d.setSupportVersion},
		&funcExecutor{typ: JobPreflight, fn: d.preflight},
	}
}

type CommitPayload struct {
	TemplateID  int64  `json:"template_id"`
	UserVersion string `json:"user_version"`
	UserDesc    string `json:"user_desc"`
	ExtJSON     string `json:"ext_json"`
}

type WeChatIDPayload struct {
	WeChatID string `json:"wechatid"`
}

type UserStrPayload struct {
	UserStr string `json:"userstr"`
}

type ActionPayload struct {
	Action string `json:"action"`
}

type VersionPayload struct {
	Version string `json:"version"`
}

type RollbackPayload struct {
	AppVersion string `json:"app_version"`
}

func (d *ExecutorDeps) commit(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	var p CommitPayload
	if err := DecodeJobPayload(job, &p); err != nil {
		return errorResult(err)
	}
	a, err := d.auths.Get(ctx, item.AuthorizerAppID)
	if err != nil {
		return errorResult(err)
	}
	ext := renderPlaceholders(p.ExtJSON, a.AppID, a.NickName, p.UserVersion)
	if !json.Valid([]byte(ext)) {
		return ItemResult{ErrorCode: "INVALID_EXT_JSON", ErrorMessage: "ext_json 不是合法的 JSON"}
	}
	req := wechat.CommitRequest{
		TemplateID:  p.TemplateID,
		ExtJSON:     ext,
		UserVersion: p.UserVersion,
		UserDesc:    p.UserDesc,
	}
	err = d.auths.CallWithToken(ctx, a.AppID, func(token string) error {
		return d.wx.Commit(ctx, a.AppID, token, req)
	})
	if err != nil {
		return errorResult(err)
	}
	return ItemResult{
		Request:  map[string]any{"template_id": p.TemplateID, "user_version": p.UserVersion, "user_desc": p.UserDesc, "ext_json": ext},
		Response: map[string]any{"ok": true, "message": "代码已上传并生成体验版"},
	}
}

func (d *ExecutorDeps) submitAudit(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	var p AuditSubmitPayload
	if err := DecodeJobPayload(job, &p); err != nil {
		return errorResult(err)
	}
	record, err := d.audits.Submit(ctx, item.AuthorizerAppID, p)
	if err != nil {
		if errors.Is(err, ErrAlreadyAuditing) {
			return ItemResult{
				Skipped:      true,
				ErrorCode:    "AUDIT_IN_PROGRESS",
				ErrorMessage: "该小程序已有版本正在审核中，已跳过",
				Request:      map[string]any{"item_list": p.ItemList},
			}
		}
		return errorResult(err)
	}
	return ItemResult{
		Request: map[string]any{
			"item_list": p.ItemList, "version_desc": p.VersionDesc,
			"preview_pics": len(p.PreviewPics), "preview_videos": len(p.PreviewVideos),
			"feedback_assets": len(p.FeedbackAssets), "ugc_declare": p.UGCDeclare != nil,
			"auto_release": p.AutoRelease,
		},
		Response: map[string]any{
			"audit_id": record.AuditID, "status": record.Status,
			"user_version": record.UserVersion, "auto_release": record.AutoRelease,
		},
		AuditID: record.AuditID,
	}
}

func (d *ExecutorDeps) release(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	appid := item.AuthorizerAppID
	err := d.auths.CallWithToken(ctx, appid, func(token string) error {
		return d.wx.Release(ctx, appid, token)
	})
	if err != nil {
		return errorResult(err)
	}
	version := ""
	if info, err := d.auths.Version(ctx, appid); err == nil {
		version = info.ReleaseInfo.ReleaseVersion
	}
	d.recordRelease(job.ID, appid, version, "", "release")
	return ItemResult{
		Request:  map[string]any{"action": "release"},
		Response: map[string]any{"ok": true, "version": version},
	}
}

func (d *ExecutorDeps) undoAudit(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	appid := item.AuthorizerAppID
	err := d.auths.CallWithToken(ctx, appid, func(token string) error {
		return d.wx.UndoAudit(ctx, appid, token)
	})
	if err != nil {
		return errorResult(err)
	}
	_ = d.audits.MarkUndone(ctx, appid)
	d.bumpUndoQuota(appid)
	return ItemResult{
		Request:  map[string]any{"action": "undo_audit"},
		Response: map[string]any{"ok": true},
	}
}

func (d *ExecutorDeps) rollback(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	var p RollbackPayload
	if err := DecodeJobPayload(job, &p); err != nil {
		return errorResult(err)
	}
	appid := item.AuthorizerAppID
	err := d.auths.CallWithToken(ctx, appid, func(token string) error {
		return d.wx.RevertCodeRelease(ctx, appid, token, p.AppVersion)
	})
	if err != nil {
		return errorResult(err)
	}
	version := p.AppVersion
	if version == "" {
		version = "上一版本"
	}
	d.recordRelease(job.ID, appid, version, "", "rollback")
	return ItemResult{
		Request:  map[string]any{"app_version": p.AppVersion},
		Response: map[string]any{"ok": true, "version": version},
	}
}

func (d *ExecutorDeps) grayRelease(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	var p wechat.GrayReleaseRequest
	if err := DecodeJobPayload(job, &p); err != nil {
		return errorResult(err)
	}
	appid := item.AuthorizerAppID
	err := d.auths.CallWithToken(ctx, appid, func(token string) error {
		return d.wx.GrayRelease(ctx, appid, token, p)
	})
	if err != nil {
		return errorResult(err)
	}
	var plan any
	if resp, err := d.auths.GrayPlan(ctx, appid); err == nil {
		plan = resp.GrayReleasePlan
	}
	d.recordRelease(job.ID, appid, "", fmt.Sprintf("灰度 %d%%", p.GrayPercentage), "gray")
	return ItemResult{
		Request:  p,
		Response: map[string]any{"ok": true, "plan": plan},
	}
}

func (d *ExecutorDeps) cancelGrayRelease(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	appid := item.AuthorizerAppID
	err := d.auths.CallWithToken(ctx, appid, func(token string) error {
		return d.wx.RevertGrayRelease(ctx, appid, token)
	})
	if err != nil {
		return errorResult(err)
	}
	d.recordRelease(job.ID, appid, "", "取消分阶段发布", "cancel_gray")
	return ItemResult{
		Request:  map[string]any{"action": "cancel_gray_release"},
		Response: map[string]any{"ok": true},
	}
}

func (d *ExecutorDeps) modifyDomain(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	var p wechat.DomainRequest
	if err := DecodeJobPayload(job, &p); err != nil {
		return errorResult(err)
	}
	appid := item.AuthorizerAppID
	var resp *wechat.DomainResponse
	err := d.auths.CallWithToken(ctx, appid, func(token string) error {
		r, err := d.wx.ModifyDomain(ctx, appid, token, p)
		if err == nil {
			resp = r
		}
		return err
	})
	if err != nil {
		return errorResult(err)
	}
	warnings := make([]string, 0)
	if resp != nil {
		if len(resp.NoICPDomain) > 0 {
			warnings = append(warnings, fmt.Sprintf("以下域名未备案: %v", resp.NoICPDomain))
		}
		if len(resp.InvalidRequestDomain)+len(resp.InvalidWSRequestDomain)+len(resp.InvalidUploadDomain)+len(resp.InvalidDownloadDomain)+len(resp.InvalidUDPDomain)+len(resp.InvalidTCPDomain) > 0 {
			warnings = append(warnings, "存在不合法域名，请查看返回详情")
		}
	}
	return ItemResult{
		Request:  p,
		Response: map[string]any{"domains": resp, "warnings": warnings},
	}
}

func (d *ExecutorDeps) bindTester(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	var p WeChatIDPayload
	if err := DecodeJobPayload(job, &p); err != nil {
		return errorResult(err)
	}
	appid := item.AuthorizerAppID
	var userstr string
	err := d.auths.CallWithToken(ctx, appid, func(token string) error {
		resp, err := d.wx.BindTester(ctx, appid, token, p.WeChatID)
		if err == nil {
			userstr = resp.UserStr
		}
		return err
	})
	if err != nil {
		return errorResult(err)
	}
	return ItemResult{
		Request:  map[string]any{"wechatid": p.WeChatID},
		Response: map[string]any{"userstr": userstr},
	}
}

func (d *ExecutorDeps) unbindTester(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	var p UserStrPayload
	if err := DecodeJobPayload(job, &p); err != nil {
		return errorResult(err)
	}
	appid := item.AuthorizerAppID
	err := d.auths.CallWithToken(ctx, appid, func(token string) error {
		return d.wx.UnbindTester(ctx, appid, token, p.UserStr)
	})
	if err != nil {
		return errorResult(err)
	}
	return ItemResult{
		Request:  map[string]any{"userstr": p.UserStr},
		Response: map[string]any{"ok": true},
	}
}

func (d *ExecutorDeps) setVisitStatus(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	var p ActionPayload
	if err := DecodeJobPayload(job, &p); err != nil {
		return errorResult(err)
	}
	appid := item.AuthorizerAppID
	err := d.auths.CallWithToken(ctx, appid, func(token string) error {
		return d.wx.SetVisitStatus(ctx, appid, token, p.Action)
	})
	if err != nil {
		return errorResult(err)
	}
	return ItemResult{
		Request:  p,
		Response: map[string]any{"ok": true, "action": p.Action},
	}
}

func (d *ExecutorDeps) setSupportVersion(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	var p VersionPayload
	if err := DecodeJobPayload(job, &p); err != nil {
		return errorResult(err)
	}
	appid := item.AuthorizerAppID
	err := d.auths.CallWithToken(ctx, appid, func(token string) error {
		return d.wx.SetSupportVersion(ctx, appid, token, p.Version)
	})
	if err != nil {
		return errorResult(err)
	}
	return ItemResult{
		Request:  p,
		Response: map[string]any{"ok": true, "version": p.Version},
	}
}

type PreflightResponse struct {
	LatestAudit any      `json:"latest_audit,omitempty"`
	Privacy     any      `json:"privacy,omitempty"`
	Version     any      `json:"version,omitempty"`
	Pages       any      `json:"pages,omitempty"`
	Categories  any      `json:"categories,omitempty"`
	Testers     any      `json:"testers,omitempty"`
	Domains     any      `json:"domains,omitempty"`
	Warnings    []string `json:"warnings"`
}

func (d *ExecutorDeps) preflight(ctx context.Context, job *model.Job, item *model.JobItem) ItemResult {
	appid := item.AuthorizerAppID
	resp := PreflightResponse{Warnings: make([]string, 0)}
	if v, err := d.auths.LatestAudit(ctx, appid); err != nil {
		resp.Warnings = append(resp.Warnings, "查询最新审核状态失败: "+err.Error())
	} else {
		resp.LatestAudit = v
		switch v.Status {
		case 1:
			resp.Warnings = append(resp.Warnings, "最近一次审核被拒绝: "+v.Reason)
		case 2:
			resp.Warnings = append(resp.Warnings, "已有版本正在审核中，此时提审会被跳过")
		}
	}
	if v, err := d.auths.PrivacyCheck(ctx, appid); err != nil {
		resp.Warnings = append(resp.Warnings, "隐私接口检测失败: "+err.Error())
	} else {
		resp.Privacy = v
		if len(v.WithoutAuthList) > 0 {
			resp.Warnings = append(resp.Warnings, "存在未授权隐私接口: "+fmt.Sprint(v.WithoutAuthList))
		}
		if len(v.WithoutConfList) > 0 {
			resp.Warnings = append(resp.Warnings, "存在未配置隐私接口: "+fmt.Sprint(v.WithoutConfList))
		}
	}
	if v, err := d.auths.Version(ctx, appid); err != nil {
		resp.Warnings = append(resp.Warnings, "查询版本信息失败: "+err.Error())
	} else {
		resp.Version = v
	}
	if v, err := d.auths.Pages(ctx, appid); err != nil {
		resp.Warnings = append(resp.Warnings, "查询页面列表失败: "+err.Error())
	} else {
		resp.Pages = v
	}
	if v, err := d.auths.SettingCategories(ctx, appid); err != nil {
		resp.Warnings = append(resp.Warnings, "查询已配置类目失败: "+err.Error())
	} else {
		resp.Categories = v
	}
	if v, err := d.auths.Testers(ctx, appid); err != nil {
		resp.Warnings = append(resp.Warnings, "查询体验者失败: "+err.Error())
	} else {
		resp.Testers = v
	}
	if v, err := d.auths.Domains(ctx, appid); err != nil {
		resp.Warnings = append(resp.Warnings, "查询域名失败: "+err.Error())
	} else {
		resp.Domains = v
	}
	return ItemResult{
		Request:  map[string]any{"action": "preflight"},
		Response: resp,
	}
}

func (d *ExecutorDeps) recordRelease(jobID uint, appid, version, desc, action string) {
	now := time.Now()
	record := model.ReleaseRecord{
		AuthorizerAppID: appid,
		Version:         version,
		Description:     desc,
		Action:          action,
		JobID:           &jobID,
		ReleasedAt:      now,
		CreatedAt:       now,
	}
	if err := d.db.Create(&record).Error; err != nil {
		d.log.Warn("failed to record release", "appid", appid, "error", err)
	}
}

func (d *ExecutorDeps) bumpUndoQuota(appid string) {
	now := time.Now()
	usage := model.UndoQuotaUsage{
		AuthorizerAppID: appid,
		UsageDate:       now.Format("2006-01-02"),
		Count:           1,
		UpdatedAt:       now,
	}
	_ = d.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "authorizer_app_id"}, {Name: "usage_date"}},
		DoUpdates: clause.Assignments(map[string]any{"count": gorm.Expr("undo_quota_usages.count + 1"), "updated_at": now}),
	}).Create(&usage).Error
}

func errorResult(err error) ItemResult {
	if err == nil {
		return ItemResult{}
	}
	var apiErr *wechat.APIError
	if errors.As(err, &apiErr) {
		return ItemResult{ErrorCode: strconv.Itoa(apiErr.Code), ErrorMessage: apiErr.Msg}
	}
	if errors.Is(err, ErrNeedReauth) {
		return ItemResult{ErrorCode: "NEED_REAUTH", ErrorMessage: err.Error()}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ItemResult{ErrorCode: "TIMEOUT", ErrorMessage: "调用微信接口超时"}
	}
	if errors.Is(err, context.Canceled) {
		return ItemResult{ErrorCode: "CANCELED", ErrorMessage: "任务已中断"}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ItemResult{ErrorCode: "NOT_FOUND", ErrorMessage: "小程序不存在"}
	}
	return ItemResult{ErrorCode: "INTERNAL", ErrorMessage: err.Error()}
}
