# -----------------------------------------------------------------------
# Build Script for iter-service
# -----------------------------------------------------------------------
#
# Usage:
#   .\scripts\build.ps1           # Build only
#   .\scripts\build.ps1 -Run      # Build and run
#   .\scripts\build.ps1 -Deploy   # Build and deploy to bin\
#
# -----------------------------------------------------------------------

param (
    [switch]$Run,
    [switch]$Deploy
)

<#
.SYNOPSIS
    Build script for iter-service

.DESCRIPTION
    Builds iter-service for local development and testing.

    Three operations supported:
    1. Default build (no parameters) - Builds executable to bin\
    2. -Deploy - Builds and deploys config to bin\
    3. -Run - Builds, deploys, and starts service

.PARAMETER Deploy
    Deploy configuration files to bin\ after building

.PARAMETER Run
    Build, deploy, and run the service
    Automatically triggers deployment before starting

.EXAMPLE
    .\build.ps1
    Build iter-service executable only

.EXAMPLE
    .\build.ps1 -Deploy
    Build and deploy configuration to bin\

.EXAMPLE
    .\build.ps1 -Run
    Build, deploy, and start the service
#>

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

# Setup paths
$scriptDir = $PSScriptRoot
$projectRoot = Split-Path -Parent $scriptDir
$binDir = Join-Path -Path $projectRoot -ChildPath "bin"
$outputPath = Join-Path -Path $binDir -ChildPath "iter-service.exe"
$versionFilePath = Join-Path -Path $projectRoot -ChildPath ".version"

# Generate version
$majorMinor = "2.1"
$datetimeStamp = Get-Date -Format "yyyyMMdd-HHmm"
$version = "$majorMinor.$datetimeStamp"

# Get git commit
try {
    $gitCommit = git rev-parse --short HEAD 2>$null
    if (-not $gitCommit) { $gitCommit = "unknown" }
}
catch {
    $gitCommit = "unknown"
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "iter-service Build Script" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Version: $version" -ForegroundColor Gray
Write-Host "Git Commit: $gitCommit" -ForegroundColor Gray
Write-Host "Project: $projectRoot" -ForegroundColor Gray
Write-Host "========================================" -ForegroundColor Cyan

# Update .version file
Set-Content -Path $versionFilePath -Value $version -NoNewline

# Check Go
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go is not installed. Install Go 1.23+ from https://go.dev/dl/"
    exit 1
}

# Create bin directory
if (-not (Test-Path $binDir)) {
    New-Item -ItemType Directory -Path $binDir | Out-Null
}

# Stop existing service if running
$existingProcess = Get-Process -Name "iter-service" -ErrorAction SilentlyContinue
if ($existingProcess) {
    Write-Host "Stopping existing iter-service..." -ForegroundColor Yellow
    Stop-Process -Name "iter-service" -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
}

# Download dependencies
Write-Host "Downloading dependencies..." -ForegroundColor Yellow
Push-Location $projectRoot
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Error "Failed to download dependencies"
    Pop-Location
    exit 1
}

# Build binary
Write-Host "Building iter-service..." -ForegroundColor Yellow

$ldflags = "-X 'main.version=$version'"
$buildArgs = @(
    "build"
    "-ldflags=$ldflags"
    "-o", $outputPath
    ".\cmd\iter-service"
)

& go @buildArgs

Pop-Location

if ($LASTEXITCODE -ne 0) {
    Write-Error "Build failed!"
    exit 1
}

# Verify build
if (-not (Test-Path $outputPath)) {
    Write-Error "Build completed but executable not found: $outputPath"
    exit 1
}

Write-Host "Binary built: $outputPath" -ForegroundColor Green

# Deploy if requested (or if Run is specified)
if ($Deploy -or $Run) {
    Write-Host ""
    Write-Host "Deploying files to bin\..." -ForegroundColor Yellow

    # Copy config example (only if config doesn't exist)
    $configDest = Join-Path -Path $binDir -ChildPath "config.toml"
    $configSource = Join-Path -Path $projectRoot -ChildPath "configs\config.example.toml"

    if (-not (Test-Path $configDest)) {
        if (Test-Path $configSource) {
            Copy-Item -Path $configSource -Destination $configDest
            Write-Host "  Created: config.toml" -ForegroundColor Green
        }
    } else {
        Write-Host "  Preserved: config.toml (already exists)" -ForegroundColor Gray
    }

    # Create data directory
    $dataDir = Join-Path -Path $binDir -ChildPath "data"
    if (-not (Test-Path $dataDir)) {
        New-Item -ItemType Directory -Path $dataDir | Out-Null
    }
    Write-Host "  Created: data\" -ForegroundColor Green

    # Create logs directory
    $logsDir = Join-Path -Path $binDir -ChildPath "logs"
    if (-not (Test-Path $logsDir)) {
        New-Item -ItemType Directory -Path $logsDir | Out-Null
    }
    Write-Host "  Created: logs\" -ForegroundColor Green

    Write-Host "Deployment complete." -ForegroundColor Green
}

# Run if requested
if ($Run) {
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Yellow
    Write-Host "Starting iter-service" -ForegroundColor Yellow
    Write-Host "========================================" -ForegroundColor Yellow

    $configPath = Join-Path -Path $binDir -ChildPath "config.toml"

    if (Test-Path $configPath) {
        Write-Host "Config: $configPath" -ForegroundColor Gray
        $startCommand = "cd /d `"$binDir`" && `"$outputPath`" serve --config `"$configPath`""
    } else {
        Write-Host "Using default configuration" -ForegroundColor Gray
        $startCommand = "cd /d `"$binDir`" && `"$outputPath`" serve"
    }

    # Start in new terminal window
    Start-Process cmd -ArgumentList "/k", $startCommand

    Write-Host ""
    Write-Host "Service started in new terminal window" -ForegroundColor Green
    Write-Host "Press Ctrl+C in the server window to stop" -ForegroundColor Yellow
    exit 0
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "Build Complete" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host "Binary: $outputPath" -ForegroundColor Cyan
Write-Host "Version: $version" -ForegroundColor Cyan
Write-Host ""
Write-Host "To run:" -ForegroundColor Gray
Write-Host "  cd $binDir; .\iter-service.exe serve" -ForegroundColor Gray
Write-Host ""
Write-Host "Or use: .\scripts\build.ps1 -Run" -ForegroundColor Gray
