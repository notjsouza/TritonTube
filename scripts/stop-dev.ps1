#!/usr/bin/env pwsh
# TritonTube Development Stop Script
# This script stops all running TritonTube processes

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Stopping TritonTube Services..." -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Stop all Go processes (storage nodes and web server)
Write-Host "Stopping Go processes..." -ForegroundColor Yellow
$goProcesses = Get-Process | Where-Object { $_.ProcessName -eq "go" -or $_.ProcessName -eq "main" }
foreach ($proc in $goProcesses) {
    try {
        Stop-Process -Id $proc.Id -Force
        Write-Host "✓ Stopped process: $($proc.ProcessName) (PID: $($proc.Id))" -ForegroundColor Green
    } catch {
        Write-Host "✗ Failed to stop process: $($proc.ProcessName) (PID: $($proc.Id))" -ForegroundColor Red
    }
}

# Stop Node.js processes (React dev server)
Write-Host "Stopping Node.js processes..." -ForegroundColor Yellow
$nodeProcesses = Get-Process | Where-Object { $_.ProcessName -eq "node" }
foreach ($proc in $nodeProcesses) {
    try {
        # Check if it's a React process by checking command line
        $commandLine = (Get-WmiObject Win32_Process -Filter "ProcessId = $($proc.Id)").CommandLine
        if ($commandLine -like "*react-scripts*") {
            Stop-Process -Id $proc.Id -Force
            Write-Host "✓ Stopped process: $($proc.ProcessName) (PID: $($proc.Id))" -ForegroundColor Green
        }
    } catch {
        # Ignore errors for processes we can't access
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  All Services Stopped!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
