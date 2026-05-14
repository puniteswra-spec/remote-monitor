# Remote Desktop Sharing Application

A WebRTC-based screen sharing and remote control application that allows users to share their screen and control remote computers through a browser.

## Features

- Screen sharing directly in the browser
- Remote control capabilities
- Secure peer-to-peer connections
- No installation required for clients
- Cross-platform compatibility

## Technologies Used

- **Frontend**: HTML, CSS, JavaScript (WebRTC)
- **Backend**: Node.js, WebSocket
- **Real-time Communication**: WebRTC for peer-to-peer screen sharing
- **Signaling**: WebSocket for connection establishment

## Quick Start (Desktop)

1. **Start the server**:
   Double-click the `RemoteDesktopLauncher` file on your desktop and type:
   ```
   ./RemoteDesktopLauncher start
   ```

2. **Access the application**:
   Open your browser and go to: `http://localhost:3000`

3. **Share your screen**:
   - Click "Share My Screen"
   - Grant screen sharing permissions when prompted
   - Share the Room ID or generated link with others

4. **Stop the server**:
   Double-click the `RemoteDesktopLauncher` file and type:
   ```
   ./RemoteDesktopLauncher stop
   ```

## Prerequisites

- Node.js (version 12 or higher)
- Modern web browser (Chrome, Firefox, Edge, Safari)

## Installation

1. Clone or download this repository
2. Navigate to the backend directory:
   ```
   cd backend
   ```

3. Install dependencies:
   ```
   npm install
   ```

## Usage

1. Start the signaling server:
   ```
   npm start
   ```
   The server will start on `http://localhost:3000`

2. Open your browser and go to `http://localhost:3000`

3. Choose your role:
   - **Share My Screen**: Start sharing your screen
   - **View Remote Screen**: Connect to view someone else's screen

4. As a host:
   - Click "Share My Screen"
   - Grant permission to share your screen when prompted
   - Share the generated room link with viewers

5. As a viewer:
   - Click "View Remote Screen"
   - Enter the room ID or use the shared link
   - Wait for host approval
   - View and optionally control the remote screen

## How It Works

1. The signaling server coordinates connection establishment between peers
2. Peers exchange connection information through WebSocket
3. Once connected, screen sharing happens directly between browsers via WebRTC
4. Control commands are sent through WebRTC DataChannels

## Security

- All WebRTC connections are encrypted
- Host must approve each viewer connection
- No media is stored on the server
- Communication happens directly between peers after initial handshake

## Limitations

- Both parties must be able to establish a WebRTC connection
- Network firewalls may interfere with peer-to-peer connections
- Remote control functionality requires additional setup for executing commands on the host machine