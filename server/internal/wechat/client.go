package wechat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const defaultBaseURL = "https://api.weixin.qq.com"

type CallInfo struct {
	Endpoint string
	AppID    string
	Method   string
	Code     int
	Msg      string
	Latency  time.Duration
}

type Client struct {
	baseURL     string
	httpClient  *http.Client
	global      *rate.Limiter
	mu          sync.Mutex
	appLimiters map[string]*rate.Limiter
	onCall      func(CallInfo)
}

func New(maxQPS int, timeout time.Duration) *Client {
	if maxQPS < 1 {
		maxQPS = 8
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Client{
		baseURL:     defaultBaseURL,
		httpClient:  &http.Client{Timeout: timeout},
		global:      rate.NewLimiter(rate.Limit(maxQPS), maxQPS),
		appLimiters: make(map[string]*rate.Limiter),
	}
}

func (c *Client) SetBaseURL(u string) {
	c.baseURL = strings.TrimRight(u, "/")
}

func (c *Client) SetOnCall(fn func(CallInfo)) {
	c.onCall = fn
}

func (c *Client) wait(ctx context.Context, appid string) error {
	if appid != "" {
		lim := c.appLimiter(appid)
		if err := lim.Wait(ctx); err != nil {
			return err
		}
	}
	return c.global.Wait(ctx)
}

func (c *Client) appLimiter(appid string) *rate.Limiter {
	c.mu.Lock()
	defer c.mu.Unlock()
	if lim, ok := c.appLimiters[appid]; ok {
		return lim
	}
	lim := rate.NewLimiter(1, 1)
	c.appLimiters[appid] = lim
	return lim
}

func (c *Client) notify(info CallInfo) {
	if c.onCall != nil {
		c.onCall(info)
	}
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, query url.Values, body any, out any, appid string, retry bool) error {
	payload, err := marshalBody(body)
	if err != nil {
		return err
	}
	attempts := 1
	if retry {
		attempts = 3
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := c.wait(ctx, appid); err != nil {
			return err
		}
		start := time.Now()
		err := c.onceJSON(ctx, method, endpoint, query, payload, out)
		code, msg := 0, "ok"
		if ae, ok := err.(*APIError); ok {
			code, msg = ae.Code, ae.Msg
		}
		c.notify(CallInfo{Endpoint: endpoint, AppID: appid, Method: method, Code: code, Msg: msg, Latency: time.Since(start)})
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !retry || !IsTransient(err) {
			break
		}
		if attempt < attempts-1 {
			time.Sleep(time.Duration(300*(1<<attempt)) * time.Millisecond)
		}
	}
	return lastErr
}

func (c *Client) onceJSON(ctx context.Context, method, endpoint string, query url.Values, payload []byte, out any) error {
	u := c.baseURL + endpoint
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "misu/1.0")
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	var env struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &env)
	}
	if env.ErrCode != 0 {
		msg := env.ErrMsg
		if friendly := FriendlyMessage(env.ErrCode); friendly != "" {
			msg = friendly + " (" + env.ErrMsg + ")"
		}
		return &APIError{Code: env.ErrCode, Msg: msg, Endpoint: endpoint}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(data))
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return &APIError{Code: resp.StatusCode, Msg: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, snippet), Endpoint: endpoint}
	}
	if out != nil {
		if len(data) == 0 {
			return fmt.Errorf("wechat api %s returned empty body", endpoint)
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode %s response: %w", endpoint, err)
		}
	}
	return nil
}

func (c *Client) doBinary(ctx context.Context, method, endpoint string, query url.Values, body any, appid string, retry bool) ([]byte, string, error) {
	payload, err := marshalBody(body)
	if err != nil {
		return nil, "", err
	}
	attempts := 1
	if retry {
		attempts = 3
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := c.wait(ctx, appid); err != nil {
			return nil, "", err
		}
		start := time.Now()
		data, contentType, err := c.onceBinary(ctx, method, endpoint, query, payload)
		code, msg := 0, "ok"
		if ae, ok := err.(*APIError); ok {
			code, msg = ae.Code, ae.Msg
		}
		c.notify(CallInfo{Endpoint: endpoint, AppID: appid, Method: method, Code: code, Msg: msg, Latency: time.Since(start)})
		if err == nil {
			return data, contentType, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		if !retry || !IsTransient(err) {
			break
		}
		if attempt < attempts-1 {
			time.Sleep(time.Duration(300*(1<<attempt)) * time.Millisecond)
		}
	}
	return nil, "", lastErr
}

func (c *Client) onceBinary(ctx context.Context, method, endpoint string, query url.Values, payload []byte) ([]byte, string, error) {
	u := c.baseURL + endpoint
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "misu/1.0")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, "", err
	}
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(strings.ToLower(ct), "application/json") || (len(data) > 0 && data[0] == '{') {
		var env struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		if json.Unmarshal(data, &env) == nil && env.ErrCode != 0 {
			msg := env.ErrMsg
			if friendly := FriendlyMessage(env.ErrCode); friendly != "" {
				msg = friendly + " (" + env.ErrMsg + ")"
			}
			return nil, ct, &APIError{Code: env.ErrCode, Msg: msg, Endpoint: endpoint}
		}
		return nil, ct, fmt.Errorf("wechat api %s returned json instead of an image", endpoint)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, ct, &APIError{Code: resp.StatusCode, Msg: fmt.Sprintf("HTTP %d", resp.StatusCode), Endpoint: endpoint}
	}
	return data, ct, nil
}

func (c *Client) doMultipart(ctx context.Context, endpoint string, query url.Values, field, filename string, data []byte, out any, appid string) error {
	if err := c.wait(ctx, appid); err != nil {
		return err
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile(field, filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}
	u := c.baseURL + endpoint
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "misu/1.0")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	var env struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	_ = json.Unmarshal(body, &env)
	code, msg := 0, "ok"
	if env.ErrCode != 0 {
		code = env.ErrCode
		msg = env.ErrMsg
		if friendly := FriendlyMessage(env.ErrCode); friendly != "" {
			msg = friendly + " (" + env.ErrMsg + ")"
		}
		err = &APIError{Code: env.ErrCode, Msg: msg, Endpoint: endpoint}
	}
	c.notify(CallInfo{Endpoint: endpoint, AppID: appid, Method: http.MethodPost, Code: code, Msg: msg, Latency: time.Since(start)})
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{Code: resp.StatusCode, Msg: fmt.Sprintf("HTTP %d", resp.StatusCode), Endpoint: endpoint}
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode %s response: %w", endpoint, err)
		}
	}
	return nil
}

func marshalBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	return json.Marshal(body)
}

func tokenQuery(key, token string) url.Values {
	return url.Values{key: []string{token}}
}
