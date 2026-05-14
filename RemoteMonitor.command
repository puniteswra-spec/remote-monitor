#!/bin/bash

BASE_DIR="/Users/upreti/Documents/Nikshay_Automation/remote-desktop-app"
PID_FILE="$BASE_DIR/server.pid"

ensure_ngrok() {
    URL=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -o '"public_url":"[^"]*' | grep -o 'http[^"]*' | head -1)
    if [ -z "$URL" ]; then
        nohup npx ngrok http 3000 --log=stdout > "$BASE_DIR/ngrok.log" 2>&1 &
        sleep 8
        URL=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -o '"public_url":"[^"]*' | grep -o 'http[^"]*' | head -1)
    fi
    echo "$URL"
}

while true; do
    clear
echo "=========================================="
echo "   REMOTE MONITOR - v6.0.0"
echo "=========================================="
    echo ""
    echo "1) Check Status"
    echo "2) Restart ngrok (get new URL)"
    echo "3) Open Dashboard"
    echo "4) View-Access link (share read-only)"
    echo "5) Choose Server (which server agents use)"
    echo "6) Set Windows PC as Server (pick one)"
    echo "7) Choose Tunnel (localhost.run / bore / auto)"
    echo "8) Switch to Internal Mode (no cloud)"
    echo "9) Make THIS Mac the Server"
    echo "10) Find Server IP (scan for active servers)"
    echo "11) Install server auto-start (Mac boot)"
    echo "12) Deploy to VPS (no laptop needed)"
    echo "13) Exit"
    echo "=========================================="
    echo -n "Choose (1-13): "
    read c
    case "$c" in
        1)
            clear
            echo "=== STATUS ==="
            echo ""
            echo "--- Servers ---"
            curl -s -o /dev/null -w "Local Mac:   HTTP %{http_code}\n" http://localhost:3000/ 2>/dev/null || echo "Local Mac:   ❌ Down"
            curl -s -o /dev/null -w "Render.com:  HTTP %{http_code}\n" --max-time 10 https://remote-monitor-1l0s.onrender.com 2>/dev/null || echo "Render.com:  ❌ Down"
            
            NGROK_URL=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -o '"public_url":"[^"]*' | grep -o 'http[^"]*' | head -1)
            if [ ! -z "$NGROK_URL" ]; then
                echo "ngrok (tunnel active): $NGROK_URL"
            else
                echo "ngrok: ❌ Not running"
            fi
            
            CF_URL=$(grep -o 'https://[a-zA-Z0-9.-]*\.trycloudflare\.com' /Users/upreti/Documents/Nikshay_Automation/remote-desktop-app/cloudflare.log 2>/dev/null | head -1)
            if [ ! -z "$CF_URL" ]; then
                curl -s -o /dev/null -w "Cloudflare: HTTP %{http_code} ($CF_URL)\n" --max-time 10 "$CF_URL" 2>/dev/null
            else
                echo "Cloudflare: ❌ Not running"
            fi
            
            echo ""
            echo "--- Active in urls.ini (what agents use) ---"
            cat /Users/upreti/Documents/Nikshay_Automation/remote-desktop-app/agent/urls.ini 2>/dev/null || echo "(no custom config - using defaults)"
            
            echo ""
            echo "--- Connected Agents ---"
            curl -s -u 'puneet:puneet12' http://localhost:3000/api/agents 2>/dev/null | python3 -c "
import json,sys
try:
    agents=json.load(sys.stdin)
    if agents:
        for a in agents:
            print(f\"  {a['name']} ({a.get('ip','?')})\")
    else:
        print('  (no agents connected)')
except: print('  (server not running)')
" 2>/dev/null
            echo -n "Press Enter..."; read
            ;;
        2)
            clear
            pkill -f ngrok 2>/dev/null
            URL=$(ensure_ngrok)
            echo "New ngrok URL: $URL"
            echo "Share with others (view-only): $URL"
            echo -n "Press Enter..."; read
            ;;
        3)
            open http://localhost:3000
            ;;
        4)
            URL=$(ensure_ngrok)
            echo ""
            echo "=========================================="
            echo "VIEW-ONLY ACCESS LINK"
            echo "=========================================="
            echo ""
            echo "Share this link with your company owner:"
            echo "  $URL"
            echo ""
            echo "They can view all agents but CANNOT control."
            echo "Only you (localhost) can control."
            echo "=========================================="
            echo -n "Press Enter..."; read
            ;;
        5)
            clear
            echo "=========================================="
            echo "   CHOOSE SERVER FOR AGENTS"
            echo "=========================================="
            echo ""
            echo "1) Render.com (default, 24/7)"
            echo "2) ngrok tunnel"
            echo "3) Cloudflare tunnel"
            echo "4) Port forwarding (direct)"
            echo "5) Auto (try all)"
            echo "=========================================="
            echo -n "Choose (1-5): "
            read sc
            case "$sc" in
                1) URL="wss://remote-monitor-1l0s.onrender.com" ;;
                2) URL="wss://deviation-tweak-charter.ngrok-free.dev" ;;
                3) URL="wss://warming-theater-photo-pentium.trycloudflare.com" ;;
                4) URL="ws://43.247.40.101:3000" ;;
                5) URL="auto" ;;
                *) echo "Invalid"; echo -n "Press Enter..."; read; continue ;;
            esac

            if [ "$URL" = "auto" ]; then
                echo "$URL" > "$BASE_DIR/agent/urls.ini"
                echo "✅ Set to AUTO mode (try all servers)"
            else
                echo "$URL" > "$BASE_DIR/agent/urls.ini"
                echo "✅ Agents will now use: $URL"
                echo ""
                echo "To switch running agents NOW:"
                echo "  curl -u 'puneet:puneet12' -X POST -H 'x-server-url: $URL' http://localhost:3000/api/switch-server"
            fi
            echo -n "Press Enter..."; read
            ;;
        6)
            clear
            echo "=========================================="
            echo "   SET WINDOWS PC AS SERVER"
            echo "=========================================="
            echo ""
            echo "Fetching connected agents..."
            echo ""
            curl -s -u 'puneet:puneet12' http://localhost:3000/api/agents 2>/dev/null | python3 -c "
import json,sys
try:
    agents=json.load(sys.stdin)
    if not agents:
        print('No agents connected')
    else:
        for i,a in enumerate(agents):
            print(f'{i+1}) {a[\"name\"]} ({a.get(\"ip\",\"?\")})')
except: print('Server not running or no auth')
" 2>/dev/null
            echo ""
            echo -n "Enter number to make server (or 0 to cancel): "
            read sn
            if [ "$sn" = "0" ] || [ -z "$sn" ]; then
                echo "Cancelled"
            else
                AGENT_ID=$(curl -s -u 'puneet:puneet12' http://localhost:3000/api/agents 2>/dev/null | python3 -c "
import json,sys
agents=json.load(sys.stdin)
try:
    i=int('$sn')-1
    if i>=0 and i<len(agents):
        print(agents[i]['id'])
except: pass
" 2>/dev/null)
                if [ ! -z "$AGENT_ID" ]; then
                    curl -s -X POST -u 'puneet:puneet12' "http://localhost:3000/api/make-server/$AGENT_ID" 2>/dev/null
                    echo "✅ Server preference sent to agent"
                else
                    echo "❌ Invalid selection"
                fi
            fi
            echo -n "Press Enter..."; read
            ;;
        7)
            clear
            echo "=========================================="
            echo "   CHOOSE TUNNEL MODE"
            echo "=========================================="
            echo ""
            echo "Select which tunnel to use for direct connections:"
            echo ""
            echo "1) Auto (try localhost.run → bore)"
            echo "2) localhost.run (SSH, no install needed)"
            echo "3) bore.pub (faster, needs one-time download)"
            echo "4) None (Render only)"
            echo ""
            echo -n "Choose (1-4): "
            read tc
            case "$tc" in
                1) echo "auto" > "$BASE_DIR/agent/tunnel.ini"; echo "Set to Auto" ;;
                2) echo "localhost.run" > "$BASE_DIR/agent/tunnel.ini"; echo "Set to localhost.run" ;;
                3) echo "bore" > "$BASE_DIR/agent/tunnel.ini"; echo "Set to bore.pub" ;;
                4) echo "none" > "$BASE_DIR/agent/tunnel.ini"; echo "Set to None (Render only)" ;;
                *) echo "Invalid" ;;
            esac
            echo "Updated tunnel.ini. Copy to all Windows PCs."
            echo -n "Press Enter..."; read
            ;;
        8)
            clear
            echo "=========================================="
            echo "   SWITCH TO INTERNAL MODE"
            echo "=========================================="
            echo ""
            echo "This disables all cloud connections."
            echo "Agents will ONLY work on the local network."
            echo ""
            echo "1) Set this Mac to Internal mode"
            echo "2) Create config for Windows PCs (internal)"
            echo ""
            echo -n "Choose (1-2): "
            read ic
            if [ "$ic" = "1" ]; then
                echo "auto-local" > "$BASE_DIR/agent/urls.ini"
                echo "✅ Mac set to Internal mode"
            elif [ "$ic" = "2" ]; then
                echo "auto-local" > "$BASE_DIR/agent/urls.ini"
                cp "$BASE_DIR/agent/urls.ini" ~/Desktop/urls.ini
                echo "✅ urls.ini created on Desktop. Copy next to .exe on Windows PCs."
            fi
            echo ""
            echo "To switch back to cloud mode, edit urls.ini or use Option 5."
            echo -n "Press Enter..."; read
            ;;
        9)
            clear
            echo "=========================================="
            echo "   MAKE THIS MAC THE SERVER"
            echo "=========================================="
            echo ""
            
            # Get local IP
            LOCAL_IP=$(ifconfig | grep "inet " | grep -v 127.0.0.1 | awk '{print $2}')
            
            # Ensure server is running
            lsof -ti :3000 2>/dev/null >/dev/null
            if [ $? -ne 0 ]; then
                cd /Users/upreti/Documents/Nikshay_Automation/remote-desktop-app/server
                nohup node server.js > /Users/upreti/Documents/Nikshay_Automation/remote-desktop-app/server.log 2>&1 &
                sleep 3
            fi
            
            # Update urls.ini to use this Mac as primary
            echo "ws://$LOCAL_IP:3000" > /Users/upreti/Documents/Nikshay_Automation/remote-desktop-app/agent/urls.ini
            echo "wss://remote-monitor-1l0s.onrender.com" >> /Users/upreti/Documents/Nikshay_Automation/remote-desktop-app/agent/urls.ini
            
            echo "✅ This Mac is now the PRIMARY SERVER"
            echo "   Dashboard: http://localhost:3000"
            echo "   Local IP:  http://$LOCAL_IP:3000"
            echo ""
            echo "📌 Agents on same network will connect directly to this Mac"
            echo "📌 Remote agents fall back to Render.com"
            echo ""
            echo "To switch back to cloud-only: Option 5 → choose 'auto'"
            echo -n "Press Enter..."; read
            ;;
        10)
            clear
            echo "=========================================="
            echo "   FIND SERVER IP ON NETWORK"
            echo "=========================================="
            echo ""
            echo "Scanning local network for active servers..."
            echo ""
            
            # Get subnet
            LOCAL_IP=$(ifconfig | grep "inet " | grep -v 127.0.0.1 | awk '{print $2}')
            SUBNET=$(echo $LOCAL_IP | cut -d. -f1-3)
            
            FOUND=0
            for i in $(seq 1 254); do
                IP="$SUBNET.$i"
                result=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 0.5 "http://$IP:3000/" 2>/dev/null)
                if [ ! -z "$result" ] && [ "$result" != "000" ]; then
                    echo "  ✅ $IP:3000 — Server found"
                    FOUND=$((FOUND+1))
                fi
            done
            
            echo ""
            if [ "$FOUND" -eq 0 ]; then
                echo "  No server found on local network"
                echo "  Try: http://localhost:3000 (your Mac)"
                echo "  Or:  https://remote-monitor-1l0s.onrender.com (cloud)"
            fi
            
            echo ""
            echo "Connected agents from API:"
            curl -s -u 'puneet:puneet12' http://localhost:3000/api/agents 2>/dev/null | python3 -c "
import json,sys
try:
    agents=json.load(sys.stdin)
    for a in agents:
        print(f\"  {a['name']} ({a.get('ip','unknown')})\")
except: print('  (no agents or server not running)')
" 2>/dev/null
            
            echo -n "Press Enter..."; read
            ;;
        11)
            clear
            echo "Installing server auto-start..."
            launchctl load ~/Library/LaunchAgents/com.remotemonitor.server.plist 2>/dev/null
            echo "✅ Server will auto-start on Mac boot."
            echo ""
            echo "NOTE: ngrok still needs to be started manually"
            echo "or you can use option 2 to get a new URL."
            echo -n "Press Enter..."; read
            ;;
        12)
            clear
            echo "=========================================="
            echo "   DEPLOY SERVER ON VPS"
            echo "=========================================="
            echo ""
            echo "Run server on a $5/month VPS instead of your laptop."
            echo ""
            echo "Steps:"
            echo "1. Get a VPS (DigitalOcean, Linode, Hetzner)"
            echo "2. SSH into it"
            echo "3. Install Node.js:"
            echo "   curl -fsSL https://deb.nodesource.com/setup_20.x | bash -"
            echo "   apt install -y nodejs"
            echo ""
            echo "4. Copy server folder:"
            echo "   scp -r server/ root@YOUR_VPS_IP:/opt/remote-monitor/"
            echo ""
            echo "5. Start server:"
            echo "   cd /opt/remote-monitor && npm install && node server.js &"
            echo ""
            echo "6. Update agent config to use VPS IP:"
            echo "   Edit main.go → serverUrl = 'ws://YOUR_VPS_IP:3000'"
            echo "   Rebuild .exe with ./build.sh"
            echo ""
            echo "✅ No ngrok! No laptop needed! Always on!"
            echo -n "Press Enter..."; read
            ;;
        13) exit 0
            ;;
        *) echo "Invalid"; echo -n "Press Enter..."; read
    esac
done