param(
    [string]$Version = "1.0.0"
)

$ErrorActionPreference = "Stop"
. "$PSScriptRoot\common.ps1"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$dist = Join-Path $root "dist\release"
if (Test-Path $dist) {
    Remove-Item -Recurse -Force $dist
}
New-Item -ItemType Directory -Path $dist | Out-Null

# xQuakShell GitHub install parses: {name}-{os}-{arch}.exe
$assets = @(
    @{ Out = (Get-GitHubReleaseAssetName -OS "windows" -Arch "amd64"); Env = @{ GOOS = "windows"; GOARCH = "amd64" } }
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
Write-Host "Upload ONLY these files to GitHub Release v$Version :"
foreach ($asset in $assets) {
    Write-Host "  - $($asset.Out)"
}
Write-Host "  - SHA256SUMS"
Write-Host ""
Write-Host "Do NOT upload $PluginBinaryName to GitHub Release - xQuakShell will not detect the platform."
Write-Host "engine.entry in xqsp.json stays $PluginBinaryName; host copies asset to that name on install."
