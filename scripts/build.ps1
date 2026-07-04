$ErrorActionPreference = "Stop"
. "$PSScriptRoot\common.ps1"

$env:CGO_ENABLED = "0"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

go build -ldflags="-s -w" -trimpath -o $PluginBinaryName ./cmd/plugin
Write-Host "Built $PluginBinaryName (local install / bundle)"
Write-GitHubAssetNamingReminder
