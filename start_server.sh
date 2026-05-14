#!/bin/bash

# Remote Desktop Server Launcher
# This script provides an easy way to start and stop the remote desktop server

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$BASE_DIR/server.pid"
LOG_FILE="$BASE_DIR/server.log"

start_server() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 $PID 2>/dev/null; then
            echo "Server is already running (PID: $PID)"
            return 1
        else
            rm -f "$PID_FILE"
        fi
    fi
    
    echo "Starting Remote Desktop Server..."
    cd "$BASE_DIR/backend"
    
    # Start server in background
    nohup npm start > "$LOG_FILE" 2>&1 &
    SERVER_PID=$!
    
    # Save PID to file
    echo $SERVER_PID > "$PID_FILE"
    
    # Wait a moment for server to start
    sleep 3
    
    if kill -0 $SERVER_PID 2>/dev/null; then
        echo "✅ Server started successfully!"
        echo "🌍 Access the application at: http://localhost:3000"
        echo "🔗 Share this link with others: http://localhost:3000"
        echo ""
        echo "📌 To share your screen:"
        echo "   1. Open http://localhost:3000 in your browser"
        echo "   2. Click 'Share My Screen'"
        echo "   3. Share the Room ID or link with the person you want to help"
        echo ""
        echo "📌 For the person receiving help:"
        echo "   1. Open http://localhost:3000 in their browser"
        echo "   2. Click 'View Remote Screen'"
        echo "   3. Enter the Room ID or use the shared link"
        echo "   4. Wait for approval from the host"
        echo ""
        echo "💡 TIP: Use different browsers or Incognito mode to test both sides on the same computer"
        echo ""
        echo "To stop the server, run: ./start_server.sh stop"
    else
        echo "❌ Failed to start server"
        rm -f "$PID_FILE"
        return 1
    fi
}

stop_server() {
    if [ ! -f "$PID_FILE" ]; then
        echo "Server is not running"
        return 1
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
        
        echo "✅ Server stopped"
    else
        echo "Server is not running"
    fi
    
    rm -f "$PID_FILE"
}

status_server() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 $PID 2>/dev/null; then
            echo "Server is running (PID: $PID)"
            echo "Access at: http://localhost:3000"
        else
            echo "Server is not running (stale PID file)"
            rm -f "$PID_FILE"
        fi
    else
        echo "Server is not running"
    fi
}

case "$1" in
    start)
        start_server
        ;;
    stop)
        stop_server
        ;;
    status)
        status_server
        ;;
    restart)
        stop_server
        sleep 2
        start_server
        ;;
    *)
        echo "Remote Desktop Server Launcher"
        echo "Usage: $0 {start|stop|status|restart}"
        echo ""
        echo "Commands:"
        echo "  start   - Start the server"
        echo "  stop    - Stop the server"
        echo "  status  - Check server status"
        echo "  restart - Restart the server"
        echo ""
        status_server
        ;;
esac