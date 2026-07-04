package domain

// Telnet command bytes (RFC 854).
const (
	IAC  byte = 255
	DONT byte = 254
	DO   byte = 253
	WONT byte = 252
	WILL byte = 251
	SB   byte = 250
	SE   byte = 240
	NOP  byte = 241
)

// Telnet option codes.
const (
	OptBinary           byte = 0
	OptEcho             byte = 1
	OptSuppressGoAhead  byte = 3
	OptTerminalType     byte = 24
	OptNAWS             byte = 31
)

// TerminalType values supported by the connection editor.
const (
	TerminalVT100         = "vt100"
	TerminalVT220         = "vt220"
	TerminalXTerm         = "xterm"
	TerminalXTerm256Color = "xterm-256color"
)

// TerminalConfig holds negotiated telnet session options.
type TerminalConfig struct {
	TerminalType string
	BinaryMode   bool
	Cols         uint16
	Rows         uint16
}

// DefaultTerminalConfig returns safe defaults.
func DefaultTerminalConfig() TerminalConfig {
	return TerminalConfig{
		TerminalType: TerminalVT100,
		BinaryMode:   false,
		Cols:         80,
		Rows:         24,
	}
}
