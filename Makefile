# xqs-plugin-telnet build targets

BINARY_WIN := xqs-plugin-telnet.exe
BINARY_UNIX := xqs-plugin-telnet
VERSION := 1.0.5
BUNDLE := dist/xqs-plugin-telnet-$(VERSION).xqsp

.PHONY: build build-windows test check-imports checksums pack release clean

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o $(BINARY_WIN) ./cmd/plugin

build-windows:
	set CGO_ENABLED=0 && go build -ldflags="-s -w" -trimpath -o $(BINARY_WIN) ./cmd/plugin

test:
	go test ./...

check-imports:
	powershell -ExecutionPolicy Bypass -File scripts/check-imports.ps1

checksums:
	powershell -ExecutionPolicy Bypass -File scripts/checksums.ps1

pack: build checksums
	powershell -ExecutionPolicy Bypass -File scripts/pack.ps1 -Version $(VERSION)

release:
	powershell -ExecutionPolicy Bypass -File scripts/release.ps1 -Version $(VERSION)

clean:
	rm -f $(BINARY_WIN) $(BINARY_UNIX) SHA256SUMS
	rm -rf dist
