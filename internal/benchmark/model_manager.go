package benchmark

import (
	"clai/internal/gpu"
	"clai/internal/logger"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ModelServer represents a running or available model server
type ModelServer struct {
	ModelPath       string `json:"model_path"`
	ModelName       string `json:"model_name"`
	Port            int    `json:"port"`
	PID             int    `json:"pid"`
	Status          string `json:"status"`        // "running", "stopped", "starting", "error"
	ErrorMessage    string `json:"error_message"` // Error details when status is "error"
	URL             string `json:"url"`
	APIType         string `json:"api_type"`
	LastChecked     int64  `json:"last_checked"`
	VRAMUsageBytes  int64  `json:"vram_usage_bytes"` // VRAM used by this server in bytes
	ContextSize     int    `json:"context_size"`     // Active context window size (n_ctx)
	ContextTrain    int    `json:"context_train"`    // Training context size (n_ctx_train)
	ParametersCount int64  `json:"parameters_count"` // Total parameters (n_params)
	ModelSizeBytes  int64  `json:"model_size_bytes"` // Model file size in bytes
	VocabSize       int    `json:"vocab_size"`       // Vocabulary size (n_vocab)
	EmbeddingDim    int    `json:"embedding_dim"`    // Embedding dimensions (n_embd)
}

// ModelManager handles starting/stopping model servers
type ModelManager struct {
	mu              sync.RWMutex
	servers         map[string]*ModelServer // key: model_path
	modelsDir       string
	llamaServerBin  string
	downloadManager *DownloadManager
	stopRefresh     chan struct{} // Signal to stop background refresh
	lastStateHash   string        // Hash of server states for change detection
	onStateChange   func()        // Callback when state changes
}

// NewModelManager creates a new model manager
func NewModelManager() *ModelManager {
	modelsDir := "/home/josh/models"
	mm := &ModelManager{
		servers:         make(map[string]*ModelServer),
		modelsDir:       modelsDir,
		llamaServerBin:  "/home/josh/llama.cpp-rocm-wmma/build/bin/llama-server",
		downloadManager: NewDownloadManager(modelsDir),
		stopRefresh:     make(chan struct{}),
	}

	// Start background refresh goroutine
	go mm.backgroundRefresh()

	return mm
}

// backgroundRefresh periodically refreshes server status in the background
func (mm *ModelManager) backgroundRefresh() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Do an initial refresh immediately
	mm.RefreshServerStatus()
	mm.UpdateVRAMUsage()
	mm.notifyStateChange()

	for {
		select {
		case <-ticker.C:
			mm.RefreshServerStatus()
			mm.UpdateVRAMUsage()
			mm.notifyStateChange()
		case <-mm.stopRefresh:
			return
		}
	}
}

// Stop gracefully stops the background refresh goroutine
func (mm *ModelManager) Stop() {
	close(mm.stopRefresh)
}

// SetStateChangeCallback sets a callback to be called when server state changes
func (mm *ModelManager) SetStateChangeCallback(callback func()) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	mm.onStateChange = callback
}

// computeStateHash computes a hash of current server states for change detection
func (mm *ModelManager) computeStateHash() string {
	// Must be called with lock held
	var builder strings.Builder

	// Sort servers by model path for consistent ordering
	paths := make([]string, 0, len(mm.servers))
	for path := range mm.servers {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		server := mm.servers[path]
		// Include key fields that represent visible state
		fmt.Fprintf(&builder, "%s:%s:%d:%d:%d|",
			server.ModelPath,
			server.Status,
			server.Port,
			server.PID,
			server.VRAMUsageBytes)
	}

	return builder.String()
}

// notifyStateChange checks if state changed and calls callback if set
func (mm *ModelManager) notifyStateChange() {
	mm.mu.Lock()
	currentHash := mm.computeStateHash()
	changed := currentHash != mm.lastStateHash
	mm.lastStateHash = currentHash
	callback := mm.onStateChange
	mm.mu.Unlock()

	if changed && callback != nil {
		logger.Info("Server state changed, triggering SSE broadcast")
		callback()
	}
}

// ScanAvailableModels scans the models directory for .gguf files
func (mm *ModelManager) ScanAvailableModels() ([]*ModelServer, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	files, err := ioutil.ReadDir(mm.modelsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read models directory: %w", err)
	}

	var models []*ModelServer
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".gguf") {
			continue
		}

		modelPath := filepath.Join(mm.modelsDir, file.Name())

		// Check if we already have this model tracked
		if server, exists := mm.servers[modelPath]; exists {
			models = append(models, server)
			continue
		}

		// New model - add to tracking
		server := &ModelServer{
			ModelPath: modelPath,
			ModelName: file.Name(),
			Status:    "stopped",
			APIType:   "llamacpp",
		}
		mm.servers[modelPath] = server
		models = append(models, server)
	}

	return models, nil
}

// RefreshServerStatus checks all running servers and updates their status
// This function does NOT hold locks during I/O operations to avoid blocking other operations
func (mm *ModelManager) RefreshServerStatus() error {
	// Step 1: Scan all ports WITHOUT holding any locks (this is slow I/O)
	type portInfo struct {
		port        int
		modelName   string
		pid         int
		contextSize int
		metadata    map[string]interface{}
	}

	var activePortsInfo []portInfo

	// Step 1a: First, check ports of servers we know about
	mm.mu.RLock()
	knownPorts := make([]int, 0, len(mm.servers))
	for _, server := range mm.servers {
		if server.Port > 0 {
			knownPorts = append(knownPorts, server.Port)
		}
	}
	mm.mu.RUnlock()

	// Check known ports
	for _, port := range knownPorts {
		url := fmt.Sprintf("http://localhost:%d/v1/models", port)
		if !mm.checkServerHealth(url) {
			continue
		}

		// Port is active - gather all info without lock
		info := portInfo{
			port:        port,
			modelName:   mm.getModelNameFromPort(port),
			pid:         mm.findPIDForPort(port),
			contextSize: mm.getContextSizeFromPort(port),
		}

		if info.modelName != "" {
			activePortsInfo = append(activePortsInfo, info)
		}
	}

	// Step 1b: Scan for any servers we don't know about (8081-8180 range)
	// This handles externally started servers or missed registrations
	for port := 8081; port <= 8180; port++ {
		// Skip if we already checked this port
		alreadyChecked := false
		for _, knownPort := range knownPorts {
			if port == knownPort {
				alreadyChecked = true
				break
			}
		}
		if alreadyChecked {
			continue
		}

		url := fmt.Sprintf("http://localhost:%d/v1/models", port)
		if !mm.checkServerHealth(url) {
			continue
		}

		// Port is active - gather all info without lock
		info := portInfo{
			port:        port,
			modelName:   mm.getModelNameFromPort(port),
			pid:         mm.findPIDForPort(port),
			contextSize: mm.getContextSizeFromPort(port),
		}

		if info.modelName != "" {
			activePortsInfo = append(activePortsInfo, info)
		}
	}

	// Step 2: Now acquire lock and update server status quickly
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Build active ports map for quick lookup
	activePorts := make(map[int]bool)
	for _, info := range activePortsInfo {
		activePorts[info.port] = true
	}

	// Update servers based on gathered port info
	for _, info := range activePortsInfo {
		// Find the server with this model
		for _, server := range mm.servers {
			if strings.Contains(info.modelName, filepath.Base(server.ModelName)) ||
				strings.Contains(filepath.Base(server.ModelName), filepath.Base(info.modelName)) {
				server.Status = "running"
				server.Port = info.port
				server.URL = fmt.Sprintf("http://localhost:%d", info.port)
				server.APIType = "llamacpp"
				server.LastChecked = time.Now().Unix()
				server.PID = info.pid
				server.ContextSize = info.contextSize
				break
			}
		}
	}

	// Mark any server that's not on an active port as stopped
	for _, server := range mm.servers {
		if server.Port > 0 && !activePorts[server.Port] {
			server.Status = "stopped"
			server.Port = 0
			server.PID = 0
			server.URL = ""
		}
	}

	return nil
}

// getModelNameFromPort fetches the model name from a running server
func (mm *ModelManager) getModelNameFromPort(port int) string {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	url := fmt.Sprintf("http://localhost:%d/v1/models", port)
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	bodyStr := string(body)

	// Extract model path from response
	if strings.Contains(bodyStr, "/home/josh/models/") {
		start := strings.Index(bodyStr, "/home/josh/models/")
		if start >= 0 {
			end := strings.Index(bodyStr[start:], "\"")
			if end >= 0 {
				return bodyStr[start : start+end]
			}
		}
	}

	return ""
}

// findPIDForPort attempts to find the PID of the process listening on a port
func (mm *ModelManager) findPIDForPort(port int) int {
	// This is a simple approach - in production you'd use lsof or netstat parsing
	cmd := exec.Command("lsof", "-t", "-i", fmt.Sprintf(":%d", port))
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	var pid int
	fmt.Sscanf(string(output), "%d", &pid)
	return pid
}

// getContextSizeFromPort fetches the context size (n_ctx) from a running llama.cpp server
func (mm *ModelManager) getContextSizeFromPort(port int) int {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	url := fmt.Sprintf("http://localhost:%d/slots", port)
	resp, err := client.Get(url)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var slots []struct {
		ID           int  `json:"id"`
		NCtx         int  `json:"n_ctx"`
		Speculative  bool `json:"speculative"`
		IsProcessing bool `json:"is_processing"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&slots); err != nil {
		return 0
	}

	// Return the n_ctx from the first slot
	if len(slots) > 0 {
		return slots[0].NCtx
	}

	return 0
}

// checkServerHealth checks if a server is responding at the given URL
func (mm *ModelManager) checkServerHealth(url string) bool {
	client := &http.Client{Timeout: 200 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// findAvailablePortForModel finds an available port starting from 8081
// Instead of limiting to 8081-8090, it scans until it finds an available port
func (mm *ModelManager) findAvailablePortForModel() (int, error) {
	// Start from 8081 and scan up to 100 ports
	const startPort = 8081
	const maxAttempts = 100

	for i := 0; i < maxAttempts; i++ {
		port := startPort + i
		addr := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			log.Printf("Found available port: %d", port)
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found after scanning %d ports starting from %d", maxAttempts, startPort)
}

// getModelMetadataFromPort fetches detailed model metadata from /v1/models endpoint
func (mm *ModelManager) getModelMetadataFromPort(port int) {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	url := fmt.Sprintf("http://localhost:%d/v1/models", port)
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Meta struct {
				NCtxTrain int   `json:"n_ctx_train"`
				NParams   int64 `json:"n_params"`
				Size      int64 `json:"size"`
				NVocab    int   `json:"n_vocab"`
				NEmbd     int   `json:"n_embd"`
			} `json:"meta"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	// Find the corresponding server and update metadata
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for _, server := range mm.servers {
		if server.Port == port && len(result.Data) > 0 {
			meta := result.Data[0].Meta
			server.ContextTrain = meta.NCtxTrain
			server.ParametersCount = meta.NParams
			server.ModelSizeBytes = meta.Size
			server.VocabSize = meta.NVocab
			server.EmbeddingDim = meta.NEmbd
			break
		}
	}
}

// UpdateVRAMUsage updates VRAM usage for all running servers
func (mm *ModelManager) UpdateVRAMUsage() error {
	// Get all GPU processes
	processes, err := gpu.GetProcessGPUMemory()
	if err != nil {
		// Don't fail if GPU info isn't available, just log
		log.Printf("Failed to get GPU process info: %v", err)
		return nil
	}

	// Create a map of PID -> VRAM usage for quick lookup
	pidToVRAM := make(map[int]int64)
	for _, proc := range processes {
		pidToVRAM[proc.PID] = proc.VRAMUsed
	}

	// Update VRAM for all running servers
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for _, server := range mm.servers {
		if server.PID > 0 {
			if vram, exists := pidToVRAM[server.PID]; exists {
				server.VRAMUsageBytes = vram
			}
		}
	}

	return nil
}

// isSystemdManaged checks if a process is managed by a systemd user service
func (mm *ModelManager) isSystemdManaged(pid int) bool {
	// Check if process is in a systemd cgroup
	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", pid)
	data, err := ioutil.ReadFile(cgroupPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "user@") && strings.Contains(string(data), ".service")
}

// getSystemdServiceName extracts the systemd service name for a PID
func (mm *ModelManager) getSystemdServiceName(pid int) string {
	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", pid)
	data, err := ioutil.ReadFile(cgroupPath)
	if err != nil {
		return ""
	}

	// Parse cgroup to extract service name
	// Format: 0::/user.slice/user-1000.slice/user@1000.service/app.slice/llama-server.service
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, ".service") {
			parts := strings.Split(line, "/")
			for _, part := range parts {
				if strings.HasSuffix(part, ".service") && !strings.Contains(part, "user@") {
					return part
				}
			}
		}
	}
	return ""
}

// StartServer starts a model server on an available port
func (mm *ModelManager) StartServer(modelPath string) error {
	// Step 1: Get server info and port with brief lock
	mm.mu.Lock()
	server, exists := mm.servers[modelPath]
	if !exists {
		mm.mu.Unlock()
		return fmt.Errorf("model not found: %s", modelPath)
	}

	if server.Status == "running" {
		port := server.Port
		mm.mu.Unlock()
		return fmt.Errorf("server already running on port %d", port)
	}

	// Find available port
	port, err := mm.findAvailablePortForModel()
	if err != nil {
		mm.mu.Unlock()
		return err
	}

	// Copy data we need for I/O operations
	modelName := server.ModelName
	isEmbeddingModel := strings.Contains(strings.ToLower(modelName), "embed")
	mm.mu.Unlock()

	// Step 2: Do ALL I/O operations WITHOUT holding lock
	logFile := filepath.Join("/tmp", fmt.Sprintf("llama-server-%d.log", port))

	args := []string{
		"-m", modelPath,
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", port),
		"-c", "131072",
		"-ngl", "999",
		"-fa", "on",
		"-b", "2048",
		"-ub", "512",
	}

	// Check if it's an embedding model
	if isEmbeddingModel {
		args = append(args, "--embedding")
	} else {
		args = append(args, "--jinja")
	}

	cmd := exec.Command(mm.llamaServerBin, args...)

	// Redirect output to log file
	logF, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	cmd.Stdout = logF
	cmd.Stderr = logF

	// Start the process
	if err := cmd.Start(); err != nil {
		logF.Close()
		return fmt.Errorf("failed to start server: %w", err)
	}

	pid := cmd.Process.Pid
	log.Printf("Started model server: %s on port %d (PID: %d)", modelName, port, pid)

	// Step 3: Acquire lock briefly to update state
	mm.mu.Lock()
	// Check again that server still exists (safety check)
	server, exists = mm.servers[modelPath]
	if !exists {
		mm.mu.Unlock()
		// Kill the process we just started since server was removed
		cmd.Process.Kill()
		logF.Close()
		return fmt.Errorf("model was removed while starting")
	}

	// Update server info
	server.Port = port
	server.PID = pid
	server.Status = "starting"
	server.ErrorMessage = "" // Clear any previous errors when starting
	server.URL = fmt.Sprintf("http://localhost:%d", port)
	mm.mu.Unlock()

	// Wait for server to be ready (in background)
	go mm.waitForServerReady(modelPath, port, 60*time.Second)

	return nil
}

// waitForServerReady polls the server until it's ready or timeout
func (mm *ModelManager) waitForServerReady(modelPath string, port int, timeout time.Duration) {
	url := fmt.Sprintf("http://localhost:%d/v1/models", port)
	deadline := time.Now().Add(timeout)
	var lastError error

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			mm.mu.Lock()
			if server, exists := mm.servers[modelPath]; exists {
				server.Status = "running"
				server.ErrorMessage = "" // Clear any previous errors
				server.LastChecked = time.Now().Unix()
				log.Printf("Model server ready: %s on port %d", server.ModelName, port)
			}
			mm.mu.Unlock()
			return
		}
		if err != nil {
			lastError = err
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}

	// Timeout - server failed to start
	mm.mu.Lock()
	if server, exists := mm.servers[modelPath]; exists {
		server.Status = "error"
		if lastError != nil {
			server.ErrorMessage = fmt.Sprintf("Failed to start: %v", lastError)
		} else {
			server.ErrorMessage = fmt.Sprintf("Server failed to respond within %v", timeout)
		}
		log.Printf("Model server failed to start: %s on port %d - %s", server.ModelName, port, server.ErrorMessage)
	}
	mm.mu.Unlock()
}

// StopServer stops a running model server
func (mm *ModelManager) StopServer(modelPath string) error {
	// Step 1: Get server info with brief lock
	mm.mu.Lock()
	server, exists := mm.servers[modelPath]
	if !exists {
		mm.mu.Unlock()
		return fmt.Errorf("model not found: %s", modelPath)
	}

	if server.Status != "running" && server.PID == 0 {
		mm.mu.Unlock()
		return fmt.Errorf("server not running")
	}

	// Copy data we need for I/O operations
	pid := server.PID
	modelName := server.ModelName
	mm.mu.Unlock()

	// Step 2: Do ALL I/O operations WITHOUT holding lock
	var stopErr error
	var stoppedViaSystemd bool

	// Check if process is managed by systemd and stop via systemctl
	if mm.isSystemdManaged(pid) {
		serviceName := mm.getSystemdServiceName(pid)
		if serviceName != "" {
			log.Printf("Detected systemd-managed service: %s, using systemctl to stop", serviceName)
			cmd := exec.Command("systemctl", "--user", "stop", serviceName)
			if err := cmd.Run(); err != nil {
				log.Printf("Warning: failed to stop systemd service %s: %v, falling back to kill", serviceName, err)
				// Fall through to kill if systemctl fails
			} else {
				log.Printf("Stopped systemd service: %s", serviceName)
				stoppedViaSystemd = true
			}
		}
	}

	// Kill the process directly if not stopped via systemd
	if !stoppedViaSystemd {
		process, err := os.FindProcess(pid)
		if err != nil {
			stopErr = fmt.Errorf("failed to find process: %w", err)
		} else if err := process.Kill(); err != nil {
			stopErr = fmt.Errorf("failed to kill process: %w", err)
		} else {
			log.Printf("Stopped model server: %s (PID: %d)", modelName, pid)
		}
	}

	// Step 3: Acquire lock briefly to update state
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Check again that server still exists (safety check)
	server, exists = mm.servers[modelPath]
	if !exists {
		return stopErr // Return any error from stop attempt
	}

	// Update server info
	server.Port = 0
	server.PID = 0
	server.Status = "stopped"
	server.URL = ""

	return stopErr
}

// DeleteModel deletes a model file from disk (only if stopped)
func (mm *ModelManager) DeleteModel(modelPath string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	server, exists := mm.servers[modelPath]
	if !exists {
		return fmt.Errorf("model not found: %s", modelPath)
	}

	if server.Status == "running" || server.Status == "starting" {
		return fmt.Errorf("cannot delete running server - stop it first")
	}

	// Delete the file from disk
	if err := os.Remove(modelPath); err != nil {
		return fmt.Errorf("failed to delete model file: %w", err)
	}

	log.Printf("Deleted model file: %s", modelPath)

	// Remove from tracking
	delete(mm.servers, modelPath)

	return nil
}

// GetServerStatus returns the current status of all servers
func (mm *ModelManager) GetServerStatus() []*ModelServer {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var servers []*ModelServer
	for _, server := range mm.servers {
		servers = append(servers, server)
	}

	// Sort by model path for consistent ordering
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].ModelPath < servers[j].ModelPath
	})

	return servers
}

// GetServerByModelPath returns server info for a specific model
func (mm *ModelManager) GetServerByModelPath(modelPath string) (*ModelServer, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	server, exists := mm.servers[modelPath]
	return server, exists
}

// ServeHTTP handlers for the model manager

// HandleListModels returns all available models and their server status as HTML
func (s *Server) HandleListModels(w http.ResponseWriter, r *http.Request) {
	log.Println("HandleListModels: Starting...")
	models, err := s.modelManager.ScanAvailableModels()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to scan models: %v", err), http.StatusInternalServerError)
		return
	}
	log.Println("HandleListModels: Scanned models")

	// Get cached server status (refreshed by background goroutine every 3s)
	log.Println("HandleListModels: Getting server status...")
	models = s.modelManager.GetServerStatus()
	log.Println("HandleListModels: Done with setup")

	w.Header().Set("Content-Type", "text/html")

	// Generate HTML with out-of-band swap to replace entire servers_list div
	html := `<div id="servers_list" hx-swap-oob="true">
	<table style="width: 100%; border-collapse: collapse;">
		<thead>
			<tr style="border-bottom: 1px solid #334155;">
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Model</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Status</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Port</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;" title="Context window size (tokens)">Context</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;" title="GPU-accessible memory (VRAM + System RAM)">Memory</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Actions</th>
			</tr>
		</thead>
		<tbody>
	`

	for i, model := range models {
		statusColor := "#94a3b8"
		statusText := model.Status
		statusTooltip := ""
		switch model.Status {
		case "running":
			statusColor = "#10b981"
			statusText = "Running"
		case "starting":
			statusColor = "#f59e0b"
			statusText = "Starting..."
		case "stopped":
			statusColor = "#64748b"
			statusText = "Stopped"
		case "error":
			statusColor = "#ef4444"
			statusText = "Error"
			if model.ErrorMessage != "" {
				statusTooltip = model.ErrorMessage
			}
		}

		portText := "-"
		if model.Port > 0 {
			portText = fmt.Sprintf("%d", model.Port)
		}

		contextText := "-"
		contextTooltip := ""
		if model.Status == "running" {
			if model.ContextSize > 0 {
				// Format active context size in K (thousands)
				if model.ContextSize >= 1000 {
					contextText = fmt.Sprintf("%dK", model.ContextSize/1000)
				} else {
					contextText = fmt.Sprintf("%d", model.ContextSize)
				}
			}

			// Add training context and model details to tooltip
			if model.ContextTrain > 0 {
				tooltipParts := []string{}
				tooltipParts = append(tooltipParts, fmt.Sprintf("Active: %s", contextText))

				// Training context
				trainText := fmt.Sprintf("%dK", model.ContextTrain/1000)
				tooltipParts = append(tooltipParts, fmt.Sprintf("Training: %s", trainText))

				// Parameters
				if model.ParametersCount > 0 {
					params := float64(model.ParametersCount) / 1e9
					tooltipParts = append(tooltipParts, fmt.Sprintf("Params: %.1fB", params))
				}

				// Model size
				if model.ModelSizeBytes > 0 {
					tooltipParts = append(tooltipParts, fmt.Sprintf("Size: %s", gpu.FormatBytes(model.ModelSizeBytes)))
				}

				// Vocab and embedding
				if model.VocabSize > 0 {
					tooltipParts = append(tooltipParts, fmt.Sprintf("Vocab: %dK", model.VocabSize/1000))
				}
				if model.EmbeddingDim > 0 {
					tooltipParts = append(tooltipParts, fmt.Sprintf("Embed: %d", model.EmbeddingDim))
				}

				contextTooltip = strings.Join(tooltipParts, " | ")
			} else {
				contextTooltip = "Context window size (tokens)"
			}
		}

		vramText := "-"
		memoryTooltip := ""
		if model.Status == "running" && model.VRAMUsageBytes > 0 {
			vramText = gpu.FormatBytes(model.VRAMUsageBytes)
			// Add helpful tooltip based on size
			// Models using >1GB are likely using GTT (system RAM)
			// Models using <1GB are likely using actual VRAM
			if model.VRAMUsageBytes > 1024*1024*1024 {
				memoryTooltip = "Primarily using system RAM (GTT) - large model"
			} else {
				memoryTooltip = "Primarily using GPU VRAM - small model/embedding"
			}
		}

		actionButton := ""
		testButton := ""
		deleteButton := ""

		// Generate stable ID for this model (use index)
		modelID := fmt.Sprintf("%d", i)

		if model.Status == "running" {
			actionButton = fmt.Sprintf(`
		<form style="display: inline;" hx-post="/api/servers/stop">
			<input type="hidden" name="model_path" value="%s" />
			<button 
				type="submit" 
				style="padding: 6px 12px; background: #dc2626; border: none; border-radius: 4px; color: white; cursor: pointer; font-size: 13px; margin-right: 8px;"
				hx-indicator="#stop-spinner-%s"
			>
				<span id="stop-spinner-%s" class="spinner htmx-indicator"></span>
				<span>Stop</span>
			</button>
		</form>
	`, model.ModelPath, modelID, modelID)
			// Add Logs button for running servers
			testButton = fmt.Sprintf(`
		<a href="/api/servers/logs?port=%d" target="_blank" 
		   style="display: inline-block; padding: 6px 12px; background: #64748b; border: none; border-radius: 4px; color: white; text-decoration: none; font-size: 13px; margin-right: 8px;">
			Logs
		</a>
	`, model.Port)
			// Can't delete while running
			deleteButton = `<span style="color: #64748b; font-size: 12px;">Stop first to delete</span>`
		} else if model.Status == "stopped" || model.Status == "error" {
			// Show Start button (or Retry for errors)
			buttonText := "Start"
			if model.Status == "error" {
				buttonText = "Retry"
			}
			actionButton = fmt.Sprintf(`
		<form style="display: inline;" hx-post="/api/servers/start">
			<input type="hidden" name="model_path" value="%s" />
			<button 
				type="submit" 
				style="padding: 6px 12px; background: #2563eb; border: none; border-radius: 4px; color: white; cursor: pointer; font-size: 13px; margin-right: 8px;"
				hx-indicator="#start-spinner-%s"
			>
				<span id="start-spinner-%s" class="spinner htmx-indicator"></span>
				<span>%s</span>
			</button>
		</form>
	`, model.ModelPath, modelID, modelID, buttonText)
			// Allow delete when stopped or error
			deleteButton = fmt.Sprintf(`
			<form style="display: inline;" hx-delete="/api/servers/delete" hx-confirm="Are you sure you want to delete %s? This will permanently remove the file from disk.">
				<input type="hidden" name="model_path" value="%s" />
				<button 
					type="submit" 
					style="padding: 6px 12px; background: #991b1b; border: none; border-radius: 4px; color: white; cursor: pointer; font-size: 13px;"
					hx-indicator="#delete-spinner-%s"
				>
					<span id="delete-spinner-%s" class="spinner htmx-indicator"></span>
					<span>Delete</span>
				</button>
			</form>
		`, model.ModelName, model.ModelPath, modelID, modelID)
		} else {
			actionButton = `<span style="color: #64748b;">Please wait...</span>`
		}

		html += fmt.Sprintf(`
		<tr style="border-bottom: 1px solid #1e293b;">
			<td style="padding: 14px 12px; color: #e2e8f0; font-size: 13px; font-family: monospace;">%s</td>
			<td style="padding: 14px 12px;" title="%s"><span style="color: %s; font-size: 13px; font-weight: 500;">%s</span></td>
			<td style="padding: 14px 12px; color: #cbd5e1; font-size: 13px;">%s</td>
			<td style="padding: 14px 12px; color: #cbd5e1; font-size: 13px;" title="%s">%s</td>
			<td style="padding: 14px 12px; color: #cbd5e1; font-size: 13px;" title="%s">%s</td>
			<td style="padding: 14px 12px;">%s%s%s</td>
		</tr>
	`, model.ModelName, statusTooltip, statusColor, statusText, portText, contextTooltip, contextText, memoryTooltip, vramText, actionButton, testButton, deleteButton)
	}

	html += `
		</tbody>
	</table>
	</div>
	`

	w.Write([]byte(html))
}

// HandleStartServer starts a model server and broadcasts SSE update
func (s *Server) HandleStartServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelPath := r.FormValue("model_path")
	if modelPath == "" {
		http.Error(w, "model_path required", http.StatusBadRequest)
		return
	}

	if err := s.modelManager.StartServer(modelPath); err != nil {
		log.Printf("Error starting server for %s: %v", modelPath, err)
		http.Error(w, fmt.Sprintf("Failed to start server: %v", err), http.StatusInternalServerError)
		return
	}

	// Wait briefly for server to start, then refresh status
	time.Sleep(500 * time.Millisecond)
	s.modelManager.RefreshServerStatus()
	// Note: SSE broadcast happens automatically via state change callback

	// Return updated server list with current status
	s.HandleListModels(w, r)
}

// HandleStopServer stops a model server and broadcasts SSE update
func (s *Server) HandleStopServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelPath := r.FormValue("model_path")
	if modelPath == "" {
		http.Error(w, "model_path required", http.StatusBadRequest)
		return
	}

	if err := s.modelManager.StopServer(modelPath); err != nil {
		log.Printf("Error stopping server for %s: %v", modelPath, err)
		http.Error(w, fmt.Sprintf("Failed to stop server: %v", err), http.StatusInternalServerError)
		return
	}

	// Wait for process to fully stop and GPU memory to be freed
	time.Sleep(500 * time.Millisecond)

	// Refresh status to get updated GPU memory
	s.modelManager.RefreshServerStatus()
	// Note: SSE broadcast happens automatically via state change callback

	// Return updated server list with current GPU status
	s.HandleListModels(w, r)
}

// HandleDeleteModel handles HTTP requests to delete a model file
func (s *Server) HandleDeleteModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelPath := r.FormValue("model_path")
	if modelPath == "" {
		http.Error(w, "Missing model_path parameter", http.StatusBadRequest)
		return
	}

	if err := s.modelManager.DeleteModel(modelPath); err != nil {
		log.Printf("Error deleting model %s: %v", modelPath, err)

		// Return error message in red box (HTMX will swap this into the page)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<div style="background-color: #fee2e2; border: 1px solid #991b1b; color: #991b1b; padding: 1rem; margin: 1rem; border-radius: 0.25rem;">
			<strong>Error:</strong> %s
		</div>`, err.Error())
		return
	}

	log.Printf("Successfully deleted model: %s", modelPath)

	// Return updated server list
	s.HandleListModels(w, r)
}

// HandleServerLogs returns the log file for a running server
func (s *Server) HandleServerLogs(w http.ResponseWriter, r *http.Request) {
	portStr := r.URL.Query().Get("port")
	if portStr == "" {
		http.Error(w, "Missing port parameter", http.StatusBadRequest)
		return
	}

	port, err := fmt.Sscanf(portStr, "%d", new(int))
	if err != nil || port == 0 {
		http.Error(w, "Invalid port parameter", http.StatusBadRequest)
		return
	}

	logFile := filepath.Join("/tmp", fmt.Sprintf("llama-server-%s.log", portStr))

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		http.Error(w, "Log file not found", http.StatusNotFound)
		return
	}

	// Read log file
	content, err := ioutil.ReadFile(logFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read log file: %v", err), http.StatusInternalServerError)
		return
	}

	// Return as plain text with proper content type
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(content)
}
