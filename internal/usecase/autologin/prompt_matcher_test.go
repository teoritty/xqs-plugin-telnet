package autologin_test

import (
	"testing"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
	"github.com/teoritty/xqs-plugin-telnet/internal/usecase/autologin"
)

func TestMatcherDetectLoginAndPassword(t *testing.T) {
	m := autologin.NewMatcher(domain.AutoLoginConfig{
		LoginPrompt:    "login:",
		PasswordPrompt: "password:",
	})

	if got := m.Detect([]byte("Welcome\r\nlogin: ")); got != autologin.PromptLogin {
		t.Fatalf("expected login prompt, got %v", got)
	}
	if got := m.Detect([]byte("Password: ")); got != autologin.PromptPassword {
		t.Fatalf("expected password prompt, got %v", got)
	}
}
