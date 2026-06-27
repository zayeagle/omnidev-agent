# Install omnidev-agent to %USERPROFILE%\.local\bin and ensure it is on PATH.
# Usage: .\scripts\install.ps1

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$BinDir = Join-Path $env:USERPROFILE ".local\bin"
$ExeName = "omnidev-agent.exe"
$Src = Join-Path $Root "bin\$ExeName"
$Dst = Join-Path $BinDir $ExeName

Push-Location $Root
try {
    $version = "0.0.0"
    $versionFile = Join-Path $Root "VERSION"
    if (Test-Path $versionFile) {
        $version = (Get-Content $versionFile -Raw).Trim().TrimStart("v")
    }
    $buildTime = Get-Date -Format "yyyy-MM-dd HH:mm:ss"

    New-Item -ItemType Directory -Force -Path (Join-Path $Root "bin") | Out-Null
    Write-Host "Building $ExeName (v$version)..."
    go build -ldflags "-X main.appVersion=$version -X main.buildTime=$buildTime" -o $Src ./cmd/omnidev-agent
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }

    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    Copy-Item -Force $Src $Dst
    Write-Host "Installed: $Dst"

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $paths = $userPath -split ";" | Where-Object { $_ -ne "" }
    if ($paths -notcontains $BinDir) {
        $newPath = ($paths + $BinDir) -join ";"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = "$env:Path;$BinDir"
        Write-Host "Added to user PATH: $BinDir"
        Write-Host "Restart the terminal (or IDE) to pick up PATH in new sessions."
    } else {
        Write-Host "PATH already contains: $BinDir"
    }

    & $Dst -version
    Write-Host ""
    Write-Host "Run from anywhere: omnidev-agent"
} finally {
    Pop-Location
}
