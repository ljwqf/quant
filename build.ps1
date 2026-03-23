# Build script for OKX Quant Trading System
$VERSION = "1.0.0"
$OUTPUT = "release\okx-quant-$VERSION"

Write-Host "========================================"
Write-Host "Building OKX Quant Trading System"
Write-Host "========================================"

# Create output directory
Write-Host "Creating output directory..."
if (Test-Path $OUTPUT) {
    Remove-Item -Recurse -Force $OUTPUT
}
New-Item -ItemType Directory -Path $OUTPUT | Out-Null
New-Item -ItemType Directory -Path "$OUTPUT\configs" | Out-Null
New-Item -ItemType Directory -Path "$OUTPUT\web\static\css" | Out-Null
New-Item -ItemType Directory -Path "$OUTPUT\web\static\js" | Out-Null
New-Item -ItemType Directory -Path "$OUTPUT\logs" | Out-Null

# Build program
Write-Host "Building program..."
go build -ldflags="-s -w" -o "$OUTPUT\trader.exe" ./cmd/trader
if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!"
    exit 1
}

# Copy config files
Write-Host "Copying config files..."
Copy-Item "configs\config.yaml.example" "$OUTPUT\configs\" -ErrorAction SilentlyContinue
Copy-Item "configs\config.yaml" "$OUTPUT\configs\" -ErrorAction SilentlyContinue

# Copy web files
Write-Host "Copying web files..."
Copy-Item "web\index.html" "$OUTPUT\web\" -ErrorAction SilentlyContinue
Copy-Item "web\config.html" "$OUTPUT\web\" -ErrorAction SilentlyContinue
Copy-Item "web\static\css\style.css" "$OUTPUT\web\static\css\" -ErrorAction SilentlyContinue
Copy-Item "web\static\js\app.js" "$OUTPUT\web\static\js\" -ErrorAction SilentlyContinue

# Create start script
Write-Host "Creating start script..."
$startBat = "@echo off`necho Starting OKX Quant Trading System...`ntrader.exe`npause"
$startBat | Out-File -FilePath "$OUTPUT\start.bat" -Encoding ASCII

# Create README
Write-Host "Creating README..."
$readme = "OKX Quant Trading System v$VERSION`n`nUsage:`n1. Edit configs\config.yaml to set API keys`n2. Run start.bat to start the system`n3. Open http://localhost:8765 for Dashboard`n`nConfig Management:`n- Click 'Config Management' in Dashboard to modify config online`n- Restart system to apply config changes"
$readme | Out-File -FilePath "$OUTPUT\README.txt" -Encoding UTF8

Write-Host "========================================"
Write-Host "Build completed!"
Write-Host "Output: $OUTPUT"
Write-Host "========================================"

Get-ChildItem $OUTPUT
