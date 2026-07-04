param(
    [string]$Version = "1.0.0"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$dist = Join-Path $root "dist\release"
if (Test-Path $dist) {
    Remove-Item -Recurse -Force $dist
}
New-Item -ItemType Directory -Path $dist | Out-Null

# GitHub release asset naming: {name}-{os}-{arch}.exe
$assets = @(
    @{ Out = "xqs-plugin-telnet-windows-amd64.exe"; Env = @{ GOOS = "windows"; GOARCH = "amd64" } }
)

$hashLines = New-Object System.Collections.Generic.List[string]

foreach ($asset in $assets) {
    $env:CGO_ENABLED = "0"
    $env:GOOS = $asset.Env.GOOS
    $env:GOARCH = $asset.Env.GOARCH
    $outPath = Join-Path $dist $asset.Out
    go build -ldflags="-s -w" -trimpath -o $outPath ./cmd/plugin
    if ($LASTEXITCODE -ne 0) {
        Write-Error "build failed for $($asset.Out)"
    }
    $hash = (Get-FileHash -Algorithm SHA256 -Path $outPath).Hash.ToLower()
    $hashLines.Add("$hash  $($asset.Out)")
    Write-Host "Built $($asset.Out)"
}

$checksumsPath = Join-Path $dist "SHA256SUMS"
$content = ($hashLines -join "`n") + "`n"
[System.IO.File]::WriteAllText($checksumsPath, $content.Replace("`r`n", "`n"))

Write-Host ""
Write-Host "Release artifacts in: $dist"
Write-Host "Upload to GitHub Release v$Version :"
Write-Host "  - xqs-plugin-telnet-windows-amd64.exe"
Write-Host "  - SHA256SUMS"
Write-Host ""
Write-Host "Ensure xqsp.json is committed on the default branch before fetching in xQuakShell."
