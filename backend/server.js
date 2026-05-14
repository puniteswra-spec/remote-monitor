const express = require('express');
const http = require('http');
const WebSocket = require('ws');
const { v4: uuidv4 } = require('uuid');

const app = express();
const server = http.createServer(app);

// Store active connections
const connections = new Map();

// CORS middleware
app.use((req, res, next) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
  next();
});

// Serve frontend files
app.use(express.static(__dirname + '/../frontend'));

// Create WebSocket server
const wss = new WebSocket.Server({ server });

wss.on('connection', (ws) => {
  console.log('New client connected');
  
  // Generate unique ID for this connection
  const clientId = uuidv4();
  connections.set(clientId, ws);
  
  // Send client their ID
  ws.send(JSON.stringify({ type: 'assign-id', id: clientId }));
  
  ws.on('message', (message) => {
    try {
      const data = JSON.parse(message);
      
      switch (data.type) {
        case 'create-room':
          // Host creates a room
          const roomId = data.roomId || uuidv4();
          ws.roomId = roomId;
          ws.isHost = true;
          ws.send(JSON.stringify({ type: 'room-created', roomId }));
          console.log(`Room ${roomId} created`);
          break;
          
        case 'join-room':
          // Client wants to join a room
          const targetRoomId = data.roomId;
          let host = null;
          
          // Find the host for this room
          for (const [id, conn] of connections) {
            if (conn.roomId === targetRoomId && conn.isHost) {
              host = conn;
              break;
            }
          }
          
          if (host) {
            ws.roomId = targetRoomId;
            ws.isHost = false;
            
            // Notify host about join request
            host.send(JSON.stringify({
              type: 'join-request',
              clientId: clientId,
              clientName: data.clientName || 'Anonymous'
            }));
            
            ws.send(JSON.stringify({ 
              type: 'join-status', 
              status: 'waiting-for-approval' 
            }));
          } else {
            ws.send(JSON.stringify({ 
              type: 'error', 
              message: 'Room not found' 
            }));
          }
          break;
          
        case 'approve-client':
          // Host approves client connection
          const clientConn = connections.get(data.clientId);
          if (clientConn) {
            clientConn.send(JSON.stringify({
              type: 'approved',
              approved: true
            }));
            
            // Store reference to each other
            ws.peer = clientConn;
            clientConn.peer = ws;
          }
          break;
          
        case 'webrtc-offer':
        case 'webrtc-answer':
        case 'webrtc-ice-candidate':
          // Forward WebRTC signaling to peer
          if (ws.peer && ws.peer.readyState === WebSocket.OPEN) {
            ws.peer.send(message);
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
    console.log('Client disconnected');
    connections.delete(clientId);
    
    // Notify peer about disconnection
    if (ws.peer && ws.peer.readyState === WebSocket.OPEN) {
      ws.peer.send(JSON.stringify({ type: 'peer-disconnected' }));
    }
  });
});

const PORT = process.env.PORT || 3000;
server.listen(PORT, () => {
  console.log(`Server running on port ${PORT}`);
});