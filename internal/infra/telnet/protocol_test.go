package telnet_test

import (
	"testing"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
	"github.com/teoritty/xqs-plugin-telnet/internal/infra/telnet"
)

func TestEscapeTelnetData(t *testing.T) {
	in := []byte{0x41, domain.IAC, 0x42}
	out := telnet.EscapeTelnetData(in)
	want := []byte{0x41, domain.IAC, domain.IAC, 0x42}
	if len(out) != len(want) {
		t.Fatalf("got %v want %v", out, want)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("byte %d: got %x want %x", i, out[i], want[i])
		}
	}
}

func TestParserStripsEchoNegotiation(t *testing.T) {
	var sent [][]byte
	neg := telnet.NewNegotiator(domain.DefaultTerminalConfig(), func(b []byte) error {
		sent = append(sent, append([]byte(nil), b...))
		return nil
	})
	p := telnet.NewParser(neg)

	in := []byte{domain.IAC, domain.WILL, domain.OptEcho, 'H', 'i'}
	out, err := p.Feed(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "Hi" {
		t.Fatalf("got %q", out)
	}
	if len(sent) == 0 {
		t.Fatal("expected negotiation response")
	}
}

func TestBuildNAWSCommand(t *testing.T) {
	frame := telnet.BuildNAWSCommand(80, 24)
	if len(frame) < 6 {
		t.Fatal("frame too short")
	}
	if frame[0] != domain.IAC || frame[2] != domain.OptNAWS {
		t.Fatal("invalid NAWS frame header")
	}
}
