package telnet

import "github.com/teoritty/xqs-plugin-telnet/internal/domain"

// BuildTTypeResponse builds TERMINAL_TYPE subnegotiation IS response.
func BuildTTypeResponse(terminalType string) []byte {
	frame := make([]byte, 0, 8+len(terminalType))
	frame = append(frame, domain.IAC, domain.SB, domain.OptTerminalType, 0)
	frame = append(frame, terminalType...)
	frame = append(frame, domain.IAC, domain.SE)
	return frame
}
