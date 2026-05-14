#!/bin/bash

# Start Remote Desktop Server with Internet Access
# This script starts the server and creates a public URL using ngrok

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$BASE_DIR/server.pid"
LOG_FILE="$BASE_DIR/server.log"
NGROK_PID_FILE="$BASE_DIR/ngrok.pid"

echo "=========================================="
echo "REMOTE DESKTOP SERVER WITH INTERNET ACCESS"
echo "=========================================="
echo

# Check if already running
if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if kill -0 $PID 2>/dev/null; then
        echo "✅ Server is already running!"
        echo "🌍 Local access: http://localhost:3000"
        echo
        echo "To get your public URL, please wait for ngrok to start..."
        echo
    else
        rm -f "$PID_FILE"
    fi
fi

# Start the backend server
echo "Starting Remote Desktop Server..."
cd "$BASE_DIR/backend"

# Start server in background
nohup npm start > "$LOG_FILE" 2>&1 &
SERVER_PID=$!

# Save PID to file
echo $SERVER_PID > "$PID_FILE"

# Wait for server to start
sleep 3

if kill -0 $SERVER_PID 2>/dev/null; then
    echo "✅ Local server started successfully!"
    echo "🌍 Local access: http://localhost:3000"
    echo
else
    echo "❌ Failed to start local server"
    rm -f "$PID_FILE"
    exit 1
fi

# Start ngrok tunnel
echo "Starting ngrok tunnel (this may take a moment)..."
cd "$BASE_DIR"

# Start ngrok in background
nohup npx ngrok http 3000 > "$BASE_DIR/ngrok.log" 2>&1 &
NGROK_PID=$!

# Save ngrok PID
echo $NGROK_PID > "$NGROK_PID_FILE"

# Wait for ngrok to start
sleep 5

# Get the public URL
echo "Getting public URL..."
PUBLIC_URL=$(curl -s http://localhost:4040/api/tunnels | grep -o '"public_url":"[^"]*' | grep -o 'http[^"]*')

if [ ! -z "$PUBLIC_URL" ]; then
    echo "✅ SUCCESS! Your remote desktop is now accessible from anywhere!"
    echo
    echo "🌐 PUBLIC ACCESS LINK (share this with anyone):"
    echo "   $PUBLIC_URL"
    echo
    echo "📌 INSTRUCTIONS:"
    echo "   1. Open the link above in your browser"
    echo "   2. Click 'Share My Screen'"
    echo "   3. Share the Room ID with the person needing help"
    echo "   4. They can use the same public link to connect"
    echo
    echo "💡 TIPS:"
    echo "   - The free ngrok account may show a warning page first"
    echo "   - Just click 'Visit Site' to proceed"
    echo "   - For permanent solution, consider upgrading ngrok"
    echo
    echo "To stop the server, double-click STOP_INTERNET_ACCESS.sh"
else
    echo "⚠️  Could not retrieve public URL from ngrok"
    echo "   Check the logs at: $BASE_DIR/ngrok.log"
    echo
    echo "🌍 You can still access locally at: http://localhost:3000"
fi

echo
echo "Press any key to close this window..."
read -n 1 -s