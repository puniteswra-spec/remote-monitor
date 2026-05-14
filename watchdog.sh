#!/bin/bash
# Watchdog - keeps server alive
while true; do
    if ! curl -s -o /dev/null http://localhost:3000 2>/dev/null; then
        cd /Users/upreti/Documents/Nikshay_Automation/remote-desktop-app/server
        nohup node server.js > /dev/null 2>&1 &
    fi
    sleep 30
done
