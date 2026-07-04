package netproxy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
	"github.com/teoritty/xqs-plugin-telnet/internal/infra/rpc"
)

const (
	maxReadChunk  = 256 << 10
	maxHandles    = 8
	defaultNetRPC = "tcp"
)

// Dialer implements domain.TransportPort via host net.* RPC.
type Dialer struct {
	caller rpc.Caller
	mu     sync.Mutex
	open   int
}

// NewDialer creates a transport dialer.
func NewDialer(caller rpc.Caller) *Dialer {
	return &Dialer{caller: caller}
}

// Dial opens a TCP connection through the host capability gate.
func (d *Dialer) Dial(ctx context.Context, host string, port int) (io.ReadWriteCloser, error) {
	d.mu.Lock()
	if d.open >= maxHandles {
		d.mu.Unlock()
		return nil, fmt.Errorf("too many open network handles")
	}
	d.open++
	d.mu.Unlock()

	dialCtx, cancel := context.WithTimeout(ctx, rpc.DialTimeout)
	defer cancel()

	raw, err := d.caller.CallCoreContext(dialCtx, "net.dial", map[string]any{
		"network": defaultNetRPC,
		"host":    host,
		"port":    port,
	})
	if err != nil {
		d.decOpen()
		return nil, err
	}

	var resp struct {
		HandleID string `json:"handleId"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || resp.HandleID == "" {
		d.decOpen()
		return nil, fmt.Errorf("invalid net.dial response")
	}

	return &conn{
		caller:   d.caller,
		handleID: resp.HandleID,
		onClose:  d.decOpen,
	}, nil
}

func (d *Dialer) decOpen() {
	d.mu.Lock()
	if d.open > 0 {
		d.open--
	}
	d.mu.Unlock()
}

type conn struct {
	caller   rpc.Caller
	handleID string
	onClose  func()
	closed   bool
	mu       sync.Mutex
}

func (c *conn) Read(p []byte) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, io.EOF
	}
	c.mu.Unlock()

	maxBytes := len(p)
	if maxBytes > maxReadChunk {
		maxBytes = maxReadChunk
	}
	if maxBytes == 0 {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), rpc.NetIOTimeout)
	defer cancel()
	raw, err := c.caller.CallCoreContext(ctx, "net.read", map[string]any{
		"handleId": c.handleID,
		"maxBytes": maxBytes,
	})
	if err != nil {
		return 0, err
	}

	var resp struct {
		ContentBase64 string `json:"contentBase64"`
		EOF           bool   `json:"eof"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, err
	}

	data, err := base64.StdEncoding.DecodeString(resp.ContentBase64)
	if err != nil {
		return 0, err
	}
	n := copy(p, data)
	if resp.EOF && n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func (c *conn) Write(p []byte) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), rpc.NetIOTimeout)
	defer cancel()
	_, err := c.caller.CallCoreContext(ctx, "net.write", map[string]any{
		"handleId":      c.handleID,
		"contentBase64": base64.StdEncoding.EncodeToString(p),
	})
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *conn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), rpc.NetIOTimeout)
	defer cancel()
	_, _ = c.caller.CallCoreContext(ctx, "net.close", map[string]string{
		"handleId": c.handleID,
	})
	if c.onClose != nil {
		c.onClose()
	}
	return nil
}

var _ domain.TransportPort = (*Dialer)(nil)
var _ io.ReadWriteCloser = (*conn)(nil)
