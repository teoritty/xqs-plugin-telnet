package autologin

import (
	"strings"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
)

// Matcher detects login/password prompts in terminal output.
type Matcher struct {
	loginPrompt    string
	passwordPrompt string
}

// NewMatcher creates a prompt matcher.
func NewMatcher(cfg domain.AutoLoginConfig) *Matcher {
	login := strings.ToLower(cfg.LoginPrompt)
	password := strings.ToLower(cfg.PasswordPrompt)
	if login == "" {
		login = "login:"
	}
	if password == "" {
		password = "password:"
	}
	return &Matcher{loginPrompt: login, passwordPrompt: password}
}

// PromptKind classifies detected prompts.
type PromptKind int

const (
	PromptNone PromptKind = iota
	PromptLogin
	PromptPassword
)

// Detect finds the last prompt in accumulated output.
func (m *Matcher) Detect(output []byte) PromptKind {
	lower := strings.ToLower(string(output))
	if strings.Contains(lower, m.passwordPrompt) {
		return PromptPassword
	}
	if strings.Contains(lower, m.loginPrompt) || strings.Contains(lower, "username:") {
		return PromptLogin
	}
	return PromptNone
}
