// Package mpweb drives the official MP admin console (mp.weixin.qq.com) through
// a scanned-login session. It complements the third-party platform flow: it
// works for accounts the operator personally administers, without the platform
// authorization grants.
package mpweb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	baseURL = "https://mp.weixin.qq.com"
	// Public login appid used by the console itself.
	loginAppID = "wx16f47d3640058686"
	userAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
)

var (
	// ErrNotLoggedIn surfaces when a console session expired or was dropped.
	ErrNotLoggedIn = errors.New("网页版尚未登录")
	tokenRegexp    = regexp.MustCompile(`token=(\d+)`)
	uuidRegexp     = regexp.MustCompile(`"uuid":"([0-9a-f]{32})"`)
)

// ScanState is returned to the browser while waiting for the operator to scan.
type ScanState struct {
	Status     string `json:"status"` // waiting | scanned | confirmed | expired
	NickName   string `json:"nick_name,omitempty"`
	Token      string `json:"-"`
	Avatar     string `json:"avatar,omitempty"`
	AccountCnt int    `json:"account_count"`
}

// Session is a logged-in console session bound to one operator.
type Session struct {
	mu      sync.Mutex
	client  *http.Client
	uuid    string
	qrJPEG  []byte
	token   string
	created time.Time
	state   ScanState

	// currentAppID tracks which mini program the console session is viewing.
	currentAppID string
}

// Client manages the login handshake. It keeps one pending login per instance;
// the app only ever has a single operator.
type Client struct {
	mu       sync.Mutex
	login    *Session
	current  *Session
	muSwitch sync.Mutex
}

func New() *Client {
	return &Client{}
}

func newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 25 * time.Second}
}

// StartLogin asks the console for a fresh QR code.
func (c *Client) StartLogin(ctx context.Context) (*ScanState, []byte, error) {
	session, err := c.newLoginSession(ctx)
	if err != nil {
		return nil, nil, err
	}
	c.mu.Lock()
	c.login = session
	c.mu.Unlock()
	return &session.state, session.qrJPEG, nil
}

func (c *Client) newLoginSession(ctx context.Context) (*Session, error) {
	session := &Session{client: newHTTPClient(), created: time.Now(), state: ScanState{Status: "waiting"}}
	// Prime the cookies (ua_id etc.) before starting the login.
	if _, err := session.get(ctx, baseURL+"/", nil); err != nil {
		return nil, err
	}
	form := url.Values{
		"userlang":     {"zh_CN"},
		"redirect_url": {baseURL + "/"},
		"login_type":   {"3"},
		"sessionid":    {randomHex(8)},
	}
	body, err := session.postForm(ctx, "/cgi-bin/bizlogin?action=startlogin", form)
	if err != nil {
		return nil, err
	}
	match := uuidRegexp.FindSubmatch(body)
	if len(match) == 0 {
		return nil, fmt.Errorf("无法创建登录会话: %s", truncate(string(body), 160))
	}
	session.uuid = string(match[1])

	qrURL := fmt.Sprintf("%s/cgi-bin/scanloginqrcode?action=getqrcode&random=%d&login_appid=%s",
		baseURL, time.Now().UnixMilli(), loginAppID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, qrURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", baseURL+"/")
	resp, err := session.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	jpeg, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if len(jpeg) < 128 {
		return nil, errors.New("获取登录二维码失败")
	}
	session.qrJPEG = jpeg
	return session, nil
}

// PollLogin advances the pending login. It returns the current state; when the
// operator has confirmed on the phone the session becomes the active one.
func (c *Client) PollLogin(ctx context.Context) (*ScanState, error) {
	c.mu.Lock()
	session := c.login
	c.mu.Unlock()
	if session == nil {
		return nil, errors.New("没有进行中的扫码登录，请刷新二维码")
	}
	body, err := session.get(ctx, baseURL+"/cgi-bin/scanloginqrcode?action=ask", nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Status     int `json:"status"`
		AcctSize   int `json:"acct_size"`
		BaseResp   struct {
			Ret    int    `json:"ret"`
			ErrMsg string `json:"err_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析扫码状态失败: %w", err)
	}
	switch payload.Status {
	case 1:
		// Scanned and confirmed: finish the console login.
		state, err := c.finishLogin(ctx, session)
		if err != nil {
			return nil, err
		}
		return state, nil
	case 2, 3:
		session.state = ScanState{Status: "expired"}
		return &session.state, nil
	case 4, 5, 6:
		session.state.Status = "scanned"
		session.state.AccountCnt = payload.AcctSize
		return &session.state, nil
	default:
		session.state.Status = "waiting"
		return &session.state, nil
	}
}

func (c *Client) finishLogin(ctx context.Context, session *Session) (*ScanState, error) {
	form := url.Values{
		"userlang":     {"zh_CN"},
		"redirect_url": {baseURL + "/"},
		"login_type":   {"3"},
		"cookie_forbidden": {"0"},
		"cookie_cleaned":   {"0"},
		"plugin_used":      {"0"},
	}
	body, err := session.postForm(ctx, "/cgi-bin/bizlogin?action=login", form)
	if err != nil {
		return nil, err
	}
	var payload struct {
		RedirectURL string `json:"redirect_url"`
		BaseResp    struct {
			Ret    int    `json:"ret"`
			ErrMsg string `json:"err_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析登录结果失败: %w", err)
	}
	token := ""
	if match := tokenRegexp.FindStringSubmatch(payload.RedirectURL); len(match) > 1 {
		token = match[1]
	}
	if token == "" && payload.RedirectURL == "" {
		return nil, fmt.Errorf("登录失败: %s", payload.BaseResp.ErrMsg)
	}
	session.mu.Lock()
	session.token = token
	session.state = ScanState{Status: "confirmed", AccountCnt: 0}
	session.mu.Unlock()

	c.mu.Lock()
	c.current = session
	c.login = nil
	c.mu.Unlock()

	// Identify the operator and prime the account list.
	if info, err := c.Profile(ctx); err == nil {
		session.mu.Lock()
		session.state.NickName = info.NickName
		session.state.Avatar = info.Avatar
		session.mu.Unlock()
	}
	if accounts, err := c.Accounts(ctx); err == nil {
		session.mu.Lock()
		session.state.AccountCnt = len(accounts)
		session.mu.Unlock()
	}
	return &session.state, nil
}

// QRCode returns the pending login QR image.
func (c *Client) QRCode() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.login == nil {
		return nil, errors.New("二维码已失效，请刷新")
	}
	return c.login.qrJPEG, nil
}

// Connected reports whether an operator session exists.
func (c *Client) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current != nil && c.current.token != ""
}

// Logout drops the current session.
func (c *Client) Logout() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = nil
	c.login = nil
}

// Profile describes the logged-in console operator.
type Profile struct {
	NickName string `json:"nick_name"`
	Avatar   string `json:"avatar"`
	Token    string `json:"-"`
}

func (c *Client) session() (*Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil || c.current.token == "" {
		return nil, ErrNotLoggedIn
	}
	return c.current, nil
}

// Account is one mini program visible to the operator.
type Account struct {
	AppID    string `json:"appid"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
	UserName string `json:"user_name"`
	Type     int    `json:"type"`
}

func (c *Client) Profile(ctx context.Context) (*Profile, error) {
	session, err := c.session()
	if err != nil {
		return nil, err
	}
	body, err := c.cgi(ctx, session, nil, "getNavInfo")
	if err != nil {
		return nil, err
	}
	var payload struct {
		NickName string `json:"nick_name"`
	}
	_ = json.Unmarshal(body, &payload)
	profile := &Profile{Token: session.token}
	// Console header data carries the operator identity; fall back to a page fetch.
	if html, err := session.get(ctx, fmt.Sprintf("%s/cgi-bin/home?t=home/index&lang=zh_CN&token=%s", baseURL, session.token), nil); err == nil {
		if m := regexp.MustCompile(`"nick_name_decode":"([^"]*)"`).FindSubmatch(html); len(m) > 1 {
			profile.NickName = string(m[1])
		}
		if m := regexp.MustCompile(`"head_img":"([^"]*)"`).FindSubmatch(html); len(m) > 1 {
			profile.Avatar = strings.ReplaceAll(string(m[1]), `\/`, `/`)
		}
	}
	return profile, nil
}

func (c *Client) Accounts(ctx context.Context) ([]Account, error) {
	session, err := c.session()
	if err != nil {
		return nil, err
	}
	body, err := c.cgi(ctx, session, nil, "getWxaList")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Ret  int `json:"ret"`
		List []struct {
			AppID    string `json:"appid"`
			Name     string `json:"app_name"`
			HeadImg  string `json:"app_headimg"`
			UserName string `json:"username"`
			Type     int    `json:"type"`
		} `json:"wax_list"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析小程序列表失败: %w", err)
	}
	out := make([]Account, 0, len(payload.List))
	for _, item := range payload.List {
		out = append(out, Account{
			AppID:    item.AppID,
			Name:     item.Name,
			Avatar:   strings.ReplaceAll(item.HeadImg, "http://", "https://"),
			UserName: item.UserName,
			Type:     item.Type,
		})
	}
	return out, nil
}

// Switch makes appid the console's current account.
func (c *Client) Switch(ctx context.Context, appid string) error {
	session, err := c.session()
	if err != nil {
		return err
	}
	session.mu.Lock()
	if session.currentAppID == appid {
		session.mu.Unlock()
		return nil
	}
	session.mu.Unlock()

	body, err := c.cgiPost(ctx, session, fmt.Sprintf("switchAccount?token=%s&lang=zh_CN", session.token),
		map[string]string{"appid": appid})
	if err != nil {
		return err
	}
	var payload struct {
		Ret int `json:"ret"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("切换小程序失败: %w", err)
	}
	if payload.Ret != 0 {
		return fmt.Errorf("切换小程序失败（ret=%d）", payload.Ret)
	}
	session.mu.Lock()
	session.currentAppID = appid
	session.mu.Unlock()
	return nil
}

// VersionState summarizes the console's code page for a mini program.
type VersionState struct {
	AppID           string `json:"appid"`
	ExperienceVer   string `json:"experience_version"`
	ExperienceDesc  string `json:"experience_desc"`
	ExperienceTime  int64  `json:"experience_time"`
	ReleaseVersion  string `json:"release_version"`
	ReleaseDesc     string `json:"release_desc"`
	ReleaseTime     int64  `json:"release_time"`
	DevelopCount    int    `json:"develop_count"`
	QRPath          string `json:"qr_path"`
}

func (c *Client) Version(ctx context.Context, appid string) (*VersionState, error) {
	if err := c.Switch(ctx, appid); err != nil {
		return nil, err
	}
	session, err := c.session()
	if err != nil {
		return nil, err
	}
	inner := fmt.Sprintf("/wxopen/wacodepage?action=getcodepage&f=json&token=%s&lang=zh_CN", session.token)
	body, err := c.cgi(ctx, session, nil, inner)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Ret      int    `json:"ret"`
		CodeData string `json:"code_data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析版本信息失败: %w", err)
	}
	var code struct {
		Experience struct {
			Basic struct {
				Version  string `json:"version"`
				Describe string `json:"describe"`
				Time     int64  `json:"time"`
			} `json:"basic_info"`
		} `json:"experience_info"`
		Online struct {
			Basic struct {
				Version  string `json:"version"`
				Describe string `json:"describe"`
				Time     int64  `json:"time"`
			} `json:"basic_info"`
		} `json:"online_info"`
		Develop struct {
			List []struct{} `json:"info_list"`
		} `json:"develop_info"`
		QRPath string `json:"qr_path"`
	}
	if payload.CodeData != "" {
		if err := json.Unmarshal([]byte(payload.CodeData), &code); err != nil {
			return nil, fmt.Errorf("解析版本数据失败: %w", err)
		}
	}
	return &VersionState{
		AppID:          appid,
		ExperienceVer:  code.Experience.Basic.Version,
		ExperienceDesc: code.Experience.Basic.Describe,
		ExperienceTime: code.Experience.Basic.Time,
		ReleaseVersion: code.Online.Basic.Version,
		ReleaseDesc:    code.Online.Basic.Describe,
		ReleaseTime:    code.Online.Basic.Time,
		DevelopCount:   len(code.Develop.List),
		QRPath:         code.QRPath,
	}, nil
}

// ---- transport helpers ----

func (c *Client) cgi(ctx context.Context, session *Session, extra url.Values, inner string) ([]byte, error) {
	query := url.Values{"token": {session.token}, "lang": {"zh_CN"}, "random": {fmt.Sprintf("%.9f", randFloat())}}
	for key, values := range extra {
		query[key] = values
	}
	if !strings.HasPrefix(inner, "/") {
		inner = "/" + inner
	}
	endpoint := fmt.Sprintf("%s/wxamp/cgi/route?path=%s&%s", baseURL, url.QueryEscape(inner), query.Encode())
	return session.get(ctx, endpoint, nil)
}

func (c *Client) cgiPost(ctx context.Context, session *Session, path string, form map[string]string) ([]byte, error) {
	values := url.Values{}
	for key, value := range form {
		values.Set(key, value)
	}
	endpoint := fmt.Sprintf("%s/wxamp/cgi/%s", baseURL, path)
	return session.postForm(ctx, endpoint, values)
}

func (s *Session) get(ctx context.Context, endpoint string, _ url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if looksLikeLoginPage(body) {
		return nil, ErrNotLoggedIn
	}
	return body, nil
}

func (s *Session) postForm(ctx context.Context, path string, form url.Values) ([]byte, error) {
	endpoint := path
	if !strings.HasPrefix(path, "http") {
		endpoint = baseURL + path
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if looksLikeLoginPage(body) {
		return nil, ErrNotLoggedIn
	}
	return body, nil
}

func looksLikeLoginPage(body []byte) bool {
	if len(body) > 4096 {
		return false
	}
	text := string(body)
	return strings.Contains(text, "loginpage") || strings.Contains(text, "扫码登录")
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = io.ReadFull(rand.Reader, buf)
	return hex.EncodeToString(buf)
}

func randFloat() float64 {
	buf := make([]byte, 8)
	_, _ = io.ReadFull(rand.Reader, buf)
	var value uint64
	for _, b := range buf {
		value = value<<8 | uint64(b)
	}
	return float64(value%1_000_000_000) / 1_000_000_000
}

func truncate(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max]
}

// SessionCookie serializes the console cookie jar through the platform state.
type SessionCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (s *Session) Cookies() []SessionCookie {
	if s == nil {
		return nil
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil
	}
	cookies := s.client.Jar.Cookies(parsed)
	out := make([]SessionCookie, 0, len(cookies))
	for _, cookie := range cookies {
		out = append(out, SessionCookie{Name: cookie.Name, Value: cookie.Value})
	}
	return out
}

// Restore rebuilds a session from persisted cookies.
func Restore(cookies []SessionCookie, token, nickName, avatar string) (*Session, error) {
	if token == "" || len(cookies) == 0 {
		return nil, ErrNotLoggedIn
	}
	session := &Session{client: newHTTPClient(), token: token, created: time.Now()}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	httpCookies := make([]*http.Cookie, 0, len(cookies))
	for _, item := range cookies {
		httpCookies = append(httpCookies, &http.Cookie{Name: item.Name, Value: item.Value, Path: "/"})
	}
	session.client.Jar.SetCookies(parsed, httpCookies)
	session.state = ScanState{Status: "confirmed", NickName: nickName, Avatar: avatar}
	return session, nil
}

// SetSession installs a restored session.
func (c *Client) SetSession(session *Session) {
	c.mu.Lock()
	c.current = session
	c.mu.Unlock()
}

// Token returns the console token of this session.
func (s *Session) Token() string {
	if s == nil {
		return ""
	}
	return s.token
}

// CurrentSession exposes the active session for persistence.
func (c *Client) CurrentSession() *Session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

