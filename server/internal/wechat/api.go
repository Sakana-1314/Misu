package wechat

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

const componentTokenKey = "component_access_token"
const accessTokenKey = "access_token"

func (c *Client) GetComponentToken(ctx context.Context, appid, secret, ticket string) (*ComponentToken, error) {
	body := map[string]string{
		"component_appid":         appid,
		"component_appsecret":     secret,
		"component_verify_ticket": ticket,
	}
	var out ComponentToken
	err := c.doJSON(ctx, http.MethodPost, "/cgi-bin/component/api_component_token", nil, body, &out, "", true)
	return &out, err
}

func (c *Client) GetPreAuthCode(ctx context.Context, componentToken, componentAppID string) (*PreAuthCode, error) {
	body := map[string]string{"component_appid": componentAppID}
	var out PreAuthCode
	err := c.doJSON(ctx, http.MethodPost, "/cgi-bin/component/api_create_preauthcode", tokenQuery(componentTokenKey, componentToken), body, &out, "", true)
	return &out, err
}

func (c *Client) QueryAuth(ctx context.Context, componentToken, componentAppID, code string) (*QueryAuthResponse, error) {
	body := map[string]string{
		"component_appid":    componentAppID,
		"authorization_code": code,
	}
	var out QueryAuthResponse
	// The authorization code is one-time use, so never retry this call.
	err := c.doJSON(ctx, http.MethodPost, "/cgi-bin/component/api_query_auth", tokenQuery(componentTokenKey, componentToken), body, &out, "", false)
	return &out, err
}

func (c *Client) RefreshAuthorizerToken(ctx context.Context, componentToken, componentAppID, authorizerAppID, refreshToken string) (*AuthorizerToken, error) {
	body := map[string]string{
		"component_appid":          componentAppID,
		"authorizer_appid":         authorizerAppID,
		"authorizer_refresh_token": refreshToken,
	}
	var out AuthorizerToken
	err := c.doJSON(ctx, http.MethodPost, "/cgi-bin/component/api_authorizer_token", tokenQuery(componentTokenKey, componentToken), body, &out, "", true)
	return &out, err
}

func (c *Client) GetAuthorizerInfo(ctx context.Context, componentToken, componentAppID, authorizerAppID string) (*AuthorizerInfoResponse, error) {
	body := map[string]string{
		"component_appid":  componentAppID,
		"authorizer_appid": authorizerAppID,
	}
	var out AuthorizerInfoResponse
	err := c.doJSON(ctx, http.MethodPost, "/cgi-bin/component/api_get_authorizer_info", tokenQuery(componentTokenKey, componentToken), body, &out, "", true)
	return &out, err
}

func (c *Client) GetAuthorizerList(ctx context.Context, componentToken, componentAppID string, offset, count int) (*AuthorizerListResponse, error) {
	body := map[string]any{
		"component_appid": componentAppID,
		"offset":          offset,
		"count":           count,
	}
	var out AuthorizerListResponse
	err := c.doJSON(ctx, http.MethodPost, "/cgi-bin/component/api_get_authorizer_list", tokenQuery(componentTokenKey, componentToken), body, &out, "", true)
	return &out, err
}

func (c *Client) StartPushTicket(ctx context.Context, appid, secret string) error {
	body := map[string]string{
		"component_appid":  appid,
		"component_secret": secret,
	}
	return c.doJSON(ctx, http.MethodPost, "/cgi-bin/component/api_start_push_ticket", nil, body, nil, "", true)
}

func (c *Client) GetVersionInfo(ctx context.Context, appid, token string) (*VersionInfoResponse, error) {
	var out VersionInfoResponse
	err := c.doJSON(ctx, http.MethodPost, "/wxa/getversioninfo", tokenQuery(accessTokenKey, token), map[string]any{}, &out, appid, true)
	return &out, err
}

func (c *Client) GetLatestAuditStatus(ctx context.Context, appid, token string) (*LatestAuditStatus, error) {
	var out LatestAuditStatus
	err := c.doJSON(ctx, http.MethodGet, "/wxa/get_latest_auditstatus", tokenQuery(accessTokenKey, token), nil, &out, appid, true)
	return &out, err
}

func (c *Client) GetAuditStatus(ctx context.Context, appid, token string, auditID int64) (*AuditStatus, error) {
	var out AuditStatus
	err := c.doJSON(ctx, http.MethodPost, "/wxa/get_auditstatus", tokenQuery(accessTokenKey, token), map[string]int64{"auditid": auditID}, &out, appid, true)
	return &out, err
}

func (c *Client) Commit(ctx context.Context, appid, token string, req CommitRequest) error {
	return c.doJSON(ctx, http.MethodPost, "/wxa/commit", tokenQuery(accessTokenKey, token), req, nil, appid, false)
}

func (c *Client) SubmitAudit(ctx context.Context, appid, token string, req SubmitAuditRequest) (*SubmitAuditResponse, error) {
	var out SubmitAuditResponse
	err := c.doJSON(ctx, http.MethodPost, "/wxa/submit_audit", tokenQuery(accessTokenKey, token), req, &out, appid, false)
	return &out, err
}

func (c *Client) UndoAudit(ctx context.Context, appid, token string) error {
	return c.doJSON(ctx, http.MethodGet, "/wxa/undocodeaudit", tokenQuery(accessTokenKey, token), nil, nil, appid, false)
}

func (c *Client) Release(ctx context.Context, appid, token string) error {
	return c.doJSON(ctx, http.MethodPost, "/wxa/release", tokenQuery(accessTokenKey, token), map[string]any{}, nil, appid, false)
}

func (c *Client) GetHistoryVersions(ctx context.Context, appid, token string) (*HistoryVersionResponse, error) {
	q := tokenQuery(accessTokenKey, token)
	q.Set("action", "get_history_version")
	var out HistoryVersionResponse
	err := c.doJSON(ctx, http.MethodGet, "/wxa/revertcoderelease", q, nil, &out, appid, true)
	return &out, err
}

func (c *Client) RevertCodeRelease(ctx context.Context, appid, token, appVersion string) error {
	q := tokenQuery(accessTokenKey, token)
	if appVersion != "" {
		q.Set("app_version", appVersion)
	}
	return c.doJSON(ctx, http.MethodGet, "/wxa/revertcoderelease", q, nil, nil, appid, false)
}

func (c *Client) GrayRelease(ctx context.Context, appid, token string, req GrayReleaseRequest) error {
	return c.doJSON(ctx, http.MethodPost, "/wxa/grayrelease", tokenQuery(accessTokenKey, token), req, nil, appid, false)
}

func (c *Client) GetGrayReleasePlan(ctx context.Context, appid, token string) (*GrayReleasePlanResponse, error) {
	var out GrayReleasePlanResponse
	err := c.doJSON(ctx, http.MethodGet, "/wxa/getgrayreleaseplan", tokenQuery(accessTokenKey, token), nil, &out, appid, true)
	return &out, err
}

func (c *Client) RevertGrayRelease(ctx context.Context, appid, token string) error {
	return c.doJSON(ctx, http.MethodGet, "/wxa/revertgrayrelease", tokenQuery(accessTokenKey, token), nil, nil, appid, false)
}

func (c *Client) ModifyDomain(ctx context.Context, appid, token string, req DomainRequest) (*DomainResponse, error) {
	var out DomainResponse
	err := c.doJSON(ctx, http.MethodPost, "/wxa/modify_domain", tokenQuery(accessTokenKey, token), req, &out, appid, false)
	return &out, err
}

func (c *Client) BindTester(ctx context.Context, appid, token, wechatID string) (*BindTesterResponse, error) {
	var out BindTesterResponse
	err := c.doJSON(ctx, http.MethodPost, "/wxa/bind_tester", tokenQuery(accessTokenKey, token), map[string]string{"wechatid": wechatID}, &out, appid, false)
	return &out, err
}

func (c *Client) UnbindTester(ctx context.Context, appid, token, userstr string) error {
	return c.doJSON(ctx, http.MethodPost, "/wxa/unbind_tester", tokenQuery(accessTokenKey, token), map[string]string{"userstr": userstr}, nil, appid, false)
}

func (c *Client) GetTesters(ctx context.Context, appid, token string) ([]string, error) {
	var out TesterListResponse
	err := c.doJSON(ctx, http.MethodPost, "/wxa/memberauth", tokenQuery(accessTokenKey, token), map[string]string{"action": "get_experiencer"}, &out, appid, true)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(out.Members))
	for _, m := range out.Members {
		result = append(result, m.UserStr)
	}
	return result, nil
}

func (c *Client) GetPages(ctx context.Context, appid, token string) ([]string, error) {
	var out PageListResponse
	err := c.doJSON(ctx, http.MethodGet, "/wxa/get_page", tokenQuery(accessTokenKey, token), nil, &out, appid, true)
	return out.PageList, err
}

func (c *Client) GetAllCategories(ctx context.Context, appid, token string) (*AllCategoriesResponse, error) {
	var out AllCategoriesResponse
	err := c.doJSON(ctx, http.MethodGet, "/cgi-bin/wxopen/getallcategories", tokenQuery(accessTokenKey, token), nil, &out, appid, true)
	return &out, err
}

func (c *Client) GetSettingCategories(ctx context.Context, appid, token string) (*SettingCategoriesResponse, error) {
	var out SettingCategoriesResponse
	err := c.doJSON(ctx, http.MethodGet, "/cgi-bin/wxopen/getcategory", tokenQuery(accessTokenKey, token), nil, &out, appid, true)
	return &out, err
}

func (c *Client) GetPrivacyCheck(ctx context.Context, appid, token string) (*PrivacyCheckResponse, error) {
	var out PrivacyCheckResponse
	err := c.doJSON(ctx, http.MethodGet, "/wxa/security/get_code_privacy_info", tokenQuery(accessTokenKey, token), nil, &out, appid, true)
	return &out, err
}

func (c *Client) SetVisitStatus(ctx context.Context, appid, token, action string) error {
	return c.doJSON(ctx, http.MethodPost, "/wxa/change_visitstatus", tokenQuery(accessTokenKey, token), map[string]string{"action": action}, nil, appid, false)
}

func (c *Client) GetVisitStatus(ctx context.Context, appid, token string) (*VisitStatusResponse, error) {
	var out VisitStatusResponse
	err := c.doJSON(ctx, http.MethodPost, "/wxa/getvisitstatus", tokenQuery(accessTokenKey, token), map[string]any{}, &out, appid, true)
	return &out, err
}

func (c *Client) SetSupportVersion(ctx context.Context, appid, token, version string) error {
	return c.doJSON(ctx, http.MethodPost, "/cgi-bin/wxopen/setweappsupportversion", tokenQuery(accessTokenKey, token), map[string]string{"version": version}, nil, appid, false)
}

func (c *Client) GetTrialQRCode(ctx context.Context, appid, token, path string) ([]byte, string, error) {
	q := tokenQuery(accessTokenKey, token)
	if path != "" {
		q.Set("path", path)
	}
	return c.doBinary(ctx, http.MethodGet, "/wxa/get_qrcode", q, nil, appid, true)
}

func (c *Client) GetUnlimitedQRCode(ctx context.Context, appid, token string, req UnlimitedQRCodeRequest) ([]byte, string, error) {
	return c.doBinary(ctx, http.MethodPost, "/wxa/getwxacodeunlimit", tokenQuery(accessTokenKey, token), req, appid, true)
}

func (c *Client) UploadAuditMedia(ctx context.Context, appid, token, filename string, data []byte) (*UploadMediaResponse, error) {
	var out UploadMediaResponse
	err := c.doMultipart(ctx, "/wxa/uploadmedia", tokenQuery(accessTokenKey, token), "media", filename, data, &out, appid)
	return &out, err
}

func (c *Client) GetMedia(ctx context.Context, appid, token, mediaID string) ([]byte, string, error) {
	q := tokenQuery(accessTokenKey, token)
	q.Set("media_id", mediaID)
	return c.doBinary(ctx, http.MethodGet, "/cgi-bin/media/get", q, nil, appid, true)
}

func (c *Client) GetTemplateList(ctx context.Context, componentToken string, templateType *int) (*TemplateListResponse, error) {
	q := tokenQuery(accessTokenKey, componentToken)
	if templateType != nil {
		q.Set("template_type", strconv.Itoa(*templateType))
	}
	var out TemplateListResponse
	err := c.doJSON(ctx, http.MethodGet, "/wxa/gettemplatelist", q, nil, &out, "", true)
	return &out, err
}

func (c *Client) GetDraftList(ctx context.Context, componentToken string) (*DraftListResponse, error) {
	var out DraftListResponse
	err := c.doJSON(ctx, http.MethodGet, "/wxa/gettemplatedraftlist", tokenQuery(accessTokenKey, componentToken), nil, &out, "", true)
	return &out, err
}

func (c *Client) AddToTemplate(ctx context.Context, componentToken string, draftID int64, templateType int) error {
	body := map[string]any{"draft_id": draftID, "template_type": templateType}
	return c.doJSON(ctx, http.MethodPost, "/wxa/addtotemplate", tokenQuery(accessTokenKey, componentToken), body, nil, "", false)
}

func (c *Client) DeleteTemplate(ctx context.Context, componentToken string, templateID int64) error {
	body := map[string]int64{"template_id": templateID}
	return c.doJSON(ctx, http.MethodPost, "/wxa/deletetemplate", tokenQuery(accessTokenKey, componentToken), body, nil, "", false)
}

// BuildTokenQuery is exported for callers that need to add extra parameters.
func BuildTokenQuery(key, token string) url.Values {
	return tokenQuery(key, token)
}
