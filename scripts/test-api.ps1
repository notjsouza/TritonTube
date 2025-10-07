# TritonTube API Health Check Script
# Tests connectivity between frontend and backend

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  TritonTube API Health Check" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$baseUrl = "http://localhost:8080"
$apiUrl = "$baseUrl/api/videos"

# Test 1: Check if backend is running
Write-Host "[1/3] Checking backend server..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri $baseUrl -Method HEAD -ErrorAction Stop
    Write-Host "Backend server is running" -ForegroundColor Green
} catch {
    Write-Host "Backend server is not running on $baseUrl" -ForegroundColor Red
    Write-Host "  Please start the backend with: go run ./cmd/web ..." -ForegroundColor Yellow
    exit 1
}

# Test 2: Check API endpoint
Write-Host "[2/3] Checking API endpoint..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri $apiUrl -Method GET -ErrorAction Stop
    Write-Host "API endpoint is accessible" -ForegroundColor Green
    
    if ($response.data) {
        Write-Host "  Found $($response.total) video(s)" -ForegroundColor White
    } else {
        Write-Host "  No videos found (this is normal for a fresh install)" -ForegroundColor White
    }
} catch {
    Write-Host "API endpoint failed: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test 3: Check CORS headers
Write-Host "[3/3] Checking CORS configuration..." -ForegroundColor Yellow
try {
    $headers = @{
        "Origin" = "http://localhost:3000"
    }
    $response = Invoke-WebRequest -Uri $apiUrl -Method OPTIONS -Headers $headers -ErrorAction Stop
    
    $corsHeader = $response.Headers["Access-Control-Allow-Origin"]
    if ($corsHeader) {
        Write-Host "CORS is properly configured" -ForegroundColor Green
        Write-Host "  Allowed origin: $corsHeader" -ForegroundColor White
    } else {
        Write-Host "WARNING: CORS headers not found (may cause issues)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "WARNING: Could not verify CORS configuration" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  All Checks Passed!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "Your TritonTube stack is ready!" -ForegroundColor Cyan
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "  1. Start the frontend: npm start" -ForegroundColor White
Write-Host "  2. Open browser: http://localhost:3000" -ForegroundColor White
Write-Host "  3. Upload a video and test streaming" -ForegroundColor White
Write-Host ""
