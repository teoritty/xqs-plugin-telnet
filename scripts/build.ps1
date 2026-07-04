$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = "0"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root
go build -ldflags="-s -w" -trimpath -o xqs-plugin-telnet.exe ./cmd/plugin
Write-Host "Built xqs-plugin-telnet.exe"
