#!/bin/bash

# Stop Remote Desktop Server with Internet Access
# This script stops the server and ngrok tunnel

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$BASE_DIR/server.pid"
NGROK_PID_FILE="$BASE_DIR/ngrok.pid"

echo "=========================================="
echo "STOPPING REMOTE DESKTOP SERVER"
echo "=========================================="
echo

# Stop ngrok if running
if [ -f "$NGROK_PID_FILE" ]; then
    NGROK_PID=$(cat "$NGROK_PID_FILE")
    if kill -0 $NGROK_PID 2>/dev/null; then
        echo "Stopping ngrok tunnel (PID: $NGROK_PID)..."
        kill $NGROK_PID
        
        # Wait for graceful shutdown
        sleep 2
        
        # Force kill if still running
        if kill -0 $NGROK_PID 2>/dev/null; then
            echo "Force killing ngrok..."
            kill -9 $NGROK_PID
        fi
        
        echo "✅ ngrok tunnel stopped"
    else
        echo "ngrok was not running"
    fi
    rm -f "$NGROK_PID_FILE"
fi

# Stop the main server
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
else
    echo "Server is not running (stale PID file)"
fi

rm -f "$PID_FILE"

echo
echo "✅ Remote desktop sharing service is now completely offline."
echo
echo "Press any key to close this window..."
read -n 1 -s