package wechat

import (
	"encoding/base64"
	"strings"
	"testing"
)

func testAESKey() string {
	key := []byte("0123456789abcdef0123456789abcdef")
	return strings.TrimRight(base64.StdEncoding.EncodeToString(key), "=")
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := testAESKey()
	if len(key) != 43 {
		t.Fatalf("test key length = %d", len(key))
	}
	plain := `<xml><AppId>wx123</AppId><InfoType>component_verify_ticket</InfoType><ComponentVerifyTicket>ticket</ComponentVerifyTicket></xml>`
	encrypted, err := EncryptMessage(key, plain, "wx123")
	if err != nil {
		t.Fatal(err)
	}
	got, appID, err := DecryptMessage(key, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != plain {
		t.Fatalf("plaintext mismatch: %q", string(got))
	}
	if appID != "wx123" {
		t.Fatalf("appid mismatch: %q", appID)
	}
}

func TestURLSignature(t *testing.T) {
	sig := URLSignature("token", "1700000000", "nonce")
	if !VerifyURL("token", sig, "1700000000", "nonce") {
		t.Fatal("valid signature rejected")
	}
	if VerifyURL("token", sig, "1700000001", "nonce") {
		t.Fatal("invalid signature accepted")
	}
}

func TestMessageSignature(t *testing.T) {
	sig := MessageSignature("token", "1700000000", "nonce", "cipher")
	if !VerifyMessage("token", sig, "1700000000", "nonce", "cipher") {
		t.Fatal("valid message signature rejected")
	}
	if VerifyMessage("token", sig, "1700000000", "nonce", "other") {
		t.Fatal("invalid message signature accepted")
	}
}

func TestParseEvent(t *testing.T) {
	body := []byte(`<xml><AppId>wxc</AppId><CreateTime>1700000000</CreateTime><InfoType>authorized</InfoType><AuthorizerAppid>wxa</AuthorizerAppid><AuthorizationCode>code</AuthorizationCode></xml>`)
	msg, err := ParseEvent(body)
	if err != nil {
		t.Fatal(err)
	}
	if msg.AuthorizerAppID != "wxa" || msg.AuthorizationCode != "code" || msg.InfoType != "authorized" {
		t.Fatalf("unexpected message: %+v", msg)
	}
}
