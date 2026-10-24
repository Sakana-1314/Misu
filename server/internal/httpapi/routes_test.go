package httpapi

import (
	"log/slog"
	"strings"
	"testing"

	"misu/internal/config"
)

func TestRouteRegistration(t *testing.T) {
	cfg := &config.Config{
		PublicBaseURL: "http://localhost:8080",
		JWTSecret:     strings.Repeat("x", 32),
		AdminPassword: "test-password",
	}
	s, err := New(cfg, slog.Default(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("engine setup panicked: %v", recovered)
		}
	}()
	if s.Engine() == nil {
		t.Fatal("engine is nil")
	}
}
