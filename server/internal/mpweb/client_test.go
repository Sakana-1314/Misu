package mpweb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// fakeConsole imitates the login endpoints of mp.weixin.qq.com.
type fakeConsole struct {
	mu        sync.Mutex
	askBody   string
	loginBody string
	wxaBody   string
	requests  []*http.Request
	forms     map[string]url.Values
}

func newFakeConsole() *fakeConsole {
	return &fakeConsole{
		askBody:   `{"acct_size":0,"base_resp":{"err_msg":"ok","ret":0},"binduin":0,"status":0}`,
		loginBody: `{"base_resp":{"err_msg":"ok","ret":0},"redirect_url":"/cgi-bin/home?t=home/index&lang=zh_CN&token=654636700"}`,
		wxaBody: `{"ret":0,"wax_list":[
			{"appid":"wx821c8dd7888db38a","app_name":"修猫学习助手","app_headimg":"http://wx.qlogo.cn/a/0","username":"gh_a262422ef9bf","type":1},
			{"appid":"wx0000000000000002","app_name":"另一个小程序","app_headimg":"","username":"gh_b000000000002","type":1}],
			"account_switch_permit":1}`,
		forms: map[string]url.Values{},
	}
}

func (f *fakeConsole) handler() http.Handler {
	jpeg := append([]byte{0xFF, 0xD8, 0xFF}, make([]byte, 200)...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r)
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			f.forms[r.URL.Path+"?"+r.URL.Query().Get("action")] = r.PostForm
		}
		f.mu.Unlock()
		action := r.URL.Query().Get("action")
		loggedIn := false
		if cookie, err := r.Cookie("slave_sid"); err == nil && cookie.Value != "" {
			loggedIn = true
		}
		switch {
		case r.URL.Path == "/" && loggedIn:
			http.Redirect(w, r, "/wxamp/index/index?lang=zh_CN&token=654636700", http.StatusFound)
		case r.URL.Path == "/":
			http.SetCookie(w, &http.Cookie{Name: "ua_id", Value: "x", Path: "/"})
			w.Write([]byte("<html>login</html>"))
		case r.URL.Path == "/cgi-bin/bizlogin" && action == "startlogin":
			http.SetCookie(w, &http.Cookie{Name: "uuid", Value: "abc", Path: "/"})
			w.Write([]byte(`{"base_resp":{"err_msg":"ok","ret":0},"uuid":"abc"}`))
		case r.URL.Path == "/cgi-bin/scanloginqrcode" && action == "getqrcode":
			w.Write(jpeg)
		case r.URL.Path == "/cgi-bin/scanloginqrcode" && action == "ask":
			f.mu.Lock()
			body := f.askBody
			f.mu.Unlock()
			w.Write([]byte(body))
		case r.URL.Path == "/cgi-bin/bizlogin" && action == "login":
			http.SetCookie(w, &http.Cookie{Name: "slave_sid", Value: "s1", Path: "/"})
			w.Write([]byte(f.loginBody))
		case r.URL.Path == "/wxamp/index/index":
			w.Write([]byte(`<script>window.__INITIAL_STATE__ = {
            userInfo: {"ret":0,"fakeId":3320105474,"iconUrl":"http://wx.qlogo.cn/a/0","userName":"gh_a262422ef9bf","nickName":"修猫学习助手","appid":"wx821c8dd7888db38a","token":"654636700"},
            wujieConfig: "{}"
        }</script>`))
		case r.URL.Path == "/wxamp/cgi/getWxaList":
			if r.URL.Query().Get("token") != "654636700" {
				w.Write([]byte(`{"ret":200003}`))
				return
			}
			w.Write([]byte(f.wxaBody))
		case r.URL.Path == "/wxamp/cgi/switchAccount" && r.Method == http.MethodPost:
			w.Write([]byte(`{"ret":0,"token":"777"}`))
		case r.URL.Path == "/wxamp/cgi/route" && strings.HasPrefix(r.URL.Query().Get("path"), "/wxopen/wacodepage?action=getcodepage"):
			w.Write([]byte(`{"ret":0,"code_data":"{\"experience_info\":{\"basic_info\":{\"version\":\"1.2.0\"}},\"online_info\":{\"basic_info\":{\"version\":\"1.1.0\"}},\"develop_info\":{\"info_list\":[{}]}}"}`))
		default:
			http.NotFound(w, r)
		}
	})
}

func (f *fakeConsole) find(path, action string) *http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.requests {
		if r.URL.Path == path && r.URL.Query().Get("action") == action {
			return r
		}
	}
	return nil
}

func TestLoginFlow(t *testing.T) {
	console := newFakeConsole()
	server := httptest.NewServer(console.handler())
	defer server.Close()
	client := NewWithBaseURL(server.URL)
	ctx := context.Background()

	state, qr, err := client.StartLogin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != StatusWaiting || len(qr) == 0 {
		t.Fatalf("unexpected start state %+v", state)
	}
	qrReq := console.find("/cgi-bin/scanloginqrcode", "getqrcode")
	if qrReq == nil || qrReq.URL.Query().Get("login_appid") != "" {
		t.Fatalf("QR must not be pinned to an appid: %v", qrReq)
	}
	if form := console.forms["/cgi-bin/bizlogin?startlogin"]; form.Get("f") != "json" || form.Get("login_type") != "3" || form.Get("fingerprint") == "" {
		t.Fatalf("startlogin form missing console params: %v", form)
	}

	state, err = client.PollLogin(ctx)
	if err != nil || state.Status != StatusWaiting {
		t.Fatalf("poll: %+v %v", state, err)
	}
	if ask := console.find("/cgi-bin/scanloginqrcode", "ask"); ask.Header.Get("Cookie") == "" || !strings.Contains(ask.Header.Get("Cookie"), "uuid=abc") {
		t.Fatalf("ask must carry the uuid cookie, got %q", ask.Header.Get("Cookie"))
	}

	console.askBody = `{"acct_size":3,"base_resp":{"ret":0},"status":4}`
	state, _ = client.PollLogin(ctx)
	if state.Status != StatusScanned || state.AccountCnt != 3 || !strings.Contains(state.Message, "选择") {
		t.Fatalf("scanned with several accounts: %+v", state)
	}

	console.askBody = `{"acct_size":1,"base_resp":{"ret":0},"status":1}`
	state, err = client.PollLogin(ctx)
	if err != nil || state.Status != StatusConfirmed {
		t.Fatalf("confirm: %+v %v", state, err)
	}
	if !client.Connected() {
		t.Fatal("client should be connected")
	}
	if state.NickName != "修猫学习助手" || state.Avatar != "https://wx.qlogo.cn/a/0" {
		t.Fatalf("profile not parsed: %+v", state)
	}
	profile, _ := client.Profile()
	if profile.UserName != "gh_a262422ef9bf" || profile.AppID != "wx821c8dd7888db38a" || profile.Uin != "3320105474" {
		t.Fatalf("unexpected profile %+v", profile)
	}
	if _, err := client.QRCode(); !errors.Is(err, ErrNoPendingLogin) {
		t.Fatal("pending QR should be cleared after login")
	}

	list, err := client.Accounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Accounts) != 2 || !list.SwitchPermitted {
		t.Fatalf("unexpected account list %+v", list)
	}
	if first := list.Accounts[0]; !first.Current || first.AppID != "wx821c8dd7888db38a" || first.Avatar != "https://wx.qlogo.cn/a/0" {
		t.Fatalf("logged-in mini program should be marked current: %+v", first)
	}

	version, err := client.Version(ctx, "wx0000000000000002")
	if err != nil {
		t.Fatal(err)
	}
	if version.ExperienceVer != "1.2.0" || version.ReleaseVersion != "1.1.0" || version.DevelopCount != 1 {
		t.Fatalf("unexpected version %+v", version)
	}
	if req := console.find("/wxamp/cgi/switchAccount", ""); req == nil {
		t.Fatal("switching should go through the wxamp console API")
	}
}

func TestInterpretAsk(t *testing.T) {
	cases := []struct {
		name   string
		in     askResponse
		status string
	}{
		{"waiting", askResponse{Status: 0}, StatusWaiting},
		{"confirmed", askResponse{Status: 1}, StatusConfirmed},
		{"cancelled", askResponse{Status: 2}, StatusCancelled},
		{"expired", askResponse{Status: 3}, StatusExpired},
		{"scanned single", askResponse{Status: 4, AcctSize: 1}, StatusScanned},
		{"scanned no account", askResponse{Status: 6, AcctSize: 0}, StatusFailed},
		{"not bound", askResponse{Status: 5}, StatusFailed},
	}
	for _, tc := range cases {
		if got := interpretAsk(tc.in); got.Status != tc.status {
			t.Errorf("%s: got %q, want %q", tc.name, got.Status, tc.status)
		}
	}
	broken := askResponse{}
	broken.BaseResp.Ret = 1
	if got := interpretAsk(broken); got.Status != StatusFailed {
		t.Errorf("session error should fail, got %q", got.Status)
	}
}

func TestLoginNeedsVerification(t *testing.T) {
	console := newFakeConsole()
	console.askBody = `{"acct_size":1,"base_resp":{"ret":0},"status":1}`
	console.loginBody = `{"base_resp":{"ret":0},"redirect_url":"/cgi-bin/readtemplate?t=user/validate_wx_tmpl&lang=zh_CN"}`
	server := httptest.NewServer(console.handler())
	defer server.Close()
	client := NewWithBaseURL(server.URL)
	if _, _, err := client.StartLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err := client.PollLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != StatusFailed || !strings.Contains(state.Message, "安全验证") {
		t.Fatalf("expected verification failure, got %+v", state)
	}
	if client.Connected() {
		t.Fatal("must not be connected without a token")
	}
}

func TestOfficialAccountListParsing(t *testing.T) {
	body := []byte(`{"base_resp":{"ret":0},"order":2,"scan_login_ctx":{"account_switch_permit":true},
		"biz_list":{"order_type":1,"list":[{"username":"gh_111","nickname":"公众号","headimgurl":"http://a/0","bizuin":1001}]},
		"service_biz_list":{"order_type":8,"list":[]},
		"wxa_list":{"order_type":2,"list":[{"username":"gh_222","nickname":"小程序","headimgurl":"","bizuin":1002}]}}`)
	list, err := parseAccountList(body, &Profile{Uin: "1002"})
	if err != nil {
		t.Fatal(err)
	}
	if !list.SwitchPermitted || len(list.Accounts) != 2 {
		t.Fatalf("unexpected list %+v", list)
	}
	first := list.Accounts[0]
	if first.Kind != "miniprogram" || first.UserName != "gh_222" || !first.Current {
		t.Fatalf("mini programs come first and the current one is marked: %+v", first)
	}
	if list.Accounts[1].Avatar != "https://a/0" || list.Accounts[1].Current {
		t.Fatalf("unexpected second account %+v", list.Accounts[1])
	}
}

func TestWxaListParsing(t *testing.T) {
	list, err := parseWxaList([]byte(`{"ret":0,"wax_list":[{"appid":"wx1","app_name":"A","username":"gh_1"}],"account_switch_permit":0}`), &Profile{AppID: "wx1"})
	if err != nil {
		t.Fatal(err)
	}
	if list.SwitchPermitted || !list.Accounts[0].Current || list.Accounts[0].Name != "A" {
		t.Fatalf("unexpected list %+v", list)
	}
	if _, err := parseWxaList([]byte(`{"ret":200002}`), nil); err == nil {
		t.Fatal("non-zero ret must fail")
	}
}

func TestCodePageParsing(t *testing.T) {
	body := []byte(`{"ret":0,"code_data":"{\"experience_info\":{\"basic_info\":{\"version\":\"1.2.0\",\"describe\":\"体验版\",\"time\":1700000000}},\"online_info\":{\"basic_info\":{\"version\":\"1.1.0\",\"describe\":\"线上版\",\"time\":1690000000}},\"develop_info\":{\"info_list\":[{},{}]}}"}`)
	state, err := parseCodePage(body)
	if err != nil {
		t.Fatal(err)
	}
	if state.ExperienceVer != "1.2.0" || state.ReleaseVersion != "1.1.0" || state.DevelopCount != 2 {
		t.Fatalf("unexpected state %+v", state)
	}
}

func TestSessionDropped(t *testing.T) {
	login, _ := url.Parse("https://mp.weixin.qq.com/")
	home, _ := url.Parse("https://mp.weixin.qq.com/cgi-bin/home?token=1")
	if !sessionDropped(login, nil) {
		t.Fatal("redirect to the login page means the session is gone")
	}
	if sessionDropped(home, []byte(`{"base_resp":{"ret":0}}`)) {
		t.Fatal("a normal response is not a dropped session")
	}
	if !sessionDropped(home, []byte(`{"base_resp":{"ret":200003}}`)) {
		t.Fatal("ret 200003 means the session expired")
	}
	if !sessionDropped(home, []byte(`{"ret":200003}`)) {
		t.Fatal("wxamp APIs report the expired session at the top level")
	}
}

func TestExportRestore(t *testing.T) {
	console := newFakeConsole()
	console.askBody = `{"acct_size":1,"base_resp":{"ret":0},"status":1}`
	server := httptest.NewServer(console.handler())
	defer server.Close()
	client := NewWithBaseURL(server.URL)
	if _, _, err := client.StartLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.PollLogin(context.Background()); err != nil {
		t.Fatal(err)
	}
	saved, err := client.Export()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Token != "654636700" || len(saved.Cookies) == 0 || saved.Fingerprint == "" {
		t.Fatalf("incomplete export %+v", saved)
	}
	restored := NewWithBaseURL(server.URL)
	if err := restored.Restore(saved); err != nil {
		t.Fatal(err)
	}
	profile, err := restored.Check(context.Background())
	if err != nil || profile.NickName != "修猫学习助手" {
		t.Fatalf("restored session should work: %+v %v", profile, err)
	}
}
