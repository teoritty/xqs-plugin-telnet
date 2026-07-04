package logger

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
	"github.com/teoritty/xqs-plugin-telnet/internal/infra/rpc"
)

const maxLogsPerSecond = 50

var sensitiveKeys = map[string]struct{}{
	"password": {},
	"secret":   {},
	"token":    {},
	"credential": {},
}

// SecureLogger implements domain.LoggerPort via log.write RPC with redaction.
type SecureLogger struct {
	caller rpc.Caller
	mu     sync.Mutex
	count  int
	window time.Time
}

// NewSecureLogger creates a redacting logger.
func NewSecureLogger(caller rpc.Caller) *SecureLogger {
	return &SecureLogger{caller: caller, window: time.Now()}
}

// Info logs an informational message.
func (l *SecureLogger) Info(msg string, fields map[string]string) {
	l.write("info", msg, fields)
}

// Warn logs a warning message.
func (l *SecureLogger) Warn(msg string, fields map[string]string) {
	l.write("warn", msg, fields)
}

func (l *SecureLogger) write(level, msg string, fields map[string]string) {
	if !l.allow() {
		return
	}
	safe := redactFields(fields)
	// Fire-and-forget: log.write must never precede an inbound JSON-RPC response on
	// stdout. Lifecycle handlers defer logging via rpc.AfterResponder until after
	// the response frame is written.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), rpc.DefaultCallTimeout)
		defer cancel()
		_, _ = l.caller.CallCoreContext(ctx, "log.write", map[string]any{
			"level":   level,
			"message": msg,
			"fields":  safe,
		})
	}()
}

func (l *SecureLogger) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.window) >= time.Second {
		l.window = now
		l.count = 0
	}
	if l.count >= maxLogsPerSecond {
		return false
	}
	l.count++
	return true
}

func redactFields(fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]string, len(fields))
	for k, v := range fields {
		key := strings.ToLower(k)
		if _, ok := sensitiveKeys[key]; ok || strings.Contains(key, "pass") {
			out[k] = "[REDACTED]"
			continue
		}
		out[k] = v
	}
	return out
}

var _ domain.LoggerPort = (*SecureLogger)(nil)
