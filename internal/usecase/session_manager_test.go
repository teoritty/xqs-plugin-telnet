package usecase_test

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
	"github.com/teoritty/xqs-plugin-telnet/internal/usecase"
)

type mockTransport struct {
	conn io.ReadWriteCloser
	err  error
}

func (m *mockTransport) Dial(_ context.Context, _ string, _ int) (io.ReadWriteCloser, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.conn, nil
}

type mockTerminal struct {
	mu     sync.Mutex
	states []string
	output [][]byte
}

func (m *mockTerminal) WriteOutput(_ context.Context, _ domain.SessionID, data []byte) error {
	m.mu.Lock()
	m.output = append(m.output, append([]byte(nil), data...))
	m.mu.Unlock()
	return nil
}

func (m *mockTerminal) UpdateState(_ context.Context, _ domain.SessionID, state domain.SessionState, _ string) error {
	m.mu.Lock()
	m.states = append(m.states, string(state))
	m.mu.Unlock()
	return nil
}

func (m *mockTerminal) snapshot() (states []string, out []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, chunk := range m.output {
		out = append(out, chunk...)
	}
	return append([]string(nil), m.states...), out
}

type mockLogger struct{}

func (mockLogger) Info(string, map[string]string) {}
func (mockLogger) Warn(string, map[string]string) {}

type pipeConn struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func newPipeConn(payload []byte) *pipeConn {
	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write(payload)
	}()
	return &pipeConn{r: pr, w: pw}
}

func (p *pipeConn) Read(b []byte) (int, error)  { return p.r.Read(b) }
func (p *pipeConn) Write(b []byte) (int, error) { return p.w.Write(b) }
func (p *pipeConn) Close() error {
	_ = p.r.Close()
	return p.w.Close()
}

func TestManagerConnectReady(t *testing.T) {
	server := newPipeConn([]byte("Hello Telnet\r\n"))
	term := &mockTerminal{}
	factory := domainFactory{}

	mgr := usecase.NewManager(
		&mockTransport{conn: server},
		term,
		factory,
		mockLogger{},
		nil,
		nil,
	)

	cfg := domain.ConnectionConfig{
		SessionID: "s1",
		Host:      "127.0.0.1",
		Port:      23,
		Protocol:  "telnet",
	}

	if err := mgr.Connect(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		states, out := term.snapshot()
		if containsState(states, "ready") && len(out) > 0 {
			mgr.Disconnect()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	mgr.Disconnect()
	t.Fatal("session not ready")
}

func containsState(states []string, target string) bool {
	for _, s := range states {
		if s == target {
			return true
		}
	}
	return false
}

// domainFactory uses real telnet factory via thin wrapper in test - import infra forbidden in usecase test.
// Use minimal inline telnet session for test.

type domainFactory struct{}

func (domainFactory) NewSession(conn io.ReadWriteCloser, cfg domain.TerminalConfig) (domain.TelnetSessionPort, error) {
	return &simpleTelnet{conn: conn, cfg: cfg}, nil
}

type simpleTelnet struct {
	conn io.ReadWriteCloser
	cfg  domain.TerminalConfig
}

func (s *simpleTelnet) Handshake(context.Context) error { return nil }
func (s *simpleTelnet) ReadUserData(context.Context) ([]byte, error) {
	buf := make([]byte, 1024)
	n, err := s.conn.Read(buf)
	if n > 0 {
		return buf[:n], nil
	}
	return nil, err
}
func (s *simpleTelnet) WriteUserData(_ context.Context, data []byte) error {
	_, err := s.conn.Write(data)
	return err
}
func (s *simpleTelnet) SetWindowSize(_, _ uint16) error { return nil }
func (s *simpleTelnet) KeepAlive() error                 { return nil }
func (s *simpleTelnet) Close() error                    { return s.conn.Close() }

type idleTelnet struct {
	simpleTelnet
	idleReads int
}

func (s *idleTelnet) ReadUserData(ctx context.Context) ([]byte, error) {
	if s.idleReads > 0 {
		return s.simpleTelnet.ReadUserData(ctx)
	}
	s.idleReads++
	return nil, context.DeadlineExceeded
}

func TestManagerReadPumpSurvivesIdleRead(t *testing.T) {
	server := newPipeConn([]byte("prompt> "))
	term := &mockTerminal{}
	factory := idleFactory{}

	mgr := usecase.NewManager(
		&mockTransport{conn: server},
		term,
		factory,
		mockLogger{},
		nil,
		nil,
	)

	cfg := domain.ConnectionConfig{
		SessionID: "s1",
		Host:      "127.0.0.1",
		Port:      23,
		Protocol:  "telnet",
	}

	if err := mgr.Connect(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		states, _ := term.snapshot()
		if containsState(states, "error") {
			mgr.Disconnect()
			t.Fatal("session entered error on idle read")
		}
		if containsState(states, "ready") {
			time.Sleep(200 * time.Millisecond)
			states, _ = term.snapshot()
			if containsState(states, "error") {
				mgr.Disconnect()
				t.Fatal("session entered error after ready")
			}
			mgr.Disconnect()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	mgr.Disconnect()
	t.Fatal("session not ready")
}

type idleFactory struct{}

func (idleFactory) NewSession(conn io.ReadWriteCloser, cfg domain.TerminalConfig) (domain.TelnetSessionPort, error) {
	return &idleTelnet{simpleTelnet: simpleTelnet{conn: conn, cfg: cfg}}, nil
}
