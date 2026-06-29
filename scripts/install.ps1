# Install omnidev-agent to %USERPROFILE%\.local\bin and ensure it is on PATH.
# Usage: .\scripts\install.ps1

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$BinDir = Join-Path $env:USERPROFILE ".local\bin"
$ExeName = "omnidev-agent.exe"
$BuildDir = Join-Path $Root "bin"
$Src = Join-Path $BuildDir $ExeName
$Dst = Join-Path $BinDir $ExeName
$GlobalConfigDir = Join-Path $env:USERPROFILE ".omnidev-agent"
$GlobalConfig = Join-Path $GlobalConfigDir "config.json"
$ProjectConfig = Join-Path $Root ".omnidev-agent.json"
$Sample = Join-Path $Root ".omnidev-agent.json.sample"

Push-Location $Root
try {
    $version = "0.0.0"
    $versionFile = Join-Path $Root "VERSION"
    if (Test-Path $versionFile) {
        $version = (Get-Content $versionFile -Raw).Trim().TrimStart("v")
    }
    $buildTime = Get-Date -Format "yyyy-MM-dd HH:mm:ss"

    New-Item -ItemType Directory -Force -Path $BuildDir | Out-Null
    Write-Host "Building $ExeName (v$version)..."
    go build -ldflags "-X main.appVersion=$version -X 'main.buildTime=$buildTime'" -o $Src ./cmd/omnidev-agent
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

    New-Item -ItemType Directory -Force -Path $GlobalConfigDir | Out-Null
    if (-not (Test-Path $GlobalConfig)) {
        Copy-Item -Force $Sample $GlobalConfig
        Write-Host "Created global config: $GlobalConfig"
    } else {
        Write-Host "Global config exists (skipped): $GlobalConfig"
    }

    if (-not (Test-Path $ProjectConfig)) {
        Copy-Item -Force $Sample $ProjectConfig
        Write-Host "Created project config: $ProjectConfig"
    } else {
        Write-Host "Project config exists (skipped): $ProjectConfig"
    }

    Write-Host ""
    Write-Host "=============================================================="
    Write-Host "  Deployment complete"
    Write-Host "=============================================================="
    Write-Host ""
    Write-Host "  [Binary]"
    Write-Host "    Build artifact:   $Src"
    Write-Host "    Installed binary: $Dst"
    Write-Host ""
    Write-Host "  [Configuration]"
    Write-Host "    Global config:  $GlobalConfig"
    Write-Host "    Project config: $ProjectConfig"
    Write-Host ""
    Write-Host "  [Config priority - higher wins]"
    Write-Host "    1. CLI flags / environment variables (e.g. OMNIDEV_API_KEY)"
    Write-Host "    2. Project config: $ProjectConfig"
    Write-Host "       (only when current working directory is the project root)"
    Write-Host "    3. Global config:  $GlobalConfig"
    Write-Host "       (used from any directory when project file is absent or not loaded)"
    Write-Host "    4. Built-in defaults"
    Write-Host ""
    Write-Host "  Tip: edit global config for install-once-use-everywhere."
    Write-Host "       edit project config only when this project needs different settings."
    Write-Host ""
    Write-Host "Run from anywhere: omnidev-agent"
} finally {
    Pop-Location
}
