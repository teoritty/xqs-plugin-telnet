package domain

import (
	"context"
	"io"
)

// TransportPort dials outbound TCP through the host capability gate.
type TransportPort interface {
	Dial(ctx context.Context, host string, port int) (io.ReadWriteCloser, error)
}

// TerminalPort streams terminal output and session state to the host UI.
type TerminalPort interface {
	WriteOutput(ctx context.Context, sessionID SessionID, data []byte) error
	UpdateState(ctx context.Context, sessionID SessionID, state SessionState, errMsg string) error
}

// LoggerPort writes audit-safe log lines to the host.
type LoggerPort interface {
	Info(msg string, fields map[string]string)
	Warn(msg string, fields map[string]string)
}

// TelnetSessionFactory creates telnet sessions over a transport connection.
type TelnetSessionFactory interface {
	NewSession(conn io.ReadWriteCloser, cfg TerminalConfig) (TelnetSessionPort, error)
}

// TelnetSessionPort handles telnet protocol on a single connection.
type TelnetSessionPort interface {
	Handshake(ctx context.Context) error
	ReadUserData(ctx context.Context) ([]byte, error)
	WriteUserData(ctx context.Context, data []byte) error
	SetWindowSize(cols, rows uint16) error
	Close() error
}
