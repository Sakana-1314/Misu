package wechat

import "encoding/json"

type FuncInfo struct {
	FuncscopeCategory struct {
		ID int `json:"id"`
	} `json:"funcscope_category"`
}

type AuthorizationInfo struct {
	AuthorizerAppID        string     `json:"authorizer_appid"`
	AuthorizerAccessToken  string     `json:"authorizer_access_token"`
	ExpiresIn              int        `json:"expires_in"`
	AuthorizerRefreshToken string     `json:"authorizer_refresh_token"`
	FuncInfo               []FuncInfo `json:"func_info"`
}

type AuthorizerInfo struct {
	NickName          string `json:"nick_name"`
	HeadImg           string `json:"head_img"`
	ServiceTypeInfo   struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"service_type_info"`
	VerifyTypeInfo struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"verify_type_info"`
	UserName        string          `json:"user_name"`
	Alias           string          `json:"alias"`
	QRCodeURL       string          `json:"qrcode_url"`
	PrincipalName   string          `json:"principal_name"`
	Signature       string          `json:"signature"`
	MiniProgramInfo json.RawMessage `json:"MiniProgramInfo"`
	RegisterType    int             `json:"register_type"`
	AccountStatus   int             `json:"account_status"`
	BusinessInfo    json.RawMessage `json:"business_info"`
}

type AuthorizerInfoResponse struct {
	AuthorizerInfo    AuthorizerInfo    `json:"authorizer_info"`
	AuthorizationInfo AuthorizationInfo `json:"authorization_info"`
	ErrCode           int               `json:"errcode"`
	ErrMsg            string            `json:"errmsg"`
}

type ComponentToken struct {
	ComponentAccessToken string `json:"component_access_token"`
	ExpiresIn            int    `json:"expires_in"`
	ErrCode              int    `json:"errcode"`
	ErrMsg               string `json:"errmsg"`
}

type PreAuthCode struct {
	PreAuthCode string `json:"pre_auth_code"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type QueryAuthResponse struct {
	AuthorizationInfo AuthorizationInfo `json:"authorization_info"`
	ErrCode           int               `json:"errcode"`
	ErrMsg            string            `json:"errmsg"`
}

type AuthorizerToken struct {
	AuthorizerAccessToken  string `json:"authorizer_access_token"`
	ExpiresIn              int    `json:"expires_in"`
	AuthorizerRefreshToken string `json:"authorizer_refresh_token"`
	ErrCode                int    `json:"errcode"`
	ErrMsg                 string `json:"errmsg"`
}

type AuthorizerListEntry struct {
	AuthorizerAppID string `json:"authorizer_appid"`
	RefreshToken    string `json:"refresh_token"`
	AuthTime        int64  `json:"auth_time"`
}

type AuthorizerListResponse struct {
	TotalCount int                   `json:"total_count"`
	List       []AuthorizerListEntry `json:"list"`
	ErrCode    int                   `json:"errcode"`
	ErrMsg     string                `json:"errmsg"`
}

type TemplateCategory struct {
	FirstClass  string `json:"first_class"`
	SecondClass string `json:"second_class"`
	ThirdClass  string `json:"third_class"`
	FirstID     int    `json:"first_id"`
	SecondID    int    `json:"second_id"`
	ThirdID     int    `json:"third_id"`
	Address     string `json:"address"`
	Tag         string `json:"tag"`
	Title       string `json:"title"`
}

type CodeTemplateEntry struct {
	TemplateID             int64              `json:"template_id"`
	DraftID                int64              `json:"draft_id"`
	UserVersion            string             `json:"user_version"`
	UserDesc               string             `json:"user_desc"`
	CreateTime             int64              `json:"create_time"`
	SourceMiniprogramAppID string             `json:"source_miniprogram_appid"`
	SourceMiniprogram      string             `json:"source_miniprogram"`
	TemplateType           int                `json:"template_type"`
	CategoryList           []TemplateCategory `json:"category_list"`
	AuditScene             string             `json:"audit_scene"`
	AuditStatus            int                `json:"audit_status"`
	Reason                 string             `json:"reason"`
}

type TemplateListResponse struct {
	TemplateList []CodeTemplateEntry `json:"template_list"`
	ErrCode      int                 `json:"errcode"`
	ErrMsg       string              `json:"errmsg"`
}

type DraftEntry struct {
	DraftID                int64  `json:"draft_id"`
	UserVersion            string `json:"user_version"`
	UserDesc               string `json:"user_desc"`
	CreateTime             int64  `json:"create_time"`
	SourceMiniprogramAppID string `json:"source_miniprogram_appid"`
	SourceMiniprogram      string `json:"source_miniprogram"`
	Developer              string `json:"developer"`
}

type DraftListResponse struct {
	DraftList []DraftEntry `json:"draft_list"`
	ErrCode   int          `json:"errcode"`
	ErrMsg    string       `json:"errmsg"`
}

type VersionInfoResponse struct {
	ExpInfo struct {
		ExpTime    int64  `json:"exp_time"`
		ExpVersion string `json:"exp_version"`
		ExpDesc    string `json:"exp_desc"`
	} `json:"exp_info"`
	ReleaseInfo struct {
		ReleaseTime    int64  `json:"release_time"`
		ReleaseVersion string `json:"release_version"`
		ReleaseDesc    string `json:"release_desc"`
	} `json:"release_info"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type LatestAuditStatus struct {
	AuditID         int64  `json:"auditid"`
	Status          int    `json:"status"`
	Reason          string `json:"reason"`
	Screenshot      string `json:"screenshot"`
	UserVersion     string `json:"user_version"`
	UserDesc        string `json:"user_desc"`
	SubmitAuditTime int64  `json:"submit_audit_time"`
	ErrCode         int    `json:"errcode"`
	ErrMsg          string `json:"errmsg"`
}

type AuditStatus struct {
	Status     int    `json:"status"`
	Reason     string `json:"reason"`
	Screenshot string `json:"screenshot"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

type AuditItem struct {
	Address     string `json:"address,omitempty"`
	Tag         string `json:"tag,omitempty"`
	FirstClass  string `json:"first_class"`
	SecondClass string `json:"second_class"`
	ThirdClass  string `json:"third_class,omitempty"`
	FirstID     int    `json:"first_id"`
	SecondID    int    `json:"second_id"`
	ThirdID     int    `json:"third_id,omitempty"`
	Title       string `json:"title,omitempty"`
}

type PreviewInfo struct {
	VideoIDList []string `json:"video_id_list"`
	PicIDList   []string `json:"pic_id_list"`
}

type UGCDeclare struct {
	Scene          []int  `json:"scene,omitempty"`
	Method         []int  `json:"method,omitempty"`
	OtherSceneDesc string `json:"other_scene_desc,omitempty"`
	HasAuditTeam   int    `json:"has_audit_team,omitempty"`
	AuditDesc      string `json:"audit_desc,omitempty"`
}

type SubmitAuditRequest struct {
	ItemList        []AuditItem  `json:"item_list"`
	FeedbackInfo    string       `json:"feedback_info,omitempty"`
	FeedbackStuff   string       `json:"feedback_stuff,omitempty"`
	VersionDesc     string       `json:"version_desc,omitempty"`
	PreviewInfo     *PreviewInfo `json:"preview_info,omitempty"`
	UGCDeclare      *UGCDeclare  `json:"ugc_declare,omitempty"`
	PrivacyAPINotUse bool        `json:"privacy_api_not_use,omitempty"`
	OrderPath       string       `json:"order_path,omitempty"`
}

type SubmitAuditResponse struct {
	AuditID int64  `json:"auditid"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type CommitRequest struct {
	TemplateID int64  `json:"template_id"`
	ExtJSON    string `json:"ext_json"`
	UserVersion string `json:"user_version"`
	UserDesc   string `json:"user_desc"`
}

type DomainRequest struct {
	Action          string   `json:"action"`
	RequestDomain   []string `json:"requestdomain,omitempty"`
	WSRequestDomain []string `json:"wsrequestdomain,omitempty"`
	UploadDomain    []string `json:"uploaddomain,omitempty"`
	DownloadDomain  []string `json:"downloaddomain,omitempty"`
	UDPDomain       []string `json:"udpdomain,omitempty"`
	TCPDomain       []string `json:"tcpdomain,omitempty"`
}

type DomainResponse struct {
	RequestDomain          []string `json:"requestdomain"`
	WSRequestDomain        []string `json:"wsrequestdomain"`
	UploadDomain           []string `json:"uploaddomain"`
	DownloadDomain         []string `json:"downloaddomain"`
	UDPDomain              []string `json:"udpdomain"`
	TCPDomain              []string `json:"tcpdomain"`
	InvalidRequestDomain   []string `json:"invalid_requestdomain"`
	InvalidWSRequestDomain []string `json:"invalid_wsrequestdomain"`
	InvalidUploadDomain    []string `json:"invalid_uploaddomain"`
	InvalidDownloadDomain  []string `json:"invalid_downloaddomain"`
	InvalidUDPDomain       []string `json:"invalid_udpdomain"`
	InvalidTCPDomain       []string `json:"invalid_tcpdomain"`
	NoICPDomain            []string `json:"no_icp_domain"`
	ErrCode                int      `json:"errcode"`
	ErrMsg                 string   `json:"errmsg"`
}

type GrayReleaseRequest struct {
	GrayPercentage          int  `json:"gray_percentage"`
	SupportDebugerFirst     bool `json:"support_debuger_first"`
	SupportExperiencerFirst bool `json:"support_experiencer_first"`
}

type GrayPlan struct {
	Status                  int   `json:"status"`
	CreateTimestamp         int64 `json:"create_timestamp"`
	GrayPercentage          int   `json:"gray_percentage"`
	SupportDebugerFirst     bool  `json:"support_debuger_first"`
	SupportExperiencerFirst bool  `json:"support_experiencer_first"`
}

type GrayReleasePlanResponse struct {
	GrayReleasePlan GrayPlan `json:"gray_release_plan"`
	ErrCode         int      `json:"errcode"`
	ErrMsg          string   `json:"errmsg"`
}

type HistoryVersion struct {
	AppVersion  string `json:"app_version"`
	UserVersion string `json:"user_version"`
	UserDesc    string `json:"user_desc"`
	CommitTime  int64  `json:"commit_time"`
}

type HistoryVersionResponse struct {
	VersionList []HistoryVersion `json:"version_list"`
	ErrCode     int              `json:"errcode"`
	ErrMsg      string           `json:"errmsg"`
}

type TesterEntry struct {
	UserStr string `json:"userstr"`
}

type TesterListResponse struct {
	Members []TesterEntry `json:"members"`
	ErrCode int           `json:"errcode"`
	ErrMsg  string        `json:"errmsg"`
}

type BindTesterResponse struct {
	UserStr string `json:"userstr"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type Category struct {
	ID            int             `json:"id"`
	Name          string          `json:"name"`
	Level         int             `json:"level"`
	Father        int             `json:"father"`
	Children      []int           `json:"children"`
	SensitiveType int             `json:"sensitive_type"`
	Qualify       json.RawMessage `json:"qualify"`
}

type AllCategoriesResponse struct {
	CategoriesList struct {
		Categories []Category `json:"categories"`
	} `json:"categories_list"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type SettingCategory struct {
	First       int    `json:"first"`
	FirstName   string `json:"first_name"`
	Second      int    `json:"second"`
	SecondName  string `json:"second_name"`
	AuditStatus int    `json:"audit_status"`
	AuditReason string `json:"audit_reason"`
}

type SettingCategoriesResponse struct {
	Categories    []SettingCategory `json:"categories"`
	Limit         int               `json:"limit"`
	Quota         int               `json:"quota"`
	CategoryLimit int               `json:"category_limit"`
	ErrCode       int               `json:"errcode"`
	ErrMsg        string            `json:"errmsg"`
}

type PrivacyCheckResponse struct {
	WithoutAuthList []string `json:"without_auth_list"`
	WithoutConfList []string `json:"without_conf_list"`
	ErrCode         int      `json:"errcode"`
	ErrMsg          string   `json:"errmsg"`
}

type UploadMediaResponse struct {
	MediaID string `json:"mediaid"`
	Type    string `json:"type"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type VisitStatusResponse struct {
	Status  int    `json:"status"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type PageListResponse struct {
	PageList []string `json:"page_list"`
	ErrCode  int      `json:"errcode"`
	ErrMsg   string   `json:"errmsg"`
}

type LineColor struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

type UnlimitedQRCodeRequest struct {
	Scene      string     `json:"scene"`
	Page       string     `json:"page,omitempty"`
	Width      int        `json:"width,omitempty"`
	AutoColor  bool       `json:"auto_color,omitempty"`
	LineColor  *LineColor `json:"line_color,omitempty"`
	IsHyaline  bool       `json:"is_hyaline,omitempty"`
	CheckPath  bool       `json:"check_path,omitempty"`
	EnvVersion string     `json:"env_version,omitempty"`
}
