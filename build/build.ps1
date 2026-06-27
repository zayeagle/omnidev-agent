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
$version = "0.0.0"
$versionFile = Join-Path $projectRoot "VERSION"
if (Test-Path $versionFile) {
    $version = (Get-Content $versionFile -Raw).Trim().TrimStart("v")
}
$buildTime = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
Write-Host "Version v$version" -ForegroundColor Cyan
go build -ldflags "-X main.appVersion=$version -X main.buildTime=$buildTime" -o bin/omnidev-agent.exe ./cmd/omnidev-agent

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
