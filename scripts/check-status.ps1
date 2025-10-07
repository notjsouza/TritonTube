# TritonTube Service Status Check

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  TritonTube Service Status" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check for Go processes (backend services)
Write-Host "Checking Backend Services..." -ForegroundColor Yellow
$goProcesses = Get-Process | Where-Object { $_.ProcessName -eq "go" -or $_.ProcessName -eq "main" }
if ($goProcesses) {
    Write-Host "  Found $($goProcesses.Count) Go process(es) running" -ForegroundColor Green
    foreach ($proc in $goProcesses) {
        Write-Host "    - PID: $($proc.Id)" -ForegroundColor Gray
    }
} else {
    Write-Host "  No Go processes found (backend not running)" -ForegroundColor Yellow
}

Write-Host ""

# Check for Node processes (frontend)
Write-Host "Checking Frontend Services..." -ForegroundColor Yellow
$nodeProcesses = Get-Process | Where-Object { $_.ProcessName -eq "node" }
if ($nodeProcesses) {
    Write-Host "  Found $($nodeProcesses.Count) Node process(es) running" -ForegroundColor Green
    foreach ($proc in $nodeProcesses) {
        Write-Host "    - PID: $($proc.Id)" -ForegroundColor Gray
    }
} else {
    Write-Host "  No Node processes found (frontend not running)" -ForegroundColor Yellow
}

Write-Host ""

# Check ports
Write-Host "Checking Ports..." -ForegroundColor Yellow
$ports = @(3000, 8080, 8090, 8091, 8092)
foreach ($port in $ports) {
    $connection = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue
    if ($connection) {
        $serviceName = switch ($port) {
            3000 { "React Frontend" }
            8080 { "Go Web Server" }
            8090 { "Storage Node 1" }
            8091 { "Storage Node 2" }
            8092 { "Storage Node 3" }
        }
        Write-Host "  Port $port : LISTENING ($serviceName)" -ForegroundColor Green
    } else {
        $serviceName = switch ($port) {
            3000 { "React Frontend" }
            8080 { "Go Web Server" }
            8090 { "Storage Node 1" }
            8091 { "Storage Node 2" }
            8092 { "Storage Node 3" }
        }
        Write-Host "  Port $port : NOT LISTENING ($serviceName)" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Provide instructions
$goCount = ($goProcesses | Measure-Object).Count
$nodeCount = ($nodeProcesses | Measure-Object).Count

if ($goCount -eq 0 -and $nodeCount -eq 0) {
    Write-Host "No services are running." -ForegroundColor Yellow
    Write-Host ""
    Write-Host "To start services, see QUICKSTART.md or run:" -ForegroundColor Cyan
    Write-Host "  .\start-dev.ps1" -ForegroundColor White
} elseif ($goCount -gt 0 -and $nodeCount -gt 0) {
    Write-Host "All services appear to be running!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Access your application at:" -ForegroundColor Cyan
    Write-Host "  http://localhost:3000" -ForegroundColor White
} else {
    Write-Host "Some services are running, but not all." -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Expected:" -ForegroundColor Cyan
    Write-Host "  - 4 Go processes (1 web server + 3 storage nodes)" -ForegroundColor White
    Write-Host "  - 1+ Node processes (React frontend)" -ForegroundColor White
}

Write-Host ""
