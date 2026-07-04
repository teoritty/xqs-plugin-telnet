package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
)

// ActiveSession holds runtime telnet session state for a single process.
type ActiveSession struct {
	Config  domain.ConnectionConfig
	Telnet  domain.TelnetSessionPort
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// Manager coordinates a single per-session plugin lifecycle.
type Manager struct {
	transport domain.TransportPort
	terminal  domain.TerminalPort
	factory   domain.TelnetSessionFactory
	log       domain.LoggerPort
	autologin AutoLoginRunner

	mu      sync.Mutex
	active  *ActiveSession
}

// AutoLoginRunner abstracts autologin to avoid circular imports.
type AutoLoginRunner interface {
	Run(ctx context.Context, session domain.TelnetSessionPort, cfg domain.AutoLoginConfig) error
}

// NewManager creates a session manager.
func NewManager(
	transport domain.TransportPort,
	terminal domain.TerminalPort,
	factory domain.TelnetSessionFactory,
	log domain.LoggerPort,
	autologin AutoLoginRunner,
) *Manager {
	return &Manager{
		transport: transport,
		terminal:  terminal,
		factory:   factory,
		log:       log,
		autologin: autologin,
	}
}

// Connect establishes a telnet session asynchronously.
func (m *Manager) Connect(ctx context.Context, cfg domain.ConnectionConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	m.Disconnect()

	sessionCtx, cancel := context.WithCancel(ctx)
	active := &ActiveSession{
		Config: cfg,
		cancel: cancel,
	}

	m.mu.Lock()
	m.active = active
	m.mu.Unlock()

	_ = m.terminal.UpdateState(sessionCtx, cfg.SessionID, domain.SessionConnecting, "")

	go m.runConnect(sessionCtx, active)
	return nil
}

func (m *Manager) runConnect(ctx context.Context, active *ActiveSession) {
	cfg := active.Config
	port := cfg.Port
	if port == 0 {
		port = 23
	}

	conn, err := m.transport.Dial(ctx, cfg.Host, port)
	if err != nil {
		m.log.Warn("dial failed", map[string]string{"reason": "transport"})
		_ = m.terminal.UpdateState(ctx, cfg.SessionID, domain.SessionError, domain.SanitizedDialError())
		m.cleanup(active)
		return
	}

	termCfg := cfg.TerminalConfigFromFields()
	telnetSession, err := m.factory.NewSession(conn, termCfg)
	if err != nil {
		_ = conn.Close()
		_ = m.terminal.UpdateState(ctx, cfg.SessionID, domain.SessionError, domain.SanitizedConnectError())
		m.cleanup(active)
		return
	}

	active.Telnet = telnetSession

	if err := telnetSession.Handshake(ctx); err != nil {
		_ = telnetSession.Close()
		_ = m.terminal.UpdateState(ctx, cfg.SessionID, domain.SessionError, domain.SanitizedConnectError())
		m.cleanup(active)
		return
	}

	if cfg.AutoLoginEnabled() && m.autologin != nil {
		autoCtx, autoCancel := context.WithTimeout(ctx, timeoutFromCfg(cfg))
		_ = m.autologin.Run(autoCtx, telnetSession, cfg.AutoLoginConfigFromConnection())
		autoCancel()
	}

	active.wg.Add(1)
	go func() {
		defer active.wg.Done()
		m.readPump(ctx, active)
	}()

	_ = m.terminal.UpdateState(ctx, cfg.SessionID, domain.SessionReady, "")
}

func (m *Manager) readPump(ctx context.Context, active *ActiveSession) {
	for {
		if ctx.Err() != nil {
			return
		}
		data, err := active.Telnet.ReadUserData(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			m.log.Warn("read pump stopped", map[string]string{"reason": "eof"})
			return
		}
		if len(data) == 0 {
			continue
		}

		if err := m.writeTerminalWithRetry(ctx, active.Config.SessionID, data); err != nil {
			return
		}
	}
}

func (m *Manager) writeTerminalWithRetry(ctx context.Context, sessionID domain.SessionID, data []byte) error {
	for attempts := 0; attempts < 5; attempts++ {
		err := m.terminal.WriteOutput(ctx, sessionID, data)
		if err == nil {
			return nil
		}
		if isRateLimited(err) {
			m.log.Warn("terminal backpressure", nil)
			time.Sleep(50 * time.Millisecond)
			continue
		}
		return err
	}
	return nil
}

// HandleInput forwards keyboard input to the telnet session.
func (m *Manager) HandleInput(ctx context.Context, data []byte) error {
	s := m.getActive()
	if s == nil || s.Telnet == nil {
		return domain.ErrSessionNotActive
	}
	return s.Telnet.WriteUserData(ctx, data)
}

// HandleResize updates NAWS window size.
func (m *Manager) HandleResize(cols, rows uint16) error {
	s := m.getActive()
	if s == nil || s.Telnet == nil {
		return domain.ErrSessionNotActive
	}
	return s.Telnet.SetWindowSize(cols, rows)
}

// Disconnect gracefully tears down the active session.
func (m *Manager) Disconnect() {
	m.mu.Lock()
	active := m.active
	m.active = nil
	m.mu.Unlock()

	if active == nil {
		return
	}

	if active.cancel != nil {
		active.cancel()
	}
	if active.Telnet != nil {
		_ = active.Telnet.Close()
	}
	active.wg.Wait()
	active.Config.ClearSecrets()
}

func (m *Manager) cleanup(active *ActiveSession) {
	m.mu.Lock()
	if m.active == active {
		m.active = nil
	}
	m.mu.Unlock()
	active.Config.ClearSecrets()
}

func (m *Manager) getActive() *ActiveSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

func timeoutFromCfg(cfg domain.ConnectionConfig) time.Duration {
	ms := cfg.AutoLoginConfigFromConnection().DelayMs
	if ms <= 0 {
		ms = 3000
	}
	return time.Duration(ms) * time.Millisecond
}

func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "32003") || contains(msg, "rate")
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}