# SystemHelper — Remote Monitor & Control v6.0.0

Windows agent for remote desktop viewing and control. Agents connect to a central server (Node.js on Mac/cloud), send screen captures, and receive mouse/keyboard commands.

## Architecture

- **Agent** (`agent/main.go`) — Go binary running on each Windows PC. Captures screen, connects via WebSocket, receives control commands.
- **Server** (`server/server.js`) — Node.js relay. Forwards frames from agents to dashboard viewers, forwards control commands from dashboard to agents.
- **Dashboard** (embedded in both agent and server) — Web UI showing connected agents, live screen view, remote control.

## Features

- Multi-display support — shows all monitors as clickable thumbnails
- Remote control (mouse move, click, keyboard)
- View-only mode for external access (no control)
- File transfer to any agent (saves to `C:\ProgramData\SystemHelper\received\`)
- Remote update — push new .exe from dashboard
- Tunnel (Expose) — make any agent accessible remotely via localhost.run or bore.pub
- Server mode — any agent can become a server when cloud is unavailable
- Activity logging (tracks idle time, uptime)
- Auto-start via Windows Registry watchdog
- Fallback servers — Render.com, ngrok, Cloudflare, direct IP

## Build

```bash
cd agent
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o SystemHelper.exe .
```

## Deploy

Copy `SystemHelper.exe` to each Windows PC and run. First run creates config at `%APPDATA%\SystemHelper\`.

## Server (Mac/cloud)

```bash
cd server
npm install
node server.js
```

Dashboard: `http://localhost:3000` (auth: puneet / puneet12)

## Commands

| Flag | Description |
|------|-------------|
| `--server` | Run as server only (no cloud connection) |
| `--org <name>` | Set organization name |
| `--internal` | Internal mode (no cloud, LAN only) |
| `--use <name>` | Use only specific server (render, ngrok, etc.) |

## Dashboard Messages

| Type | Direction | Purpose |
|------|-----------|---------|
| `become-server` | Dashboard → Agent | Start tunnel + save server preference |
| `push-update` | Dashboard → Agent | Replace .exe remotely |
| `file-transfer` | Dashboard → Agent | Send file to agent |
| `start-tunnel` | Dashboard → Agent | Start tunnel for direct access |
| `control` | Dashboard → Agent | Mouse/keyboard command |
| `set-server-preference` | Dashboard → Agent | Flag PC as server on next restart |
