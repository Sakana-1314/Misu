package mpweb

import (
	"encoding/json"
	"testing"
)

func TestVersionStateParsing(t *testing.T) {
	codeData := `{
		"experience_info": {"basic_info": {"version": "1.2.0", "describe": "体验版", "time": 1700000000}},
		"online_info": {"basic_info": {"version": "1.1.0", "describe": "线上版", "time": 1690000000}},
		"develop_info": {"info_list": [{}, {}]},
		"qr_path": "pages/index/index"
	}`
	raw, err := json.Marshal(map[string]any{"ret": 0, "code_data": codeData})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Ret      int    `json:"ret"`
		CodeData string `json:"code_data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.CodeData == "" {
		t.Fatal("code_data should be present")
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
	if err := json.Unmarshal([]byte(payload.CodeData), &code); err != nil {
		t.Fatal(err)
	}
	if code.Experience.Basic.Version != "1.2.0" || code.Online.Basic.Version != "1.1.0" {
		t.Fatalf("unexpected versions: %+v", code)
	}
	if len(code.Develop.List) != 2 {
		t.Fatalf("expected 2 develop entries, got %d", len(code.Develop.List))
	}
}

func TestAccountListParsing(t *testing.T) {
	body := []byte(`{"ret":0,"wax_list":[{"appid":"wx821c8dd7888db38a","app_name":"修猫学习助手 Lite","app_headimg":"http://wx.qlogo.cn/x/0","username":"gh_a262422ef9bf","type":1}]}`)
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
		t.Fatal(err)
	}
	if len(payload.List) != 1 {
		t.Fatalf("expected 1 account, got %d", len(payload.List))
	}
	if payload.List[0].AppID != "wx821c8dd7888db38a" {
		t.Fatalf("unexpected appid %q", payload.List[0].AppID)
	}
}

func TestTokenExtraction(t *testing.T) {
	cases := map[string]string{
		"https://mp.weixin.qq.com/cgi-bin/home?t=home/index&lang=zh_CN&token=654636700": "654636700",
		"/cgi-bin/home?token=123&lang=zh_CN":                                           "123",
		"https://mp.weixin.qq.com/":                                                    "",
	}
	for input, want := range cases {
		got := ""
		if match := tokenRegexp.FindStringSubmatch(input); len(match) > 1 {
			got = match[1]
		}
		if got != want {
			t.Fatalf("token from %q = %q, want %q", input, got, want)
		}
	}
}

func TestUUIDExtraction(t *testing.T) {
	body := []byte(`{"base_resp":{"err_msg":"ok","ret":0},"uuid":"dadd9248f6ece3f9418031af8f331ba3"}`)
	match := uuidRegexp.FindSubmatch(body)
	if len(match) == 0 {
		t.Fatal("uuid not found")
	}
	if string(match[1]) != "dadd9248f6ece3f9418031af8f331ba3" {
		t.Fatalf("unexpected uuid %q", string(match[1]))
	}
}

func TestLoginPageDetection(t *testing.T) {
	if !looksLikeLoginPage([]byte(`<html><script src="loginpage.js"></script></html>`)) {
		t.Fatal("login page should be detected")
	}
	long := make([]byte, 5000)
	if looksLikeLoginPage(long) {
		t.Fatal("large payload is not a login page")
	}
}
