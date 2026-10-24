package service

import (
	"testing"

	"misu/internal/wechat"
)

func TestRenderPlaceholders(t *testing.T) {
	out := renderPlaceholders(`{"appid":"{{appid}}","name":"{{nick_name}}","v":"{{user_version}}"}`, "wx123", "测试小程序", "1.0.0")
	want := `{"appid":"wx123","name":"测试小程序","v":"1.0.0"}`
	if out != want {
		t.Fatalf("got %s want %s", out, want)
	}
}

func TestValidDomain(t *testing.T) {
	valid := []string{"example.com", "api.example.com:8080", "sub.example.com"}
	for _, d := range valid {
		if !validDomain(d) {
			t.Fatalf("expected valid: %s", d)
		}
	}
	invalid := []string{"http://example.com", "https://example.com/path", "1.2.3.4", "localhost", "example .com", "example.com/path"}
	for _, d := range invalid {
		if validDomain(d) {
			t.Fatalf("expected invalid: %s", d)
		}
	}
}

func TestValidateDomains(t *testing.T) {
	if err := validateDomains(wechat.DomainRequest{Action: "add", RequestDomain: []string{"example.com"}}); err != nil {
		t.Fatal(err)
	}
	if err := validateDomains(wechat.DomainRequest{Action: "add"}); err == nil {
		t.Fatal("empty domains should fail")
	}
	if err := validateDomains(wechat.DomainRequest{Action: "add", RequestDomain: []string{"bad domain"}}); err == nil {
		t.Fatal("bad domain should fail")
	}
}
