#!/bin/bash

# One-click start script for Remote Desktop Server
# Simply double-click this file to start the server

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$BASE_DIR/server.pid"
LOG_FILE="$BASE_DIR/server.log"

echo "=========================================="
echo "REMOTE DESKTOP SERVER LAUNCHER"
echo "=========================================="
echo

# Check if already running
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 $PID 2>/dev/null; then
        echo "✅ Server is already running!"
        echo "🌍 Access the application at: http://localhost:3000"
        echo
        echo "🔗 SHARE THIS LINK with the person you want to help:"
        echo "   http://localhost:3000"
        echo
        echo "📌 INSTRUCTIONS:"
        echo "   1. Open http://localhost:3000 in your browser"
        echo "   2. Click 'Share My Screen'"
        echo "   3. Share the Room ID with the person needing help"
        echo
        echo "To stop the server, double-click STOP_SERVER.command"
        exit 0
    else
        rm -f "$PID_FILE"
    fi
fi

echo "Starting Remote Desktop Server..."
echo

cd "$BASE_DIR/backend"

# Start server in background
nohup npm start > "$LOG_FILE" 2>&1 &
SERVER_PID=$!

# Save PID to file
echo $SERVER_PID > "$PID_FILE"

# Wait a moment for server to start
sleep 3

if kill -0 $SERVER_PID 2>/dev/null; then
    echo "✅ SUCCESS! Server started successfully!"
    echo
    echo "🌍 ACCESS THE APPLICATION:"
    echo "   Open your browser and go to: http://localhost:3000"
    echo
    echo "🔗 SHARE THIS LINK with the person you want to help:"
    echo "   http://localhost:3000"
    echo
    echo "📌 EASY INSTRUCTIONS:"
    echo "   1. Open http://localhost:3000 in your browser"
    echo "   2. Click 'Share My Screen'"
    echo "   3. Share the Room ID with the person needing help"
    echo
    echo "To stop the server, double-click STOP_SERVER.command"
    echo
    echo "Press any key to close this window..."
    read -n 1 -s
else
    echo "❌ Failed to start server"
    rm -f "$PID_FILE"
fi