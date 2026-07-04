package domain

import (
	"strconv"
	"strings"
)

// Allowed field IDs declared in plugin.json manifest.
const (
	FieldUsername      = "username"
	FieldPassword      = "password"
	FieldAutoLogin     = "autoLogin"
	FieldTerminalType  = "terminalType"
	FieldBinaryMode    = "binaryMode"
	FieldLoginPrompt   = "loginPrompt"
	FieldPasswordPrompt = "passwordPrompt"
	FieldLoginDelayMs  = "loginDelayMs"
)

// ConnectionConfig is the session-scoped connection value object.
type ConnectionConfig struct {
	SessionID    SessionID
	ConnectionID string
	Protocol     string
	Host         string
	Port         int
	Username     string
	Fields       map[string]string
}

// Password returns the secret password field (session-scoped, in-memory only).
func (c ConnectionConfig) Password() string {
	if c.Fields == nil {
		return ""
	}
	return c.Fields[FieldPassword]
}

// AutoLoginEnabled reports whether credential auto-injection is opted in.
func (c ConnectionConfig) AutoLoginEnabled() bool {
	return parseBoolField(c.Fields, FieldAutoLogin)
}

// TerminalConfigFromFields builds telnet options from manifest field values.
func (c ConnectionConfig) TerminalConfigFromFields() TerminalConfig {
	cfg := DefaultTerminalConfig()
	if c.Fields == nil {
		return cfg
	}
	if t := strings.TrimSpace(c.Fields[FieldTerminalType]); t != "" {
		cfg.TerminalType = t
	}
	cfg.BinaryMode = parseBoolField(c.Fields, FieldBinaryMode)
	return cfg
}

// AutoLoginConfig holds prompt-matching settings for auto-login.
type AutoLoginConfig struct {
	Username       string
	Password       string
	LoginPrompt    string
	PasswordPrompt string
	DelayMs        int
}

// AutoLoginConfigFromConnection builds auto-login settings from connection fields.
func (c ConnectionConfig) AutoLoginConfigFromConnection() AutoLoginConfig {
	cfg := AutoLoginConfig{
		Username:       firstNonEmpty(c.Username, fieldOrEmpty(c.Fields, FieldUsername)),
		Password:       c.Password(),
		LoginPrompt:    "login:",
		PasswordPrompt: "password:",
		DelayMs:        3000,
	}
	if c.Fields == nil {
		return cfg
	}
	if p := strings.TrimSpace(c.Fields[FieldLoginPrompt]); p != "" {
		cfg.LoginPrompt = p
	}
	if p := strings.TrimSpace(c.Fields[FieldPasswordPrompt]); p != "" {
		cfg.PasswordPrompt = p
	}
	if v := strings.TrimSpace(c.Fields[FieldLoginDelayMs]); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.DelayMs = n
		}
	}
	return cfg
}

// Validate checks domain rules for connection parameters.
func (c ConnectionConfig) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return ErrInvalidConnection
	}
	if c.Port != 0 && (c.Port < 1 || c.Port > 65535) {
		return ErrInvalidConnection
	}
	if c.Protocol != "" && c.Protocol != "telnet" {
		return ErrInvalidConnection
	}
	return nil
}

// ClearSecrets zeroes secret field values in the fields map.
func (c *ConnectionConfig) ClearSecrets() {
	if c == nil || c.Fields == nil {
		return
	}
	if pw, ok := c.Fields[FieldPassword]; ok {
		b := []byte(pw)
		clear(b)
		c.Fields[FieldPassword] = ""
		delete(c.Fields, FieldPassword)
	}
}

func parseBoolField(fields map[string]string, key string) bool {
	if fields == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(fields[key]))
	return v == "true" || v == "1" || v == "yes"
}

func fieldOrEmpty(fields map[string]string, key string) string {
	if fields == nil {
		return ""
	}
	return fields[key]
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
