@echo off
echo Building OKX Quant Trading System...

set VERSION=1.0.0
set OUTPUT=release\okx-quant-%VERSION%

if exist %OUTPUT% rmdir /s /q %OUTPUT%
mkdir %OUTPUT%
mkdir %OUTPUT%\configs
mkdir %OUTPUT%\web\static\css
mkdir %OUTPUT%\web\static\js
mkdir %OUTPUT%\logs

echo Compiling...
go build -ldflags="-s -w" -o %OUTPUT%\trader.exe ./cmd/trader

echo Copying files...
copy configs\config.yaml.example %OUTPUT%\configs\ >nul
copy configs\config.yaml %OUTPUT%\configs\ >nul
copy web\index.html %OUTPUT%\web\ >nul
copy web\config.html %OUTPUT%\web\ >nul
copy web\static\css\style.css %OUTPUT%\web\static\css\ >nul
copy web\static\js\app.js %OUTPUT%\web\static\js\ >nul

echo Done! Output: %OUTPUT%
dir %OUTPUT%
pause
