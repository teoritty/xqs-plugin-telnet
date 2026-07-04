$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$exclude = @("SHA256SUMS", ".git", "dist", "test")
$files = Get-ChildItem -File -Recurse | Where-Object {
    $rel = $_.FullName.Substring($root.Length + 1)
    if ($rel -match '\\dist\\' -or $rel -match '\\\.git\\' -or $rel -match '\\test\\') { return $false }
    if ($_.Name -eq "SHA256SUMS") { return $false }
    if ($_.Extension -in @(".go", ".mod", ".sum", ".md", ".ps1", ".gitignore")) { return $false }
    return $true
}

$lines = New-Object System.Collections.Generic.List[string]
foreach ($f in $files) {
    $rel = $f.FullName.Substring($root.Length + 1).Replace("\", "/")
    $hash = (Get-FileHash -Algorithm SHA256 -Path $f.FullName).Hash.ToLower()
    $lines.Add("$hash  $rel")
}

$lines.Sort()
$content = ($lines -join "`n") + "`n"
[System.IO.File]::WriteAllText((Join-Path $root "SHA256SUMS"), $content.Replace("`r`n", "`n"))
Write-Host "SHA256SUMS written with $($lines.Count) entries"
