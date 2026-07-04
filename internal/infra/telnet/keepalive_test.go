package telnet_test

import (
	"io"
	"testing"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
	"github.com/teoritty/xqs-plugin-telnet/internal/infra/telnet"
)

type keepaliveConn struct {
	written [][]byte
}

func (k *keepaliveConn) Read([]byte) (int, error) { return 0, io.EOF }
func (k *keepaliveConn) Write(p []byte) (int, error) {
	k.written = append(k.written, append([]byte(nil), p...))
	return len(p), nil
}
func (k *keepaliveConn) Close() error { return nil }

func TestSessionKeepAliveSendsNOP(t *testing.T) {
	rec := &keepaliveConn{}
	factory := telnet.NewFactory()
	sess, err := factory.NewSession(rec, domain.DefaultTerminalConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.KeepAlive(); err != nil {
		t.Fatal(err)
	}
	if len(rec.written) != 1 {
		t.Fatalf("expected 1 write, got %d", len(rec.written))
	}
	got := rec.written[0]
	if len(got) != 2 || got[0] != domain.IAC || got[1] != domain.NOP {
		t.Fatalf("unexpected keepalive frame: %v", got)
	}
}
