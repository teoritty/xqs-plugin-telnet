package logger_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/teoritty/xqs-plugin-telnet/internal/infra/logger"
	"github.com/teoritty/xqs-plugin-telnet/internal/infra/rpc"
)

type recordingCaller struct {
	mu    sync.Mutex
	calls []map[string]any
}

func (r *recordingCaller) CallCoreContext(_ context.Context, method string, params any) ([]byte, error) {
	if method != "log.write" {
		return []byte(`{"ok":true}`), nil
	}
	data, _ := json.Marshal(params)
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	r.mu.Lock()
	r.calls = append(r.calls, m)
	r.mu.Unlock()
	return []byte(`{"ok":true}`), nil
}

func TestSecureLoggerRedactsPassword(t *testing.T) {
	caller := &recordingCaller{}
	log := logger.NewSecureLogger(caller)
	log.Info("test", map[string]string{"password": "secret-value"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		caller.mu.Lock()
		n := len(caller.calls)
		caller.mu.Unlock()
		if n >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	caller.mu.Lock()
	defer caller.mu.Unlock()
	if len(caller.calls) != 1 {
		t.Fatal("expected one log call")
	}
	fields, ok := caller.calls[0]["fields"].(map[string]any)
	if !ok {
		t.Fatal("missing fields")
	}
	if fields["password"] != "[REDACTED]" {
		t.Fatalf("got %v", fields["password"])
	}
}

var _ rpc.Caller = (*recordingCaller)(nil)
