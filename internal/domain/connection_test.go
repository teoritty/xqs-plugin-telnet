package domain_test

import (
	"testing"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
)

func TestConnectionConfigValidate(t *testing.T) {
	cfg := domain.ConnectionConfig{Host: "example.com", Port: 23, Protocol: "telnet"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}

	cfg.Host = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty host")
	}
}

func TestAutoLoginEnabledOptIn(t *testing.T) {
	cfg := domain.ConnectionConfig{
		Fields: map[string]string{domain.FieldAutoLogin: "false"},
	}
	if cfg.AutoLoginEnabled() {
		t.Fatal("auto login should be off by default")
	}

	cfg.Fields[domain.FieldAutoLogin] = "true"
	if !cfg.AutoLoginEnabled() {
		t.Fatal("auto login should be enabled when opted in")
	}
}

func TestClearSecrets(t *testing.T) {
	cfg := domain.ConnectionConfig{
		Fields: map[string]string{domain.FieldPassword: "secret"},
	}
	cfg.ClearSecrets()
	if _, ok := cfg.Fields[domain.FieldPassword]; ok {
		t.Fatal("password field should be removed")
	}
}
