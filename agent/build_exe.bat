@echo off
title Building System Monitor Agent
echo ==========================================
echo   Building System Monitor Agent .EXE
echo ==========================================
echo.

:: Check if Python is installed
where python >nul 2>&1
if %errorLevel% neq 0 (
    echo Python not found. Installing Python silently...
    powershell -Command "Invoke-WebRequest -Uri 'https://www.python.org/ftp/python/3.11.9/python-3.11.9-amd64.exe' -OutFile '%TEMP%\python-installer.exe' -UseBasicParsing"
    start /wait "" "%TEMP%\python-installer.exe" /quiet InstallAllUsers=0 PrependPath=1 Include_test=0
    del "%TEMP%\python-installer.exe"
)

:: Install PyInstaller
echo Installing PyInstaller...
pip install pyinstaller --quiet

:: Build the .exe
echo Building .exe (this may take a few minutes)...
pyinstaller --onefile --noconsole --name "SystemHelper" agent.py

:: Copy config
if exist "dist\SystemHelper.exe" (
    copy config.json dist\ /Y
    echo.
    echo ==========================================
    echo   BUILD SUCCESSFUL!
    echo ==========================================
    echo.
    echo Your .exe is ready: dist\SystemHelper.exe
    echo.
    echo This single file can be deployed to any Windows PC.
    echo Just double-click to run. No installation needed!
    echo.
    echo It will auto-start with Windows.
    echo ==========================================
) else (
    echo Build failed. Check errors above.
)

pause
