#!/bin/bash

BASE_DIR="/Users/upreti/Documents/Nikshay_Automation/remote-desktop-app"
PID_FILE="$BASE_DIR/server.pid"
LOCAL_IP=$(ifconfig | grep "inet " | grep -v 127.0.0.1 | awk '{print $2}')

start_server() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 $PID 2>/dev/null; then return 0; fi
        rm -f "$PID_FILE"
    fi
    cd "$BASE_DIR/backend"
    nohup node server.js > "$BASE_DIR/server.log" 2>&1 &
    echo $! > "$PID_FILE"
    sleep 2
    kill -0 $(cat "$PID_FILE") 2>/dev/null
    return $?
}

stop_all() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE"); kill $PID 2>/dev/null; sleep 1; kill -9 $PID 2>/dev/null; rm -f "$PID_FILE"
    fi
    pkill -f "ngrok" 2>/dev/null
    echo "✅ Server stopped"
}

while true; do
    clear
    echo "=========================================="
    echo "   REMOTE DESKTOP CONTROLLER"
    echo "=========================================="
    echo ""
    echo " YOUR LOCAL IP: $LOCAL_IP"
    echo ""
    echo "1) Start - Local Network (same Wi-Fi)"
    echo "2) Start - Internet (ngrok public link)"
    echo "3) Setup ngrok (free, 30 sec)"
    echo "4) Stop Server"
    echo "5) Exit"
    echo "=========================================="
    echo -n "Choose (1-5): "
    read c
    case "$c" in
        1)
            clear
            if start_server; then
                echo "=========================================="
                echo "✅ SERVER READY - LOCAL NETWORK"
                echo "=========================================="
                echo ""
                echo "🔗 SEND THIS LINK (same Wi-Fi only):"
                echo "   http://$LOCAL_IP:3000"
                echo ""
                echo "📌 HOW TO USE (YOU are the controller):"
                echo ""
                echo "1) Person needing help opens the link above"
                echo "2) They click 'Share My Screen' → grant permissions"
                echo "3) They tell you the Room ID"
                echo "4) You open http://localhost:3000"
                echo "5) Click 'View Remote Screen' → enter Room ID"
                echo "6) Wait for them to approve → click 'Toggle Remote Control'"
                echo "=========================================="
            else
                echo "❌ Failed to start"
            fi
            echo -n "Press Enter..."; read
            ;;
        2)
            clear
            # Check ngrok auth (check all possible paths)
            NGROK_CONFIG=""
            for p in "$HOME/Library/Application Support/ngrok/ngrok.yml" "$HOME/.config/ngrok/ngrok.yml" "$HOME/.ngrok2/ngrok.yml"; do
                if [ -f "$p" ]; then NGROK_CONFIG="$p"; break; fi
            done
            if [ -z "$NGROK_CONFIG" ]; then
                echo "❌ ngrok needs setup first."
                echo "Select option 3 to setup (free, 30 seconds)."
                echo ""
                echo "Meanwhile use local network option 1."
                echo -n "Press Enter..."; read
                continue
            fi
            
            if start_server; then
                echo "Starting ngrok tunnel..."
                nohup npx ngrok http 3000 --log=stdout > "$BASE_DIR/ngrok.log" 2>&1 &
                sleep 6
                
                URL=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -o '"public_url":"[^"]*' | grep -o 'http[^"]*' | head -1)
                
                if [ ! -z "$URL" ]; then
                    echo ""
                    echo "=========================================="
                    echo "✅ INTERNET ACCESS READY!"
                    echo "=========================================="
                    echo ""
                    echo "🔗 SEND THIS PUBLIC LINK (works anywhere):"
                    echo "   $URL"
                    echo ""
                    echo "📌 Same steps as local network above."
                    echo "   Person opens link → shares screen → you control."
                    echo "=========================================="
                else
                    echo "❌ Tunnel failed. Check ngrok.log"
                fi
            fi
            echo -n "Press Enter..."; read
            ;;
        3)
            clear
            echo "=========================================="
            echo "NGROK FREE SETUP (30 seconds)"
            echo "=========================================="
            echo ""
            echo "Step 1: Open https://ngrok.com/signup"
            echo "Step 2: Sign up (free, use Google/GitHub)"
            echo "Step 3: After login, copy your auth token"
            echo "Step 4: In Terminal, run:"
            echo ""
            echo '  npx ngrok config add-authtoken YOUR_TOKEN'
            echo ""
            echo "Then option 2 will work!"
            echo ""
            echo "Alternatively for same-Wi-Fi testing:"
            echo "  http://$LOCAL_IP:3000"
            echo "=========================================="
            echo -n "Press Enter..."; read
            ;;
        4)
            stop_all
            echo -n "Press Enter..."; read
            ;;
        5)
            stop_all
            echo "Goodbye!"
            exit 0
            ;;
        *)
            echo "Invalid choice"
            echo -n "Press Enter..."; read
    esac
done