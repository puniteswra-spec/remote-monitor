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
echo  -- Agent --
echo  1) Check Status
echo  2) Open Dashboard
echo  3) Choose Server (which URL agents use)
echo  4) Make THIS PC the Server
echo  5) Make Any PC Server by IP
echo  6) Make Any PC Server (from list)
echo  7) Choose Tunnel Mode
echo  8) Find Server IP (scan network)
echo  9) View-Access link (share read-only)
echo.
echo  -- Maintenance --
echo  10) Cleanup Logs
echo  11) Stop Agent / Server
echo  12) Internal Network Setup
echo.
echo  0) Exit
echo ==========================================
set /p c="Choose: "

if "%c%"=="1"  goto STATUS
if "%c%"=="2"  goto DASHBOARD
if "%c%"=="3"  goto CHOOSE_SERVER
if "%c%"=="4"  goto MAKE_SERVER
if "%c%"=="5"  goto MAKE_SERVER_IP
if "%c%"=="6"  goto MAKE_SERVER_LIST
if "%c%"=="7"  goto TUNNEL
if "%c%"=="8"  goto FIND_IP
if "%c%"=="9"  goto VIEW_LINK
if "%c%"=="10" goto CLEANUP
if "%c%"=="11" goto STOP_ALL
if "%c%"=="12" goto INTERNAL_SETUP
if "%c%"=="0"  exit /b
goto MENU

:: ──────────────────────────────────────────
:STATUS
cls
echo ==========================================
echo   SYSTEM STATUS
echo ==========================================
echo.
echo Testing Render.com...
powershell -Command "try{$r=Invoke-WebRequest -Uri 'https://remote-monitor-1l0s.onrender.com' -TimeoutSec 10 -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}; Write-Host 'Render.com:  OK (' $r.StatusCode ')'}catch{Write-Host 'Render.com:  DOWN'}"
echo.
netstat -an | find ":3000" | find "LISTEN" >nul
if %errorlevel% equ 0 (echo Local Server:  Running ^(port 3000^)) else (echo Local Server:  Not running)
echo.
echo Connected agents:
powershell -Command "try{$r=Invoke-WebRequest -Uri 'https://remote-monitor-1l0s.onrender.com/api/agents' -TimeoutSec 10 -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}; ($r.Content|ConvertFrom-Json)|%%{Write-Host '  ' $_.name '(' $_.ip ')'}}catch{Write-Host '  (none)'}"
echo.
pause
goto MENU

:: ──────────────────────────────────────────
:DASHBOARD
cls
echo Which dashboard?
echo 1) Render.com (always on)
echo 2) Local (http://localhost:3000)
echo 3) Custom URL
echo.
set /p dc="Choose (1-3): "
if "%dc%"=="1" start https://remote-monitor-1l0s.onrender.com
if "%dc%"=="2" start http://localhost:3000
if "%dc%"=="3" (set /p durl="Enter URL: " & start !durl!)
goto MENU

:: ──────────────────────────────────────────
:CHOOSE_SERVER
cls
echo ==========================================
echo   CHOOSE SERVER FOR AGENTS
echo ==========================================
echo.
echo 1) Render.com (default, 24/7)
echo 2) ngrok tunnel
echo 3) Port forwarding (direct IP)
echo 4) Local network only
echo 5) Auto (try all)
echo.
set /p sc="Choose (1-5): "
if "%sc%"=="1" (> urls.ini echo wss://remote-monitor-1l0s.onrender.com & echo Set to Render.com)
if "%sc%"=="2" (> urls.ini echo wss://deviation-tweak-charter.ngrok-free.dev & echo Set to ngrok)
if "%sc%"=="3" (> urls.ini echo ws://43.247.40.101:3000 & echo Set to Port forwarding)
if "%sc%"=="4" (> urls.ini echo auto-local & echo Set to Local network only)
if "%sc%"=="5" (> urls.ini echo auto & echo Set to Auto mode)
echo.
echo urls.ini updated. Copy next to SystemHelper.exe on all PCs.
pause
goto MENU

:: ──────────────────────────────────────────
:MAKE_SERVER
cls
echo Making THIS PC the server...
if exist "SystemHelper.exe" (
    start "" SystemHelper.exe --server
    echo Server started. Dashboard: http://localhost:3000
) else (
    echo SystemHelper.exe not found in this folder.
)
echo.
pause
goto MENU

:: ──────────────────────────────────────────
:MAKE_SERVER_IP
cls
set /p target_ip="Enter target PC IP address: "
if "%target_ip%"=="" goto MENU
echo Sending server preference to %target_ip%...
powershell -Command "try{$r=Invoke-WebRequest -Uri 'https://remote-monitor-1l0s.onrender.com/api/agents' -TimeoutSec 10 -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}; $agents=($r.Content|ConvertFrom-Json); $found=$agents|?{$_.ip -eq '%target_ip%'}; if($found){Invoke-WebRequest -Method POST -Uri ('https://remote-monitor-1l0s.onrender.com/api/make-server/'+$found.id) -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}|Out-Null; Write-Host 'Done: ' $found.name}else{Write-Host 'No agent at ' '%target_ip%'}}catch{Write-Host 'Error: ' $_.Exception.Message}"
echo.
pause
goto MENU

:: ──────────────────────────────────────────
:MAKE_SERVER_LIST
cls
echo Fetching connected agents...
echo.
powershell -Command "try{$r=Invoke-WebRequest -Uri 'https://remote-monitor-1l0s.onrender.com/api/agents' -TimeoutSec 10 -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}; $agents=($r.Content|ConvertFrom-Json); $i=1; $agents|%%{Write-Host ($i++).ToString()+') '+$_.name+' ('+$_.ip+')'}; $c=Read-Host 'Enter number'; if($c -gt 0 -and $c -le $agents.Count){$a=$agents[$c-1]; Invoke-WebRequest -Method POST -Uri ('https://remote-monitor-1l0s.onrender.com/api/make-server/'+$a.id) -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}|Out-Null; Write-Host 'Sent to ' $a.name}else{Write-Host 'Invalid'}}catch{Write-Host 'Error: ' $_.Exception.Message}"
echo.
pause
goto MENU

:: ──────────────────────────────────────────
:TUNNEL
cls
echo ==========================================
echo   CHOOSE TUNNEL MODE
echo ==========================================
echo.
echo 1) Auto (try localhost.run then bore)
echo 2) localhost.run (SSH, no install)
echo 3) bore.pub (faster, one-time download)
echo 4) None (Render only)
echo.
set /p tc="Choose (1-4): "
if "%tc%"=="1" (echo auto > tunnel.ini & echo Set to Auto)
if "%tc%"=="2" (echo localhost.run > tunnel.ini & echo Set to localhost.run)
if "%tc%"=="3" (echo bore > tunnel.ini & echo Set to bore.pub)
if "%tc%"=="4" (echo none > tunnel.ini & echo Set to None)
echo.
echo tunnel.ini saved. Copy next to SystemHelper.exe.
pause
goto MENU

:: ──────────────────────────────────────────
:FIND_IP
cls
echo Scanning local network for active servers...
echo.
for /f "tokens=2 delims=:" %%a in ('ipconfig ^| find "IPv4"') do set IP=%%a
set IP=%IP: =%
for /f "tokens=1-3 delims=." %%a in ("%IP%") do set SUBNET=%%a.%%b.%%c
echo Scanning %SUBNET%.1-254...
for /l %%i in (1,1,254) do (
    powershell -Command "if(try{(Invoke-WebRequest -Uri 'http://%SUBNET%.%%i:3000' -TimeoutSec 1 -UseBasicParsing).StatusCode -eq 200}catch{$false}){Write-Host '  Found: %SUBNET%.%%i:3000'}" 2>nul
)
echo.
echo Done.
pause
goto MENU

:: ──────────────────────────────────────────
:VIEW_LINK
cls
echo ==========================================
echo   VIEW-ONLY ACCESS LINK
echo ==========================================
echo.
echo Share this link (view-only, no control):
echo   https://remote-monitor-1l0s.onrender.com
echo.
echo Login: puneet / puneet12
echo.
pause
goto MENU

:: ──────────────────────────────────────────
:CLEANUP
cls
set /p confirm="Clear all server logs + agent history? (y/n): "
if /i "%confirm%"=="y" (
    powershell -Command "try{$r=Invoke-WebRequest -Method POST -Uri 'https://remote-monitor-1l0s.onrender.com/api/cleanup' -TimeoutSec 15 -Headers @{'Authorization'='Basic cHVuZWV0OnB1bmVldDEy'}; Write-Host $r.Content}catch{Write-Host 'Error: '$_.Exception.Message}"
    echo Cleanup done.
) else (echo Cancelled.)
pause
goto MENU

:: ──────────────────────────────────────────
:STOP_ALL
cls
echo Stopping SystemHelper agent and local Node server...
taskkill /f /im SystemHelper.exe 2>nul
taskkill /f /im node.exe 2>nul
del "%~dp0agent.lock" 2>nul
echo Done. All processes stopped.
timeout /t 2 /nobreak >nul
goto MENU

:: ──────────────────────────────────────────
:INTERNAL_SETUP
cls
echo ==========================================
echo   INTERNAL NETWORK SETUP
echo ==========================================
echo.
echo Configure for internal network (no cloud/internet required).
echo.
echo 1) Make THIS PC the Server (internal)
echo 2) Make THIS PC an Agent (internal)
echo 3) Create organization config folder
echo.
set /p st="Choose (1-3): "

if "%st%"=="1" (
    echo auto-local > urls.ini
    echo.
    echo urls.ini set to internal mode.
    echo Run:  SystemHelper.exe --server
    echo Dashboard: http://[THIS-PC-IP]:3000
)
if "%st%"=="2" (
    echo auto-local > urls.ini
    echo.
    echo urls.ini set to internal mode.
    echo Run:  SystemHelper.exe
    echo Agent will auto-discover server on local network.
)
if "%st%"=="3" (
    set /p org_name="Enter organization name: "
    if not "!org_name!"=="" (
        mkdir "!org_name!" 2>nul
        echo auto-local > "!org_name!\urls.ini"
        copy SystemHelper.exe "!org_name!\" /Y >nul 2>&1
        echo Internal Server Setup > "!org_name!\README.txt"
        echo Organization: !org_name! >> "!org_name!\README.txt"
        echo. >> "!org_name!\README.txt"
        echo Server: run SystemHelper.exe --server >> "!org_name!\README.txt"
        echo Agent:  run SystemHelper.exe >> "!org_name!\README.txt"
        echo Dashboard: http://[SERVER-IP]:3000 >> "!org_name!\README.txt"
        echo.
        echo Folder created: !org_name!\
        echo Copy to all PCs in the organization.
    )
)
echo.
pause
goto MENU
