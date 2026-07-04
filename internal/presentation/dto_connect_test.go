package presentation_test

import (
	"encoding/json"
	"testing"

	"github.com/teoritty/xqs-plugin-telnet/internal/presentation"
)

func TestMapConnectDTO(t *testing.T) {
	raw := json.RawMessage(`{
		"sessionId": "s1",
		"connectionId": "c1",
		"protocol": "telnet",
		"host": "router.local",
		"port": 23,
		"fields": {
			"username": "admin",
			"password": "pw",
			"unknown": "ignored"
		}
	}`)

	cfg, err := presentation.MapConnectDTO(raw)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "router.local" {
		t.Fatalf("host %q", cfg.Host)
	}
	if cfg.Password() != "pw" {
		t.Fatal("password not mapped")
	}
	if _, ok := cfg.Fields["unknown"]; ok {
		t.Fatal("unknown field should be stripped")
	}
}
