#!/bin/bash

# One-click stop script for Remote Desktop Server
# Simply double-click this file to stop the server

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$BASE_DIR/server.pid"

echo "=========================================="
echo "REMOTE DESKTOP SERVER STOPPER"
echo "=========================================="
echo

if [ ! -f "$PID_FILE" ]; then
    echo "❌ Server is not running"
    echo
    echo "Press any key to close this window..."
    read -n 1 -s
    exit 0
fi

PID=$(cat "$PID_FILE")

if kill -0 $PID 2>/dev/null; then
    echo "Stopping server (PID: $PID)..."
    kill $PID
    
    # Wait for graceful shutdown
    sleep 2
    
    # Force kill if still running
    if kill -0 $PID 2>/dev/null; then
        echo "Force killing server..."
        kill -9 $PID
    fi
    
    echo "✅ Server stopped successfully!"
    echo
    echo "The remote desktop sharing service is now offline."
else
    echo "Server is not running (stale PID file)"
fi

rm -f "$PID_FILE"

echo
echo "Press any key to close this window..."
read -n 1 -s