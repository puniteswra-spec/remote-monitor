#!/bin/bash
# Start ALL tunnels simultaneously for redundancy
# Run this on the Mac or any server PC

BASE_DIR="/Users/upreti/Documents/Nikshay_Automation/remote-desktop-app"
echo "Starting redundant tunnels..."

# 1. Start Node.js server
node "$BASE_DIR/server/server.js" &
sleep 2
echo "✅ Server running on port 3000"

# 2. Start ngrok
nohup npx ngrok http 3000 --log=stdout > "$BASE_DIR/ngrok.log" 2>&1 &
sleep 6
NGROK_URL=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -o '"public_url":"[^"]*' | grep -o 'http[^"]*' | head -1)
echo "✅ ngrok: $NGROK_URL"

# 3. Start Cloudflare Tunnel (if installed)
if command -v cloudflared &>/dev/null; then
    nohup cloudflared tunnel --url http://localhost:3000 > "$BASE_DIR/cloudflare.log" 2>&1 &
    sleep 6
    CF_URL=$(grep -o 'https://[a-zA-Z0-9.-]*\.trycloudflare\.com' "$BASE_DIR/cloudflare.log" 2>/dev/null | head -1)
    echo "✅ Cloudflare: $CF_URL"
fi

# 4. Start serveo (SSH tunnel, no binary needed)
ssh -o StrictHostKeyChecking=no -o ServerAliveInterval=30 -R 80:localhost:3000 serveo.net > "$BASE_DIR/serveo.log" 2>&1 &
sleep 5
SV_URL=$(grep -o 'https://[a-zA-Z0-9.-]*\.serveo\.net' "$BASE_DIR/serveo.log" 2>/dev/null | head -1)
echo "✅ Serveo: $SV_URL"

# 5. Save all URLs to a file
echo "NGROK=$NGROK_URL" > "$BASE_DIR/tunnel_urls.txt"
echo "CLOUDFLARE=$CF_URL" >> "$BASE_DIR/tunnel_urls.txt"
echo "SERVEO=$SV_URL" >> "$BASE_DIR/tunnel_urls.txt"

echo ""
echo "=========================================="
echo "ALL TUNNELS ACTIVE - SYSTEM IS BULLETPROOF"
echo "=========================================="
echo ""
echo "Agent will try each URL until one connects"
echo ""
echo "To stop: killall node ngrok cloudflared ssh"
echo "=========================================="