package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/teoritty/xqs-plugin-telnet/internal/infra/logger"
	"github.com/teoritty/xqs-plugin-telnet/internal/infra/netproxy"
	"github.com/teoritty/xqs-plugin-telnet/internal/infra/rpc"
	"github.com/teoritty/xqs-plugin-telnet/internal/infra/terminal"
	"github.com/teoritty/xqs-plugin-telnet/internal/infra/telnet"
	"github.com/teoritty/xqs-plugin-telnet/internal/presentation"
	"github.com/teoritty/xqs-plugin-telnet/internal/usecase"
	"github.com/teoritty/xqs-plugin-telnet/internal/usecase/autologin"
)

func main() {
	host := rpc.NewHost()
	caller := rpc.HostCaller{Host: host}

	secureLog := logger.NewSecureLogger(caller)
	transport := netproxy.NewDialer(caller)
	term := terminal.NewAdapter(caller)
	factory := telnet.NewFactory()
	autoRunner := autologin.NewRunner(secureLog)

	manager := usecase.NewManager(transport, term, factory, secureLog, autoRunner)

	lifecycle := presentation.LifecycleHandlers{Log: secureLog}
	session := presentation.SessionHandlers{Manager: manager}

	host.Register("initialize", lifecycle.HandleInitialize)
	host.Register("activate", lifecycle.HandleActivate)
	host.Register("ping", func(_ json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	host.Register("shutdown", func(params json.RawMessage) (any, error) {
		return lifecycle.HandleShutdown(params, manager)
	})
	host.Register("session.connect", session.HandleConnect)
	host.RegisterNotification("session.writeInput", session.HandleWriteInput)
	host.RegisterNotification("session.resize", session.HandleResize)
	host.RegisterNotification("session.disconnect", session.HandleDisconnect)
	host.RegisterNotification("deactivate", func(_ json.RawMessage) {
		manager.Disconnect()
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	done := make(chan error, 1)
	go func() {
		done <- host.Run()
	}()

	select {
	case <-ctx.Done():
		manager.Disconnect()
	case err := <-done:
		if err != nil {
			log.Fatal(err)
		}
	}
}
