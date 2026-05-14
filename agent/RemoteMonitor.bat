@echo off
title Remote Monitor Manager
cd /d "%~dp0"
setlocal enabledelayedexpansion

:MENU
cls
echo ==========================================
echo   REMOTE MONITOR MANAGER - v6.0.0
echo ==========================================
echo.
echo 1) Check Status
echo 2) Open Dashboard
echo 3) Choose Server (which server agents use)
echo 4) Make THIS PC the Server (permanent)
echo 5) Make Any PC Server by IP
echo 6) Make Any PC Server (from list)
echo 7) Choose Tunnel Mode (localhost.run / bore)
echo 8) Find Server IP (scan network)
echo 9) View-Access link (share read-only)
echo 10) Cleanup Logs (clear all)
echo 11) Deploy to VPS info
echo 12) Stop Local Server (if running)
echo 13) Start Local Server
echo 0) Exit
echo ==========================================
set /p c="Choose (0-9): "

if "%c%"=="1" goto STATUS
if "%c%"=="2" goto DASHBOARD
if "%c%"=="3" goto CHOOSE_SERVER
if "%c%"=="4" goto MAKE_SERVER
if "%c%"=="5" goto MAKE_SERVER_IP
if "%c%"=="6" goto MAKE_SERVER_LIST
if "%c%"=="7" goto TUNNEL
if "%c%"=="8" goto FIND_IP
if "%c%"=="9" goto VIEW_LINK
if "%c%"=="10" goto CLEANUP
if "%c%"=="11" goto VPS_INFO
if "%c%"=="12" goto STOP_SERVER
if "%c%"=="13" goto START_SERVER
if "%c%"=="0" exit /b
goto MENU

:STATUS
cls
echo ==========================================
echo   SYSTEM STATUS
echo ==========================================
echo.
echo Testing servers...
echo.

:: Test Render (always on)
powershell -Command "try{$r=Invoke-WebRequest -Uri 'https://remote-monitor-1l0s.onrender.com' -TimeoutSec 10 -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}; Write-Host 'Render.com:  ' $r.StatusCode ' (https://remote-monitor-1l0s.onrender.com)'}catch{Write-Host 'Render.com:  ❌ (https://remote-monitor-1l0s.onrender.com)'}"
echo.

:: Check if local server is running
netstat -an | find ":3000" | find "LISTEN" >nul
if %errorlevel% equ 0 (
    echo Local Server:  Running (port 3000)
) else (
    echo Local Server:  Not running
)
echo.
echo Connected agents (via Render):
powershell -Command "try{$r=Invoke-WebRequest -Uri 'https://remote-monitor-1l0s.onrender.com/api/agents' -TimeoutSec 10 -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}; Write-Host $r.Content}catch{Write-Host '(none)'}"
echo.
pause
goto MENU

:DASHBOARD
cls
echo ==========================================
echo   OPEN DASHBOARD
echo ==========================================
echo.
echo Which dashboard?
echo 1) Render.com (always on)
echo 2) Local Server (if running)
echo 3) Enter custom URL
echo.
set /p dc="Choose (1-3): "
if "%dc%"=="1" start https://remote-monitor-1l0s.onrender.com
if "%dc%"=="2" start http://localhost:3000
if "%dc%"=="3" (
    set /p curl="Enter URL: "
    start !curl!
)
goto MENU

:CHOOSE_SERVER
cls
echo ==========================================
echo   CHOOSE SERVER FOR AGENTS
echo ==========================================
echo.
echo Select which server agents should use:
echo.
echo 1) Render.com (default, 24/7)
echo 2) ngrok tunnel
echo 3) Port forwarding (direct)
echo 4) Local network only
echo 5) Auto (try all)
echo.
set /p sc="Choose (1-5): "

if "%sc%"=="1" (
    > urls.ini echo wss://remote-monitor-1l0s.onrender.com
    echo Set to Render.com
)
if "%sc%"=="2" (
    > urls.ini echo wss://deviation-tweak-charter.ngrok-free.dev
    echo Set to ngrok
)
if "%sc%"=="3" (
    > urls.ini echo ws://43.247.40.101:3000
    echo Set to Port forwarding
)
if "%sc%"=="4" (
    > urls.ini echo auto-local
    echo Set to Local network only
)
if "%sc%"=="5" (
    > urls.ini echo auto
    echo Set to Auto mode
)
echo.
echo urls.ini updated. Copy this file next to SystemHelper.exe on all PCs.
pause
goto MENU

:MAKE_SERVER
cls
echo ==========================================
echo   MAKE THIS PC THE SERVER
echo ==========================================
echo.
echo Starting server on this PC...
echo.
:: Check if SystemHelper.exe is in this folder
if exist "SystemHelper.exe" (
    start "" SystemHelper.exe --server
    echo Server started. Dashboard: http://localhost:3000
) else (
    echo SystemHelper.exe not found in this folder.
    echo Please run this from the folder containing the .exe
)
echo.
pause
goto MENU

:MAKE_SERVER_IP
cls
echo ==========================================
echo   MAKE ANY PC SERVER BY IP
echo ==========================================
echo.
set /p target_ip="Enter target PC IP address: "
if "%target_ip%"=="" goto MENU
echo.
echo Sending server preference to %target_ip%...
powershell -Command "try{$r=Invoke-WebRequest -Uri 'https://remote-monitor-1l0s.onrender.com/api/agents' -TimeoutSec 10 -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}; $agents=($r.Content|ConvertFrom-Json); $found=$agents|?{$_.ip -eq '%target_ip%'}; if($found){Write-Host 'Found:' $found.name; Invoke-WebRequest -Method POST -Uri ('https://remote-monitor-1l0s.onrender.com/api/make-server/'+$found.id) -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'} | Out-Null; Write-Host 'Server preference sent!'}else{Write-Host 'No agent found with IP:' '%target_ip%'}}catch{Write-Host 'Error:' $_.Exception.Message}"
echo.
pause
goto MENU

:MAKE_SERVER_LIST
cls
echo ==========================================
echo   SELECT AGENT TO MAKE SERVER
echo ==========================================
echo.
echo Fetching connected agents...
echo.
powershell -Command "try{$r=Invoke-WebRequest -Uri 'https://remote-monitor-1l0s.onrender.com/api/agents' -TimeoutSec 10 -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}; $agents=($r.Content|ConvertFrom-Json); $i=1; $agents|%{Write-Host ('{0}) {1} ({2})' -f $i++, $_.name, $_.ip)};$c=Read-Host 'Enter number'; if($c -gt 0 -and $c -le $agents.Count){$a=$agents[$c-1]; Invoke-WebRequest -Method POST -Uri ('https://remote-monitor-1l0s.onrender.com/api/make-server/'+$a.id) -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'} | Out-Null; Write-Host ('Server preference sent to '+$a.name)}else{Write-Host 'Invalid'}}catch{Write-Host 'Error: '$_.Exception.Message}"
echo.
pause
goto MENU

:TUNNEL
cls
echo ==========================================
echo   CHOOSE TUNNEL MODE
echo ==========================================
echo.
echo Select tunnel for direct connections:
echo.
echo 1) Auto (try localhost.run -^> bore)
echo 2) localhost.run (SSH, no install)
echo 3) bore.pub (faster, one-time download)
echo 4) None (Render only)
echo.
set /p tc="Choose (1-4): "
if "%tc%"=="1" echo auto > tunnel.ini & echo Set to Auto
if "%tc%"=="2" echo localhost.run > tunnel.ini & echo Set to localhost.run
if "%tc%"=="3" echo bore > tunnel.ini & echo Set to bore.pub
if "%tc%"=="4" echo none > tunnel.ini & echo Set to None
echo.
echo tunnel.ini created. Copy next to SystemHelper.exe
pause
goto MENU

:FIND_IP
cls
echo ==========================================
echo   FIND ACTIVE SERVERS ON NETWORK
echo ==========================================
echo.
echo Scanning local network...
echo.
:: Get local IP
for /f "tokens=2 delims=:" %%a in ('ipconfig ^| find "IPv4"') do set IP=%%a
set IP=%IP: =%
for /f "tokens=1-3 delims=." %%a in ("%IP%") do set SUBNET=%%a.%%b.%%c

echo Scanning %SUBNET%.1-254...
echo.
set FOUND=0
for /l %%i in (1,1,254) do (
    powershell -Command "if(try{$r=Invoke-WebRequest -Uri 'http://%SUBNET%.%%i:3000' -TimeoutSec 1 -UseBasicParsing; $r.StatusCode -eq 200}catch{$false}){Write-Host '  Server: %SUBNET%.%%i:3000'}" 2>nul
)
echo.
echo Scan complete.
pause
goto MENU

:VIEW_LINK
cls
echo ==========================================
echo   VIEW-ONLY ACCESS LINK
echo ==========================================
echo.
echo Share this link with others (view-only, no control):
echo.
echo   https://remote-monitor-1l0s.onrender.com
echo.
echo Login: puneet / puneet12
echo.
echo They can see all agents but CANNOT control.
echo Only you can enable control by clicking the lock icon.
echo.
pause
goto MENU

:CLEANUP
cls
echo ==========================================
echo   CLEANUP LOGS
echo ==========================================
echo.
echo This will clear all logs from:
echo   - Server history
echo   - All connected agents
echo.
set /p confirm="Are you sure? (y/n): "
if /i "%confirm%"=="y" (
    powershell -Command "try{$r=Invoke-WebRequest -Method POST -Uri 'https://remote-monitor-1l0s.onrender.com/api/cleanup' -TimeoutSec 15 -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}; Write-Host $r.Content}catch{Write-Host 'Error: '$_.Exception.Message}"
    echo.
    echo Cleanup completed.
) else (
    echo Cancelled.
)
pause
goto MENU

:VPS_INFO
cls
echo ==========================================
echo   DEPLOY SERVER ON VPS
echo ==========================================
echo.
echo To run the server on a VPS (24/7, no laptop needed):
echo.
echo 1. Rent a VPS (DigitalOcean, Linode, Hetzner)
echo    $5-10/month
echo.
echo 2. Install Node.js:
echo    curl -fsSL https://deb.nodesource.com/setup_20.x ^| bash -
echo    apt install -y nodejs
echo.
echo 3. Copy server files:
echo    scp -r server/ root@YOUR_VPS_IP:/opt/remote-monitor/
echo.
echo 4. Start server:
echo    cd /opt/remote-monitor
echo    npm install
echo    node server.js
echo.
echo 5. Update agent URLs to point to your VPS IP
echo.
pause
goto MENU

:STOP_SERVER
cls
echo Stopping local server...
taskkill /f /im SystemHelper.exe 2>nul
taskkill /f /im node.exe 2>nul
echo Done.
timeout /t 2 /nobreak >nul
goto MENU

:START_SERVER
cls
echo Starting local server...
if exist "SystemHelper.exe" (
    start "" SystemHelper.exe --server
    echo Server started.
) else (
    echo SystemHelper.exe not found here.
)
timeout /t 2 /nobreak >nul
goto MENU
