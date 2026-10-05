$ErrorActionPreference = "Stop"

$Root = $PSScriptRoot
$CaddyDir = Join-Path $Root "caddy"
$CaddyExe = Join-Path $CaddyDir "caddy.exe"
$CaddyConfig = Join-Path $CaddyDir "Caddyfile"

Write-Host "=================================" -ForegroundColor Cyan
Write-Host " Starting Social Network" -ForegroundColor Cyan
Write-Host "=================================" -ForegroundColor Cyan

# -------------------------------
# Caddy
# -------------------------------

if (!(Test-Path $CaddyExe)) {
    Write-Host "Caddy not found. Downloading..." -ForegroundColor Yellow

    New-Item -ItemType Directory -Force -Path $CaddyDir | Out-Null

    $CaddyUrl = "https://caddyserver.com/api/download?os=windows&arch=amd64"

    Invoke-WebRequest `
        -Uri $CaddyUrl `
        -OutFile $CaddyExe

    Write-Host "Caddy downloaded." -ForegroundColor Green
}

# -------------------------------
# Start Caddy
# -------------------------------

Write-Host "Starting Caddy..." -ForegroundColor Green

Start-Process `
    -FilePath $CaddyExe `
    -ArgumentList "run", "--config", "`"$CaddyConfig`"" `
    -WorkingDirectory $CaddyDir

# -------------------------------
# Start Backend
# -------------------------------

Write-Host "Starting backend..." -ForegroundColor Green

Start-Process `
    powershell `
    -ArgumentList "-NoExit", "-Command", "cd '$Root\backend'; go run ./cmd/server"

# -------------------------------
# Start Frontend
# -------------------------------

Write-Host "Starting frontend..." -ForegroundColor Green

Start-Process `
    powershell `
    -ArgumentList "-NoExit", "-Command", "cd '$Root\frontend'; npm ci; npm run dev"

Write-Host ""
Write-Host "=================================" -ForegroundColor Green
Write-Host " Everything started!" -ForegroundColor Green
Write-Host "=================================" -ForegroundColor Green
