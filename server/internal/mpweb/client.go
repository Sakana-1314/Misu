// Package mpweb drives the official MP admin console (mp.weixin.qq.com) through
// a scanned-login session. It complements the third-party platform flow: it
// works for accounts the operator personally administers, without the platform
// authorization grants.
//
// The handshake mirrors the console's own login page:
//
//	POST /cgi-bin/bizlogin?action=startlogin   -> uuid cookie
//	GET  /cgi-bin/scanloginqrcode?action=getqrcode
//	GET  /cgi-bin/scanloginqrcode?action=ask   (poll)
//	POST /cgi-bin/bizlogin?action=login        -> redirect_url carrying token
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
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://mp.weixin.qq.com"
	userAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
	// The console expires a QR code after a few minutes; we stop polling a
	// little earlier so the browser can offer a fresh one.
	qrLifetime = 4 * time.Minute
)

// Scan states reported to the browser.
const (
	StatusWaiting   = "waiting"   // QR shown, nobody scanned yet
	StatusScanned   = "scanned"   // scanned, waiting for confirmation on the phone
	StatusConfirmed = "confirmed" // logged in
	StatusExpired   = "expired"   // QR timed out
	StatusCancelled = "cancelled" // operator tapped cancel on the phone
	StatusFailed    = "failed"    // cannot continue, see Message
)

var (
	// ErrNotLoggedIn surfaces when a console session expired or was dropped.
	ErrNotLoggedIn = errors.New("网页版登录已失效，请重新扫码")
	// ErrNoPendingLogin is returned when polling without a QR code.
	ErrNoPendingLogin = errors.New("二维码已失效，请刷新后重新扫码")

	tokenRegexp = regexp.MustCompile(`token=(\d+)`)
)

// Error is a console-side failure worth showing to the operator verbatim.
type Error struct {
	Ret int
	Msg string
}

func (e *Error) Error() string { return e.Msg }

// ScanState is returned to the browser while waiting for the operator to scan.
type ScanState struct {
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	NickName   string `json:"nick_name,omitempty"`
	Avatar     string `json:"avatar,omitempty"`
	AccountCnt int    `json:"account_count"`
	ExpiresAt  int64  `json:"expires_at,omitempty"`
}

// Session is a logged-in (or logging-in) console session.
type Session struct {
	mu          sync.Mutex
	client      *http.Client
	fingerprint string
	qrJPEG      []byte
	token       string
	created     time.Time
	state       ScanState
	profile     Profile
}

// Client manages the login handshake. It keeps one pending login per instance;
// the app only ever has a single operator.
type Client struct {
	baseURL string

	mu      sync.Mutex
	login   *Session
	current *Session
}

func New() *Client {
	return &Client{baseURL: defaultBaseURL}
}

// NewWithBaseURL points the client at another host; used by tests.
func NewWithBaseURL(base string) *Client {
	return &Client{baseURL: strings.TrimRight(base, "/")}
}

func newHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 25 * time.Second}
}

func newSession() *Session {
	return &Session{
		client:      newHTTPClient(),
		fingerprint: randomHex(16),
		created:     time.Now(),
	}
}

// StartLogin asks the console for a fresh QR code.
func (c *Client) StartLogin(ctx context.Context) (*ScanState, []byte, error) {
	session := newSession()
	// Prime the cookies (ua_id etc.) the way the login page does.
	if _, err := c.do(ctx, session, http.MethodGet, "/", nil, nil); err != nil {
		return nil, nil, err
	}
	sessionID := strconv.FormatInt(time.Now().UnixMilli(), 10) + fmt.Sprintf("%02d", randInt(100))
	body, err := c.do(ctx, session, http.MethodPost, "/cgi-bin/bizlogin", url.Values{"action": {"startlogin"}}, url.Values{
		"userlang":     {"zh_CN"},
		"redirect_url": {""},
		"login_type":   {"3"},
		"sessionid":    {sessionID},
	})
	if err != nil {
		return nil, nil, err
	}
	if err := checkBase(body, "创建登录会话失败"); err != nil {
		return nil, nil, err
	}

	// login_appid stays empty: a value would pin the scan to that one account.
	jpeg, err := c.do(ctx, session, http.MethodGet, "/cgi-bin/scanloginqrcode", url.Values{
		"action":      {"getqrcode"},
		"random":      {strconv.FormatInt(time.Now().UnixMilli(), 10)},
		"login_appid": {""},
	}, nil)
	if err != nil {
		return nil, nil, err
	}
	if len(jpeg) < 128 || jpeg[0] != 0xFF {
		return nil, nil, errors.New("获取登录二维码失败，请稍后重试")
	}
	session.qrJPEG = jpeg
	session.state = ScanState{Status: StatusWaiting, ExpiresAt: session.created.Add(qrLifetime).Unix()}

	c.mu.Lock()
	c.login = session
	c.mu.Unlock()
	state := session.state
	return &state, jpeg, nil
}

type askResponse struct {
	Status   int   `json:"status"`
	AcctSize int   `json:"acct_size"`
	BindUin  int64 `json:"binduin"`
	BaseResp struct {
		Ret    int    `json:"ret"`
		ErrMsg string `json:"err_msg"`
	} `json:"base_resp"`
}

// PollLogin advances the pending login. It returns the current state; when the
// operator has confirmed on the phone the session becomes the active one.
func (c *Client) PollLogin(ctx context.Context) (*ScanState, error) {
	c.mu.Lock()
	session := c.login
	c.mu.Unlock()
	if session == nil {
		return nil, ErrNoPendingLogin
	}
	if session.state.Status != StatusWaiting && session.state.Status != StatusScanned {
		state := session.state
		return &state, nil
	}
	if time.Since(session.created) > qrLifetime {
		session.state = ScanState{Status: StatusExpired, Message: "二维码已过期"}
		state := session.state
		return &state, nil
	}

	body, err := c.do(ctx, session, http.MethodGet, "/cgi-bin/scanloginqrcode", url.Values{"action": {"ask"}}, nil)
	if err != nil {
		return nil, err
	}
	var payload askResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析扫码状态失败: %w", err)
	}
	next := interpretAsk(payload)
	next.ExpiresAt = session.state.ExpiresAt
	if next.Status == StatusConfirmed {
		return c.finishLogin(ctx, session)
	}
	session.state = next
	return &next, nil
}

// interpretAsk maps the console's scan status codes, following its login page.
func interpretAsk(payload askResponse) ScanState {
	if payload.BaseResp.Ret != 0 {
		return ScanState{Status: StatusFailed, Message: "登录会话已失效，请刷新二维码"}
	}
	switch payload.Status {
	case 1:
		return ScanState{Status: StatusConfirmed}
	case 2:
		return ScanState{Status: StatusCancelled, Message: "已在手机上取消登录"}
	case 3:
		return ScanState{Status: StatusExpired, Message: "二维码已过期"}
	case 4, 6:
		switch {
		case payload.AcctSize == 1:
			return ScanState{Status: StatusScanned, Message: "请在手机上确认登录", AccountCnt: 1}
		case payload.AcctSize > 1:
			return ScanState{Status: StatusScanned, Message: "请在手机上选择要登录的账号", AccountCnt: payload.AcctSize}
		default:
			return ScanState{Status: StatusFailed, Message: "该微信号不是任何公众号或小程序的管理员"}
		}
	case 5:
		if payload.BindUin != 0 {
			return ScanState{Status: StatusFailed, Message: "该微信号绑定的是 QQ 号账号，请先在公众平台完成换绑"}
		}
		return ScanState{Status: StatusFailed, Message: "该微信号没有绑定公众平台账号"}
	default:
		return ScanState{Status: StatusWaiting}
	}
}

func (c *Client) finishLogin(ctx context.Context, session *Session) (*ScanState, error) {
	body, err := c.do(ctx, session, http.MethodPost, "/cgi-bin/bizlogin", url.Values{"action": {"login"}}, url.Values{
		"userlang":         {"zh_CN"},
		"redirect_url":     {""},
		"cookie_forbidden": {"0"},
		"cookie_cleaned":   {"0"},
		"plugin_used":      {"0"},
		"login_type":       {"3"},
	})
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
	token := extractToken(payload.RedirectURL)
	if payload.BaseResp.Ret != 0 || token == "" {
		message := loginFailureMessage(payload.BaseResp.Ret, payload.RedirectURL)
		session.state = ScanState{Status: StatusFailed, Message: message}
		state := session.state
		return &state, nil
	}

	session.mu.Lock()
	session.token = token
	session.state = ScanState{Status: StatusConfirmed}
	session.mu.Unlock()

	// Read the logged-in account from its landing page.
	if profile, err := c.loadProfile(ctx, session); err == nil {
		session.mu.Lock()
		session.profile = *profile
		session.state.NickName = profile.NickName
		session.state.Avatar = profile.Avatar
		session.mu.Unlock()
	}

	c.mu.Lock()
	c.current = session
	c.login = nil
	c.mu.Unlock()
	state := session.snapshot()
	return &state, nil
}

func loginFailureMessage(ret int, redirect string) string {
	switch {
	case strings.Contains(redirect, "validate_wx_tmpl"):
		return "微信要求对本次登录做安全验证，请先在浏览器登录一次公众平台后再试"
	case ret == 200007:
		return "该账号当前处于访问受限状态"
	case ret == 200008:
		return "需要完成验证码校验，请稍后重试"
	case ret != 0:
		return fmt.Sprintf("登录失败（%d），请重新扫码", ret)
	default:
		return "登录失败，请重新扫码"
	}
}

// QRCode returns the pending login QR image.
func (c *Client) QRCode() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.login == nil {
		return nil, ErrNoPendingLogin
	}
	return c.login.qrJPEG, nil
}

// CancelLogin drops a pending QR code.
func (c *Client) CancelLogin() {
	c.mu.Lock()
	c.login = nil
	c.mu.Unlock()
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

func (c *Client) session() (*Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current == nil || c.current.token == "" {
		return nil, ErrNotLoggedIn
	}
	return c.current, nil
}

func (s *Session) snapshot() ScanState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Session) currentToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token
}

// Profile describes the account the console session is logged into.
type Profile struct {
	NickName string `json:"nick_name"`
	Avatar   string `json:"avatar"`
	UserName string `json:"user_name"`
	AppID    string `json:"appid,omitempty"`
	Uin      string `json:"uin"`
}

var (
	initialStatePattern = regexp.MustCompile(`userInfo\s*:\s*`)
	nickPattern         = regexp.MustCompile(`(?:nick_name|nickname)["']?\s*[:=]\s*["']([^"']+)["']`)
	headPattern         = regexp.MustCompile(`(?:head_img|headimgurl|head_img_url)["']?\s*[:=]\s*["']([^"']+)["']`)
	userNamePattern     = regexp.MustCompile(`["']?(?:user_name|username)["']?\s*[:=]\s*["'](gh_[0-9a-f]+)["']`)
	uinPattern          = regexp.MustCompile(`["']?(?:uin|bizuin)["']?\s*[:=]\s*["']?(\d{5,})`)
)

// parseProfile pulls the account identity out of a console page. Mini program
// logins land on the wxamp app, whose __INITIAL_STATE__ carries userInfo as
// JSON; official account logins land on /cgi-bin/home with wx.commonData.
func parseProfile(html []byte) *Profile {
	if loc := initialStatePattern.FindIndex(html); loc != nil {
		var info struct {
			NickName string `json:"nickName"`
			IconURL  string `json:"iconUrl"`
			UserName string `json:"userName"`
			AppID    string `json:"appid"`
			FakeID   int64  `json:"fakeId"`
		}
		if err := json.NewDecoder(strings.NewReader(string(html[loc[1]:]))).Decode(&info); err == nil && (info.NickName != "" || info.AppID != "") {
			profile := &Profile{
				NickName: info.NickName,
				Avatar:   secureURL(info.IconURL),
				UserName: info.UserName,
				AppID:    info.AppID,
			}
			if info.FakeID != 0 {
				profile.Uin = strconv.FormatInt(info.FakeID, 10)
			}
			return profile
		}
	}
	profile := &Profile{}
	if m := nickPattern.FindSubmatch(html); len(m) > 1 {
		profile.NickName = unescapeJS(string(m[1]))
	}
	if m := headPattern.FindSubmatch(html); len(m) > 1 {
		profile.Avatar = secureURL(unescapeJS(string(m[1])))
	}
	if m := userNamePattern.FindSubmatch(html); len(m) > 1 {
		profile.UserName = string(m[1])
	}
	if m := uinPattern.FindSubmatch(html); len(m) > 1 {
		profile.Uin = string(m[1])
	}
	return profile
}

// loadProfile opens the console root, which redirects to the landing page of
// whichever console (mini program or official account) the session belongs to.
func (c *Client) loadProfile(ctx context.Context, session *Session) (*Profile, error) {
	finalURL, body, err := c.follow(ctx, session, "/")
	if err != nil {
		return nil, err
	}
	if token := extractToken(finalURL); token != "" {
		session.mu.Lock()
		session.token = token
		session.mu.Unlock()
	}
	return parseProfile(body), nil
}

// Profile returns the identity captured at login.
func (c *Client) Profile() (*Profile, error) {
	session, err := c.session()
	if err != nil {
		return nil, err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	profile := session.profile
	return &profile, nil
}

// Check verifies the session is still accepted by the console and refreshes
// the cached identity.
func (c *Client) Check(ctx context.Context) (*Profile, error) {
	session, err := c.session()
	if err != nil {
		return nil, err
	}
	profile, err := c.loadProfile(ctx, session)
	if err != nil {
		return nil, err
	}
	session.mu.Lock()
	if profile.NickName == "" {
		profile.NickName = session.profile.NickName
	}
	if profile.Avatar == "" {
		profile.Avatar = session.profile.Avatar
	}
	session.profile = *profile
	session.mu.Unlock()
	return profile, nil
}

// Account is one official account or mini program the operator can switch to.
type Account struct {
	AppID    string `json:"appid,omitempty"`
	UserName string `json:"user_name"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"`
	BizUin   int64  `json:"bizuin,omitempty"`
	// Kind is miniprogram, official (公众号) or service (服务号).
	Kind    string `json:"kind"`
	Current bool   `json:"current"`
}

// AccountList is every account bound to the scanning WeChat user.
type AccountList struct {
	Accounts []Account `json:"accounts"`
	// SwitchPermitted tells whether the console lets this session hop accounts.
	SwitchPermitted bool `json:"switch_permitted"`
}

// parseWxaList reads the mini program console's account list
// (GET /wxamp/cgi/getWxaList).
func parseWxaList(body []byte, current *Profile) (*AccountList, error) {
	var payload struct {
		Ret  int `json:"ret"`
		List []struct {
			AppID    string `json:"appid"`
			Name     string `json:"app_name"`
			HeadImg  string `json:"app_headimg"`
			UserName string `json:"username"`
		} `json:"wax_list"`
		// 0 means the operator did not allow switching when scanning.
		SwitchPermit *int `json:"account_switch_permit"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析小程序列表失败: %w", err)
	}
	if payload.Ret != 0 {
		return nil, &Error{Ret: payload.Ret, Msg: "读取小程序列表失败"}
	}
	out := &AccountList{SwitchPermitted: payload.SwitchPermit == nil || *payload.SwitchPermit != 0}
	for _, item := range payload.List {
		account := Account{
			AppID:    item.AppID,
			UserName: item.UserName,
			Name:     item.Name,
			Avatar:   secureURL(item.HeadImg),
			Kind:     "miniprogram",
		}
		if current != nil {
			account.Current = (current.AppID != "" && current.AppID == item.AppID) ||
				(current.UserName != "" && current.UserName == item.UserName)
		}
		out.Accounts = append(out.Accounts, account)
	}
	return out, nil
}

type acctItem struct {
	UserName   string `json:"username"`
	NickName   string `json:"nickname"`
	HeadImgURL string `json:"headimgurl"`
	BizUin     int64  `json:"bizuin"`
	AppID      string `json:"appid"`
}

// parseAccountList reads the official account console's account list
// (GET /cgi-bin/switchacct?action=get_acct_list).
func parseAccountList(body []byte, current *Profile) (*AccountList, error) {
	var payload struct {
		BaseResp struct {
			Ret int `json:"ret"`
		} `json:"base_resp"`
		BizList struct {
			List []acctItem `json:"list"`
		} `json:"biz_list"`
		ServiceBizList struct {
			List []acctItem `json:"list"`
		} `json:"service_biz_list"`
		WxaList struct {
			List []acctItem `json:"list"`
		} `json:"wxa_list"`
		ScanLoginCtx struct {
			AccountSwitchPermit bool `json:"account_switch_permit"`
		} `json:"scan_login_ctx"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析账号列表失败: %w", err)
	}
	if payload.BaseResp.Ret != 0 {
		return nil, &Error{Ret: payload.BaseResp.Ret, Msg: "读取账号列表失败"}
	}
	out := &AccountList{SwitchPermitted: payload.ScanLoginCtx.AccountSwitchPermit}
	add := func(kind string, items []acctItem) {
		for _, item := range items {
			account := Account{
				AppID:    item.AppID,
				UserName: item.UserName,
				Name:     item.NickName,
				Avatar:   secureURL(item.HeadImgURL),
				BizUin:   item.BizUin,
				Kind:     kind,
			}
			if current != nil {
				account.Current = (current.UserName != "" && current.UserName == item.UserName) ||
					(current.Uin != "" && current.Uin == strconv.FormatInt(item.BizUin, 10))
			}
			out.Accounts = append(out.Accounts, account)
		}
	}
	add("miniprogram", payload.WxaList.List)
	add("official", payload.BizList.List)
	add("service", payload.ServiceBizList.List)
	return out, nil
}

// Accounts lists the accounts bound to the operator. Mini program logins use
// the wxamp console API, official account logins the switchacct API; when
// neither answers, the logged-in account is the only entry.
func (c *Client) Accounts(ctx context.Context) (*AccountList, error) {
	session, err := c.session()
	if err != nil {
		return nil, err
	}
	session.mu.Lock()
	profile := session.profile
	session.mu.Unlock()

	var list *AccountList
	if body, err := c.do(ctx, session, http.MethodGet, "/wxamp/cgi/getWxaList", nil, nil); err != nil {
		if errors.Is(err, ErrNotLoggedIn) {
			return nil, err
		}
	} else if parsed, err := parseWxaList(body, &profile); err == nil && len(parsed.Accounts) > 0 {
		list = parsed
	}
	if list == nil {
		body, err := c.do(ctx, session, http.MethodGet, "/cgi-bin/switchacct", url.Values{"action": {"get_acct_list"}}, nil)
		if errors.Is(err, ErrNotLoggedIn) {
			return nil, err
		}
		if err == nil {
			if parsed, err := parseAccountList(body, &profile); err == nil && len(parsed.Accounts) > 0 {
				list = parsed
			}
		}
	}
	if list == nil {
		list = &AccountList{Accounts: []Account{{
			AppID:    profile.AppID,
			UserName: profile.UserName,
			Name:     profile.NickName,
			Avatar:   profile.Avatar,
			Kind:     "miniprogram",
		}}}
	}
	if len(list.Accounts) == 1 {
		list.Accounts[0].Current = true
	}
	return list, nil
}

// Switch moves the console session to another account bound to the operator.
// id is a mini program appid (wx...) or an official account username (gh_...).
func (c *Client) Switch(ctx context.Context, id string) error {
	session, err := c.session()
	if err != nil {
		return err
	}
	session.mu.Lock()
	same := id != "" && (session.profile.AppID == id || session.profile.UserName == id)
	session.mu.Unlock()
	if same {
		return nil
	}

	var ret int
	if strings.HasPrefix(id, "wx") {
		body, err := c.do(ctx, session, http.MethodPost, "/wxamp/cgi/switchAccount", nil, url.Values{"appid": {id}})
		if err != nil {
			return err
		}
		var payload struct {
			Ret int `json:"ret"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("切换小程序失败: %w", err)
		}
		ret = payload.Ret
	} else {
		body, err := c.do(ctx, session, http.MethodPost, "/cgi-bin/switchacct", url.Values{"action": {"switch"}}, url.Values{"username": {id}})
		if err != nil {
			return err
		}
		var payload struct {
			BaseResp struct {
				Ret int `json:"ret"`
			} `json:"base_resp"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("切换账号失败: %w", err)
		}
		ret = payload.BaseResp.Ret
	}
	switch ret {
	case 0:
	case 200003:
		return ErrNotLoggedIn
	case 201501:
		return &Error{Ret: ret, Msg: "扫码时未允许切换账号，请重新扫码并在手机上勾选「允许切换登录账号」"}
	default:
		return &Error{Ret: ret, Msg: fmt.Sprintf("切换账号失败（%d）", ret)}
	}

	// The console lands on "/" after switching; the redirect carries the new token.
	profile, err := c.loadProfile(ctx, session)
	if err != nil {
		return err
	}
	session.mu.Lock()
	session.profile = *profile
	session.mu.Unlock()
	return nil
}

// VersionState summarizes the console's code page for a mini program.
type VersionState struct {
	AppID          string `json:"appid"`
	ExperienceVer  string `json:"experience_version"`
	ExperienceDesc string `json:"experience_desc"`
	ExperienceTime int64  `json:"experience_time"`
	ReleaseVersion string `json:"release_version"`
	ReleaseDesc    string `json:"release_desc"`
	ReleaseTime    int64  `json:"release_time"`
	DevelopCount   int    `json:"develop_count"`
}

type versionBasic struct {
	Version  string `json:"version"`
	Describe string `json:"describe"`
	Time     int64  `json:"time"`
}

func parseCodePage(body []byte) (*VersionState, error) {
	var payload struct {
		Ret      int    `json:"ret"`
		CodeData string `json:"code_data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("解析版本信息失败: %w", err)
	}
	if payload.Ret != 0 {
		return nil, &Error{Ret: payload.Ret, Msg: "读取版本信息失败"}
	}
	var code struct {
		Experience struct {
			Basic versionBasic `json:"basic_info"`
		} `json:"experience_info"`
		Online struct {
			Basic versionBasic `json:"basic_info"`
		} `json:"online_info"`
		Develop struct {
			List []json.RawMessage `json:"info_list"`
		} `json:"develop_info"`
	}
	if payload.CodeData != "" {
		if err := json.Unmarshal([]byte(payload.CodeData), &code); err != nil {
			return nil, fmt.Errorf("解析版本数据失败: %w", err)
		}
	}
	return &VersionState{
		ExperienceVer:  code.Experience.Basic.Version,
		ExperienceDesc: code.Experience.Basic.Describe,
		ExperienceTime: code.Experience.Basic.Time,
		ReleaseVersion: code.Online.Basic.Version,
		ReleaseDesc:    code.Online.Basic.Describe,
		ReleaseTime:    code.Online.Basic.Time,
		DevelopCount:   len(code.Develop.List),
	}, nil
}

// Version reads the code management page of the given mini program.
func (c *Client) Version(ctx context.Context, appid string) (*VersionState, error) {
	if err := c.Switch(ctx, appid); err != nil {
		return nil, err
	}
	session, err := c.session()
	if err != nil {
		return nil, err
	}
	inner := "/wxopen/wacodepage?action=getcodepage&f=json&token=" + session.currentToken()
	body, err := c.do(ctx, session, http.MethodGet, "/wxamp/cgi/route", url.Values{"path": {inner}}, nil)
	if err != nil {
		return nil, err
	}
	state, err := parseCodePage(body)
	if err != nil {
		return nil, err
	}
	state.AppID = appid
	return state, nil
}

// ---- transport helpers ----

// do sends a console API request with the parameters the console's own pages
// attach to it.
func (c *Client) do(ctx context.Context, session *Session, method, path string, query, form url.Values) ([]byte, error) {
	_, body, err := c.send(ctx, session, method, path, query, form)
	return body, err
}

// follow issues a plain page GET and reports where redirects ended up.
func (c *Client) follow(ctx context.Context, session *Session, path string) (string, []byte, error) {
	return c.send(ctx, session, http.MethodGet, path, nil, nil)
}

type requestKind int

const (
	kindPage  requestKind = iota // HTML page, no extra parameters
	kindWxamp                    // mini program console API under /wxamp/cgi
	kindCGI                      // legacy /cgi-bin ajax API
)

func classify(path string) requestKind {
	switch {
	case strings.HasPrefix(path, "/wxamp/cgi"):
		return kindWxamp
	case path == "/" || strings.HasPrefix(path, "/cgi-bin/home") || strings.HasPrefix(path, "/wxamp/"):
		return kindPage
	default:
		return kindCGI
	}
}

func (c *Client) send(ctx context.Context, session *Session, method, path string, query, form url.Values) (string, []byte, error) {
	endpoint, err := url.Parse(c.baseURL + path)
	if err != nil {
		return "", nil, err
	}
	kind := classify(path)
	token := session.currentToken()
	params := endpoint.Query()
	for key, values := range query {
		params[key] = values
	}
	setDefault := func(values url.Values, key, value string) {
		if _, set := values[key]; !set {
			values.Set(key, value)
		}
	}
	body := url.Values{}
	for key, v := range form {
		body[key] = v
	}
	referer := c.baseURL + "/"
	switch kind {
	case kindWxamp:
		// The wxamp app keeps token and lang in the query, even for POSTs.
		setDefault(params, "token", token)
		setDefault(params, "lang", "zh_CN")
		setDefault(params, "f", "json")
		setDefault(params, "random", fmt.Sprintf("%.9f", randFloat()))
		referer = c.baseURL + "/wxamp/index/index?lang=zh_CN&token=" + token
	case kindCGI:
		common := url.Values{
			"token":       {token},
			"lang":        {"zh_CN"},
			"f":           {"json"},
			"ajax":        {"1"},
			"fingerprint": {session.fingerprint},
		}
		target := params
		if method == http.MethodPost {
			target = body
		}
		for key, v := range common {
			setDefault(target, key, v[0])
		}
	}
	endpoint.RawQuery = params.Encode()

	var reader io.Reader
	if method == http.MethodPost {
		reader = strings.NewReader(body.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", referer)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	}
	switch kind {
	case kindCGI:
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	case kindWxamp:
		req.Header.Set("Accept", "application/json, text/plain, */*")
	}
	resp, err := session.client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("连接微信公众平台失败: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", nil, err
	}
	finalURL := resp.Request.URL.String()
	if token != "" && sessionDropped(resp.Request.URL, data) {
		return finalURL, nil, ErrNotLoggedIn
	}
	return finalURL, data, nil
}

// sessionDropped recognises the console bouncing a logged-in request back to
// the login page, or answering with an invalid-session ret code.
func sessionDropped(final *url.URL, body []byte) bool {
	if final != nil && (final.Path == "/" || final.Path == "") && !strings.Contains(final.RawQuery, "token=") {
		return true
	}
	if len(body) > 0 && body[0] == '{' {
		var payload struct {
			Ret      int `json:"ret"`
			BaseResp struct {
				Ret int `json:"ret"`
			} `json:"base_resp"`
		}
		if json.Unmarshal(body, &payload) == nil {
			for _, ret := range []int{payload.Ret, payload.BaseResp.Ret} {
				if ret == 200003 || ret == 200040 {
					return true
				}
			}
		}
	}
	return false
}

func checkBase(body []byte, fallback string) error {
	var payload struct {
		BaseResp struct {
			Ret    int    `json:"ret"`
			ErrMsg string `json:"err_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("%s: 响应格式异常", fallback)
	}
	if payload.BaseResp.Ret != 0 {
		return &Error{Ret: payload.BaseResp.Ret, Msg: fmt.Sprintf("%s（%d）", fallback, payload.BaseResp.Ret)}
	}
	return nil
}

func extractToken(text string) string {
	if match := tokenRegexp.FindStringSubmatch(text); len(match) > 1 {
		return match[1]
	}
	return ""
}

func secureURL(raw string) string {
	if strings.HasPrefix(raw, "http://") {
		return "https://" + strings.TrimPrefix(raw, "http://")
	}
	return raw
}

func unescapeJS(text string) string {
	text = strings.ReplaceAll(text, `\/`, `/`)
	if strings.Contains(text, `\u`) || strings.Contains(text, `\x`) {
		if unquoted, err := strconv.Unquote(`"` + strings.ReplaceAll(text, `"`, `\"`) + `"`); err == nil {
			return unquoted
		}
	}
	return text
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = io.ReadFull(rand.Reader, buf)
	return hex.EncodeToString(buf)
}

func randFloat() float64 {
	buf := make([]byte, 4)
	_, _ = io.ReadFull(rand.Reader, buf)
	return float64(uint32(buf[0])<<24|uint32(buf[1])<<16|uint32(buf[2])<<8|uint32(buf[3])) / (1 << 32)
}

func randInt(max int) int {
	buf := make([]byte, 1)
	_, _ = io.ReadFull(rand.Reader, buf)
	return int(buf[0]) % max
}

// SessionCookie serializes the console cookie jar for persistence.
type SessionCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Cookies exports the console cookies of this session.
func (s *Session) Cookies(base string) []SessionCookie {
	if s == nil {
		return nil
	}
	parsed, err := url.Parse(base)
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

// Token returns the console token of this session.
func (s *Session) Token() string {
	if s == nil {
		return ""
	}
	return s.currentToken()
}

// Fingerprint returns the browser fingerprint this session presents.
func (s *Session) Fingerprint() string {
	if s == nil {
		return ""
	}
	return s.fingerprint
}

// SavedSession is what the service persists between restarts.
type SavedSession struct {
	Cookies     []SessionCookie `json:"cookies"`
	Token       string          `json:"-"`
	Fingerprint string          `json:"fingerprint"`
	Profile     Profile         `json:"profile"`
}

// Export captures the active session for persistence.
func (c *Client) Export() (*SavedSession, error) {
	session, err := c.session()
	if err != nil {
		return nil, err
	}
	session.mu.Lock()
	profile := session.profile
	session.mu.Unlock()
	return &SavedSession{
		Cookies:     session.Cookies(c.baseURL),
		Token:       session.currentToken(),
		Fingerprint: session.fingerprint,
		Profile:     profile,
	}, nil
}

// Restore installs a persisted session.
func (c *Client) Restore(saved *SavedSession) error {
	if saved == nil || saved.Token == "" || len(saved.Cookies) == 0 {
		return ErrNotLoggedIn
	}
	session := newSession()
	if saved.Fingerprint != "" {
		session.fingerprint = saved.Fingerprint
	}
	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	httpCookies := make([]*http.Cookie, 0, len(saved.Cookies))
	for _, item := range saved.Cookies {
		httpCookies = append(httpCookies, &http.Cookie{Name: item.Name, Value: item.Value, Path: "/"})
	}
	session.client.Jar.SetCookies(parsed, httpCookies)
	session.token = saved.Token
	session.profile = saved.Profile
	session.state = ScanState{Status: StatusConfirmed, NickName: saved.Profile.NickName, Avatar: saved.Profile.Avatar}

	c.mu.Lock()
	c.current = session
	c.mu.Unlock()
	return nil
}
