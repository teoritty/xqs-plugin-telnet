package telnet

import "github.com/teoritty/xqs-plugin-telnet/internal/domain"

// EscapeTelnetData doubles IAC bytes for user data transmission.
func EscapeTelnetData(data []byte) []byte {
	var out []byte
	for _, b := range data {
		if b == domain.IAC {
			out = append(out, domain.IAC, domain.IAC)
			continue
		}
		out = append(out, b)
	}
	return out
}
