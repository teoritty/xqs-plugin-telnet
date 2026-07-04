package telnet

import (
	"bytes"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
)

// Parser strips telnet commands from an inbound byte stream.
type Parser struct {
	negotiator *Negotiator
	buf        []byte
	sbOpt      byte
	sbBuf      []byte
	inSB       bool
	pendingIAC bool
}

// NewParser creates an IAC stream parser.
func NewParser(negotiator *Negotiator) *Parser {
	return &Parser{negotiator: negotiator}
}

// Feed processes raw bytes and returns user-visible data.
func (p *Parser) Feed(data []byte) ([]byte, error) {
	p.buf = append(p.buf, data...)
	var out bytes.Buffer

	for len(p.buf) > 0 {
		b := p.buf[0]
		p.buf = p.buf[1:]

		if p.pendingIAC {
			p.pendingIAC = false
			if b == domain.IAC {
				out.WriteByte(domain.IAC)
				continue
			}
			if err := p.handleIACCommand(b); err != nil {
				return out.Bytes(), err
			}
			continue
		}

		if p.inSB {
			if b == domain.IAC && len(p.buf) > 0 && p.buf[0] == domain.SE {
				p.buf = p.buf[1:]
				payload := append([]byte(nil), p.sbBuf...)
				opt := p.sbOpt
				p.inSB = false
				p.sbOpt = 0
				p.sbBuf = p.sbBuf[:0]
				if err := p.negotiator.HandleSubnegotiation(opt, payload); err != nil {
					return out.Bytes(), err
				}
				continue
			}
			p.sbBuf = append(p.sbBuf, b)
			continue
		}

		if b == domain.IAC {
			p.pendingIAC = true
			continue
		}

		out.WriteByte(b)
	}

	return out.Bytes(), nil
}

func (p *Parser) handleIACCommand(b byte) error {
	switch b {
	case domain.WILL, domain.WONT, domain.DO, domain.DONT:
		if len(p.buf) < 1 {
			p.buf = append([]byte{b}, p.buf...)
			p.pendingIAC = true
			return nil
		}
		opt := p.buf[0]
		p.buf = p.buf[1:]
		return p.negotiator.HandleCommand(b, opt)
	case domain.SB:
		if len(p.buf) < 1 {
			p.buf = append([]byte{b}, p.buf...)
			p.pendingIAC = true
			return nil
		}
		p.sbOpt = p.buf[0]
		p.buf = p.buf[1:]
		p.inSB = true
		p.sbBuf = p.sbBuf[:0]
		return nil
	default:
		return nil
	}
}
