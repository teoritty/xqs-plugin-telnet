$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

$forbiddenByPrefix = @{
    "internal\domain"        = @("internal\infra", "internal\usecase", "internal\presentation", "cmd\")
    "internal\usecase"       = @("internal\infra", "internal\presentation")
    "internal\infra"         = @("internal\usecase", "internal\presentation")
    "internal\presentation"  = @("internal\infra")
}

$module = "github.com/teoritty/xqs-plugin-telnet"
$violations = @()

Get-ChildItem -Path (Join-Path $root "internal") -Filter "*.go" -Recurse | ForEach-Object {
    $rel = $_.FullName.Substring($root.Length + 1)
    $prefix = $null
    foreach ($key in $forbiddenByPrefix.Keys) {
        if ($rel -like "$key*") {
            $prefix = $key
            break
        }
    }
    if (-not $prefix) { return }

    $content = Get-Content $_.FullName -Raw
    foreach ($forbidden in $forbiddenByPrefix[$prefix]) {
        $importPath = "$module/$($forbidden.Replace('\','/').TrimEnd('/'))"
        if ($content -match [regex]::Escape('"' + $importPath + '"')) {
            $violations += "$rel -> $importPath"
        }
    }
}

if ($violations.Count -gt 0) {
    Write-Host "Import boundary violations:"
    $violations | ForEach-Object { Write-Host "  $_" }
    exit 1
}

Write-Host "Import boundaries OK"
