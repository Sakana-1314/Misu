package openplatform

import (
	"context"
	"crypto/md5"
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

	"misu/internal/config"
)

const baseURL = "https://open.weixin.qq.com"

var tokenPattern = regexp.MustCompile(`token=([0-9a-f]{16,})`)

// Client drives the open platform console. It is only used before the third
// party platform is published, to keep the authorization test whitelist in
// sync automatically.
type Client struct {
	cfg  *config.Config
	http *http.Client

	mu      sync.Mutex
	token   string
	tokenAt time.Time
}

func New(cfg *config.Config) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Jar:     jar,
			Timeout: 20 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 8 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
	}
}

func (c *Client) Configured() bool {
	return c.cfg.OpenPlatformConfigured()
}

// AddTestAppID makes sure appid is present in the authorization test list.
func (c *Client) AddTestAppID(ctx context.Context, appid string) error {
	if !c.Configured() {
		return errors.New("未配置开放平台账号，无法自动加入测试名单")
	}
	appid = strings.TrimSpace(appid)
	if appid == "" {
		return errors.New("appid 不能为空")
	}
	token, err := c.session(ctx)
	if err != nil {
		return err
	}
	current, err := c.testList(ctx, token)
	if err != nil {
		return err
	}
	for _, item := range current {
		if strings.EqualFold(item, appid) {
			return nil
		}
	}
	if len(current) >= 20 {
		return errors.New("授权测试账号列表已达到 20 个上限，请先在开放平台移除不再测试的账号")
	}
	next := append(append([]string{}, current...), appid)
	return c.saveTestList(ctx, token, next)
}

func (c *Client) session(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Since(c.tokenAt) < 20*time.Minute {
		return c.token, nil
	}
	if err := c.primeCookies(ctx); err != nil {
		return "", err
	}
	form := url.Values{
		"account": {c.cfg.OpenPlatformAccount},
		"passwd":  {md5Hex(c.cfg.OpenPlatformPassword)},
		"token":   {""},
		"lang":    {"zh_CN"},
		"f":       {"json"},
		"ajax":    {"1"},
		"key":     {"1"},
	}
	resp, err := c.post(ctx, baseURL+"/cgi-bin/login", form)
	if err != nil {
		return "", fmt.Errorf("登录开放平台失败: %w", err)
	}
	defer resp.Body.Close()
	var payload struct {
		BaseResp struct {
			Ret    int    `json:"ret"`
			ErrMsg string `json:"err_msg"`
			Token  string `json:"token"`
		} `json:"base_resp"`
		NickName string `json:"nick_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("解析开放平台登录结果失败: %w", err)
	}
	if payload.BaseResp.Ret != 0 || payload.BaseResp.Token == "" {
		return "", fmt.Errorf("登录开放平台失败: %s", payload.BaseResp.ErrMsg)
	}
	c.token = payload.BaseResp.Token
	c.tokenAt = time.Now()
	return c.token, nil
}

// primeCookies loads the landing page first: the login endpoint rejects
// requests that arrive without the ticket cookies it sets there.
func (c *Client) primeCookies(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/", nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("访问开放平台失败: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return nil
}

func (c *Client) testList(ctx context.Context, token string) ([]string, error) {
	raw, err := c.detail(ctx, token)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			out = append(out, value)
		}
	}
	return out, nil
}

func (c *Client) detail(ctx context.Context, token string) (string, error) {
	endpoint := c.detailURL(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("读取开放平台配置失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	var payload struct {
		DevInfo struct {
			RawWhiteAcct string `json:"raw_white_acct"`
		} `json:"dev_info"`
		BaseResp struct {
			Ret    int    `json:"ret"`
			ErrMsg string `json:"err_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("解析开放平台配置失败: %w", err)
	}
	if payload.BaseResp.Ret != 0 && payload.BaseResp.Ret != 1003 {
		return "", fmt.Errorf("读取开放平台配置失败: %s", payload.BaseResp.ErrMsg)
	}
	c.refreshTokenFromBody(body)
	if payload.BaseResp.Ret == 1003 {
		return "", errors.New("开放平台登录状态已失效，请稍后重试")
	}
	return payload.DevInfo.RawWhiteAcct, nil
}

func (c *Client) saveTestList(ctx context.Context, token string, appids []string) error {
	endpoint := c.detailURL(token)
	form := url.Values{
		"token":      {token},
		"action":     {"direct_modify"},
		"white_acct": {strings.Join(appids, ";")},
		"white_ip":   {c.cfg.EgressIP},
		"type":       {"acct"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", endpoint)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("更新授权测试账号列表失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var payload struct {
		BaseResp struct {
			Ret    int    `json:"ret"`
			ErrMsg string `json:"err_msg"`
		} `json:"base_resp"`
		ErrCode     int      `json:"err_code"`
		ErrMsg      string   `json:"err_msg"`
		InvalidAcct []string `json:"invalid_acct"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("解析授权测试账号更新结果失败: %w", err)
	}
	if payload.BaseResp.Ret != 0 || payload.ErrCode != 0 {
		msg := payload.ErrMsg
		if msg == "" {
			msg = payload.BaseResp.ErrMsg
		}
		return fmt.Errorf("更新授权测试账号列表失败: %s", msg)
	}
	if len(payload.InvalidAcct) > 0 {
		return fmt.Errorf("以下 AppID 无效: %s", strings.Join(payload.InvalidAcct, "、"))
	}
	return nil
}

func (c *Client) detailURL(token string) string {
	return fmt.Sprintf("%s/cgi-bin/component_acct?action=detail&t=manage/plugin_detail&token=%s&appid=%s",
		baseURL, url.QueryEscape(token), url.QueryEscape(c.cfg.ComponentAppID))
}

// refreshTokenFromBody keeps the console session token fresh when the detail
// page carries a rotated one.
func (c *Client) refreshTokenFromBody(body []byte) {
	match := tokenPattern.FindSubmatch(body)
	if len(match) == 0 {
		return
	}
	token := string(match[1])
	if token == "" || token == c.token {
		return
	}
	c.mu.Lock()
	c.token = token
	c.tokenAt = time.Now()
	c.mu.Unlock()
}

func (c *Client) post(ctx context.Context, endpoint string, form url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("Origin", baseURL)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	return c.http.Do(req)
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
