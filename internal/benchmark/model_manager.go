package benchmark

import (
	"clai/internal/db"
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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ModelServer represents a running or available model server
type ModelServer struct {
	ModelPath       string  `json:"model_path"`
	ModelName       string  `json:"model_name"`
	Port            int     `json:"port"`
	PID             int     `json:"pid"`
	Status          string  `json:"status"`        // "running", "loading", "stopped", "starting", "error"
	ErrorMessage    string  `json:"error_message"` // Error details when status is "error"
	URL             string  `json:"url"`
	APIType         string  `json:"api_type"`
	LastChecked     int64   `json:"last_checked"`
	VRAMUsageBytes  int64   `json:"vram_usage_bytes"` // VRAM used by this server in bytes
	CPUPercent      float64 `json:"cpu_percent"`      // CPU usage percentage
	MemoryBytes     int64   `json:"memory_bytes"`     // RAM (RSS) used by this process in bytes
	ContextSize     int     `json:"context_size"`     // Active context window size (n_ctx)
	ContextTrain    int     `json:"context_train"`    // Training context size (n_ctx_train)
	ParametersCount int64   `json:"parameters_count"` // Total parameters (n_params)
	ModelSizeBytes  int64   `json:"model_size_bytes"` // Model file size in bytes
	VocabSize       int     `json:"vocab_size"`       // Vocabulary size (n_vocab)
	EmbeddingDim    int     `json:"embedding_dim"`    // Embedding dimensions (n_embd)

	// Split model metadata
	IsSplitModel    bool     `json:"is_split_model"`    // True if this is a multi-part GGUF model
	SplitPartNumber int      `json:"split_part_number"` // Current part number (1-based)
	SplitTotalParts int      `json:"split_total_parts"` // Total number of parts
	SplitPartsFound int      `json:"split_parts_found"` // Number of parts found on disk
	SplitAllParts   []string `json:"split_all_parts"`   // Paths to all parts (for tracking)
	SplitIsComplete bool     `json:"split_is_complete"` // True if all parts are present
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
func NewModelManager(dbStore *db.Store) *ModelManager {
	modelsDir := "/home/josh/models"
	mm := &ModelManager{
		servers:         make(map[string]*ModelServer),
		modelsDir:       modelsDir,
		llamaServerBin:  "/home/josh/llama.cpp-rocm-wmma/build/bin/llama-server",
		downloadManager: NewDownloadManager(modelsDir, dbStore),
		stopRefresh:     make(chan struct{}),
	}

	// Start background refresh goroutine
	go mm.backgroundRefresh()

	return mm
}

// NewModelManagerForTest creates a model manager without database (for testing)
func NewModelManagerForTest() *ModelManager {
	return NewModelManager(nil)
}

// backgroundRefresh periodically refreshes server status in the background
func (mm *ModelManager) backgroundRefresh() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Do an initial refresh immediately
	mm.RefreshServerStatus()
	mm.UpdateVRAMUsage()
	mm.UpdateCPUAndMemory()
	mm.notifyStateChange()

	for {
		select {
		case <-ticker.C:
			mm.RefreshServerStatus()
			mm.UpdateVRAMUsage()
			mm.UpdateCPUAndMemory()
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
// For split models (e.g., model-00001-of-00004.gguf), only the first part is shown as launchable
func (mm *ModelManager) ScanAvailableModels() ([]*ModelServer, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	files, err := ioutil.ReadDir(mm.modelsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read models directory: %w", err)
	}

	// Get list of files that are currently being downloaded
	activeDownloads := make(map[string]bool)
	if mm.downloadManager != nil {
		downloads := mm.downloadManager.GetDownloads()
		for _, dl := range downloads {
			// Only exclude files with "downloading" status
			// Completed and failed downloads should be shown in model list
			if dl.Status == "downloading" {
				activeDownloads[dl.FilePath] = true
			}
		}
	}

	// Track split model prefixes we've already processed
	processedSplitModels := make(map[string]bool)

	var models []*ModelServer
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".gguf") {
			continue
		}

		modelPath := filepath.Join(mm.modelsDir, file.Name())

		// Skip files that are currently being downloaded
		if activeDownloads[modelPath] {
			continue
		}

		// Parse split model info
		splitInfo := parseSplitModelFilename(file.Name())

		// If this is a split model but not the first part, skip it
		// (we only want to show the first part as a launchable model)
		if splitInfo.IsSplit {
			// Check if we've already processed this split model set
			if processedSplitModels[splitInfo.Prefix] {
				continue
			}

			// Only show first part (00001) as the launchable model
			if splitInfo.PartNumber != 1 {
				continue
			}

			// Mark this split model set as processed
			processedSplitModels[splitInfo.Prefix] = true
		}

		// Check if we already have this model tracked
		if server, exists := mm.servers[modelPath]; exists {
			// Update split model metadata
			if splitInfo.IsSplit {
				partsFound, totalParts, isComplete := mm.checkSplitModelComplete(modelPath)
				server.IsSplitModel = true
				server.SplitPartNumber = splitInfo.PartNumber
				server.SplitTotalParts = totalParts
				server.SplitPartsFound = partsFound
				server.SplitIsComplete = isComplete
				server.SplitAllParts = mm.getSplitModelParts(modelPath)
				// Recalculate total size of all parts
				server.ModelSizeBytes = mm.calculateTotalSize(server.SplitAllParts)
			}
			models = append(models, server)
			continue
		}

		// New model - add to tracking
		server := &ModelServer{
			ModelPath:      modelPath,
			ModelName:      file.Name(),
			Status:         "stopped",
			APIType:        "llamacpp",
			ModelSizeBytes: file.Size(),
		}

		// Set split model metadata if applicable
		if splitInfo.IsSplit {
			partsFound, totalParts, isComplete := mm.checkSplitModelComplete(modelPath)
			server.IsSplitModel = true
			server.SplitPartNumber = splitInfo.PartNumber
			server.SplitTotalParts = totalParts
			server.SplitPartsFound = partsFound
			server.SplitIsComplete = isComplete
			server.SplitAllParts = mm.getSplitModelParts(modelPath)
			// Calculate total size of all parts
			server.ModelSizeBytes = mm.calculateTotalSize(server.SplitAllParts)
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

				// Check if server is loading or ready
				isLoading := mm.checkIfServerLoading(info.port)
				if isLoading {
					server.Status = "loading"
				} else {
					server.Status = "running"
				}

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
	// Use lsof to find process listening on port
	cmd := exec.Command("lsof", "-t", "-i", fmt.Sprintf(":%d", port))
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	// lsof can return multiple PIDs (one per line), find the llama-server process
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		var pid int
		if _, err := fmt.Sscanf(line, "%d", &pid); err == nil && pid > 0 {
			// Check if this PID is a llama-server process
			cmdline, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
			if err == nil && strings.Contains(string(cmdline), "llama-server") {
				return pid
			}
		}
	}

	// Fallback: return the first PID if no llama-server found
	if len(lines) > 0 {
		var pid int
		fmt.Sscanf(lines[0], "%d", &pid)
		return pid
	}

	return 0
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

// checkIfServerLoading checks if a server port is responding but still loading
// Returns true if the server is loading (503 with "Loading model" message)
func (mm *ModelManager) checkIfServerLoading(port int) bool {
	healthURL := fmt.Sprintf("http://localhost:%d/health", port)
	client := &http.Client{Timeout: 200 * time.Millisecond}
	resp, err := client.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Check if it's specifically "Loading model" (503 status)
	if resp.StatusCode == http.StatusServiceUnavailable {
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil && strings.Contains(string(body), "Loading model") {
			return true
		}
	}
	return false
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
		log.Printf("UpdateVRAMUsage: Failed to get GPU process info: %v", err)
		return nil
	}

	log.Printf("UpdateVRAMUsage: Found %d GPU processes", len(processes))

	// Create a map of PID -> VRAM usage for quick lookup
	pidToVRAM := make(map[int]int64)
	for _, proc := range processes {
		pidToVRAM[proc.PID] = proc.VRAMUsed
		log.Printf("UpdateVRAMUsage: GPU Process - PID: %d, Name: %s, VRAM: %d bytes", proc.PID, proc.ProcessName, proc.VRAMUsed)
	}

	// Update VRAM for all running servers
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for _, server := range mm.servers {
		if server.PID > 0 {
			log.Printf("UpdateVRAMUsage: Checking server %s (PID: %d)", server.ModelName, server.PID)
			if vram, exists := pidToVRAM[server.PID]; exists {
				server.VRAMUsageBytes = vram
				log.Printf("UpdateVRAMUsage: Updated server %s VRAM to %d bytes", server.ModelName, vram)
			} else {
				log.Printf("UpdateVRAMUsage: No VRAM data found for PID %d", server.PID)
			}
		}
	}

	return nil
}

// UpdateCPUAndMemory updates CPU and memory usage for all running servers
func (mm *ModelManager) UpdateCPUAndMemory() error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for _, server := range mm.servers {
		if server.PID > 0 && (server.Status == "running" || server.Status == "loading") {
			// Read /proc/[pid]/stat for CPU and memory info
			statPath := fmt.Sprintf("/proc/%d/stat", server.PID)
			statData, err := ioutil.ReadFile(statPath)
			if err != nil {
				// Process may have exited
				continue
			}

			// Parse stat file - fields are space-separated
			// We need RSS (field 24, 0-indexed = 23)
			statFields := strings.Fields(string(statData))
			if len(statFields) < 24 {
				continue
			}

			// Get RSS (Resident Set Size) in pages
			rssPages, err := strconv.ParseInt(statFields[23], 10, 64)
			if err != nil {
				continue
			}

			// Convert pages to bytes (typical page size is 4096 bytes)
			pageSize := int64(4096)
			server.MemoryBytes = rssPages * pageSize

			// Get CPU usage by reading utime + stime
			// For simplicity, we'll use ps command to get accurate CPU percentage
			cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", server.PID), "-o", "%cpu=")
			output, err := cmd.Output()
			if err == nil {
				cpuStr := strings.TrimSpace(string(output))
				if cpuPercent, err := strconv.ParseFloat(cpuStr, 64); err == nil {
					server.CPUPercent = cpuPercent
				}
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

// StartServer starts a model server on an available port with default context size
func (mm *ModelManager) StartServer(modelPath string) error {
	return mm.StartServerWithContext(modelPath, 131072, 999) // 999 = all GPU layers
}

// StartServerWithContext starts a model server with a custom context size and GPU layer count
func (mm *ModelManager) StartServerWithContext(modelPath string, contextSize int, ngl int) error {
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

	// Check if this is a split model and verify all parts exist
	isSplit := server.IsSplitModel
	splitComplete := server.SplitIsComplete
	partsFound := server.SplitPartsFound
	totalParts := server.SplitTotalParts

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

	// Verify split model is complete before starting
	if isSplit && !splitComplete {
		return fmt.Errorf("cannot start split model: only %d of %d parts found - download all parts first", partsFound, totalParts)
	}

	// Step 2: Do ALL I/O operations WITHOUT holding lock
	logFile := filepath.Join("/tmp", fmt.Sprintf("llama-server-%d.log", port))

	args := []string{
		"-m", modelPath,
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", port),
		"-c", fmt.Sprintf("%d", contextSize),
		"-ngl", fmt.Sprintf("%d", ngl),
		"-fa", "on",
		"-b", "2048",
		"-ub", "512",
		"--verbose", // Enable verbose logging for detailed tensor loading progress
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
	// Use 2-hour timeout for large models (78B+ can take 30-60+ minutes to load)
	go mm.waitForServerReady(modelPath, port, 2*time.Hour)

	return nil
}

// waitForServerReady polls the server until it's ready or timeout
// Uses /health endpoint which correctly returns 503 during loading and 200 when ready
func (mm *ModelManager) waitForServerReady(modelPath string, port int, timeout time.Duration) {
	url := fmt.Sprintf("http://localhost:%d/health", port)
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

		// Try to read the last 20 lines of the log file to get the actual error
		logFile := filepath.Join("/tmp", fmt.Sprintf("llama-server-%d.log", port))
		logContent := ""
		if data, err := os.ReadFile(logFile); err == nil {
			lines := strings.Split(string(data), "\n")
			// Get last 20 non-empty lines
			var lastLines []string
			for i := len(lines) - 1; i >= 0 && len(lastLines) < 20; i-- {
				line := strings.TrimSpace(lines[i])
				if line != "" {
					lastLines = append([]string{line}, lastLines...)
				}
			}
			if len(lastLines) > 0 {
				logContent = strings.Join(lastLines, "\n")
			}
		}

		if lastError != nil {
			server.ErrorMessage = fmt.Sprintf("Failed to start: %v", lastError)
		} else {
			server.ErrorMessage = fmt.Sprintf("Server failed to respond within %v", timeout)
		}

		// Append log content if available
		if logContent != "" {
			server.ErrorMessage += "\n\nLast 20 lines from log:\n" + logContent
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

	// For split models, delete all parts
	if server.IsSplitModel && len(server.SplitAllParts) > 0 {
		deletedParts := 0
		missingParts := 0
		for _, partPath := range server.SplitAllParts {
			if err := os.Remove(partPath); err != nil {
				if os.IsNotExist(err) {
					missingParts++
					log.Printf("Split model part already gone: %s", partPath)
				} else {
					log.Printf("Warning: failed to delete split model part %s: %v", partPath, err)
				}
			} else {
				deletedParts++
				log.Printf("Deleted split model part: %s", partPath)
			}
		}
		if missingParts > 0 {
			log.Printf("Deleted %d/%d split model parts (%d were already missing)", deletedParts, len(server.SplitAllParts), missingParts)
		} else {
			log.Printf("Deleted all %d split model parts", deletedParts)
		}
	} else {
		// Single file - delete it
		if err := os.Remove(modelPath); err != nil {
			return fmt.Errorf("failed to delete model file: %w", err)
		}
		log.Printf("Deleted model file: %s", modelPath)
	}

	// Remove from tracking
	delete(mm.servers, modelPath)

	return nil
}

// splitModelInfo contains metadata about a split GGUF file
type splitModelInfo struct {
	Prefix     string // e.g., "model-name"
	PartNumber int    // e.g., 1 (from 00001)
	TotalParts int    // e.g., 4 (from 00004)
	IsSplit    bool   // true if this is a split model file
}

// parseSplitModelFilename detects and parses split GGUF filenames
// Pattern: model-name-00001-of-00004.gguf
// Returns splitModelInfo with IsSplit=true if it matches, IsSplit=false otherwise
func parseSplitModelFilename(filename string) splitModelInfo {
	// Pattern: anything ending with -NNNNN-of-NNNNN.gguf
	re := regexp.MustCompile(`^(.+)-(\d{5})-of-(\d{5})\.gguf$`)
	matches := re.FindStringSubmatch(filename)

	if len(matches) != 4 {
		return splitModelInfo{IsSplit: false}
	}

	prefix := matches[1]
	partNum, err1 := strconv.Atoi(matches[2])
	totalParts, err2 := strconv.Atoi(matches[3])

	if err1 != nil || err2 != nil || partNum < 1 || totalParts < 1 || partNum > totalParts {
		return splitModelInfo{IsSplit: false}
	}

	return splitModelInfo{
		Prefix:     prefix,
		PartNumber: partNum,
		TotalParts: totalParts,
		IsSplit:    true,
	}
}

// buildSplitModelFilename constructs a split model filename from components
// e.g., buildSplitModelFilename("model-name", 1, 4) => "model-name-00001-of-00004.gguf"
func buildSplitModelFilename(prefix string, partNum, totalParts int) string {
	return fmt.Sprintf("%s-%05d-of-%05d.gguf", prefix, partNum, totalParts)
}

// getSplitModelParts returns paths to all parts of a split model
// Given any part of a split model, returns paths to all parts (including non-existent ones)
func (mm *ModelManager) getSplitModelParts(modelPath string) []string {
	filename := filepath.Base(modelPath)
	info := parseSplitModelFilename(filename)

	if !info.IsSplit {
		return []string{modelPath} // Not a split model, return as-is
	}

	dir := filepath.Dir(modelPath)
	var parts []string

	for i := 1; i <= info.TotalParts; i++ {
		partFilename := buildSplitModelFilename(info.Prefix, i, info.TotalParts)
		partPath := filepath.Join(dir, partFilename)
		parts = append(parts, partPath)
	}

	return parts
}

// checkSplitModelComplete verifies all parts of a split model exist
// Returns (existingParts, totalParts, allExist)
func (mm *ModelManager) checkSplitModelComplete(modelPath string) (int, int, bool) {
	parts := mm.getSplitModelParts(modelPath)

	if len(parts) == 1 {
		// Not a split model - just check if the single file exists
		_, err := os.Stat(modelPath)
		if err == nil {
			return 1, 1, true
		}
		return 0, 1, false
	}

	// Split model - check all parts
	existingCount := 0
	for _, part := range parts {
		if _, err := os.Stat(part); err == nil {
			existingCount++
		}
	}

	allExist := existingCount == len(parts)
	return existingCount, len(parts), allExist
}

// calculateTotalSize sums up the file sizes of all provided paths
func (mm *ModelManager) calculateTotalSize(paths []string) int64 {
	var totalSize int64
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil {
			totalSize += info.Size()
		}
	}
	return totalSize
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
				<th style="text-align: left; padding: 12px; color: #94a3b8;" title="Last benchmark score (% of tests passed)">Score</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Actions</th>
			</tr>
		</thead>
		<tbody>
	`

	for i, model := range models {
		statusColor := "#94a3b8"
		statusText := model.Status
		statusTooltip := ""
		statusHTML := ""
		switch model.Status {
		case "running":
			statusColor = "#10b981"
			statusText = "🟢 Running"
		case "loading":
			statusColor = "#f59e0b"
			statusText = "⏳ Loading..."
			statusTooltip = "Model is loading into memory"
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
				statusTooltip = "Click to view error details"
				// Make error clickable to show modal with full error
				// Use data attributes to avoid complex escaping issues
				escapedModelName := htmlEscape(model.ModelName)
				escapedError := htmlEscape(model.ErrorMessage)
				statusHTML = fmt.Sprintf(`<span style="color: %s; font-size: 13px; font-weight: 500; cursor: pointer; text-decoration: underline;" class="error-status" data-model-name="%s" data-error-message="%s">%s ⓘ</span>`,
					statusColor, escapedModelName, escapedError, statusText)
			}
		}

		// Default status HTML if not set above
		if statusHTML == "" {
			statusHTML = fmt.Sprintf(`<span style="color: %s; font-size: 13px; font-weight: 500;">%s</span>`, statusColor, statusText)
		}

		// Fetch last benchmark score for this model
		var scoreText string
		var scoreColor string
		lastBenchmark, err := s.store.GetLastBenchmarkForModel(model.ModelName)
		if err != nil || lastBenchmark == nil {
			scoreText = "-"
			scoreColor = "#64748b"
		} else {
			scoreText = fmt.Sprintf("%.1f%%", lastBenchmark.SuccessRate)
			// Color based on score: green (>80%), yellow (50-80%), red (<50%)
			if lastBenchmark.SuccessRate >= 80 {
				scoreColor = "#10b981"
			} else if lastBenchmark.SuccessRate >= 50 {
				scoreColor = "#f59e0b"
			} else {
				scoreColor = "#ef4444"
			}
		}

		portText := "-"
		if model.Port > 0 {
			portText = fmt.Sprintf("%d", model.Port)
		}

		contextText := "-"
		contextTooltip := ""
		if model.Status == "running" || model.Status == "loading" {
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
		if (model.Status == "running" || model.Status == "loading") && model.VRAMUsageBytes > 0 {
			vramText = gpu.FormatBytes(model.VRAMUsageBytes)
			// Add helpful tooltip based on size
			// Models using >1GB are likely using GTT (system RAM)
			// Models using <1GB are likely using actual VRAM
			if model.VRAMUsageBytes > 1024*1024*1024 {
				memoryTooltip = "Primarily using system RAM (GTT) - large model"
			} else {
				memoryTooltip = "Primarily using GPU VRAM - small model/embedding"
			}
		} else if model.ModelSizeBytes > 0 {
			// For stopped models, show file size instead of "-"
			vramText = gpu.FormatBytes(model.ModelSizeBytes)
			memoryTooltip = "Model file size (start server to see actual memory usage)"
		}

		actionButton := ""
		benchmarkButton := ""
		testButton := ""
		deleteButton := ""

		// Generate stable ID for this model (use index)
		modelID := fmt.Sprintf("%d", i)

		if model.Status == "running" || model.Status == "loading" {
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

			// Only show Run Benchmark for fully loaded models
			if model.Status == "running" {
				benchmarkButton = fmt.Sprintf(`
	<form style="display: inline;" hx-post="/api/benchmark/run" hx-target="#benchmark_status_%s" hx-swap="innerHTML">
		<input type="hidden" name="model_path" value="%s" />
		<button 
			type="submit" 
			style="padding: 6px 12px; background: #7c3aed; border: none; border-radius: 4px; color: white; cursor: pointer; font-size: 13px; margin-right: 8px;"
			hx-indicator="#benchmark-spinner-%s"
		>
			<span id="benchmark-spinner-%s" class="spinner htmx-indicator"></span>
			<span>Run Benchmark</span>
		</button>
	</form>
	<div id="benchmark_status_%s"></div>
`, modelID, model.ModelPath, modelID, modelID, modelID)
			} else {
				// Loading state - show disabled message
				benchmarkButton = fmt.Sprintf(`
	<span style="color: #64748b; font-size: 12px; font-style: italic;">Wait for model to load...</span>
	<div id="benchmark_status_%s"></div>
`, modelID)
			}
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
		<form 
			style="display: flex; flex-direction: column; gap: 8px;" 
			hx-post="/api/servers/start"
			data-model-size="%d"
			data-parameters="%d"
		>
			<div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
				<input type="hidden" name="model_path" value="%s" />
				<select 
					name="context_size" 
					style="padding: 6px 8px; border: 1px solid #475569; background: #1e293b; color: #e2e8f0; border-radius: 4px; font-size: 12px; cursor: pointer;"
					title="Context window size - smaller contexts load faster and use less memory"
				>
					<option value="">Default (131K)</option>
					<option value="2048">2K - Minimal</option>
					<option value="4096">4K - Small</option>
					<option value="8192">8K - Standard</option>
					<option value="16384">16K - Medium</option>
					<option value="32768">32K - Large</option>
					<option value="65536">65K - Very Large</option>
					<option value="131072">131K - Huge</option>
				</select>
				<select 
					name="ngl" 
					style="padding: 6px 8px; border: 1px solid #475569; background: #1e293b; color: #e2e8f0; border-radius: 4px; font-size: 12px; cursor: pointer;"
					title="GPU layers - fewer layers = less VRAM usage"
				>
					<option value="999">Auto (All Layers)</option>
					<option value="0">CPU Only (0 Layers)</option>
					<option value="10">10 Layers</option>
					<option value="20">20 Layers</option>
					<option value="30">30 Layers</option>
					<option value="40">40 Layers</option>
					<option value="50">50 Layers ⭐</option>
					<option value="60">60 Layers</option>
					<option value="70">70 Layers</option>
					<option value="80">80 Layers</option>
				</select>
				<button 
					type="submit" 
					style="padding: 6px 12px; background: #2563eb; border: none; border-radius: 4px; color: white; cursor: pointer; font-size: 13px;"
					hx-indicator="#start-spinner-%s"
				>
					<span id="start-spinner-%s" class="spinner htmx-indicator"></span>
					<span>%s</span>
				</button>
			</div>
			<div class="memory-estimate" style="margin-top: 4px;">
				<!-- Memory estimate will be dynamically populated by JavaScript -->
			</div>
		</form>
	`, model.ModelSizeBytes, model.ParametersCount, model.ModelPath, modelID, modelID, buttonText)
			// Allow delete when stopped or error
			deleteConfirmMsg := fmt.Sprintf("Are you sure you want to delete %s? This will permanently remove the file from disk.", model.ModelName)
			if model.IsSplitModel && model.SplitTotalParts > 1 {
				if model.SplitIsComplete {
					deleteConfirmMsg = fmt.Sprintf("Are you sure you want to delete %s? This will permanently remove all %d part files from disk.", model.ModelName, model.SplitTotalParts)
				} else {
					deleteConfirmMsg = fmt.Sprintf("Are you sure you want to delete %s? This will permanently remove the %d downloaded part file(s) from disk. %d part(s) are still missing.",
						model.ModelName, model.SplitPartsFound, model.SplitTotalParts-model.SplitPartsFound)
				}
			}

			deleteButton = fmt.Sprintf(`
			<form style="display: inline;" hx-delete="/api/servers/delete" hx-confirm="%s">
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
		`, deleteConfirmMsg, model.ModelPath, modelID, modelID)
		} else {
			actionButton = `<span style="color: #64748b;">Please wait...</span>`
		}

		// Format model name with split model indicator if applicable
		modelNameDisplay := model.ModelName
		modelNameTooltip := ""
		if model.IsSplitModel {
			if model.SplitIsComplete {
				modelNameDisplay = fmt.Sprintf("%s <span style=\"color: #10b981; font-size: 11px;\">(%d parts)</span>",
					model.ModelName, model.SplitTotalParts)
				modelNameTooltip = fmt.Sprintf("Split model with %d parts - all parts present", model.SplitTotalParts)
			} else {
				modelNameDisplay = fmt.Sprintf("%s <span style=\"color: #f59e0b; font-size: 11px;\">(%d/%d parts)</span>",
					model.ModelName, model.SplitPartsFound, model.SplitTotalParts)
				modelNameTooltip = fmt.Sprintf("Split model: %d of %d parts found - download remaining parts to start",
					model.SplitPartsFound, model.SplitTotalParts)

				// If split model is incomplete, disable Start button
				if model.Status == "stopped" {
					actionButton = `<span style="color: #f59e0b; font-size: 12px;">Download all parts first</span>`
					deleteButton = "" // Don't show delete for incomplete downloads
				}
			}
		}

		// Add toggle button for logs if model is starting, loading, or running
		logsToggleButton := ""
		if model.Status == "running" || model.Status == "loading" || model.Status == "starting" {
			logsToggleButton = fmt.Sprintf(`
				<button 
					onclick="document.getElementById('logs-%s').classList.toggle('hidden')" 
					style="padding: 6px 12px; background: #475569; border: none; border-radius: 4px; color: white; cursor: pointer; font-size: 13px; margin-right: 8px;"
				>
					📋 Logs
				</button>
			`, modelID)
		}

		html += fmt.Sprintf(`
		<tr style="border-bottom: 1px solid #1e293b;">
			<td style="padding: 14px 12px; color: #e2e8f0; font-size: 13px; font-family: monospace;" title="%s">%s</td>
			<td style="padding: 14px 12px;" title="%s">%s</td>
			<td style="padding: 14px 12px; color: #cbd5e1; font-size: 13px;">%s</td>
			<td style="padding: 14px 12px; color: #cbd5e1; font-size: 13px;" title="%s">%s</td>
			<td style="padding: 14px 12px; color: #cbd5e1; font-size: 13px;" title="%s">%s</td>
			<td style="padding: 14px 12px; color: %s; font-size: 13px; font-weight: 500;">%s</td>
			<td style="padding: 14px 12px;">%s%s%s%s%s</td>
		</tr>
	`, modelNameTooltip, modelNameDisplay, statusTooltip, statusHTML, portText, contextTooltip, contextText, memoryTooltip, vramText, scoreColor, scoreText, actionButton, benchmarkButton, testButton, logsToggleButton, deleteButton)

		// Add collapsible log viewer row with SSE updates
		if model.Status == "running" || model.Status == "loading" || model.Status == "starting" {
			html += fmt.Sprintf(`
		<tr id="logs-%s" class="hidden" style="border-bottom: 1px solid #1e293b;">
			<td colspan="7" style="padding: 0;">
				<div style="background: #0f172a; padding: 12px; margin: 8px; border-radius: 4px; border: 1px solid #334155;">
					<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
						<span style="color: #94a3b8; font-size: 12px; font-weight: 500;">Server Logs (Last 50 lines - Live)</span>
						<a href="/api/servers/logs?port=%d" target="_blank" 
						   style="color: #60a5fa; font-size: 12px; text-decoration: none;">
							View Full Logs ↗
						</a>
					</div>
					<div hx-ext="sse" sse-connect="/api/servers/logs/stream?port=%d" sse-swap="log_update">
						<pre id="log-content-%s" style="background: #020617; color: #e2e8f0; padding: 12px; border-radius: 4px; font-size: 11px; font-family: monospace; overflow-x: auto; max-height: 400px; overflow-y: auto; margin: 0;">Connecting to log stream...</pre>
					</div>
				</div>
			</td>
		</tr>
	`, modelID, model.Port, model.Port, modelID)
		}
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

	// Check for custom context size parameter
	contextSize := 131072 // default
	ctxStr := r.FormValue("context_size")
	log.Printf("DEBUG: Received context_size parameter: '%s'", ctxStr)
	if ctxStr != "" {
		if ctx, err := strconv.Atoi(ctxStr); err == nil && ctx > 0 {
			contextSize = ctx
			log.Printf("Using custom context size: %d for %s", contextSize, modelPath)
		} else {
			log.Printf("DEBUG: Failed to parse context_size: err=%v", err)
		}
	} else {
		log.Printf("DEBUG: No context_size parameter provided, using default")
	}

	// Check for custom ngl (GPU layers) parameter
	ngl := 999 // default = all layers
	nglStr := r.FormValue("ngl")
	log.Printf("DEBUG: Received ngl parameter: '%s'", nglStr)
	if nglStr != "" {
		if n, err := strconv.Atoi(nglStr); err == nil && n >= 0 {
			ngl = n
			log.Printf("Using custom ngl: %d for %s", ngl, modelPath)
		} else {
			log.Printf("DEBUG: Failed to parse ngl: err=%v", err)
		}
	} else {
		log.Printf("DEBUG: No ngl parameter provided, using default (all layers)")
	}

	if err := s.modelManager.StartServerWithContext(modelPath, contextSize, ngl); err != nil {
		log.Printf("Error starting server for %s: %v", modelPath, err)
		http.Error(w, fmt.Sprintf("Failed to start server: %v", err), http.StatusInternalServerError)
		return
	}

	// Wait briefly for server to start, then refresh status
	time.Sleep(500 * time.Millisecond)
	s.modelManager.RefreshServerStatus()
	s.modelManager.UpdateVRAMUsage()
	// Trigger SSE broadcast for state change
	s.modelManager.notifyStateChange()

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
	s.modelManager.UpdateVRAMUsage()
	// Trigger SSE broadcast for state change
	s.modelManager.notifyStateChange()

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

// HandleServerLogsStream streams log updates via SSE for real-time viewing
func (s *Server) HandleServerLogsStream(w http.ResponseWriter, r *http.Request) {
	portStr := r.URL.Query().Get("port")
	if portStr == "" {
		http.Error(w, "Missing port parameter", http.StatusBadRequest)
		return
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port == 0 {
		http.Error(w, "Invalid port parameter", http.StatusBadRequest)
		return
	}

	logFile := filepath.Join("/tmp", fmt.Sprintf("llama-server-%d.log", port))

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial log content
	sendLogUpdate := func() error {
		data, err := os.ReadFile(logFile)
		if err != nil {
			// File might not exist yet if server is just starting
			data = []byte("Waiting for server to start...")
		}

		// Get last 50 lines
		lines := strings.Split(string(data), "\n")
		startIdx := 0
		if len(lines) > 50 {
			startIdx = len(lines) - 50
		}
		logContent := strings.Join(lines[startIdx:], "\n")

		// Escape HTML
		logContent = htmlEscape(logContent)

		// Send SSE event
		fmt.Fprintf(w, "event: log_update\n")
		fmt.Fprintf(w, "data: %s\n\n", logContent)
		flusher.Flush()
		return nil
	}

	// Send initial update
	if err := sendLogUpdate(); err != nil {
		return
	}

	// Stream updates every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := sendLogUpdate(); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}
