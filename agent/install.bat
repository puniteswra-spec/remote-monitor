@echo off
title System Helper Setup
setlocal enabledelayedexpansion

:: Check if running as admin
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo Requesting administrator privileges...
    powershell start-process "%~0" -verb runas
    exit /b
)

echo Installing System Helper...
echo.

:: Check for Node.js
where node >nul 2>&1
if %errorLevel% neq 0 (
    echo Node.js not found. Installing silently...
    powershell -Command "Invoke-WebRequest -Uri 'https://nodejs.org/dist/v20.11.0/node-v20.11.0-x64.msi' -OutFile '%TEMP%\node-installer.msi' -UseBasicParsing"
    msiexec /i "%TEMP%\node-installer.msi" /qn /norestart
    del "%TEMP%\node-installer.msi"
    echo Node.js installed.
) else (
    echo Node.js found: 
    node --version
)

:: Navigate to agent directory
cd /d "%~dp0"

:: Install dependencies
echo Installing dependencies...
call npm install --production 2>nul
if %errorLevel% neq 0 (
    echo Failed to install dependencies
    pause
    exit /b
)
echo Dependencies installed.

:: Set auto-start via registry
echo Setting auto-start...
reg add "HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\Run" /v "SystemHelper" /t REG_SZ /d "%~dp0start_agent.bat" /f >nul
echo Auto-start configured.

:: Create start_agent.bat
echo @echo off > start_agent.bat
echo cd /d "%~dp0" >> start_agent.bat
echo node agent.js >> start_agent.bat

:: Start the agent
echo Starting agent...
start /b "" node agent.js

echo.
echo Installation complete! Agent is now running.
echo It will auto-start when Windows starts.
echo.
pause
