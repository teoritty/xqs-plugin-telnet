# xqs-plugin-telnet

Production-ready **Telnet** session plugin for [xQuakShell](https://github.com/teoritty/xQuakShell) (`feature/api-core`).

## Security warning

Telnet transmits credentials and session data in **plaintext**. Use only on trusted networks or for legacy devices. Prefer SSH when available.

This plugin requests `allowArbitraryOutbound` and `allowPrivateNetworks` so you can reach user-defined hosts after explicit install-time consent in xQuakShell.

## Features

- Full telnet option negotiation (ECHO, SGA, TERMINAL_TYPE, NAWS, optional BINARY)
- Manifest-driven connection UI (username, password, auto-login, terminal type)
- Terminal I/O via xQuakShell core (`Terminal.svelte`)
- Opt-in auto-login with configurable prompts
- Clean architecture: `domain` → `usecase` → `infra` / `presentation`
- Security-first: no secret persistence, redacted logging, sanitized errors

## Requirements

- Go 1.25+
- xQuakShell `feature/api-core` or newer (`minCoreVersion: 0.2.0`)
- Windows build target (match host GOOS at install)

## Build

Two binary names are used on purpose:

| Purpose | File name | Used by |
|---------|-----------|---------|
| Local install / `.xqsp` bundle | `xqs-plugin-telnet.exe` | `plugin.json` → `engine.entry` |
| **GitHub Release asset** | `xqs-plugin-telnet-windows-amd64.exe` | xQuakShell platform detection |

xQuakShell parses release assets as `{name}-{os}-{arch}.exe`. Uploading `xqs-plugin-telnet.exe` to GitHub **will not work** — platforms stay empty and fetch/install fails.

### Local build (install folder / bundle)

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

Or:

```powershell
$env:CGO_ENABLED=0
go build -ldflags="-s -w" -trimpath -o xqs-plugin-telnet.exe ./cmd/plugin
```

### GitHub Release build

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\release.ps1 -Version 1.0.0
```

Output: `dist/release/xqs-plugin-telnet-windows-amd64.exe` + `SHA256SUMS`

Or tag a release in git (`git tag v1.0.0 && git push origin v1.0.0`) — GitHub Actions uploads assets with the correct names automatically.

## Install

1. Build the plugin (produces `xqs-plugin-telnet.exe` + `plugin.json`).
2. Generate checksums: `.\scripts\checksums.ps1`
3. In xQuakShell: **Settings → Plugins → Install folder…** and select this directory.
4. Accept network capability consent when prompted.
5. Create a connection with protocol **Telnet**, set host/port, optional credentials.
6. Open a session tab.

### Bundle (`.xqsp`)

```powershell
.\scripts\pack.ps1 -Version 1.0.0
```

Install via **Settings → Plugins → Install bundle…**

## Install from GitHub

xQuakShell discovers plugins via **`xqsp.json`** in the repository root (not `plugin.json` alone).

1. Register the repo in xQuakShell: **Settings → Plugins → GitHub repositories** → add `https://github.com/teoritty/xqs-plugin-telnet`
2. Ensure `xqsp.json` exists on the default branch (included in this repo).
3. Publish a **GitHub Release** with assets named for the platform, e.g.:
   - `xqs-plugin-telnet-windows-amd64.exe`
   - `SHA256SUMS` (recommended)
4. **Fetch plugins** from the registered repo, then **Install**.

Build release artifacts locally:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\release.ps1 -Version 1.0.0
```

Upload **only** from `dist/release/` (not the root `xqs-plugin-telnet.exe`).

Keep `xqsp.json` and `plugin.json` in sync when changing capabilities or connection fields.

## Architecture

```
cmd/plugin/main.go          # composition root (DI only)
internal/domain/          # entities + ports (no infra imports)
internal/usecase/         # session orchestration
internal/infra/           # RPC, net proxy, telnet protocol, logger
internal/presentation/    # JSON-RPC handlers (thin adapters)
```

Run layer checks:

```powershell
.\scripts\check-imports.ps1
```

## Tests

```powershell
go test ./...
```

## Plugin ID

`io.xquakshell.plugin.telnet`

## License

GPL-3.0 (see LICENSE)
