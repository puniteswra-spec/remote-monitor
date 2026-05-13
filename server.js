const express = require('express');
const http = require('http');
const WebSocket = require('ws');
const crypto = require('crypto');

const app = express();
const server = http.createServer(app);
const wss = new WebSocket.Server({ server });

const AUTH_USER = 'puneet';
const AUTH_PASS = 'puneet12';
const AUTH_TOKEN = crypto.createHash('sha256').update(AUTH_USER + ':' + AUTH_PASS).digest('hex');

// Basic Auth middleware
function auth(req, res, next) {
  const authHeader = req.headers['authorization'];
  if (!authHeader) {
    res.setHeader('WWW-Authenticate', 'Basic realm="Remote Monitor"');
    return res.status(401).send('Unauthorized');
  }
  const base64 = authHeader.split(' ')[1];
  const creds = Buffer.from(base64, 'base64').toString().split(':');
  const user = creds[0], pass = creds[1];
  if (user !== AUTH_USER || pass !== AUTH_PASS) {
    res.setHeader('WWW-Authenticate', 'Basic realm="Remote Monitor"');
    return res.status(401).send('Unauthorized');
  }
  next();
}

function wsAuth(req) {
  // Check token in URL
  const url = new URL(req.url, 'http://localhost');
  if (url.searchParams.get('token') === AUTH_TOKEN) return true;
  
  // Check Basic Auth from upgrade request
  const authHeader = req.headers['authorization'];
  if (authHeader) {
    const base64 = authHeader.split(' ')[1];
    const creds = Buffer.from(base64, 'base64').toString().split(':');
    if (creds[0] === AUTH_USER && creds[1] === AUTH_PASS) return true;
  }
  return false;
}

app.use(auth);
app.use(express.static(__dirname + '/dashboard'));

// Store connected agents: { agentId: { ws, name, lastFrame, viewers: Set } }
const agents = new Map();
// Store connected dashboards (browsers)
const dashboards = new Set();

// API endpoint to get list of connected agents
app.get('/api/agents', (req, res) => {
  const list = [];
  for (const [id, agent] of agents) {
    list.push({
      id,
      name: agent.name,
      connected: true,
      lastSeen: agent.lastSeen,
      viewers: agent.viewers.size
    });
  }
  res.json(list);
});

// API endpoint to get latest frame of an agent
app.get('/api/frame/:agentId', (req, res) => {
  const agent = agents.get(req.params.agentId);
  if (agent && agent.lastFrame) {
    res.json({ frame: agent.lastFrame });
  } else {
    res.status(404).json({ error: 'Agent not found or no frame' });
  }
});

wss.on('connection', (ws, req) => {
  // Reject unauthenticated WebSocket connections
  if (!wsAuth(req)) {
    ws.close(4001, 'Unauthorized');
    return;
  }
  
  ws.on('message', (message) => {
    try {
      const data = JSON.parse(message);
      
      switch (data.type) {
        // Agent registration
        case 'agent-hello':
          ws.role = 'agent';
          ws.agentId = data.agentId;
          agents.set(data.agentId, {
            ws,
            name: data.name || 'Unknown',
            lastSeen: Date.now(),
            lastFrame: null,
            viewers: new Set()
          });
          console.log(`Agent connected: ${data.name} (${data.agentId})`);
          broadcastToDashboards({ type: 'agent-connected', agentId: data.agentId, name: data.name });
          break;

        // Agent sends screen frame
        case 'agent-frame':
          const agent = agents.get(data.agentId);
          if (agent) {
            agent.lastFrame = data.frame;
            agent.lastSeen = Date.now();
            // Forward frame to all viewers of this agent
            for (const viewerWs of agent.viewers) {
              if (viewerWs.readyState === WebSocket.OPEN) {
                viewerWs.send(JSON.stringify({
                  type: 'frame',
                  agentId: data.agentId,
                  frame: data.frame
                }));
              }
            }
          }
          break;

        // Agent sends log
        case 'agent-log':
          console.log(`[Agent ${data.agentId}]: ${data.message}`);
          break;

        // Browser (dashboard) registers
        case 'dashboard-hello':
          ws.role = 'dashboard';
          dashboards.add(ws);
          // Send current agent list
          const agentList = [];
          for (const [id, a] of agents) {
            agentList.push({ id, name: a.name, viewers: a.viewers.size });
          }
          ws.send(JSON.stringify({ type: 'agent-list', agents: agentList }));
          console.log('Dashboard connected');
          break;

        // Dashboard wants to view an agent
        case 'view-agent':
          if (ws.role === 'dashboard') {
            const targetAgent = agents.get(data.agentId);
            if (targetAgent) {
              targetAgent.viewers.add(ws);
              ws.viewingAgent = data.agentId;
              // Send current frame immediately
              if (targetAgent.lastFrame) {
                ws.send(JSON.stringify({
                  type: 'frame',
                  agentId: data.agentId,
                  frame: targetAgent.lastFrame
                }));
              }
              // Notify agent to increase frame rate
              targetAgent.ws.send(JSON.stringify({
                type: 'set-fps',
                fps: 10
              }));
              console.log(`Dashboard viewing: ${data.agentId}`);
            }
          }
          break;

        // Dashboard stops viewing an agent
        case 'stop-viewing':
          if (ws.role === 'dashboard' && ws.viewingAgent) {
            const prevAgent = agents.get(ws.viewingAgent);
            if (prevAgent) {
              prevAgent.viewers.delete(ws);
              if (prevAgent.viewers.size === 0) {
                prevAgent.ws.send(JSON.stringify({ type: 'set-fps', fps: 1 }));
              }
            }
            ws.viewingAgent = null;
          }
          break;

        // Control command from dashboard
        case 'control':
          if (ws.role === 'dashboard' && data.agentId) {
            const targetAgent = agents.get(data.agentId);
            if (targetAgent) {
              targetAgent.ws.send(JSON.stringify({
                type: 'control',
                command: data.command,
                params: data.params
              }));
            }
          }
          break;

        default:
          console.log('Unknown message type:', data.type);
      }
    } catch (err) {
      console.error('Error processing message:', err);
    }
  });

  ws.on('close', () => {
    if (ws.role === 'agent' && ws.agentId) {
      // Only remove if this websocket is still the registered one for this agent
      // (prevents race condition when agent reconnects quickly)
      const agent = agents.get(ws.agentId);
      if (agent && agent.ws === ws) {
        agents.delete(ws.agentId);
        broadcastToDashboards({ type: 'agent-disconnected', agentId: ws.agentId });
        console.log(`Agent disconnected: ${ws.agentId}`);
      }
    }
    if (ws.role === 'dashboard') {
      dashboards.delete(ws);
      // Clean up viewer subscriptions
      if (ws.viewingAgent) {
        const prevAgent = agents.get(ws.viewingAgent);
        if (prevAgent) {
          prevAgent.viewers.delete(ws);
          if (prevAgent.viewers.size === 0) {
            prevAgent.ws.send(JSON.stringify({ type: 'set-fps', fps: 1 }));
          }
        }
      }
    }
  });
});

function broadcastToDashboards(data) {
  for (const ws of dashboards) {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(data));
    }
  }
}

const PORT = process.env.PORT || 3000;
server.listen(PORT, '0.0.0.0', () => {
  console.log(`Server running on port ${PORT}`);
  console.log(`Dashboard: http://localhost:${PORT}`);
});