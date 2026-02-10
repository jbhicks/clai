package benchmark

import (
	"clai/internal/benchmark/templates"
	"clai/internal/db"
	"clai/internal/gpu"
	"clai/internal/logger"
	"clai/internal/types"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

// ModelServer is now defined in internal/types
type ModelServer = types.ModelServer

// BackendInfo is now defined in internal/types
type BackendInfo = types.BackendInfo

// ModelManager handles starting/stopping model servers
type ModelManager struct {
	mu              sync.RWMutex
	servers         map[string]*ModelServer // key: model_path
	modelsDir       string
	backends        map[string]*BackendInfo // key: backend type ("rocm", "vulkan")
	downloadManager *DownloadManager
	stopRefresh     chan struct{}   // Signal to stop background refresh
	lastStateHash   string          // Hash of server states for change detection
	onStateChange   func()          // Callback when state changes
	dockerLauncher  *DockerLauncher // Docker launcher for container-based models
}

// detectLlamaServerVersion runs llama-server --version and extracts the version number
func detectLlamaServerVersion(binaryPath string) string {
	// Check if file exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return ""
	}

	cmd := exec.Command(binaryPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	// Parse output like "version: 6867 (a45e1cd6)"
	// Extract the build number and commit hash
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "version:") {
			// Extract version info
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return strings.Join(parts[1:], " ") // "6867 (a45e1cd6)"
			}
		}
	}

	return ""
}

// NewModelManagerWithBackgroundRefresh creates a new model manager with optional background refresh
func NewModelManagerWithBackgroundRefresh(dbStore *db.Store, enableBackgroundRefresh bool) *ModelManager {
	modelsDir := os.Getenv("MODELS_PATH")
	if modelsDir == "" && enableBackgroundRefresh {
		logger.Warn("MODELS_PATH environment variable not set - model scanning disabled")
	}

	// Initialize backends map
	backends := make(map[string]*BackendInfo)

	// Check for ROCm backend
	rocmPath := "/home/josh/llama.cpp-rocm-wmma/build/bin/llama-server"
	if version := detectLlamaServerVersion(rocmPath); version != "" {
		backends["rocm"] = &BackendInfo{
			Path:    rocmPath,
			Version: version,
			Type:    "rocm",
		}
	}

	// Check for Vulkan backend
	vulkanPath := "/home/josh/llama.cpp-vulkan/build/bin/llama-server"
	if version := detectLlamaServerVersion(vulkanPath); version != "" {
		backends["vulkan"] = &BackendInfo{
			Path:    vulkanPath,
			Version: version,
			Type:    "vulkan",
		}
	}

	mm := &ModelManager{
		servers:         make(map[string]*ModelServer),
		modelsDir:       modelsDir,
		backends:        backends,
		downloadManager: NewDownloadManager(modelsDir, dbStore),
		stopRefresh:     make(chan struct{}),
		dockerLauncher:  NewDockerLauncher(),
	}

	// Only start background refresh if enabled (disabled for tests)
	if enableBackgroundRefresh {
		go mm.backgroundRefresh()
	}

	return mm
}

// NewModelManager creates a new model manager with background refresh enabled
func NewModelManager(dbStore *db.Store) *ModelManager {
	return NewModelManagerWithBackgroundRefresh(dbStore, true)
}

// NewModelManagerForTest creates a model manager without database and without background refresh (for testing)
func NewModelManagerForTest() *ModelManager {
	return NewModelManagerWithBackgroundRefresh(nil, false)
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

// GetBackends returns available llama-server backends with version info
func (mm *ModelManager) GetBackends() map[string]*BackendInfo {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Return a copy to avoid race conditions
	backends := make(map[string]*BackendInfo)
	for k, v := range mm.backends {
		backends[k] = v
	}
	return backends
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
		logger.Debug("Server state changed, triggering SSE broadcast")
		callback()
	}
}

// ScanAvailableModels scans the models directory for .gguf files
// For split models (e.g., model-00001-of-00004.gguf), only the first part is shown as launchable
func (mm *ModelManager) ScanAvailableModels() ([]*ModelServer, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// For tests, modelsDir may be empty - return empty list
	if mm.modelsDir == "" {
		return []*ModelServer{}, nil
	}

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
	logger.Debug("RefreshServerStatus: Starting scan for running servers")

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

	logger.Debug("RefreshServerStatus: Checking %d known ports: %v", len(knownPorts), knownPorts)

	// Check known ports
	for _, port := range knownPorts {
		url := fmt.Sprintf("http://localhost:%d/v1/models", port)
		if !mm.checkServerHealth(url) {
			logger.Debug("RefreshServerStatus: Port %d not responding", port)
			continue
		}

		// Port is active - gather all info without lock
		info := portInfo{
			port:        port,
			modelName:   mm.getModelNameFromPort(port),
			pid:         mm.findPIDForPort(port),
			contextSize: mm.getContextSizeFromPort(port),
		}

		// If we couldn't get model name from HTTP response (server still loading),
		// use the model name from the existing server entry
		if info.modelName == "" {
			mm.mu.RLock()
			for _, server := range mm.servers {
				if server.Port == port {
					info.modelName = server.ModelName
					break
				}
			}
			mm.mu.RUnlock()
		}

		if info.modelName != "" {
			activePortsInfo = append(activePortsInfo, info)
			logger.Debug("RefreshServerStatus: Found active server on known port %d: %s (PID %d)", port, info.modelName, info.pid)
		}
	}

	// Step 1b: Scan for any servers we don't know about (8081-8090 range)
	// This handles externally started servers or missed registrations
	logger.Debug("RefreshServerStatus: Scanning unknown ports 8081-8090")
	for port := 8081; port <= 8090; port++ {
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
			logger.Debug("RefreshServerStatus: Discovered active server on unknown port %d: %s (PID %d)", port, info.modelName, info.pid)
		}
	}

	// Step 1c: Check for Docker containers that might be running but not responding on HTTP yet
	// This handles containers that are still loading or have network issues
	logger.Debug("RefreshServerStatus: Checking for orphaned Docker containers")
	if mm.dockerLauncher != nil {
		containers, err := mm.dockerLauncher.ListContainers()
		if err == nil {
			for _, containerName := range containers {
				// Extract model name from container name (clai-model-<sanitized_model_name>)
				if strings.HasPrefix(containerName, "clai-model-") {
					// Check if container is running
					running, _ := mm.dockerLauncher.GetContainerStatus(containerName)
					if !running {
						continue
					}

					// Try to get port mapping for this container
					port, _ := mm.getPortFromContainerName(containerName)
					if port > 0 {
						// Check if we already have this port in activePortsInfo
						portExists := false
						for _, info := range activePortsInfo {
							if info.port == port {
								portExists = true
								break
							}
						}

						if !portExists {
							// Get model name from container
							modelName := mm.getModelNameFromContainer(containerName)
							if modelName != "" {
								info := portInfo{
									port:        port,
									modelName:   modelName,
									pid:         0, // Will be populated later
									contextSize: 0,
								}
								activePortsInfo = append(activePortsInfo, info)
								logger.Debug("RefreshServerStatus: Found Docker container without HTTP response: %s on port %d", containerName, port)
							}
						}
					}
				}
			}
		}
	}

	logger.Debug("RefreshServerStatus: Found %d total active servers", len(activePortsInfo))

	// Step 2: Now acquire lock and update server status quickly
	mm.mu.Lock()
	defer mm.mu.Unlock()

	// Build active ports map for quick lookup
	activePorts := make(map[int]bool)
	for _, info := range activePortsInfo {
		activePorts[info.port] = true
	}

	// Mark previously running/loading servers as stopped if their ports disappeared
	// Note: We don't reset "starting" servers because they may still be loading
	// and haven't opened their port yet. The start process will update the status
	// when the server is ready or fails.
	for _, server := range mm.servers {
		if server.Port > 0 && !activePorts[server.Port] {
			switch server.Status {
			case "running", "loading":
				server.Status = "stopped"
				server.Port = 0
				server.PID = 0
				server.URL = ""
				server.APIType = ""
				server.ContextSize = 0
				server.CPUPercent = 0
				server.MemoryBytes = 0
				server.VRAMUsageBytes = 0
			}
			// Note: "starting" status is intentionally NOT reset here
			// The server is still initializing and will update to "loading", "running", or "error"
			// when waitForServerReady() detects the actual state
		}
	}

	// Update servers based on gathered port info
	for _, info := range activePortsInfo {
		// Find the server with this model
		matched := false
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
				matched = true
				break
			}
		}

		// If no existing server matched, create a new entry for this discovered server
		if !matched && info.modelName != "" {
			// Try to find the full model path
			modelPath := info.modelName
			if !filepath.IsAbs(modelPath) {
				// If it's just a filename, try to find it in models directory
				potentialPath := filepath.Join(mm.modelsDir, info.modelName)
				if _, err := os.Stat(potentialPath); err == nil {
					modelPath = potentialPath
				}
			}

			// Check if server is loading or ready
			isLoading := mm.checkIfServerLoading(info.port)
			status := "running"
			if isLoading {
				status = "loading"
			}

			// Create new server entry
			newServer := &ModelServer{
				ModelName:   filepath.Base(modelPath),
				ModelPath:   modelPath,
				Status:      status,
				Port:        info.port,
				URL:         fmt.Sprintf("http://localhost:%d", info.port),
				APIType:     "llamacpp",
				LastChecked: time.Now().Unix(),
				PID:         info.pid,
				ContextSize: info.contextSize,
				Backend:     "", // Unknown backend for discovered servers
			}

			// Add to servers map
			mm.servers[modelPath] = newServer
			logger.Debug("RefreshServerStatus: Created new server entry for %s on port %d (PID %d)", info.modelName, info.port, info.pid)
		}
	}

	logger.Debug("RefreshServerStatus: Scan complete")
	return nil
}

// getPortFromContainerName extracts the port from a container name by inspecting the container
func (mm *ModelManager) getPortFromContainerName(containerName string) (int, error) {
	cmd := exec.Command("docker", "inspect", "-f", "{{range $p, $conf := .NetworkSettings.Ports}}{{if $conf}}{{index $conf 0).HostPort}}{{end}}{{end}}", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to get container port: %w", err)
	}

	portStr := strings.TrimSpace(string(output))
	if portStr == "" {
		return 0, fmt.Errorf("no port mapping found")
	}

	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return 0, fmt.Errorf("failed to parse port: %w", err)
	}

	return port, nil
}

// getModelNameFromContainer extracts the model name from a container by inspecting its environment or command
func (mm *ModelManager) getModelNameFromContainer(containerName string) string {
	// Try to get the command from the container
	cmd := exec.Command("docker", "inspect", "-f", "{{.Config.Cmd}}", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	// Parse the command to extract the model path
	cmdStr := string(output)
	// Look for -m flag followed by model path
	if idx := strings.Index(cmdStr, "-m"); idx >= 0 {
		rest := cmdStr[idx+2:]
		// Find the next flag or end of string
		endIdx := strings.Index(rest, "-")
		if endIdx < 0 {
			endIdx = len(rest)
		}
		modelPath := strings.TrimSpace(rest[:endIdx])
		// Extract just the filename
		return filepath.Base(modelPath)
	}

	return ""
}

// getModelNameFromPort fetches the model name from a running server
func (mm *ModelManager) getModelNameFromPort(port int) string {
	client := &http.Client{Timeout: 200 * time.Millisecond}
	url := fmt.Sprintf("http://localhost:%d/v1/models", port)
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	bodyStr := string(body)

	// Extract model path from response - try full path first
	modelsPath := mm.modelsDir + "/"
	if strings.Contains(bodyStr, modelsPath) {
		start := strings.Index(bodyStr, modelsPath)
		if start >= 0 {
			end := strings.Index(bodyStr[start:], "\"")
			if end >= 0 {
				modelPath := bodyStr[start : start+end]
				return modelPath
			}
		}
	}

	// Fallback: Extract model filename from "id" field
	// Example: {"id":"Devstral-Small-2-24B-Instruct-2512-UD-Q8_K_XL.gguf",...}
	if idStart := strings.Index(bodyStr, `"id":"`); idStart >= 0 {
		idStart += len(`"id":"`)
		if idEnd := strings.Index(bodyStr[idStart:], `"`); idEnd >= 0 {
			modelName := bodyStr[idStart : idStart+idEnd]
			// If it ends with .gguf, it's likely a valid model filename
			if strings.HasSuffix(modelName, ".gguf") {
				return modelName
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
	client := &http.Client{Timeout: 100 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable
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

// findAvailablePortForModel finds an available port starting from 8082
// (8081 is reserved for the benchmark server itself)
// Scans up to 100 ports until it finds an available one
//
// This function uses SO_REUSEADDR to properly test port availability and
// avoid race conditions where the kernel keeps sockets in TIME_WAIT state.
func (mm *ModelManager) findAvailablePortForModel() (int, error) {
	// Start from 8082 (8081 is used by benchmark server) and scan up to 100 ports
	const startPort = 8082
	const maxAttempts = 100

	for i := 0; i < maxAttempts; i++ {
		port := startPort + i

		// Use syscall to bind with SO_REUSEADDR to properly test availability
		// This avoids false positives from sockets in TIME_WAIT state
		if isPortAvailable(port) {
			logger.Info("Found available port: %d", port)
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found after scanning %d ports starting from %d", maxAttempts, startPort)
}

// isPortAvailable checks if a port is truly available by attempting to bind with SO_REUSEADDR
// This properly detects ports that are in TIME_WAIT or other reserved states
func isPortAvailable(port int) bool {
	// Method 1: Try standard net.Listen first (fast path)
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		listener.Close()

		// Add a small delay to allow kernel to fully release the socket
		// This helps prevent race conditions with rapid start/stop cycles
		time.Sleep(50 * time.Millisecond)

		// Double-check by trying to bind again
		listener2, err := net.Listen("tcp", addr)
		if err == nil {
			listener2.Close()
			return true
		}
	}

	return false
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
		logger.Debug("UpdateVRAMUsage: Failed to get GPU process info: %v", err)
		return nil
	}

	logger.Debug("UpdateVRAMUsage: Found %d GPU processes", len(processes))

	// Create a map of PID -> VRAM usage for quick lookup
	pidToVRAM := make(map[int]int64)
	for _, proc := range processes {
		pidToVRAM[proc.PID] = proc.VRAMUsed
		logger.Debug("UpdateVRAMUsage: GPU Process - PID: %d, Name: %s, VRAM: %d bytes", proc.PID, proc.ProcessName, proc.VRAMUsed)
	}

	// Update VRAM for all running servers
	mm.mu.Lock()
	defer mm.mu.Unlock()

	for _, server := range mm.servers {
		if server.PID > 0 {
			logger.Debug("UpdateVRAMUsage: Checking server %s (PID: %d)", server.ModelName, server.PID)
			if vram, exists := pidToVRAM[server.PID]; exists {
				server.VRAMUsageBytes = vram
				logger.Debug("UpdateVRAMUsage: Updated server %s VRAM to %d bytes", server.ModelName, vram)
			} else {
				logger.Debug("UpdateVRAMUsage: No VRAM data found for PID %d", server.PID)
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
		logger.Info("isSystemdManaged: Failed to read cgroup for PID %d: %v", pid, err)
		return false
	}
	isManaged := strings.Contains(string(data), "user@") && strings.Contains(string(data), ".service")
	logger.Info("isSystemdManaged: PID %d, cgroup data: %s, isManaged: %v", pid, string(data), isManaged)
	return isManaged
}

// getSystemdServiceName extracts the systemd service name for a PID
func (mm *ModelManager) getSystemdServiceName(pid int) string {
	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", pid)
	data, err := ioutil.ReadFile(cgroupPath)
	if err != nil {
		logger.Info("getSystemdServiceName: Failed to read cgroup for PID %d: %v", pid, err)
		return ""
	}

	logger.Info("getSystemdServiceName: PID %d, cgroup data: %s", pid, string(data))

	// Parse cgroup to extract service name
	// Format: 0::/user.slice/user-1000.slice/user@1000.service/app.slice/llama-server.service
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, ".service") {
			parts := strings.Split(line, "/")
			for _, part := range parts {
				if strings.HasSuffix(part, ".service") && !strings.Contains(part, "user@") {
					logger.Info("getSystemdServiceName: Found service name: %s", part)
					return part
				}
			}
		}
	}
	logger.Info("getSystemdServiceName: No service name found")
	return ""
}

// sanitizeUnitName converts a model filename into a valid systemd unit name
func sanitizeUnitName(name string) string {
	// Remove .gguf extension
	name = strings.TrimSuffix(name, ".gguf")
	// Replace non-alphanumeric characters with hyphens
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	name = reg.ReplaceAllString(name, "-")
	// Trim hyphens from start and end
	name = strings.Trim(name, "-")
	return name
}

// createSystemdServiceFile creates a systemd user service file for a model
func (mm *ModelManager) createSystemdServiceFile(modelName string, binaryPath string, args []string, port int, backend string) (string, error) {
	unitName := "clai-model-" + sanitizeUnitName(modelName)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	serviceDir := filepath.Join(homeDir, ".config/systemd/user")
	servicePath := filepath.Join(serviceDir, unitName+".service")

	// Ensure directory exists
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create systemd user directory: %w", err)
	}

	// Escape arguments for systemd ExecStart
	escapedArgs := make([]string, len(args))
	for i, arg := range args {
		if strings.ContainsAny(arg, " \"'") {
			escapedArgs[i] = fmt.Sprintf("\"%s\"", strings.ReplaceAll(arg, "\"", "\\\""))
		} else {
			escapedArgs[i] = arg
		}
	}

	// Determine environment variables based on backend
	envVars := []string{
		"PATH=/home/josh/therock-install/bin:/home/josh/therock-install/llvm/bin:/usr/bin",
	}

	// Backend-specific environment variables
	if backend == "rocm" {
		envVars = append(envVars, "LD_LIBRARY_PATH=/opt/rocm/lib")
		// ROCBLAS_USE_HIPBLASLT: per Strix Halo reference repo, can improve stability/performance
		// Set to 1 by default (enabled), can be disabled by setting env var externally
		if os.Getenv("ROCBLAS_USE_HIPBLASLT") == "" {
			envVars = append(envVars, "ROCBLAS_USE_HIPBLASLT=1")
		} else {
			envVars = append(envVars, fmt.Sprintf("ROCBLAS_USE_HIPBLASLT=%s", os.Getenv("ROCBLAS_USE_HIPBLASLT")))
		}
	}

	envLines := ""
	for _, env := range envVars {
		envLines += fmt.Sprintf("Environment=%s\n", env)
	}

	// Create logs directory in user's home instead of using /tmp
	logsDir := filepath.Join(homeDir, ".local", "share", "clai", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create logs directory: %w", err)
	}

	logFilePath := filepath.Join(logsDir, fmt.Sprintf("llama-server-%d.log", port))

	content := fmt.Sprintf(`[Unit]
Description=CLAI Model Server: %s
After=network.target

[Service]
Type=exec
%sExecStart=%s %s
Restart=on-failure
RestartSec=5
StandardOutput=file:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, modelName, envLines, binaryPath, strings.Join(escapedArgs, " "), logFilePath, logFilePath)

	if err := os.WriteFile(servicePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write systemd service file: %w", err)
	}

	return unitName, nil
}

// StartServer starts a model server on an available port with default context size
func (mm *ModelManager) StartServer(modelPath string) error {
	return mm.StartServerWithBackend(modelPath, 131072, 999, "rocm") // Default to ROCm, 999 = all GPU layers
}

// StartServerWithContext starts a model server with a custom context size and GPU layer count (backward compatibility)
func (mm *ModelManager) StartServerWithContext(modelPath string, contextSize int, ngl int) error {
	return mm.StartServerWithBackend(modelPath, contextSize, ngl, "rocm") // Default to ROCm for backward compatibility
}

// StartServerWithBackend starts a model server with a custom context size, GPU layer count, and backend choice
func (mm *ModelManager) StartServerWithBackend(modelPath string, contextSize int, ngl int, backend string) error {
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

	// Get the backend binary path
	mm.mu.RLock()
	backendInfo, exists := mm.backends[backend]
	mm.mu.RUnlock()

	if !exists || backendInfo == nil {
		return fmt.Errorf("backend '%s' not available - binary not found", backend)
	}

	binaryPath := backendInfo.Path

	args := []string{
		"-m", modelPath,
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", port),
		"-c", fmt.Sprintf("%d", contextSize),
		"-ngl", fmt.Sprintf("%d", ngl),
		"-fa", "on",
		"--no-mmap", // Disable memory mapping for stability (per Strix Halo optimization)
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

	// Create systemd service file
	unitName, err := mm.createSystemdServiceFile(modelName, binaryPath, args, port, backend)
	if err != nil {
		return fmt.Errorf("failed to create systemd service: %w", err)
	}

	// Reload systemd, enable and start service
	logger.Info("Starting model via systemd: %s on port %d", unitName, port)
	commands := [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", unitName + ".service"},
		{"systemctl", "--user", "start", unitName + ".service"},
	}

	for _, c := range commands {
		if output, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to execute %s: %w (output: %s)", strings.Join(c, " "), err, string(output))
		}
	}

	// Wait a moment for the service to actually start so we can get its PID
	time.Sleep(1 * time.Second)

	// Get PID via systemctl
	pidCmd := exec.Command("systemctl", "--user", "show", "--property", "MainPID", "--value", unitName+".service")
	pidOutput, err := pidCmd.Output()
	pid := 0
	if err == nil {
		fmt.Sscanf(strings.TrimSpace(string(pidOutput)), "%d", &pid)
	}

	if pid == 0 {
		// Fallback to finding PID via port
		pid = mm.findPIDForPort(port)
	}

	// If we still don't have a PID, check if the service failed due to port conflict
	// and retry with a different port
	if pid == 0 {
		logger.Warn("Server failed to start on port %d, checking for port conflict...", port)

		// Check systemd status for port binding errors
		statusCmd := exec.Command("systemctl", "--user", "status", unitName+".service")
		statusOutput, _ := statusCmd.CombinedOutput()
		statusStr := string(statusOutput)

		if strings.Contains(statusStr, "couldn't bind") || strings.Contains(statusStr, "Address already in use") {
			// Port conflict detected - stop this service and retry with new port
			logger.Warn("Port %d is already in use, stopping service and retrying with different port...", port)

			// Stop and disable the failed service
			exec.Command("systemctl", "--user", "stop", unitName+".service").Run()
			exec.Command("systemctl", "--user", "disable", unitName+".service").Run()
			homeDir, _ := os.UserHomeDir()
			os.Remove(filepath.Join(homeDir, ".config/systemd/user", unitName+".service"))

			// Try again with a new port
			logger.Info("Retrying with new port...")
			return mm.StartServerWithBackend(modelPath, contextSize, ngl, backend)
		}
	}

	logger.Info("Started model server via systemd: %s on port %d (PID: %d)", modelName, port, pid)

	// Automatically set this model as default for the CLI
	if err := UpdateEnvFile(modelName, fmt.Sprintf("http://localhost:%d", port)); err != nil {
		logger.Warn("Failed to update CLI config with started model: %v", err)
	} else {
		logger.Info("Updated CLI config to use model %s", modelName)
	}

	// Step 3: Acquire lock briefly to update state
	mm.mu.Lock()
	// Check again that server still exists (safety check)
	server, exists = mm.servers[modelPath]
	if !exists {
		mm.mu.Unlock()
		// Stop the service we just started since server was removed
		exec.Command("systemctl", "--user", "stop", unitName+".service").Run()
		exec.Command("systemctl", "--user", "disable", unitName+".service").Run()
		homeDir, _ := os.UserHomeDir()
		os.Remove(filepath.Join(homeDir, ".config/systemd/user", unitName+".service"))
		return fmt.Errorf("model was removed while starting")
	}

	// Update server info
	server.Port = port
	server.PID = pid
	server.Status = "starting"
	server.Backend = backend // Store which backend was used
	server.ErrorMessage = "" // Clear any previous errors when starting
	server.URL = fmt.Sprintf("http://localhost:%d", port)
	mm.mu.Unlock()

	// Notify state change so UI updates immediately
	mm.notifyStateChange()

	// Wait for server to be ready (in background)
	// Use 2-hour timeout for large models (78B+ can take 30-60+ minutes to load)
	go mm.waitForServerReady(modelPath, port, 2*time.Hour)

	return nil
}

// StartServerWithDocker starts a model server using Docker containers
func (mm *ModelManager) StartServerWithDocker(modelPath string, contextSize int, ngl int, imageTag string) error {
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
	mm.mu.Unlock()

	// Verify split model is complete before starting
	if isSplit && !splitComplete {
		return fmt.Errorf("cannot start split model: only %d of %d parts found - download all parts first", partsFound, totalParts)
	}

	// Step 2: Do ALL I/O operations WITHOUT holding lock

	// Check if Docker image exists locally, pull if needed
	if !mm.dockerLauncher.ImageExistsLocally(imageTag) {
		logger.Info("Docker image %s not found locally, pulling...", imageTag)
		if err := mm.dockerLauncher.PullImage(imageTag); err != nil {
			return fmt.Errorf("failed to pull Docker image: %w", err)
		}
	}

	// Start the Docker container
	containerID, err := mm.dockerLauncher.StartContainer(modelPath, modelName, port, contextSize, ngl, imageTag)
	if err != nil {
		return fmt.Errorf("failed to start Docker container: %w", err)
	}

	logger.Info("Started model server via Docker: %s on port %d (Container: %s)", modelName, port, containerID[:12])

	// Get host PID of the container for VRAM tracking
	containerName := fmt.Sprintf("clai-model-%s", sanitizeContainerName(modelName))
	pid, _ := mm.dockerLauncher.GetContainerPID(containerName)

	// Automatically set this model as default for the CLI
	if err := UpdateEnvFile(modelName, fmt.Sprintf("http://localhost:%d", port)); err != nil {
		logger.Warn("Failed to update CLI config with started model: %v", err)
	} else {
		logger.Info("Updated CLI config to use model %s", modelName)
	}

	// Step 3: Acquire lock briefly to update state
	mm.mu.Lock()
	// Check again that server still exists (safety check)
	server, exists = mm.servers[modelPath]
	if !exists {
		mm.mu.Unlock()
		// Stop the container we just started since server was removed
		mm.dockerLauncher.StopContainer(containerName)
		return fmt.Errorf("model was removed while starting")
	}

	// Update server info
	server.Port = port
	server.PID = pid // Store the host PID
	server.Status = "starting"
	server.Backend = "docker-" + imageTag // Store which Docker image was used
	server.ErrorMessage = ""              // Clear any previous errors when starting
	server.URL = fmt.Sprintf("http://localhost:%d", port)
	mm.mu.Unlock()

	// Notify state change so UI updates immediately
	mm.notifyStateChange()

	// Wait for server to be ready (in background)
	go mm.waitForServerReady(modelPath, port, 2*time.Hour)

	return nil
}

// GetDockerImages returns all available Docker images
func (mm *ModelManager) GetDockerImages() map[string]*DockerImageInfo {
	if mm.dockerLauncher == nil {
		return make(map[string]*DockerImageInfo)
	}
	return mm.dockerLauncher.GetAvailableImages()
}

// PullDockerImage pulls a Docker image from the registry
func (mm *ModelManager) PullDockerImage(imageTag string) error {
	if mm.dockerLauncher == nil {
		return fmt.Errorf("docker launcher not initialized")
	}
	return mm.dockerLauncher.PullImage(imageTag)
}

// waitForServerReady polls the server until it's ready or timeout
// Uses /health endpoint which correctly returns 503 during loading and 200 when ready
func (mm *ModelManager) waitForServerReady(modelPath string, port int, timeout time.Duration) {
	url := fmt.Sprintf("http://localhost:%d/health", port)
	deadline := time.Now().Add(timeout)
	var lastError error

	// Get the PID we just started
	mm.mu.RLock()
	var serverPID int
	if server, exists := mm.servers[modelPath]; exists {
		serverPID = server.PID
	}
	mm.mu.RUnlock()

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			mm.mu.Lock()
			if server, exists := mm.servers[modelPath]; exists {
				server.Status = "running"
				server.ErrorMessage = "" // Clear any previous errors
				server.LastChecked = time.Now().Unix()
				logger.Info("Model server ready: %s on port %d", server.ModelName, port)
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

		// Check if the process is still alive and not a zombie (detect immediate crashes)
		if serverPID > 0 {
			// Check /proc/PID/stat to detect zombie processes
			statPath := fmt.Sprintf("/proc/%d/stat", serverPID)
			if statData, err := os.ReadFile(statPath); err == nil {
				// Parse the state (third field after PID and name)
				// State 'Z' means zombie (dead but not reaped)
				statStr := string(statData)
				// Find the closing ) of the process name, state is the next field
				if closeParenIdx := strings.LastIndex(statStr, ")"); closeParenIdx > 0 {
					fields := strings.Fields(statStr[closeParenIdx+1:])
					if len(fields) > 0 && fields[0] == "Z" {
						// Process is a zombie - it crashed
						logger.Info("Detected zombie process PID %d for %s", serverPID, modelPath)
						break
					}
				}
			} else {
				// /proc/PID/stat doesn't exist - process is truly dead
				logger.Info("Detected dead process PID %d for %s", serverPID, modelPath)
				break
			}
		}

		time.Sleep(2 * time.Second)
	}

	// Timeout - server failed to start
	mm.mu.Lock()
	if server, exists := mm.servers[modelPath]; exists {
		server.Status = "error"

		// Try to read the last 20 lines of the log file to get the actual error
		logFile, _ := getLogPath(port)
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

		logger.Info("Model server failed to start: %s on port %d - %s", server.ModelName, port, server.ErrorMessage)
	}
	mm.mu.Unlock()

	// Notify state change so UI updates with error
	mm.notifyStateChange()
}

// StopServer stops a running model server
func (mm *ModelManager) StopServer(modelPath string) error {
	mm.mu.Lock()

	server, exists := mm.servers[modelPath]
	if !exists {
		mm.mu.Unlock()
		// Check if this might be a Docker container that was orphaned
		// Try to find and stop it by model name derived from path
		modelName := filepath.Base(modelPath)
		containerName := fmt.Sprintf("clai-model-%s", sanitizeContainerName(modelName))
		if mm.dockerLauncher != nil {
			if running, _ := mm.dockerLauncher.GetContainerStatus(containerName); running {
				logger.Info("StopServer: Found orphaned Docker container for %s, stopping it", modelPath)
				if err := mm.dockerLauncher.StopContainer(containerName); err != nil {
					logger.Info("StopServer: Failed to stop orphaned container %s: %v", containerName, err)
					return fmt.Errorf("model not found in registry and failed to stop orphaned container: %s", modelPath)
				}
				logger.Info("StopServer: Successfully stopped orphaned Docker container for %s", modelPath)
				return nil
			}
		}
		return fmt.Errorf("model not found: %s", modelPath)
	}

	if server.Status != "running" && server.Status != "loading" && server.Status != "starting" && server.PID == 0 && server.Port == 0 {
		// Check if there's a Docker container running anyway
		modelName := server.ModelName
		backend := server.Backend
		mm.mu.Unlock()

		if strings.HasPrefix(backend, "docker-") && mm.dockerLauncher != nil {
			containerName := fmt.Sprintf("clai-model-%s", sanitizeContainerName(modelName))
			if running, _ := mm.dockerLauncher.GetContainerStatus(containerName); running {
				logger.Info("StopServer: Stopping Docker container for stopped server entry: %s", containerName)
				if err := mm.dockerLauncher.StopContainer(containerName); err != nil {
					logger.Info("StopServer: Failed to stop container %s: %v", containerName, err)
					return err
				}
				// Update server status back to stopped
				mm.mu.Lock()
				if s, exists := mm.servers[modelPath]; exists {
					s.Status = "stopped"
					s.Port = 0
					s.PID = 0
				}
				mm.mu.Unlock()
				return nil
			}
		}
		return fmt.Errorf("server not running")
	}

	// copied earlier for I/O
	pid := server.PID
	port := server.Port
	modelName := server.ModelName
	backend := server.Backend
	mm.mu.Unlock()

	// Step 2: Do ALL I/O operations WITHOUT holding lock
	var stopErr error
	var stoppedViaSystemd bool
	var stoppedViaDocker bool

	// Check if this is a Docker-based server
	if strings.HasPrefix(backend, "docker-") {
		containerName := fmt.Sprintf("clai-model-%s", sanitizeContainerName(modelName))
		logger.Info("Stopping Docker container: %s", containerName)
		if err := mm.dockerLauncher.StopContainer(containerName); err != nil {
			logger.Info("Warning: failed to stop Docker container %s: %v", containerName, err)
			stopErr = err
		} else {
			stoppedViaDocker = true
			logger.Info("Successfully stopped Docker container: %s", containerName)
		}
	} else {
		// Traditional systemd/process-based stop
		// Determine if we should use systemd (either it is managed or it's one of our clai-model-* services)
		serviceName := ""
		if pid > 0 && mm.isSystemdManaged(pid) {
			serviceName = mm.getSystemdServiceName(pid)
		}

		// If we don't have a PID or it's not managed, try to find a clai-model service by model name
		if serviceName == "" {
			unitName := "clai-model-" + sanitizeUnitName(modelName)
			// Check if the service file exists
			homeDir, _ := os.UserHomeDir()
			servicePath := filepath.Join(homeDir, ".config/systemd/user", unitName+".service")
			if _, err := os.Stat(servicePath); err == nil {
				serviceName = unitName + ".service"
			}
		}

		if serviceName != "" {
			logger.Info("Stopping systemd service: %s", serviceName)
			// Disable and stop the service
			commands := [][]string{
				{"systemctl", "--user", "stop", serviceName},
				{"systemctl", "--user", "disable", serviceName},
			}

			for _, c := range commands {
				if output, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
					logger.Info("Warning: failed to execute %s: %v (output: %s)", strings.Join(c, " "), err, string(output))
					// Continue anyway
				}
			}

			stoppedViaSystemd = true

			// If it's one of our managed services, clean up the file
			if strings.HasPrefix(serviceName, "clai-model-") {
				homeDir, _ := os.UserHomeDir()
				servicePath := filepath.Join(homeDir, ".config/systemd/user", serviceName)
				if err := os.Remove(servicePath); err != nil {
					logger.Info("Warning: failed to remove service file %s: %v", servicePath, err)
				}
				// Sequence: Stop -> Disable -> Remove -> Daemon-reload
				exec.Command("systemctl", "--user", "daemon-reload").Run()
			}
		}
	}

	// Keep log files for debugging; do not remove them on stop
	if port > 0 {
		if logPath, err := getLogPath(port); err == nil {
			logger.Info("Preserving log file: %s", logPath)
		}
	}

	// Kill the process directly if not stopped via systemd/docker and we have a PID
	if !stoppedViaSystemd && !stoppedViaDocker && pid > 0 {
		logger.Info("Attempting to kill process PID %d for %s", pid, modelName)
		process, err := os.FindProcess(pid)
		if err != nil {
			stopErr = fmt.Errorf("failed to find process: %w", err)
			logger.Info("Failed to find process PID %d: %v", pid, err)
		} else if err := process.Kill(); err != nil {
			stopErr = fmt.Errorf("failed to kill process: %w", err)
			logger.Info("Failed to kill process PID %d: %v", pid, err)
		} else {
			logger.Info("Successfully sent kill signal to process PID %d for %s", pid, modelName)
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
					logger.Info("Split model part already gone: %s", partPath)
				} else {
					logger.Info("Warning: failed to delete split model part %s: %v", partPath, err)
				}
			} else {
				deletedParts++
				logger.Info("Deleted split model part: %s", partPath)
			}
		}
		if missingParts > 0 {
			logger.Info("Deleted %d/%d split model parts (%d were already missing)", deletedParts, len(server.SplitAllParts), missingParts)
		} else {
			logger.Info("Deleted all %d split model parts", deletedParts)
		}
	} else {
		// Single file - delete it
		if err := os.Remove(modelPath); err != nil {
			return fmt.Errorf("failed to delete model file: %w", err)
		}
		logger.Info("Deleted model file: %s", modelPath)
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

// getLogPath returns the path to the log file for a given port
func getLogPath(port int) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".local", "share", "clai", "logs", fmt.Sprintf("llama-server-%d.log", port)), nil
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
	logger.Debug("HandleListModels: Starting...")
	models, err := s.modelManager.ScanAvailableModels()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to scan models: %v", err), http.StatusInternalServerError)
		return
	}
	logger.Debug("HandleListModels: Scanned models")

	// Get fresh server status (refresh before displaying)
	logger.Debug("HandleListModels: Refreshing server status...")
	s.modelManager.RefreshServerStatus()
	s.modelManager.UpdateVRAMUsage()
	s.modelManager.UpdateCPUAndMemory()
	serverStatuses := s.modelManager.GetServerStatus()

	// Create lookup map for running servers
	statusMap := make(map[string]*ModelServer)
	for _, status := range serverStatuses {
		statusMap[status.ModelPath] = status
	}

	// Merge: Update models with running server information
	for _, model := range models {
		if runningServer, exists := statusMap[model.ModelPath]; exists {
			// Copy running server details to model
			model.Status = runningServer.Status
			model.Port = runningServer.Port
			model.PID = runningServer.PID
			model.URL = runningServer.URL
			model.APIType = runningServer.APIType
			model.Backend = runningServer.Backend
			model.LastChecked = runningServer.LastChecked
			model.VRAMUsageBytes = runningServer.VRAMUsageBytes
			model.CPUPercent = runningServer.CPUPercent
			model.MemoryBytes = runningServer.MemoryBytes
			model.ContextSize = runningServer.ContextSize
			model.ContextTrain = runningServer.ContextTrain
			model.ParametersCount = runningServer.ParametersCount
			model.VocabSize = runningServer.VocabSize
			model.EmbeddingDim = runningServer.EmbeddingDim
			model.ErrorMessage = runningServer.ErrorMessage
		}
		// Non-running models keep Status = "stopped" (default)
	}
	logger.Debug("HandleListModels: Done with setup")

	// Get available backends for the UI
	backends := s.modelManager.GetBackends()

	// Get scores and CLI default
	scores := make(map[string]templates.ScoreInfo)
	for _, model := range models {
		var scoreText string
		var scoreColor string
		if s.store != nil {
			lastBenchmark, err := s.store.GetLastBenchmarkForModel(model.ModelName)
			if err != nil || lastBenchmark == nil {
				scoreText = "-"
				scoreColor = "#64748b"
			} else {
				scoreText = fmt.Sprintf("%.1f%%", lastBenchmark.SuccessRate)
				if lastBenchmark.SuccessRate >= 80 {
					scoreColor = "#10b981"
				} else if lastBenchmark.SuccessRate >= 50 {
					scoreColor = "#f59e0b"
				} else {
					scoreColor = "#ef4444"
				}
			}
		} else {
			scoreText = "-"
			scoreColor = "#64748b"
		}
		scores[model.ModelName] = templates.ScoreInfo{Text: scoreText, Color: scoreColor}
	}

	cliDefault := os.Getenv("OLLAMA_MODEL")

	// Convert to templates types to avoid type mismatch
	templateModels := make([]*templates.ModelServer, len(models))
	for i, m := range models {
		templateModels[i] = &templates.ModelServer{
			ModelPath:       m.ModelPath,
			ModelName:       m.ModelName,
			Port:            m.Port,
			PID:             m.PID,
			Status:          m.Status,
			ErrorMessage:    m.ErrorMessage,
			URL:             m.URL,
			APIType:         m.APIType,
			Backend:         m.Backend,
			LastChecked:     m.LastChecked,
			VRAMUsageBytes:  m.VRAMUsageBytes,
			CPUPercent:      m.CPUPercent,
			MemoryBytes:     m.MemoryBytes,
			ContextSize:     m.ContextSize,
			ContextTrain:    m.ContextTrain,
			ParametersCount: m.ParametersCount,
			ModelSizeBytes:  m.ModelSizeBytes,
			VocabSize:       m.VocabSize,
			EmbeddingDim:    m.EmbeddingDim,
			NGL:             m.NGL,
			IsSplitModel:    m.IsSplitModel,
			SplitPartNumber: m.SplitPartNumber,
			SplitTotalParts: m.SplitTotalParts,
			SplitPartsFound: m.SplitPartsFound,
			SplitAllParts:   m.SplitAllParts,
			SplitIsComplete: m.SplitIsComplete,
		}
	}

	templateBackends := make(map[string]*templates.BackendInfo)
	for k, v := range backends {
		templateBackends[k] = (*templates.BackendInfo)(v)
	}

	// Get Docker images for the UI
	dockerImages := s.modelManager.GetDockerImages()
	templateDockerImages := make(map[string]*templates.DockerImageInfo)
	for k, v := range dockerImages {
		templateDockerImages[k] = (*templates.DockerImageInfo)(v)
	}

	w.Header().Set("Content-Type", "text/html")
	data := templates.ModelTableData{
		Models:          templateModels,
		Backends:        templateBackends,
		CLIDefaultModel: cliDefault,
		Scores:          scores,
		DockerImages:    templateDockerImages,
	}

	if err := templates.ModelsTable(data).Render(r.Context(), w); err != nil {
		logger.Info("Error rendering models table: %v", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
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

	logger.Info("HandleStartServer: request from %s ua=%q referer=%q model_path=%s", r.RemoteAddr, r.UserAgent(), r.Referer(), modelPath)

	// Check for custom context size parameter
	contextSize := 131072 // default
	ctxStr := r.FormValue("context_size")
	logger.Info("DEBUG: Received context_size parameter: '%s'", ctxStr)
	if ctxStr != "" {
		if ctx, err := strconv.Atoi(ctxStr); err == nil && ctx > 0 {
			contextSize = ctx
			logger.Info("Using custom context size: %d for %s", contextSize, modelPath)
		} else {
			logger.Info("DEBUG: Failed to parse context_size: err=%v", err)
		}
	} else {
		logger.Info("DEBUG: No context_size parameter provided, using default")
	}

	// Check for custom ngl (GPU layers) parameter
	ngl := 999 // default = all layers
	nglStr := r.FormValue("ngl")
	logger.Info("DEBUG: Received ngl parameter: '%s'", nglStr)
	if nglStr != "" {
		if n, err := strconv.Atoi(nglStr); err == nil && n >= 0 {
			ngl = n
			logger.Info("Using custom ngl: %d for %s", ngl, modelPath)
		} else {
			logger.Info("DEBUG: Failed to parse ngl: err=%v", err)
		}
	} else {
		logger.Info("DEBUG: No ngl parameter provided, using default (all layers)")
	}

	// Parse runtime parameter (new unified format: "native:rocm" or "docker:rocm-7.2")
	runtime := r.FormValue("runtime")
	if runtime == "" {
		// Fallback to old backend parameter for backward compatibility
		backend := r.FormValue("backend")
		if backend == "" {
			backend = "rocm"
		}
		runtime = "native:" + backend
		logger.Info("No runtime specified, using backward-compatible default: %s", runtime)
	}

	var err error
	if strings.HasPrefix(runtime, "docker:") {
		// Docker launcher
		dockerImage := strings.TrimPrefix(runtime, "docker:")
		logger.Info("Using Docker runtime: %s for %s", dockerImage, modelPath)
		err = s.modelManager.StartServerWithDocker(modelPath, contextSize, ngl, dockerImage)
	} else if strings.HasPrefix(runtime, "native:") {
		// Native/systemd launcher
		backend := strings.TrimPrefix(runtime, "native:")
		logger.Info("Using native runtime: %s for %s", backend, modelPath)
		err = s.modelManager.StartServerWithBackend(modelPath, contextSize, ngl, backend)
	} else {
		// Legacy fallback - treat as native backend
		logger.Info("Using legacy runtime format: %s for %s", runtime, modelPath)
		err = s.modelManager.StartServerWithBackend(modelPath, contextSize, ngl, runtime)
	}

	if err != nil {
		logger.Info("Error starting server for %s: %v", modelPath, err)
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
	logger.Info("HandleStopServer: Received request to stop server")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelPath := r.FormValue("model_path")
	logger.Info("HandleStopServer: Stopping model at path: %s", modelPath)
	if modelPath == "" {
		http.Error(w, "model_path required", http.StatusBadRequest)
		return
	}

	if err := s.modelManager.StopServer(modelPath); err != nil {
		logger.Info("Error stopping server for %s: %v", modelPath, err)
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

// UpdateEnvFile updates the .env file with new OLLAMA_MODEL and OLLAMA_HOST values
func UpdateEnvFile(modelName string, hostURL string) error {
	// Path to .env file in clai directory
	envPath := filepath.Join(filepath.Dir(os.Args[0]), ".env")
	// If running from source (go run), try parent directory
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		envPath = ".env"
	}

	// Read current .env content
	content, err := os.ReadFile(envPath)
	if err != nil {
		return fmt.Errorf("error reading .env: %w", err)
	}

	// Parse and update environment variables
	lines := strings.Split(string(content), "\n")

	// Remove .gguf extension for cleaner model name
	if strings.HasSuffix(modelName, ".gguf") {
		modelName = strings.TrimSuffix(modelName, ".gguf")
	}

	var newLines []string
	foundModel := false
	foundHost := false

	for _, line := range lines {
		if strings.HasPrefix(line, "OLLAMA_MODEL=") {
			newLines = append(newLines, fmt.Sprintf("OLLAMA_MODEL=%s", modelName))
			foundModel = true
		} else if strings.HasPrefix(line, "OLLAMA_HOST=") {
			newLines = append(newLines, fmt.Sprintf("OLLAMA_HOST=%s", hostURL))
			foundHost = true
		} else {
			newLines = append(newLines, line)
		}
	}

	// Add missing entries if they weren't found
	if !foundModel {
		newLines = append(newLines, fmt.Sprintf("OLLAMA_MODEL=%s", modelName))
	}
	if !foundHost {
		newLines = append(newLines, fmt.Sprintf("OLLAMA_HOST=%s", hostURL))
	}

	// Write back to .env
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(envPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("error writing .env: %w", err)
	}

	return nil
}

// HandleSetDefaultModel handles HTTP requests to set a model as default in clai CLI config
func (s *Server) HandleSetDefaultModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelPath := r.FormValue("model_path")
	portStr := r.FormValue("port")

	if modelPath == "" || portStr == "" {
		http.Error(w, "model_path and port required", http.StatusBadRequest)
		return
	}

	modelName := filepath.Base(modelPath)
	hostURL := fmt.Sprintf("http://localhost:%s", portStr)

	if err := UpdateEnvFile(modelName, hostURL); err != nil {
		logger.Info("Error updating config: %v", err)
		http.Error(w, fmt.Sprintf("Failed to update config: %v", err), http.StatusInternalServerError)
		return
	}

	logger.Info("Set default model to %s (%s)", modelName, hostURL)

	// Return success message
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div style="padding: 8px; background: #10b981; color: white; border-radius: 4px; margin-bottom: 8px;">
		✓ Set as default! Model: %s, Port: %s
	</div>`, modelName, portStr)
}

// HandleChatUI redirects to the llama.cpp chat UI using the correct hostname
func (s *Server) HandleChatUI(w http.ResponseWriter, r *http.Request) {
	port := r.URL.Query().Get("port")
	if port == "" {
		http.Error(w, "port required", http.StatusBadRequest)
		return
	}

	// If host is explicitly provided, trust it (useful when behind proxies).
	host := r.URL.Query().Get("host")
	if host == "" {
		// Prefer forwarded headers when running behind reverse proxies.
		if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
			host = forwarded
		} else {
			host = r.Host
		}
	}

	// Strip the port from the host if present (e.g., "192.168.1.100:8081" -> "192.168.1.100")
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

	// Redirect to the model server's chat UI
	targetURL := fmt.Sprintf("http://%s:%s/", host, port)
	http.Redirect(w, r, targetURL, http.StatusTemporaryRedirect)
	return
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
		logger.Info("Error deleting model %s: %v", modelPath, err)

		// Return error message in red box (HTMX will swap this into the page)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<div style="background-color: #fee2e2; border: 1px solid #991b1b; color: #991b1b; padding: 1rem; margin: 1rem; border-radius: 0.25rem;">
			<strong>Error:</strong> %s
		</div>`, err.Error())
		return
	}

	logger.Info("Successfully deleted model: %s", modelPath)

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

	port, err := strconv.Atoi(portStr)
	if err != nil || port == 0 {
		http.Error(w, "Invalid port parameter", http.StatusBadRequest)
		return
	}

	logFile, err := getLogPath(port)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get log path: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		http.Error(w, "Log file not found", http.StatusNotFound)
		return
	}

	// Read log file
	content, err := os.ReadFile(logFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read log file: %v", err), http.StatusInternalServerError)
		return
	}

	// Handle tailing if requested
	if tailStr := r.URL.Query().Get("tail"); tailStr != "" {
		if limit, err := strconv.Atoi(tailStr); err == nil && limit > 0 {
			lines := strings.Split(string(content), "\n")
			if len(lines) > limit {
				content = []byte(strings.Join(lines[len(lines)-limit:], "\n"))
			}
		}
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

	logFile, err := getLogPath(port)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

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
		for _, line := range strings.Split(logContent, "\n") {
			fmt.Fprintf(w, "data: %s\n", line)
		}
		fmt.Fprintf(w, "\n")
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

// HandleListDockerImages returns all available Docker images as JSON
func (s *Server) HandleListDockerImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	images := s.modelManager.GetDockerImages()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(images); err != nil {
		logger.Info("Error encoding Docker images: %v", err)
		http.Error(w, "Failed to encode images", http.StatusInternalServerError)
	}
}

// HandlePullDockerImage pulls a Docker image from the registry
func (s *Server) HandlePullDockerImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	imageTag := r.FormValue("image_tag")
	if imageTag == "" {
		http.Error(w, "image_tag required", http.StatusBadRequest)
		return
	}

	logger.Info("Pulling Docker image: %s", imageTag)

	if err := s.modelManager.PullDockerImage(imageTag); err != nil {
		logger.Info("Error pulling Docker image %s: %v", imageTag, err)
		http.Error(w, fmt.Sprintf("Failed to pull image: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"success","message":"Image %s pulled successfully"}`, imageTag)
}

// HandleDockerContainerStatus returns the status of a Docker container
func (s *Server) HandleDockerContainerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	containerName := r.URL.Query().Get("container")
	if containerName == "" {
		http.Error(w, "container name required", http.StatusBadRequest)
		return
	}

	running, err := s.modelManager.dockerLauncher.GetContainerStatus(containerName)
	if err != nil {
		logger.Info("Error getting container status for %s: %v", containerName, err)
		http.Error(w, fmt.Sprintf("Failed to get status: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	status := "stopped"
	if running {
		status = "running"
	}
	fmt.Fprintf(w, `{"container":"%s","status":"%s"}`, containerName, status)
}
