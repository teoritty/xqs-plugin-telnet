package presentation

import (
	"context"
	"encoding/json"

	"github.com/teoritty/xqs-plugin-telnet/internal/infra/rpc"
	"github.com/teoritty/xqs-plugin-telnet/internal/usecase"
)

// SessionHandlers wires session-related IPC handlers.
type SessionHandlers struct {
	Manager *usecase.Manager
}

// HandleConnect processes session.connect RPC.
func (h SessionHandlers) HandleConnect(params json.RawMessage) (any, error) {
	cfg, err := MapConnectDTO(params)
	if err != nil {
		return nil, err
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), rpc.DialTimeout)
		defer cancel()
		_ = h.Manager.Connect(ctx, cfg)
	}()

	return map[string]bool{"accepted": true}, nil
}

// HandleWriteInput processes session.writeInput notification.
func (h SessionHandlers) HandleWriteInput(params json.RawMessage) {
	data, err := MapInputDTO(params)
	if err != nil || len(data) == 0 {
		return
	}
	payload := append([]byte(nil), data...)
	// Fire-and-forget: must not block the single-threaded RPC readLoop.
	go func() {
		ctx := context.Background()
		_ = h.Manager.HandleInput(ctx, payload)
	}()
}

// HandleResize processes session.resize notification.
func (h SessionHandlers) HandleResize(params json.RawMessage) {
	cols, rows, err := MapResizeDTO(params)
	if err != nil {
		return
	}
	go func() {
		_ = h.Manager.HandleResize(cols, rows)
	}()
}

// HandleDisconnect processes session.disconnect notification.
func (h SessionHandlers) HandleDisconnect(_ json.RawMessage) {
	h.Manager.Disconnect()
}
