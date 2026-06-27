param(
    [switch]$Run
)

$ErrorActionPreference = "Stop"
$projectRoot = Split-Path -Parent $PSScriptRoot
Set-Location $projectRoot

Write-Host "=== omnidev-agent Build ===" -ForegroundColor Cyan
Write-Host "Module: github.com/zayeagle/omnidev-agent" -ForegroundColor Cyan

# ── Build ──
Write-Host "`nBuilding..." -ForegroundColor Yellow
$version = "dev"
try {
    $version = (git describe --tags --always --dirty 2>$null)
    if (-not $version) { $version = "dev" }
} catch { }
$buildTime = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
go build -ldflags "-X main.version=$version -X main.buildTime=$buildTime" -o bin/omnidev-agent.exe ./cmd/omnidev-agent

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Build successful" -ForegroundColor Green
    Write-Host "   Binary: bin/omnidev-agent.exe" -ForegroundColor Green

    if ($Run) {
        Write-Host "`nStarting omnidev-agent..." -ForegroundColor Cyan
        & ".\bin\omnidev-agent.exe"
    }
} else {
    Write-Host "❌ Build failed" -ForegroundColor Red
    exit 1
}
