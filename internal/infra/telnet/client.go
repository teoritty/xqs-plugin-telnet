package telnet

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
)

const readBufferSize = 4096

// Session implements domain.TelnetSessionPort.
type Session struct {
	conn        io.ReadWriteCloser
	cfg         domain.TerminalConfig
	negotiator  *Negotiator
	parser      *Parser
	writeMu     sync.Mutex
	closed      bool
	closeMu     sync.Mutex
	readScratch []byte
}

// Factory implements domain.TelnetSessionFactory.
type Factory struct{}

// NewFactory creates a telnet session factory.
func NewFactory() *Factory {
	return &Factory{}
}

// NewSession implements domain.TelnetSessionFactory.
func (f *Factory) NewSession(conn io.ReadWriteCloser, cfg domain.TerminalConfig) (domain.TelnetSessionPort, error) {
	if conn == nil {
		return nil, domain.ErrInvalidConnection
	}
	s := &Session{
		conn:        conn,
		cfg:         cfg,
		readScratch: make([]byte, readBufferSize),
	}
	s.negotiator = NewNegotiator(cfg, s.rawWrite)
	s.parser = NewParser(s.negotiator)
	return s, nil
}

// Handshake sends initial telnet option negotiation.
func (s *Session) Handshake(ctx context.Context) error {
	if err := s.negotiator.SendInitialOptions(); err != nil {
		return err
	}
	if s.cfg.Cols > 0 && s.cfg.Rows > 0 {
		return s.SetWindowSize(s.cfg.Cols, s.cfg.Rows)
	}
	return nil
}

// ReadUserData reads from the connection and strips telnet commands.
func (s *Session) ReadUserData(ctx context.Context) ([]byte, error) {
	if s.isClosed() {
		return nil, io.EOF
	}

	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		n, err := s.conn.Read(s.readScratch)
		if n > 0 {
			user, perr := s.parser.Feed(s.readScratch[:n])
			if perr != nil {
				ch <- result{err: perr}
				return
			}
			if len(user) > 0 {
				ch <- result{data: user}
				return
			}
			ch <- result{data: nil, err: nil}
			return
		}
		ch <- result{err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		if len(res.data) == 0 {
			return nil, nil
		}
		return res.data, nil
	}
}

// WriteUserData sends user keystrokes with IAC escaping.
func (s *Session) WriteUserData(ctx context.Context, data []byte) error {
	if s.isClosed() {
		return io.ErrClosedPipe
	}
	if len(data) == 0 {
		return nil
	}
	escaped := EscapeTelnetData(data)
	return s.writeWithContext(ctx, escaped)
}

// SetWindowSize sends NAWS subnegotiation.
func (s *Session) SetWindowSize(cols, rows uint16) error {
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	s.cfg.Cols = cols
	s.cfg.Rows = rows
	return s.rawWrite(BuildNAWSCommand(cols, rows))
}

// Close closes the underlying connection.
func (s *Session) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.conn.Close()
}

func (s *Session) rawWrite(data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.isClosed() {
		return io.ErrClosedPipe
	}
	_, err := s.conn.Write(data)
	return err
}

func (s *Session) writeWithContext(ctx context.Context, data []byte) error {
	done := make(chan error, 1)
	go func() {
		done <- s.rawWrite(data)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func (s *Session) isClosed() bool {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	return s.closed
}

// RawWrite exposes raw bytes for auto-login use (no IAC escape for credentials).
func (s *Session) RawWrite(data []byte) error {
	return s.rawWrite(data)
}

// ReadRaw exposes parser feed for auto-login sniffing.
func (s *Session) ReadRaw(ctx context.Context, timeout time.Duration) ([]byte, error) {
	if s.isClosed() {
		return nil, io.EOF
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		user, err := s.ReadUserData(ctx)
		if err != nil {
			return nil, err
		}
		if len(user) > 0 {
			return user, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil, context.DeadlineExceeded
}

var _ domain.TelnetSessionFactory = (*Factory)(nil)
var _ domain.TelnetSessionPort = (*Session)(nil)
