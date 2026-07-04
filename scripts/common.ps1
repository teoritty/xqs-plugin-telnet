# Shared naming constants for xQuakShell plugin packaging.
#
# Local install (Install folder / .xqsp bundle):
#   - Binary name must match plugin.json / xqsp.json engine.entry
#
# GitHub Release assets (Fetch/Install from GitHub):
#   - Asset name MUST be {plugin}-{os}-{arch}.exe (see xQuakShell parseGitHubAssetName)

$script:PluginBinaryName = "xqs-plugin-telnet.exe"

function Get-GitHubReleaseAssetName {
    param(
        [string]$OS = "windows",
        [string]$Arch = "amd64"
    )
    $base = [System.IO.Path]::GetFileNameWithoutExtension($script:PluginBinaryName)
    return "$base-$OS-$Arch.exe"
}

function Write-GitHubAssetNamingReminder {
    $asset = Get-GitHubReleaseAssetName
    Write-Host ""
    Write-Host "GitHub Release requires platform-specific asset names." -ForegroundColor Yellow
    Write-Host "  Local install binary : $PluginBinaryName"
    Write-Host "  GitHub release asset : $asset"
    Write-Host "  Run: powershell -ExecutionPolicy Bypass -File scripts\release.ps1"
}
