package rpc_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/teoritty/xqs-plugin-telnet/internal/infra/rpc"
)

// TestHostNoDeadlockUnderConcurrentOutgoingCalls reproduces the activate +
// session.connect + concurrent log.write / session.updateState IPC pattern.
// The mock core sends an incoming ping before answering each outbound CallCore,
// which deadlocks when readLoop handles requests synchronously.
func TestHostNoDeadlockUnderConcurrentOutgoingCalls(t *testing.T) {
	t.Parallel()

	coreToPluginR, coreToPluginW := io.Pipe()
	pluginToCoreR, pluginToCoreW := io.Pipe()

	host := rpc.NewHostFromStreams(coreToPluginR, pluginToCoreW)
	caller := rpc.HostCaller{Host: host}

	host.Register("ping", func(_ json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})

	host.Register("activate", func(_ json.RawMessage) (any, error) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), rpc.DefaultCallTimeout)
			defer cancel()
			_, _ = caller.CallCoreContext(ctx, "log.write", map[string]any{
				"level":   "info",
				"message": "plugin activated",
			})
		}()
		return map[string]bool{"ok": true}, nil
	})

	host.Register("session.connect", func(_ json.RawMessage) (any, error) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), rpc.DefaultCallTimeout)
			defer cancel()
			_, _ = caller.CallCoreContext(ctx, "session.updateState", map[string]string{
				"sessionId": "s1",
				"state":     "connecting",
			})
		}()
		return map[string]bool{"accepted": true}, nil
	})

	var coreNextID atomic.Int64
	coreNextID.Store(1000)

	fromPlugin := bufio.NewReader(pluginToCoreR)
	incoming := make(chan rpc.Message, 16)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		defer close(incoming)
		for {
			msg, err := readTestMessage(fromPlugin)
			if err != nil {
				return
			}
			incoming <- msg
		}
	}()

	var coreMu sync.Mutex
	writeCore := func(msg rpc.Message) error {
		coreMu.Lock()
		defer coreMu.Unlock()
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if _, err := coreToPluginW.Write(data); err != nil {
			return err
		}
		_, err = coreToPluginW.Write([]byte{'\n'})
		return err
	}

	sendCoreRequest := func(id int64, method string, params any) error {
		var rawParams json.RawMessage
		if params != nil {
			data, err := json.Marshal(params)
			if err != nil {
				return err
			}
			rawParams = data
		}
		return writeCore(rpc.Message{
			JSONRPC: rpc.JSONRPCVersion,
			ID:      &id,
			Method:  method,
			Params:  rawParams,
		})
	}

	respondCore := func(id int64, result any) error {
		data, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return writeCore(rpc.Message{
			JSONRPC: rpc.JSONRPCVersion,
			ID:      &id,
			Result:  data,
		})
	}

	var outboundPending atomic.Int32

	var respondOutbound func(*testing.T, rpc.Message)
	respondOutbound = func(t *testing.T, msg rpc.Message) {
		t.Helper()
		outboundPending.Add(1)
		defer outboundPending.Add(-1)

		pingID := coreNextID.Add(1)
		if err := sendCoreRequest(pingID, "ping", nil); err != nil {
			t.Fatalf("send ping: %v", err)
		}
		if !waitCoreResponse(t, incoming, pingID, respondOutbound) {
			t.Fatalf("timed out waiting for ping response id=%d", pingID)
		}

		if err := respondCore(*msg.ID, map[string]bool{"ok": true}); err != nil {
			t.Fatalf("respond outbound: %v", err)
		}
	}

	if err := sendCoreRequest(1, "activate", map[string]string{"reason": "tab"}); err != nil {
		t.Fatal(err)
	}
	if !waitCoreResponse(t, incoming, 1, respondOutbound) {
		t.Fatal("timed out waiting for activate response")
	}

	if err := sendCoreRequest(2, "session.connect", map[string]string{"sessionId": "s1"}); err != nil {
		t.Fatal(err)
	}
	if !waitCoreResponse(t, incoming, 2, respondOutbound) {
		t.Fatal("timed out waiting for session.connect response")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
	drain:
		for {
			select {
			case msg, ok := <-incoming:
				if !ok {
					break drain
				}
				if msg.ID != nil && msg.Method != "" {
					respondOutbound(t, msg)
				}
			default:
				break drain
			}
		}
		if outboundPending.Load() == 0 {
			time.Sleep(100 * time.Millisecond)
			if outboundPending.Load() == 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if outboundPending.Load() != 0 {
		t.Fatalf("outbound calls still pending after timeout")
	}

	_ = coreToPluginW.Close()
	_ = pluginToCoreW.Close()
	<-readerDone
}

func waitCoreResponse(
	t *testing.T,
	ch <-chan rpc.Message,
	expectID int64,
	onOutbound func(*testing.T, rpc.Message),
) bool {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			return false
		case msg, ok := <-ch:
			if !ok {
				return false
			}
			if msg.ID != nil && msg.Method != "" {
				onOutbound(t, msg)
				continue
			}
			if msg.ID != nil && *msg.ID == expectID && msg.Method == "" {
				return true
			}
		}
	}
}

func readTestMessage(r *bufio.Reader) (rpc.Message, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return rpc.Message{}, err
	}
	var msg rpc.Message
	if err := json.Unmarshal(line, &msg); err != nil {
		return rpc.Message{}, err
	}
	return msg, nil
}

// TestHostInitializeResponseBeforeLogWrite ensures the JSON-RPC response to
// initialize is written before any outbound log.write from AfterResponse hooks.
func TestHostInitializeResponseBeforeLogWrite(t *testing.T) {
	t.Parallel()

	coreToPluginR, coreToPluginW := io.Pipe()
	pluginToCoreR, pluginToCoreW := io.Pipe()

	host := rpc.NewHostFromStreams(coreToPluginR, pluginToCoreW)
	caller := rpc.HostCaller{Host: host}

	host.Register("initialize", func(_ json.RawMessage) (any, error) {
		return testInitResult{
			after: func() {
				ctx, cancel := context.WithTimeout(context.Background(), rpc.DefaultCallTimeout)
				defer cancel()
				_, _ = caller.CallCoreContext(ctx, "log.write", map[string]any{
					"level":   "info",
					"message": "plugin initialized",
				})
			},
		}, nil
	})

	fromPlugin := bufio.NewReader(pluginToCoreR)
	var coreMu sync.Mutex
	writeCore := func(msg rpc.Message) error {
		coreMu.Lock()
		defer coreMu.Unlock()
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if _, err := coreToPluginW.Write(data); err != nil {
			return err
		}
		_, err = coreToPluginW.Write([]byte{'\n'})
		return err
	}

	initID := int64(1)
	if err := writeCore(rpc.Message{
		JSONRPC: rpc.JSONRPCVersion,
		ID:      &initID,
		Method:  "initialize",
		Params:  json.RawMessage(`{"pluginId":"io.xquakshell.plugin.telnet"}`),
	}); err != nil {
		t.Fatal(err)
	}

	var firstMethod string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := readTestMessage(fromPlugin)
		if err != nil {
			t.Fatalf("read plugin stdout: %v", err)
		}
		if msg.ID != nil && msg.Method != "" {
			firstMethod = msg.Method
			if err := writeCore(rpc.Message{
				JSONRPC: rpc.JSONRPCVersion,
				ID:      msg.ID,
				Result:  json.RawMessage(`{"ok":true}`),
			}); err != nil {
				t.Fatal(err)
			}
			break
		}
		if msg.ID != nil && *msg.ID == initID && msg.Method == "" {
			firstMethod = ""
			break
		}
	}

	if firstMethod == "log.write" {
		t.Fatal("log.write reached core before initialize JSON-RPC response")
	}

	_ = coreToPluginW.Close()
	_ = pluginToCoreW.Close()
}

type testInitResult struct {
	after func()
}

func (r testInitResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]bool{"ok": true})
}

func (r testInitResult) AfterResponse() {
	if r.after != nil {
		r.after()
	}
}
