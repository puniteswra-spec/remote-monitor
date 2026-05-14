REMOTE MONITOR & CONTROL SYSTEM
================================
Version: 5.5.1
Author: Puneet
Created: May 2026

====================
WHAT THIS SYSTEM DOES
====================

Monitor and control remote Windows computers from anywhere.
Install one .exe file on any Windows PC → see its screen on your dashboard → control mouse/keyboard remotely.

No installation needed for the person being helped (just open a browser link).

====================
ARCHITECTURE OVERVIEW
====================

[Your Mac/Laptop]                    [Windows PCs]
      │                                    │
      ├── RemoteMonitor.command            ├── SystemHelper_v5.5.1.exe
      │   (Launcher menu)                  │   (Agent + Local Server)
      │                                    │
      ├── Node.js Server (port 3000)       ├── Connects to Render.com
      │   - Serves dashboard               ├── Also runs local server
      │   - WebSocket relay                ├── Captures screen
      │   - Agent management               ├── Executes remote control
      │   - File sharing                   │
      │                                    │
      ├── render-deploy/                   │
      │   (GitHub → Render.com cloud)      │
      │   - 24/7 always on                 │
      │   - Same as Node.js server         │
      │                                    │
      └── [Browser] Dashboard              │
          http://localhost:3000            │
          https://render.com URL           │

[Cloud Servers - always running]
      ├── Render.com (primary - 24/7 free)
      ├── ngrok (backup tunnel)
      ├── Cloudflare Tunnel (backup)
      └── Port forwarding (direct)

====================
FILES AND THEIR PURPOSE
====================

1. agent/main.go
   - The Windows agent executable source
   - Written in Go (Golang)
   - Cross-compiled for Windows from Mac
   - Single binary, no dependencies on Windows
   
   Features:
   - Screen capture (JPEG, 1-10 fps)
   - Remote control (mouse move, click, keyboard)
   - WebSocket connection to server
   - Auto-discovery of other agents on network
   - Local server mode (serves dashboard to LAN)
   - Activity logging (idle/active tracking)
   - Temp file cleaner on startup
   - Watchdog auto-restart (survives Task Manager kill)
   - Remote update (push new .exe from dashboard)
   - File transfer (send files during remote control)
   - Auto-start with Windows
   - All config/logs in %APPDATA%\SystemHelper\ (hidden)
   - Auth token for WebSocket connection

2. server/server.js
   - Node.js server (runs on Mac OR Render.com)
   - Express + WebSocket (ws library)
   
   Features:
   - Serves dashboard HTML
   - Accepts agent WebSocket connections
   - Accepts dashboard WebSocket connections
   - Forwards screen frames from agents to dashboard
   - Forwards control commands from dashboard to agents
   - File upload endpoint (send files to agents)
   - Remote update endpoint (push new .exe)
   - Server switch endpoint (change agent server URL)
   - Agent activity logging (connect/disconnect tracking)
   - Report API (JSON + CSV export)
   - Remote session page (browser-based screen sharing)
   - Basic Auth password protection (puneet/puneet12)

3. server/dashboard/index.html
   - Web-based dashboard interface
   - Single HTML file (no build tools needed)
   
   Features:
   - Agent list with IP + hostname
   - Live screen viewer
   - Remote control (mouse/keyboard)
   - Server badge (shows which server you're on)
   - Hidden auth (🔒 → enter password → control)
   - View-only mode (for shared access)
   - Hide agents from view
   - File sharing button
   - Report link (📊) + CSV download
   - Remote session link (🔗)
   - Auto-failover to backup servers
   - TOKEN_PLACEHOLDER for auth token

4. render-deploy/
   - Same as server/ but with paths adjusted for Render.com
   - server.js uses root paths (no /dashboard folder)
   - index.html at root level
   - package.json with dependencies

5. agent/RemoteMonitor.bat
   - Windows batch file launcher
   - Same menu as RemoteMonitor.command
   - Check status, choose server, scan network, etc.

6. RemoteMonitor.command
   - Mac shell script launcher
   - Menu-driven interface
   - Start/stop server, check status
   - Choose server, scan for agents
   - Deploy to VPS info

7. agent/build.sh
   - Build script for the Go agent
   - Usage: ./build.sh 1.0.0
   - Creates SystemHelper_v1.0.0.exe

====================
INSTALLATION
====================

ON YOUR MAC (SERVER):
1. Double-click RemoteMonitor.command
2. Option 1: Start server + ngrok
3. Open http://localhost:3000

ON RENDER.COM (CLOUD - OPTIONAL):
1. Upload render-deploy/ files to GitHub
2. Connect GitHub repo to Render.com
3. Get URL: https://remote-monitor-1l0s.onrender.com

ON WINDOWS PCS:
1. Copy SystemHelper_v5.5.1.exe to USB
2. On Windows PC: double-click the .exe
3. Nothing visible (runs hidden)
4. Check %APPDATA%\SystemHelper\agent.log
5. Open https://remote-monitor-1l0s.onrender.com on Mac
6. PC appears in agent list

REMOTE SESSION (NO INSTALL):
1. Click 🔗 Session in dashboard
2. Send the URL to anyone
3. They open it → Share MY Screen
4. You see their screen, can control

====================
URLS
====================

Dashboard (you):  http://localhost:3000
Cloud dashboard:  https://remote-monitor-1l0s.onrender.com
Agent connects:   wss://remote-monitor-1l0s.onrender.com
Login:            puneet / puneet12

====================
UPDATING THE AGENT
====================

1. Edit agent/main.go
2. Run: cd agent && ./build.sh 5.6.0
3. Deploy SystemHelper_v5.6.0.exe to Windows PCs
4. Or push update from dashboard (🔓 → Upload new .exe)

====================
SUPPORTED FEATURES
====================

✅ Screen sharing (view any PC from dashboard)
✅ Remote control (mouse + keyboard)
✅ File transfer (send files during control)
✅ Remote update (push new .exe without touching PC)
✅ Activity logging (idle/active time tracking)
✅ Auto-restart (watchdog survives Task Manager kill)
✅ Temp file cleaning (on startup)
✅ Multi-server failover (Render → ngrok → local)
✅ View-only mode (share with owner, no control)
✅ Hidden auth (🔒 → enter password → enable control)
✅ Hide agents (remove from view-only mode)
✅ Browser session (no install for one-time help)
✅ Reports (JSON + CSV export)
✅ Auto-discovery (first PC = server)
✅ Secret server preference (set from dashboard)
✅ Persistent agent ID (survives updates)
✅ All configs in %APPDATA% (hidden from user)

v5.8.0 - Multi-Server Support
=============================
- Agent connects to CLOUD + LOCAL simultaneously
- Visible on BOTH dashboards at the same time
- Zero latency when using local server
- Grid view: see ALL agents at once
- Smart focus: click agent → full control, rest stay visible

DASHBOARD DESIGN
=================
- Dark purple/blue gradient theme
- Created by Puneet Upreti
- Agent cards with glow effect on selection
