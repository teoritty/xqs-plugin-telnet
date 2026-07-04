package terminal

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
	"github.com/teoritty/xqs-plugin-telnet/internal/infra/rpc"
)

// Adapter implements domain.TerminalPort via session.writeTerminal and session.updateState.
type Adapter struct {
	caller rpc.Caller
}

// NewAdapter creates a terminal port adapter.
func NewAdapter(caller rpc.Caller) *Adapter {
	return &Adapter{caller: caller}
}

// WriteOutput sends terminal bytes to the host UI.
func (a *Adapter) WriteOutput(ctx context.Context, sessionID domain.SessionID, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	_, err := a.caller.CallCoreContext(ctx, "session.writeTerminal", map[string]string{
		"sessionId":    string(sessionID),
		"outputBase64": base64.StdEncoding.EncodeToString(data),
	})
	return err
}

// UpdateState reports session lifecycle state to the host.
func (a *Adapter) UpdateState(ctx context.Context, sessionID domain.SessionID, state domain.SessionState, errMsg string) error {
	params := map[string]string{
		"sessionId": string(sessionID),
		"state":     string(state),
	}
	if errMsg != "" {
		params["error"] = errMsg
	}
	_, err := a.caller.CallCoreContext(ctx, "session.updateState", params)
	if err != nil {
		return fmt.Errorf("update state: %w", err)
	}
	return nil
}

// ParseRateLimit reports whether an RPC error is terminal backpressure (-32003).
func ParseRateLimit(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "32003") || strings.Contains(msg, "rate")
}

var _ domain.TerminalPort = (*Adapter)(nil)
