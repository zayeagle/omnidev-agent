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
go build -o bin/omnidev-agent.exe ./cmd/omnidev-agent

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
