#!/bin/bash
# Remote Monitor - Server Management Script
# Usage: ./manage.sh {start|stop|restart|status|watchdog|tunnels}
#
# Commands:
#   start    - Start the Node.js server
#   stop     - Stop the server (and ngrok if running)
#   restart  - Restart the server
#   status   - Check server + tunnel status
#   watchdog - Keep server alive (auto-restart loop)
#   tunnels  - Start server + all tunnels (ngrok, cloudflare, serveo)

BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_FILE="$BASE_DIR/server.pid"
LOG_FILE="$BASE_DIR/server.log"
NGROK_PID_FILE="$BASE_DIR/ngrok.pid"

# ──────────────────────────────────────────────
# START
# ──────────────────────────────────────────────
cmd_start() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 $PID 2>/dev/null; then
            echo "✅ Server already running (PID: $PID)"
            echo "   Dashboard: http://localhost:3000"
            return 0
        else
            rm -f "$PID_FILE"
        fi
    fi

    echo "Starting Remote Monitor Server..."
    cd "$BASE_DIR"
    nohup node server.js > "$LOG_FILE" 2>&1 &
    SERVER_PID=$!
    echo $SERVER_PID > "$PID_FILE"
    sleep 3

    if kill -0 $SERVER_PID 2>/dev/null; then
        echo "✅ Server started (PID: $SERVER_PID)"
        echo "   Dashboard: http://localhost:3000"
        echo "   Login:     puneet / puneet12"
    else
        echo "❌ Failed to start server. Check $LOG_FILE"
        rm -f "$PID_FILE"
        return 1
    fi
}

# ──────────────────────────────────────────────
# STOP
# ──────────────────────────────────────────────
cmd_stop() {
    # Stop ngrok if running
    if [ -f "$NGROK_PID_FILE" ]; then
        NGROK_PID=$(cat "$NGROK_PID_FILE")
        if kill -0 $NGROK_PID 2>/dev/null; then
            echo "Stopping ngrok (PID: $NGROK_PID)..."
            kill $NGROK_PID 2>/dev/null
            sleep 1
            kill -9 $NGROK_PID 2>/dev/null
        fi
        rm -f "$NGROK_PID_FILE"
    fi
    # Also kill any other tunnel processes
    pkill -f "ngrok" 2>/dev/null
    pkill -f "cloudflared" 2>/dev/null
    pkill -f "serveo" 2>/dev/null

    # Stop server
    if [ ! -f "$PID_FILE" ]; then
        echo "Server is not running"
        return 0
    fi

    PID=$(cat "$PID_FILE")
    if kill -0 $PID 2>/dev/null; then
        echo "Stopping server (PID: $PID)..."
        kill $PID
        sleep 2
        kill -9 $PID 2>/dev/null
        echo "✅ Server stopped"
    else
        echo "Server was not running (stale PID)"
    fi
    rm -f "$PID_FILE"
}

# ──────────────────────────────────────────────
# STATUS
# ──────────────────────────────────────────────
cmd_status() {
    echo "=== Server Status ==="
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 $PID 2>/dev/null; then
            echo "✅ Server running (PID: $PID)"
            echo "   Dashboard: http://localhost:3000"
        else
            echo "❌ Server not running (stale PID)"
            rm -f "$PID_FILE"
        fi
    else
        echo "❌ Server not running"
    fi

    echo ""
    echo "=== Tunnels ==="
    NGROK_URL=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -o '"public_url":"[^"]*' | grep -o 'http[^"]*' | head -1)
    [ -n "$NGROK_URL" ] && echo "ngrok:      $NGROK_URL" || echo "ngrok:      not running"

    CF_URL=$(grep -o 'https://[a-zA-Z0-9.-]*\.trycloudflare\.com' "$BASE_DIR/cloudflare.log" 2>/dev/null | tail -1)
    [ -n "$CF_URL" ] && echo "Cloudflare: $CF_URL" || echo "Cloudflare: not running"

    SV_URL=$(grep -o 'https://[a-zA-Z0-9.-]*\.serveo\.net' "$BASE_DIR/serveo.log" 2>/dev/null | tail -1)
    [ -n "$SV_URL" ] && echo "Serveo:     $SV_URL" || echo "Serveo:     not running"

    echo ""
    echo "=== Connected Agents ==="
    curl -s -u 'puneet:puneet12' http://localhost:3000/api/agents 2>/dev/null | \
        python3 -c "
import json,sys
try:
    agents=json.load(sys.stdin)
    if agents:
        for a in agents: print(f\"  {a['name']} ({a.get('ip','?')})\")
    else: print('  (none connected)')
except: print('  (server not running)')
" 2>/dev/null
}

# ──────────────────────────────────────────────
# WATCHDOG  (blocking loop — run in background)
# ──────────────────────────────────────────────
cmd_watchdog() {
    echo "Watchdog started — checks every 30s, auto-restarts if server is down"
    echo "Press Ctrl+C to stop."
    while true; do
        if ! curl -s -o /dev/null -u 'puneet:puneet12' http://localhost:3000 2>/dev/null; then
            echo "$(date): Server down — restarting..."
            cmd_start
        fi
        sleep 30
    done
}

# ──────────────────────────────────────────────
# TUNNELS  (start server + all tunnels)
# ──────────────────────────────────────────────
cmd_tunnels() {
    cmd_start
    echo ""
    echo "Starting all tunnels..."

    # ngrok
    nohup npx ngrok http 3000 --log=stdout > "$BASE_DIR/ngrok.log" 2>&1 &
    echo $! > "$NGROK_PID_FILE"
    sleep 6
    NGROK_URL=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -o '"public_url":"[^"]*' | grep -o 'http[^"]*' | head -1)
    [ -n "$NGROK_URL" ] && echo "✅ ngrok: $NGROK_URL" || echo "⚠️  ngrok: failed (check ngrok.log)"

    # Cloudflare
    if command -v cloudflared &>/dev/null; then
        nohup cloudflared tunnel --url http://localhost:3000 > "$BASE_DIR/cloudflare.log" 2>&1 &
        sleep 6
        CF_URL=$(grep -o 'https://[a-zA-Z0-9.-]*\.trycloudflare\.com' "$BASE_DIR/cloudflare.log" 2>/dev/null | head -1)
        [ -n "$CF_URL" ] && echo "✅ Cloudflare: $CF_URL" || echo "⚠️  Cloudflare: no URL yet (check cloudflare.log)"
    else
        echo "⚠️  Cloudflare: cloudflared not installed"
    fi

    # Serveo (SSH)
    ssh -o StrictHostKeyChecking=no -o ServerAliveInterval=30 -R 80:localhost:3000 serveo.net > "$BASE_DIR/serveo.log" 2>&1 &
    sleep 5
    SV_URL=$(grep -o 'https://[a-zA-Z0-9.-]*\.serveo\.net' "$BASE_DIR/serveo.log" 2>/dev/null | head -1)
    [ -n "$SV_URL" ] && echo "✅ Serveo: $SV_URL" || echo "⚠️  Serveo: no URL yet (check serveo.log)"

    echo ""
    echo "All tunnels started. To stop everything: ./manage.sh stop"
}

# ──────────────────────────────────────────────
# DISPATCH
# ──────────────────────────────────────────────
case "$1" in
    start)    cmd_start ;;
    stop)     cmd_stop ;;
    restart)  cmd_stop; sleep 2; cmd_start ;;
    status)   cmd_status ;;
    watchdog) cmd_watchdog ;;
    tunnels)  cmd_tunnels ;;
    *)
        echo "Remote Monitor - Server Management"
        echo ""
        echo "Usage: $0 {start|stop|restart|status|watchdog|tunnels}"
        echo ""
        echo "  start    - Start the Node.js server"
        echo "  stop     - Stop server and all tunnels"
        echo "  restart  - Restart the server"
        echo "  status   - Show server + tunnel + agent status"
        echo "  watchdog - Auto-restart loop (run in background with &)"
        echo "  tunnels  - Start server + ngrok + cloudflare + serveo"
        echo ""
        cmd_status
        ;;
esac
