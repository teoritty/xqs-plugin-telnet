package usecase

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
	"github.com/teoritty/xqs-plugin-telnet/internal/infra/rpc"
)

const (
	initDrainTimeout        = 2 * time.Second
	stateUpdateRetries      = 8
	stateUpdateBackoff      = 100 * time.Millisecond
	stateUpdateBackoffMax   = 500 * time.Millisecond
	dialRetryInitial        = 200 * time.Millisecond
	dialRetryMax            = 500 * time.Millisecond
	telnetKeepaliveInterval = 30 * time.Second
	hostPingInterval        = 2 * time.Minute
	keepaliveFailThreshold  = 3
)

// ActiveSession holds runtime telnet session state for a single process.
type ActiveSession struct {
	Config     domain.ConnectionConfig
	Telnet     domain.TelnetSessionPort
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	connectGen uint64
}

// Manager coordinates a single per-session plugin lifecycle.
type Manager struct {
	transport domain.TransportPort
	terminal  domain.TerminalPort
	factory   domain.TelnetSessionFactory
	log       domain.LoggerPort
	autologin AutoLoginRunner
	host      rpc.Caller

	mu         sync.Mutex
	active     *ActiveSession
	connectGen uint64
	stateMu    sync.Mutex
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
	host rpc.Caller,
) *Manager {
	return &Manager{
		transport: transport,
		terminal:  terminal,
		factory:   factory,
		log:       log,
		autologin: autologin,
		host:      host,
	}
}

// Connect establishes a telnet session asynchronously.
func (m *Manager) Connect(ctx context.Context, cfg domain.ConnectionConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	if cur := m.active; cur != nil && cur.Config.SessionID == cfg.SessionID && cur.Telnet == nil {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	m.Disconnect()

	m.mu.Lock()
	m.connectGen++
	gen := m.connectGen
	// Connect owns its deadline; callers must not cancel ctx when Connect returns
	// (HandleConnect runs asynchronously).
	sessionCtx, cancel := context.WithTimeout(context.Background(), rpc.SessionConnectTimeout)
	active := &ActiveSession{
		Config:     cfg,
		cancel:     cancel,
		connectGen: gen,
	}
	m.active = active
	m.mu.Unlock()

	go m.runConnect(sessionCtx, active)
	return nil
}

func (m *Manager) runConnect(ctx context.Context, active *ActiveSession) {
	cfg := active.Config

	port := cfg.Port
	if port == 0 {
		port = 23
	}

	conn, err := m.dialWithRetry(ctx, cfg.Host, port)
	if err != nil {
		m.failConnect(active, domain.SanitizedDialErrorFrom(err), dialLogReason(err))
		return
	}

	if !m.ownsConnect(active) {
		_ = conn.Close()
		return
	}

	termCfg := cfg.TerminalConfigFromFields()
	telnetSession, err := m.factory.NewSession(conn, termCfg)
	if err != nil {
		_ = conn.Close()
		m.failConnect(active, domain.SanitizedConnectError(), "session_create")
		return
	}

	active.Telnet = telnetSession

	if err := telnetSession.Handshake(ctx); err != nil {
		_ = telnetSession.Close()
		m.failConnect(active, domain.SanitizedConnectError(), "handshake")
		return
	}

	if !m.ownsConnect(active) {
		return
	}

	pending := m.initDrain(ctx, telnetSession)
	if len(pending) > 0 {
		_ = m.writeTerminalWithRetry(ctx, cfg.SessionID, pending)
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

	active.wg.Add(1)
	go func() {
		defer active.wg.Done()
		m.keepaliveLoop(ctx, active)
	}()

	m.updateStateForActive(active, cfg.SessionID, domain.SessionReady, "")
}

func (m *Manager) dialWithRetry(ctx context.Context, host string, port int) (io.ReadWriteCloser, error) {
	backoff := dialRetryInitial
	var lastErr error

	for {
		if ctx.Err() != nil {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, ctx.Err()
		}

		conn, err := m.transport.Dial(ctx, host, port)
		if err == nil {
			return conn, nil
		}
		lastErr = err

		if !isDialRetriable(err) {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, lastErr
		case <-time.After(backoff):
		}

		if backoff < dialRetryMax {
			backoff += 100 * time.Millisecond
			if backoff > dialRetryMax {
				backoff = dialRetryMax
			}
		}
	}
}

func (m *Manager) initDrain(ctx context.Context, session domain.TelnetSessionPort) []byte {
	drainCtx, cancel := context.WithTimeout(ctx, initDrainTimeout)
	defer cancel()

	var buf []byte
	for drainCtx.Err() == nil {
		data, err := session.ReadUserData(drainCtx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || drainCtx.Err() != nil {
				break
			}
			break
		}
		if len(data) > 0 {
			buf = append(buf, data...)
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	return buf
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
			if isReadIdle(err) {
				continue
			}
			if errors.Is(err, io.EOF) {
				m.log.Warn("read pump stopped", map[string]string{"reason": "eof"})
			} else {
				m.log.Warn("read pump stopped", map[string]string{"reason": "read_error"})
			}
			m.updateStateForActive(active, active.Config.SessionID, domain.SessionError, domain.SanitizedConnectError())
			return
		}
		if len(data) == 0 {
			continue
		}

		if err := m.writeTerminalWithRetry(ctx, active.Config.SessionID, data); err != nil {
			m.updateStateForActive(active, active.Config.SessionID, domain.SessionError, domain.SanitizedConnectError())
			return
		}
	}
}

func (m *Manager) keepaliveLoop(ctx context.Context, active *ActiveSession) {
	telnetTicker := time.NewTicker(telnetKeepaliveInterval)
	pingTicker := time.NewTicker(hostPingInterval)
	defer telnetTicker.Stop()
	defer pingTicker.Stop()

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-telnetTicker.C:
			telnet := active.Telnet
			if telnet == nil {
				continue
			}
			if err := telnet.KeepAlive(); err != nil {
				failures++
				m.log.Warn("keepalive failed", map[string]string{
					"failures": strconv.Itoa(failures),
				})
				if failures >= keepaliveFailThreshold {
					m.updateStateForActive(active, active.Config.SessionID, domain.SessionError, domain.SanitizedConnectError())
					return
				}
				continue
			}
			failures = 0
		case <-pingTicker.C:
			if m.host == nil {
				continue
			}
			go m.pingHost()
		}
	}
}

func (m *Manager) pingHost() {
	ctx, cancel := context.WithTimeout(context.Background(), rpc.DefaultCallTimeout)
	defer cancel()
	if _, err := m.host.CallCoreContext(ctx, "ping", nil); err != nil {
		m.log.Warn("host ping failed", nil)
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

func (m *Manager) ownsConnect(active *ActiveSession) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active == active
}

func (m *Manager) updateStateForActive(active *ActiveSession, sessionID domain.SessionID, state domain.SessionState, errMsg string) {
	if !m.ownsConnect(active) {
		return
	}

	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	if !m.ownsConnect(active) {
		return
	}
	m.updateStateWithRetry(sessionID, state, errMsg)
}

func (m *Manager) updateStateWithRetry(sessionID domain.SessionID, state domain.SessionState, errMsg string) {
	var lastErr error
	for i := 0; i < stateUpdateRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), rpc.DefaultCallTimeout)
		err := m.terminal.UpdateState(ctx, sessionID, state, errMsg)
		cancel()
		if err == nil {
			return
		}
		lastErr = err
		if i+1 < stateUpdateRetries {
			backoff := stateUpdateBackoff + time.Duration(i)*50*time.Millisecond
			if backoff > stateUpdateBackoffMax {
				backoff = stateUpdateBackoffMax
			}
			time.Sleep(backoff)
		}
	}
	m.log.Warn("updateState failed", map[string]string{
		"state":  string(state),
		"reason": sanitizeRPCError(lastErr),
	})
}

func (m *Manager) failConnect(active *ActiveSession, userMsg, logReason string) {
	if !m.ownsConnect(active) {
		return
	}
	m.log.Warn("connect failed", map[string]string{"reason": logReason})
	m.updateStateForActive(active, active.Config.SessionID, domain.SessionError, userMsg)
	if active.cancel != nil {
		active.cancel()
	}
	m.cleanup(active)
}

func isDialRetriable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	if isDialDenied(err) || isDialLimit(err) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	return contains(msg, "32603") ||
		contains(msg, "request failed") ||
		contains(msg, "lookup failed") ||
		contains(msg, "network dial")
}

func dialLogReason(err error) string {
	if err == nil {
		return "transport"
	}
	if errors.Is(err, context.Canceled) {
		return "dial_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "dial_timeout"
	}
	if isDialDenied(err) {
		return "dial_denied"
	}
	if isDialLimit(err) {
		return "dial_limit"
	}
	return "transport"
}

func isDialDenied(err error) bool {
	msg := err.Error()
	return contains(msg, "32001") || contains(msg, "capability denied")
}

func isDialLimit(err error) bool {
	return contains(err.Error(), "too many open network handles")
}

func sanitizeRPCError(err error) string {
	if err == nil {
		return "unknown"
	}
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

// HandleInput forwards keyboard input to the telnet session.
func (m *Manager) HandleInput(ctx context.Context, data []byte) error {
	s := m.getActive()
	if s == nil || s.Telnet == nil {
		m.log.Warn("input ignored", map[string]string{"reason": "session_not_active"})
		return domain.ErrSessionNotActive
	}
	if err := s.Telnet.WriteUserData(ctx, data); err != nil {
		m.log.Warn("input write failed", map[string]string{"reason": "write_error"})
		return err
	}
	return nil
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

func isReadIdle(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := err.Error()
	return contains(msg, "i/o timeout") || contains(msg, "deadline exceeded")
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
