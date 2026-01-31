# iter-service Windows installation script
# Installs iter-service and optionally creates a scheduled task

$ErrorActionPreference = "Stop"

$InstallDir = "$env:LOCALAPPDATA\iter-service\bin"
$DataDir = "$env:APPDATA\iter-service"

Write-Host "Installing iter-service..." -ForegroundColor Green

# Create directories
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
New-Item -ItemType Directory -Force -Path "$DataDir\logs" | Out-Null

# Build the binary
Write-Host "Building iter-service..."
Push-Location (Split-Path -Parent $PSScriptRoot)

$version = "dev"
if (Test-Path ".version") {
    $version = Get-Content ".version" -Raw
    $version = $version.Trim()
}

go build -ldflags "-X main.version=$version" -o iter-service.exe ./cmd/iter-service
if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    Pop-Location
    exit 1
}

# Install binary
Write-Host "Installing binary to $InstallDir..."
Copy-Item "iter-service.exe" -Destination "$InstallDir\iter-service.exe" -Force

Pop-Location

# Add to PATH if not present
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$InstallDir*") {
    Write-Host "Adding $InstallDir to PATH..." -ForegroundColor Yellow
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
}

# Create example config
$configPath = "$DataDir\config.yaml"
if (-not (Test-Path $configPath)) {
    Write-Host "Creating example configuration..."
    @"
# iter-service configuration

service:
  host: "127.0.0.1"
  port: 8420
  data_dir: "$($DataDir -replace '\\', '/')"
  log_level: "info"

api:
  enabled: true
  api_key: ""

mcp:
  enabled: true

llm:
  api_key: ""
"@ | Out-File -FilePath $configPath -Encoding UTF8
}

Write-Host ""
Write-Host "Installation complete!" -ForegroundColor Green
Write-Host ""
Write-Host "To start the service:"
Write-Host "  iter-service"
Write-Host ""
Write-Host "To start in background (scheduled task):"
Write-Host "  schtasks /create /tn 'iter-service' /tr '$InstallDir\iter-service.exe serve' /sc onlogon"
Write-Host ""
Write-Host "Configuration: $configPath"
Write-Host "Logs: $DataDir\logs\"
Write-Host ""
Write-Host "Note: You may need to restart your terminal for PATH changes to take effect."
