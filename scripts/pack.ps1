param(
    [string]$Version = "1.0.2"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$binary = "xqs-plugin-telnet.exe"
if (-not (Test-Path $binary)) {
    Write-Error "Binary not found: $binary. Run build first."
}

$dist = Join-Path $root "dist"
if (-not (Test-Path $dist)) {
    New-Item -ItemType Directory -Path $dist | Out-Null
}

$stage = Join-Path $dist "stage"
if (Test-Path $stage) {
    Remove-Item -Recurse -Force $stage
}
New-Item -ItemType Directory -Path $stage | Out-Null

Copy-Item $binary $stage
Copy-Item "plugin.json" $stage
if (Test-Path "SHA256SUMS") {
    Copy-Item "SHA256SUMS" $stage
}

$bundle = Join-Path $dist "xqs-plugin-telnet-$Version.xqsp"
if (Test-Path $bundle) {
    Remove-Item -Force $bundle
}

Push-Location $stage
& powershell -ExecutionPolicy Bypass -File (Join-Path $root "scripts\checksums.ps1")
Compress-Archive -Path * -DestinationPath $bundle -Force
Pop-Location

Write-Host "Bundle created: $bundle"
