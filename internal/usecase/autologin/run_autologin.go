package autologin

import (
	"context"
	"time"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
)

// Runner performs opt-in credential auto-login.
type Runner struct {
	log domain.LoggerPort
}

// NewRunner creates an auto-login runner.
func NewRunner(log domain.LoggerPort) *Runner {
	return &Runner{log: log}
}

// Run waits for prompts and sends credentials when auto-login is enabled.
func (r *Runner) Run(ctx context.Context, session domain.TelnetSessionPort, cfg domain.AutoLoginConfig) error {
	if stringsTrim(cfg.Username) == "" && stringsTrim(cfg.Password) == "" {
		return nil
	}

	matcher := NewMatcher(cfg)
	deadline := time.Now().Add(time.Duration(cfg.DelayMs) * time.Millisecond)
	var buf []byte
	sentLogin := false
	sentPassword := false

	r.log.Info("auto-login started", map[string]string{"phase": "begin"})

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		chunk, err := session.ReadUserData(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			break
		}
		if len(chunk) > 0 {
			buf = append(buf, chunk...)
		}

		switch matcher.Detect(buf) {
		case PromptLogin:
			if !sentLogin && stringsTrim(cfg.Username) != "" {
				if err := session.WriteUserData(ctx, []byte(cfg.Username+"\r\n")); err != nil {
					return err
				}
				sentLogin = true
				buf = buf[:0]
			}
		case PromptPassword:
			if !sentPassword && stringsTrim(cfg.Password) != "" {
				if err := session.WriteUserData(ctx, []byte(cfg.Password+"\r\n")); err != nil {
					return err
				}
				sentPassword = true
				buf = buf[:0]
			}
		}

		if sentLogin && (stringsTrim(cfg.Password) == "" || sentPassword) {
			r.log.Info("auto-login completed", map[string]string{"phase": "done"})
			return nil
		}

		if len(chunk) == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}

	r.log.Warn("auto-login timed out", map[string]string{"phase": "timeout"})
	return nil
}

func stringsTrim(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
}
