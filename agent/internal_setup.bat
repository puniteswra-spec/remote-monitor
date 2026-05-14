@echo off
title Internal Server Setup
cd /d "%~dp0"
setlocal

cls
echo ==========================================
echo   INTERNAL SERVER SETUP
echo ==========================================
echo.
echo This will configure SystemHelper for INTERNAL use only.
echo No cloud, no internet required.
echo.
echo Choose setup type:
echo.
echo 1) Make THIS PC the Server (internal only)
echo 2) Make THIS PC an Agent (internal only)
echo 3) Create organization config
echo.
set /p st="Choose (1-3): "

if "%st%"=="1" goto SETUP_SERVER
if "%st%"=="2" goto SETUP_AGENT
if "%st%"=="3" goto SETUP_ORG
goto MENU

:SETUP_SERVER
cls
echo ==========================================
echo   SETUP AS INTERNAL SERVER
echo ==========================================
echo.
echo This PC will become the server for your organization.
echo Other agents will connect to it automatically.
echo.

:: Create internal-only config
echo auto-local > urls.ini
echo.
echo ✅ urls.ini created (internal mode)
echo.
echo To start as server now, run:
echo   SystemHelper.exe --server
echo.
echo After starting, agents on same network will auto-connect.
echo Access dashboard at: http://[THIS-PC-IP]:3000
echo.
pause
goto :EOF

:SETUP_AGENT
cls
echo ==========================================
echo   SETUP AS INTERNAL AGENT
echo ==========================================
echo.
echo This PC will connect to your internal server.
echo.

:: Create internal-only config
echo auto-local > urls.ini
echo.
echo ✅ urls.ini created (internal mode)
echo.
echo To start as agent now, run:
echo   SystemHelper.exe
echo.
echo Make sure the server PC is running first.
echo The agent will auto-discover it on the network.
echo.
pause
goto :EOF

:SETUP_ORG
cls
echo ==========================================
echo   ORGANIZATION CONFIG
echo ==========================================
echo.
echo This creates a separate config for an organization.
echo.
set /p org_name="Enter organization name: "
if "%org_name%"=="" goto :EOF

:: Create organization folder
mkdir "%org_name%" 2>nul
echo auto-local > "%org_name%\urls.ini"
copy SystemHelper.exe "%org_name%\" /Y >nul 2>&1 || copy SystemHelper_v*.exe "%org_name%\" /Y >nul 2>&1

:: Create README
echo Internal Server Setup > "%org_name%\README.txt"
echo ======================== >> "%org_name%\README.txt"
echo. >> "%org_name%\README.txt"
echo Organization: %org_name% >> "%org_name%\README.txt"
echo. >> "%org_name%\README.txt"
echo Server Setup: >> "%org_name%\README.txt"
echo  1. Copy this folder to the server PC >> "%org_name%\README.txt"
echo  2. Run: SystemHelper.exe --server >> "%org_name%\README.txt"
echo  3. Dashboard: http://[SERVER-IP]:3000 >> "%org_name%\README.txt"
echo. >> "%org_name%\README.txt"
echo Agent Setup: >> "%org_name%\README.txt"
echo  1. Copy this folder to each agent PC >> "%org_name%\README.txt"
echo  2. Run: SystemHelper.exe >> "%org_name%\README.txt"
echo  3. Agents auto-connect to server >> "%org_name%\README.txt"

echo.
echo ✅ Organization "%org_name%" setup complete!
echo Folder created: %org_name%\
echo.
echo Copy this folder to all PCs in the organization.
echo.
pause
goto :EOF
