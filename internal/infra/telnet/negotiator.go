package telnet

import "github.com/teoritty/xqs-plugin-telnet/internal/domain"

// Negotiator handles telnet option negotiation responses.
type Negotiator struct {
	cfg          domain.TerminalConfig
	terminalType string
	onSend       func([]byte) error
}

// NewNegotiator creates an option negotiator.
func NewNegotiator(cfg domain.TerminalConfig, onSend func([]byte) error) *Negotiator {
	tt := cfg.TerminalType
	if tt == "" {
		tt = domain.TerminalVT100
	}
	return &Negotiator{cfg: cfg, terminalType: tt, onSend: onSend}
}

// HandleCommand processes WILL/WONT/DO/DONT and SB requests.
func (n *Negotiator) HandleCommand(cmd byte, opt byte) error {
	switch cmd {
	case domain.WILL:
		return n.handleWill(opt)
	case domain.WONT:
		return n.handleWont(opt)
	case domain.DO:
		return n.handleDo(opt)
	case domain.DONT:
		return n.handleDont(opt)
	}
	return nil
}

// HandleSubnegotiation handles SB ... SE payloads.
func (n *Negotiator) HandleSubnegotiation(opt byte, payload []byte) error {
	if opt != domain.OptTerminalType || len(payload) == 0 {
		return nil
	}
	if payload[0] == 1 {
		return n.onSend(BuildTTypeResponse(n.terminalType))
	}
	return nil
}

func (n *Negotiator) handleWill(opt byte) error {
	switch opt {
	case domain.OptEcho:
		return n.sendDo(opt)
	case domain.OptSuppressGoAhead:
		return n.sendDo(opt)
	case domain.OptBinary:
		if n.cfg.BinaryMode {
			return n.sendDo(opt)
		}
		return n.sendDont(opt)
	default:
		return n.sendDont(opt)
	}
}

func (n *Negotiator) handleWont(opt byte) error {
	switch opt {
	case domain.OptEcho:
		return n.sendDont(opt)
	default:
		return nil
	}
}

func (n *Negotiator) handleDo(opt byte) error {
	switch opt {
	case domain.OptEcho:
		return n.sendWont(opt)
	case domain.OptSuppressGoAhead:
		return n.sendWill(opt)
	case domain.OptTerminalType:
		return n.sendWill(opt)
	case domain.OptNAWS:
		return n.sendWill(opt)
	case domain.OptBinary:
		if n.cfg.BinaryMode {
			return n.sendWill(opt)
		}
		return n.sendWont(opt)
	default:
		return n.sendWont(opt)
	}
}

func (n *Negotiator) handleDont(opt byte) error {
	switch opt {
	case domain.OptEcho:
		return n.sendWont(opt)
	case domain.OptSuppressGoAhead:
		return n.sendWont(opt)
	case domain.OptTerminalType:
		return n.sendWont(opt)
	case domain.OptNAWS:
		return n.sendWont(opt)
	default:
		return nil
	}
}

func (n *Negotiator) sendDo(opt byte) error {
	return n.onSend([]byte{domain.IAC, domain.DO, opt})
}

func (n *Negotiator) sendDont(opt byte) error {
	return n.onSend([]byte{domain.IAC, domain.DONT, opt})
}

func (n *Negotiator) sendWill(opt byte) error {
	return n.onSend([]byte{domain.IAC, domain.WILL, opt})
}

func (n *Negotiator) sendWont(opt byte) error {
	return n.onSend([]byte{domain.IAC, domain.WONT, opt})
}

// SendInitialOptions proactively negotiates common client options.
func (n *Negotiator) SendInitialOptions() error {
	if err := n.sendWill(domain.OptSuppressGoAhead); err != nil {
		return err
	}
	if err := n.sendWill(domain.OptTerminalType); err != nil {
		return err
	}
	if err := n.sendWill(domain.OptNAWS); err != nil {
		return err
	}
	if n.cfg.BinaryMode {
		if err := n.sendWill(domain.OptBinary); err != nil {
			return err
		}
	}
	return n.sendDo(domain.OptSuppressGoAhead)
}
