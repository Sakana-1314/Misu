package model

import (
	"time"

	"gorm.io/datatypes"
)

const (
	JobQueued   = "queued"
	JobRunning  = "running"
	JobSuccess  = "success"
	JobPartial  = "partial"
	JobFailed   = "failed"
	JobCanceled = "canceled"

	ItemPending  = "pending"
	ItemRunning  = "running"
	ItemSuccess  = "success"
	ItemFailed   = "failed"
	ItemSkipped  = "skipped"
	ItemCanceled = "canceled"

	AuthorizerActive       = "active"
	AuthorizerUnauthorized = "unauthorized"
)

// PlatformState is a singleton row holding the component verify ticket and
// component_access_token. Both secrets are encrypted at rest.
type PlatformState struct {
	ID                      uint       `gorm:"primaryKey"`
	ComponentAppID          string     `gorm:"size:64"`
	VerifyTicketEnc         string     `gorm:"type:text"`
	TicketCreatedAt         *time.Time
	TicketReceivedAt        *time.Time
	ComponentTokenEnc       string `gorm:"type:text"`
	ComponentTokenExpiresAt *time.Time
	LastPushOKAt            *time.Time
	LastPushError           string `gorm:"type:text"`
	LastTokenError          string `gorm:"type:text"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type Authorizer struct {
	ID                   uint    `gorm:"primaryKey" json:"id"`
	AppID                string  `gorm:"uniqueIndex;size:64" json:"appid"`
	NickName             string  `gorm:"size:255" json:"nick_name"`
	HeadImg              string  `gorm:"type:text" json:"head_img"`
	PrincipalName        string  `gorm:"size:255" json:"principal_name"`
	UserName             string  `gorm:"size:128" json:"user_name"`
	Alias                string  `gorm:"size:128" json:"alias"`
	QRCodeURL            string  `gorm:"type:text" json:"qrcode_url"`
	Signature            string  `gorm:"type:text" json:"signature"`
	ServiceTypeID        int     `json:"service_type_id"`
	ServiceTypeName      string  `gorm:"size:64" json:"service_type_name"`
	VerifyTypeID         int     `json:"verify_type_id"`
	VerifyTypeName       string  `gorm:"size:64" json:"verify_type_name"`
	AccountStatus        int     `json:"account_status"`
	RegisterType         int     `json:"register_type"`
	MiniProgramInfo      datatypes.JSON `gorm:"type:jsonb" json:"miniprogram_info"`
	FuncInfo             datatypes.JSON `gorm:"type:jsonb" json:"func_info"`
	RefreshTokenEnc      string  `gorm:"type:text" json:"-"`
	AccessTokenEnc       string  `gorm:"type:text" json:"-"`
	AccessTokenExpiresAt *time.Time `json:"access_token_expires_at"`
	AuthTime             *time.Time `json:"auth_time"`
	Status               string  `gorm:"index;size:20;default:active" json:"status"`
	LastSyncedAt         *time.Time `json:"last_synced_at"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (a *Authorizer) IsActive() bool { return a.Status == AuthorizerActive }

type CodeTemplate struct {
	ID                     uint   `gorm:"primaryKey" json:"id"`
	TemplateID             int64  `gorm:"uniqueIndex" json:"template_id"`
	DraftID                int64  `json:"draft_id"`
	UserVersion            string `gorm:"size:128" json:"user_version"`
	UserDesc               string `gorm:"type:text" json:"user_desc"`
	CreateTime             int64  `json:"create_time"`
	SourceMiniprogramAppID string `gorm:"size:64" json:"source_miniprogram_appid"`
	SourceMiniprogram      string `gorm:"size:255" json:"source_miniprogram"`
	TemplateType           int    `json:"template_type"`
	CategoryList           datatypes.JSON `gorm:"type:jsonb" json:"category_list"`
	AuditScene             string `gorm:"type:text" json:"audit_scene"`
	AuditStatus            int    `json:"audit_status"`
	Reason                 string `gorm:"type:text" json:"reason"`
	SyncedAt               time.Time `json:"synced_at"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type AuditAsset struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Filename  string    `gorm:"size:255" json:"filename"`
	Mime      string    `gorm:"size:128" json:"mime"`
	Size      int64     `json:"size"`
	Data      []byte    `gorm:"type:bytea" json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditRecord struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	AuthorizerAppID  string `gorm:"index:idx_audit_app;size:64" json:"authorizer_appid"`
	AuditID          int64  `gorm:"index:idx_audit_app" json:"audit_id"`
	Status           int    `gorm:"index" json:"status"`
	Reason           string `gorm:"type:text" json:"reason"`
	Screenshot       string `gorm:"type:text" json:"screenshot"`
	UserVersion      string `gorm:"size:128" json:"user_version"`
	UserDesc         string `gorm:"type:text" json:"user_desc"`
	SubmitAuditTime  *time.Time `json:"submit_audit_time"`
	SubmittedAt      *time.Time `json:"submitted_at"`
	LastCheckedAt    *time.Time `json:"last_checked_at"`
	AutoRelease      bool   `json:"auto_release"`
	AutoReleaseJobID *uint  `json:"auto_release_job_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type ReleaseRecord struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	AuthorizerAppID string `gorm:"index;size:64" json:"authorizer_appid"`
	Version         string `gorm:"size:128" json:"version"`
	Description     string `gorm:"type:text" json:"description"`
	Action          string `gorm:"size:32" json:"action"`
	JobID           *uint  `json:"job_id"`
	ReleasedAt      time.Time `json:"released_at"`
	CreatedAt       time.Time `json:"created_at"`
}

type Job struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	Type            string `gorm:"index;size:40" json:"type"`
	Title           string `gorm:"size:255" json:"title"`
	Status          string `gorm:"index;size:20" json:"status"`
	Payload         datatypes.JSON `gorm:"type:jsonb" json:"payload"`
	Total           int    `json:"total"`
	SuccessCount    int    `json:"success_count"`
	FailedCount     int    `json:"failed_count"`
	SkippedCount    int    `json:"skipped_count"`
	CreatedBy       string `gorm:"size:64" json:"created_by"`
	Error           string `gorm:"type:text" json:"error"`
	CancelRequested bool   `json:"cancel_requested"`
	SourceAuditID   *int64 `gorm:"index" json:"source_audit_id"`
	StartedAt       *time.Time `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type JobItem struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	JobID           uint   `gorm:"index;size:20" json:"job_id"`
	AuthorizerAppID string `gorm:"index;size:64" json:"authorizer_appid"`
	Status          string `gorm:"index;size:20" json:"status"`
	Attempts        int    `json:"attempts"`
	Request         datatypes.JSON `gorm:"type:jsonb" json:"request"`
	Response        datatypes.JSON `gorm:"type:jsonb" json:"response"`
	ErrorCode       string `gorm:"size:64" json:"error_code"`
	ErrorMessage    string `gorm:"type:text" json:"error_message"`
	AuditID         int64  `json:"audit_id"`
	StartedAt       *time.Time `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type APICallLog struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Endpoint  string `gorm:"size:255;index" json:"endpoint"`
	AppID     string `gorm:"size:64;index" json:"appid"`
	Method    string `gorm:"size:10" json:"method"`
	Errcode   int    `json:"errcode"`
	Errmsg    string `gorm:"type:text" json:"errmsg"`
	LatencyMS int    `json:"latency_ms"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

type OperationLog struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Actor     string `gorm:"size:64;index" json:"actor"`
	Action    string `gorm:"size:64;index" json:"action"`
	Target    string `gorm:"size:128;index" json:"target"`
	Detail    datatypes.JSON `gorm:"type:jsonb" json:"detail"`
	IP        string `gorm:"size:64" json:"ip"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

type CallbackEvent struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	InfoType        string `gorm:"size:64;index" json:"info_type"`
	AuthorizerAppID string `gorm:"size:64;index" json:"authorizer_appid"`
	AppID           string `gorm:"size:64" json:"appid"`
	EventTime       *time.Time `json:"event_time"`
	Payload         datatypes.JSON `gorm:"type:jsonb" json:"payload"`
	DedupKey        string `gorm:"uniqueIndex;size:128" json:"-"`
	CreatedAt       time.Time `gorm:"index" json:"created_at"`
}

type UndoQuotaUsage struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	AuthorizerAppID string `gorm:"uniqueIndex:idx_undo_app_date;size:64" json:"authorizer_appid"`
	UsageDate       string `gorm:"uniqueIndex:idx_undo_app_date;size:10" json:"usage_date"`
	Count           int    `json:"count"`
	UpdatedAt       time.Time `json:"updated_at"`
}
