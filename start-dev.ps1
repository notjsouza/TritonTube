# TritonTube Development Startup Script
# This script starts both the backend (Go) and frontend (React) servers

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  TritonTube Development Server Startup" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if Go is installed
Write-Host "[1/6] Checking Go installation..." -ForegroundColor Yellow
try {
    $goVersion = go version
    Write-Host "Go is installed: $goVersion" -ForegroundColor Green
} catch {
    Write-Host "Go is not installed. Please install Go from https://golang.org/dl/" -ForegroundColor Red
    exit 1
}

# Check if Node.js is installed
Write-Host "[2/6] Checking Node.js installation..." -ForegroundColor Yellow
try {
    $nodeVersion = node --version
    Write-Host "Node.js is installed: $nodeVersion" -ForegroundColor Green
} catch {
    Write-Host "Node.js is not installed. Please install Node.js from https://nodejs.org/" -ForegroundColor Red
    exit 1
}

# Check if FFmpeg is installed (required for video processing)
Write-Host "[3/6] Checking FFmpeg installation..." -ForegroundColor Yellow
try {
    $ffmpegVersion = ffmpeg -version | Select-Object -First 1
    Write-Host "FFmpeg is installed: $ffmpegVersion" -ForegroundColor Green
} catch {
    Write-Host "WARNING: FFmpeg is not installed. Video uploads will not work!" -ForegroundColor Yellow
    Write-Host "  Install FFmpeg from https://ffmpeg.org/download.html" -ForegroundColor Yellow
}

# Install Node dependencies if needed
Write-Host "[4/6] Checking Node.js dependencies..." -ForegroundColor Yellow
if (-not (Test-Path "node_modules")) {
    Write-Host "Installing Node.js dependencies..." -ForegroundColor Yellow
    npm install
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Failed to install Node.js dependencies" -ForegroundColor Red
        exit 1
    }
    Write-Host "Node.js dependencies installed" -ForegroundColor Green
} else {
    Write-Host "Node.js dependencies already installed" -ForegroundColor Green
}

# Create storage directories if they don't exist
Write-Host "[5/6] Setting up storage directories..." -ForegroundColor Yellow
$storageDirs = @("storage/8090", "storage/8091", "storage/8092")
foreach ($dir in $storageDirs) {
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
}
Write-Host "Storage directories ready" -ForegroundColor Green

# Initialize SQLite database if it doesn't exist
Write-Host "[6/6] Checking database..." -ForegroundColor Yellow
if (-not (Test-Path "metadata.db")) {
    Write-Host "Database will be created on first run" -ForegroundColor Yellow
} else {
    Write-Host "Database exists" -ForegroundColor Green
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Starting Services..." -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Start storage nodes in background
Write-Host "Starting Storage Node 1 (Port 8090)..." -ForegroundColor Yellow
$storage1 = Start-Process powershell -ArgumentList "-NoExit", "-Command", "go run ./cmd/storage -port 8090 ./storage/8090" -PassThru -WindowStyle Normal

Write-Host "Starting Storage Node 2 (Port 8091)..." -ForegroundColor Yellow
$storage2 = Start-Process powershell -ArgumentList "-NoExit", "-Command", "go run ./cmd/storage -port 8091 ./storage/8091" -PassThru -WindowStyle Normal

Write-Host "Starting Storage Node 3 (Port 8092)..." -ForegroundColor Yellow
$storage3 = Start-Process powershell -ArgumentList "-NoExit", "-Command", "go run ./cmd/storage -port 8092 ./storage/8092" -PassThru -WindowStyle Normal

# Wait for storage nodes to start
Write-Host "Waiting for storage nodes to initialize..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

# Start the web server (admin node)
Write-Host "Starting Web Server (Port 8080)..." -ForegroundColor Yellow
$webserver = Start-Process powershell -ArgumentList "-NoExit", "-Command", "go run ./cmd/web -port 8080 -host localhost sqlite ./metadata.db nw localhost:8081,localhost:8090,localhost:8091,localhost:8092" -PassThru -WindowStyle Normal

# Wait for web server to start
Write-Host "Waiting for web server to initialize..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

# Start the React frontend
Write-Host "Starting React Frontend (Port 3000)..." -ForegroundColor Yellow
$frontend = Start-Process powershell -ArgumentList "-NoExit", "-Command", "npm start" -PassThru -WindowStyle Normal

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  All Services Started!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "Services:" -ForegroundColor Cyan
Write-Host "  Frontend:      http://localhost:3000" -ForegroundColor White
Write-Host "  Backend API:   http://localhost:8080/api/videos" -ForegroundColor White
Write-Host "  Storage Node 1: localhost:8090" -ForegroundColor White
Write-Host "  Storage Node 2: localhost:8091" -ForegroundColor White
Write-Host "  Storage Node 3: localhost:8092" -ForegroundColor White
Write-Host ""
Write-Host "To stop all services, close the terminal windows or run:" -ForegroundColor Yellow
Write-Host "  .\scripts\stop-dev.ps1" -ForegroundColor White
Write-Host ""
Write-Host "Press Ctrl+C to exit this script (services will continue running)" -ForegroundColor Yellow
Write-Host ""

# Keep script running
try {
    while ($true) {
        Start-Sleep -Seconds 1
    }
} finally {
    Write-Host "Script terminated" -ForegroundColor Yellow
}
