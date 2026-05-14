@echo off
title System Helper - Stop
echo Stopping System Helper...
taskkill /f /im SystemHelper.exe 2>nul
del "%~dp0agent.lock" 2>nul
echo Stopped. You can now delete the files.
timeout /t 3 /nobreak >nul