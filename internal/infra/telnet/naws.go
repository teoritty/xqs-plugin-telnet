package telnet

import "github.com/teoritty/xqs-plugin-telnet/internal/domain"

// EncodeNAWS builds NAWS subnegotiation payload (cols, rows big-endian).
func EncodeNAWS(cols, rows uint16) []byte {
	return []byte{
		byte(cols >> 8), byte(cols),
		byte(rows >> 8), byte(rows),
	}
}

// BuildNAWSCommand returns IAC SB NAWS ... IAC SE frame.
func BuildNAWSCommand(cols, rows uint16) []byte {
	payload := EncodeNAWS(cols, rows)
	frame := make([]byte, 0, 6+len(payload))
	frame = append(frame, domain.IAC, domain.SB, domain.OptNAWS)
	frame = append(frame, payload...)
	frame = append(frame, domain.IAC, domain.SE)
	return frame
}
