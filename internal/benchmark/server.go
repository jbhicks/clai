package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"clai/internal/benchmark/templates"
	"clai/internal/db"
	"clai/internal/gpu"
	"clai/internal/llm"
	"clai/internal/tools"
)

// Server represents the benchmark web server
type Server struct {
	store        *db.Store
	port         int
	modelManager *ModelManager
	sseClients   map[chan string]bool
	sseClientsMu sync.RWMutex
}

// NewServer creates a new benchmark server
func NewServer(store *db.Store) *Server {
	s := &Server{
		store:        store,
		modelManager: NewModelManager(store),
		sseClients:   make(map[chan string]bool),
	}

	// Set up state change callback to only broadcast when state changes
	s.modelManager.SetStateChangeCallback(func() {
		s.broadcastServerUpdate()
	})

	return s
}

// findAvailablePort finds the first available port in the 8080-8089 range
// If preferredPort is provided and available, it will be used
func findAvailablePort(preferredPort int) (int, error) {
	// Try preferred port first
	if preferredPort > 0 {
		addr := fmt.Sprintf(":%d", preferredPort)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return preferredPort, nil
		}
	}

	// Fall back to scanning range
	for port := 8080; port <= 8089; port++ {
		if port == preferredPort {
			continue // Already tried this
		}
		addr := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range 8080-8089")
}

// Start starts the web server on an available port
// If preferredPort is set, it will try to use that port first
func (s *Server) Start() error {
	return s.StartWithPreferredPort(0)
}

// StartWithPreferredPort starts the web server, preferring the given port
func (s *Server) StartWithPreferredPort(preferredPort int) error {
	port, err := findAvailablePort(preferredPort)
	if err != nil {
		return err
	}
	s.port = port

	mux := http.NewServeMux()

	// Static routes
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/models", s.handleModelsPage)
	mux.HandleFunc("/testing", s.handleTestingPage)
	mux.HandleFunc("/run/{id}", s.handleRunDetails)
	mux.HandleFunc("/new", s.handleNewTest)
	mux.HandleFunc("/servers", s.handleServersPage)

	// API routes
	mux.HandleFunc("/api/run", s.handleStartRun)
	mux.HandleFunc("/api/models", s.handleGetModels)
	mux.HandleFunc("/api/models/options", s.handleGetModelOptions)
	mux.HandleFunc("/api/models/search", s.handleSearchHuggingFace)
	mux.HandleFunc("/api/models/info", s.handleGetModelInfo)
	mux.HandleFunc("/api/models/download", s.handleDownloadModel)
	mux.HandleFunc("/api/models/download-group", s.handleDownloadGroup)
	mux.HandleFunc("/api/models/downloads", s.handleGetDownloads)
	mux.HandleFunc("/api/models/downloads/single", s.handleGetSingleDownload)
	mux.HandleFunc("/api/models/downloads/stream", s.handleDownloadsSSE)
	mux.HandleFunc("/api/models/downloads/clear", s.handleClearDownload)
	mux.HandleFunc("/api/models/downloads/clear-all", s.handleClearAllDownloads)
	mux.HandleFunc("/api/models/downloads/resume", s.handleResumeDownload)
	mux.HandleFunc("/api/models/downloads/cleanup", s.handleCleanupDownload)
	mux.HandleFunc("/api/servers/list", s.HandleListModels)
	mux.HandleFunc("/api/servers/start", s.HandleStartServer)
	mux.HandleFunc("/api/servers/stop", s.HandleStopServer)
	mux.HandleFunc("/api/servers/delete", s.HandleDeleteModel)
	mux.HandleFunc("/api/servers/logs", s.HandleServerLogs)
	mux.HandleFunc("/api/test/run", s.HandleRunTest)
	mux.HandleFunc("/api/test/results", s.HandleGetTestResults)
	mux.HandleFunc("/api/test/results/detailed", s.handleGetDetailedTestResults)
	mux.HandleFunc("/api/benchmark/run", s.HandleRunBenchmark)
	mux.HandleFunc("/api/benchmark/results", s.HandleGetBenchmarkResults)
	mux.HandleFunc("/api/benchmark/clear", s.handleClearBenchmarkResults)
	mux.HandleFunc("/api/test/result/detail", s.handleGetTestDetail)
	mux.HandleFunc("/api/gpu/status", s.HandleGPUStatus)
	mux.HandleFunc("/api/servers/events", s.handleServerEvents)
	mux.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting benchmark server on http://localhost%s\n", addr)

	return http.ListenAndServe(addr, mux)
}

// GetPort returns the port the server is running on
func (s *Server) GetPort() int {
	return s.port
}

// handleHealth is a simple health check endpoint for auto-reload detection
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// handleServerEvents streams server status updates via SSE
// Uses Pattern 1: sse-swap - SSE sends content directly, no hx-get needed
func (s *Server) handleServerEvents(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create client channel for event signals
	clientChan := make(chan string, 10)

	// Register client
	s.sseClientsMu.Lock()
	s.sseClients[clientChan] = true
	clientCount := len(s.sseClients)
	s.sseClientsMu.Unlock()

	log.Printf("SSE client connected, total clients: %d", clientCount)

	// Deregister client on disconnect
	defer func() {
		s.sseClientsMu.Lock()
		delete(s.sseClients, clientChan)
		remainingClients := len(s.sseClients)
		close(clientChan)
		s.sseClientsMu.Unlock()
		log.Printf("SSE client disconnected, remaining clients: %d", remainingClients)
	}()

	// Keep connection alive and send updates
	for {
		select {
		case eventName := <-clientChan:
			// Generate HTML content based on event type
			var htmlContent string
			switch eventName {
			case "servers_update":
				htmlContent = s.renderServersListHTML()
			case "benchmark_update":
				htmlContent = s.renderBenchmarkResultsHTML()
			default:
				htmlContent = ""
			}

			// Send SSE event with content to swap directly
			// HTMX expects: event: eventname\ndata: HTML content\n\n
			if htmlContent != "" {
				log.Printf("SSE: Sending %s event (%d bytes)", eventName, len(htmlContent))
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, htmlContent)
			} else {
				log.Printf("SSE: Sending %s event (no content)", eventName)
				fmt.Fprintf(w, "event: %s\ndata: \n\n", eventName)
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			log.Println("SSE: Client context done, closing connection")
			return
		}
	}
}

// renderServersListHTML generates HTML for the servers list
func (s *Server) renderServersListHTML() string {
	servers := s.modelManager.GetServerStatus()
	gpus, _ := gpu.GetGPUInfo()

	var gpuInfo string
	if len(gpus) > 0 {
		g := gpus[0]
		gpuInfo = fmt.Sprintf("%s (%.1f GB VRAM free)", g.Name, float64(g.VRAMFreeBytes)/(1024*1024*1024))
	} else {
		gpuInfo = "No GPU detected"
	}

	html := fmt.Sprintf(`<div style="background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 12px 20px; margin-bottom: 20px;">
		<span style="color: #94a3b8; font-size: 13px;">GPU: %s</span>
	</div>
	<table>
		<thead>
			<tr>
				<th>Model</th>
				<th>Status</th>
				<th>Port</th>
				<th>Actions</th>
			</tr>
		</thead>
		<tbody>`, gpuInfo)

	for _, server := range servers {
		statusClass := "status-fail"
		if server.Status == "running" {
			statusClass = "status-pass"
		}

		html += fmt.Sprintf(`<tr>
			<td><strong>%s</strong></td>
			<td><span class="%s">%s</span></td>
			<td>%d</td>
			<td>
				%s
			</td>
		</tr>`, server.ModelName, statusClass, server.Status, server.Port,
			s.renderServerActions(server))
	}

	if len(servers) == 0 {
		html += `<tr><td colspan="4" style="text-align: center; color: #64748b; padding: 20px;">No servers running</td></tr>`
	}

	html += `</tbody></table>`
	return html
}

// renderServerActions generates action buttons for a server
func (s *Server) renderServerActions(server *ModelServer) string {
	if server.Status == "running" {
		return fmt.Sprintf(`<form hx-post="/api/servers/stop" hx-target="#servers_list" hx-swap="innerHTML" style="display: inline;">
			<input type="hidden" name="model_path" value="%s">
			<button type="submit" class="btn" style="padding: 4px 12px; font-size: 12px; background: #ef4444;">Stop</button>
		</form>`, server.ModelPath)
	}
	return fmt.Sprintf(`<span style="color: #64748b; font-size: 12px;">Stopped</span>`)
}

// renderBenchmarkResultsHTML generates HTML for benchmark results
func (s *Server) renderBenchmarkResultsHTML() string {
	runs, err := s.store.GetBenchmarkRuns(5)
	if err != nil {
		return `<p style="color: #ef4444;">Error loading results</p>`
	}

	if len(runs) == 0 {
		return `<p style="color: #64748b; font-size: 13px;">No benchmark results yet</p>`
	}

	html := `<table style="width: 100%%; border-collapse: collapse;">
		<thead>
			<tr style="border-bottom: 1px solid #334155;">
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Model</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Success</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Tests</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Completed</th>
			</tr>
		</thead>
		<tbody>`

	for _, run := range runs {
		html += fmt.Sprintf(`<tr style="border-bottom: 1px solid #1e293b;">
			<td style="padding: 12px; color: #e2e8f0;">%s</td>
			<td style="padding: 12px;"><span class="%s">%.1f%%</span></td>
			<td style="padding: 12px; color: #94a3b8;">%d/%d</td>
			<td style="padding: 12px; color: #64748b;">%s</td>
		</tr>`,
			run.ModelName,
			func() string {
				if run.SuccessRate >= 70 {
					return "success-high"
				}
				if run.SuccessRate >= 50 {
					return "success-medium"
				}
				return "success-low"
			}(),
			run.SuccessRate,
			run.PassedTests, run.TotalTests,
			run.CompletedAt.Format("Jan 2, 3:04 PM"))
	}

	html += `</tbody></table>`
	return html
}

// broadcastServerUpdate sends a server list update to all SSE clients
// Triggers clients to refresh their server list via sse-swap
func (s *Server) broadcastServerUpdate() {
	s.sseClientsMu.RLock()
	defer s.sseClientsMu.RUnlock()

	log.Printf("Broadcasting servers_update event to %d SSE clients", len(s.sseClients))

	if len(s.sseClients) == 0 {
		log.Println("No SSE clients connected, skipping broadcast")
		return
	}

	// Signal clients to refresh - they'll receive HTML content in SSE data
	msg := "servers_update"
	for clientChan := range s.sseClients {
		select {
		case clientChan <- msg:
			log.Println("Sent servers_update event to client")
		default:
			log.Println("Client buffer full, skipping")
		}
	}
}

// broadcastBenchmarkUpdate sends a benchmark completion event to all SSE clients
func (s *Server) broadcastBenchmarkUpdate() {
	s.sseClientsMu.RLock()
	defer s.sseClientsMu.RUnlock()

	log.Printf("Broadcasting benchmark_update event to %d SSE clients", len(s.sseClients))

	if len(s.sseClients) == 0 {
		log.Println("No SSE clients connected, skipping broadcast")
		return
	}

	msg := "benchmark_update"
	for clientChan := range s.sseClients {
		select {
		case clientChan <- msg:
			log.Println("Sent benchmark_update event to client")
		default:
			log.Println("Client buffer full, skipping")
		}
	}
}

// handleDashboard redirects to /new
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Redirect to /models page
	http.Redirect(w, r, "/models", http.StatusSeeOther)
}

// handleRunsList shows all benchmark runs
func (s *Server) handleRunsList(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.GetBenchmarkRuns(100) // Get more runs for the full list
	if err != nil {
		http.Error(w, "Failed to load benchmark runs", http.StatusInternalServerError)
		log.Printf("Error loading runs: %v", err)
		return
	}

	component := templates.Dashboard(runs)
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Error rendering runs list: %v", err)
	}
}

// handleRunDetails shows detailed results for a specific run
func (s *Server) handleRunDetails(w http.ResponseWriter, r *http.Request) {
	// Extract run ID from URL path
	idStr := r.URL.Path[len("/run/"):]
	runID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid run ID", http.StatusBadRequest)
		return
	}

	run, err := s.store.GetBenchmarkRun(runID)
	if err != nil {
		http.Error(w, "Failed to load run", http.StatusInternalServerError)
		log.Printf("Error loading run %d: %v", runID, err)
		return
	}
	if run == nil {
		http.Error(w, "Run not found", http.StatusNotFound)
		return
	}

	results, err := s.store.GetBenchmarkResults(runID)
	if err != nil {
		http.Error(w, "Failed to load results", http.StatusInternalServerError)
		log.Printf("Error loading results for run %d: %v", runID, err)
		return
	}

	component := templates.RunDetails(run, results)
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Error rendering run details: %v", err)
	}
}

// handleNewTest shows the form to start a new benchmark test
func (s *Server) handleNewTest(w http.ResponseWriter, r *http.Request) {
	component := templates.NewTest()
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Error rendering new test page: %v", err)
	}
}

// handleServersPage shows the server management page
func (s *Server) handleServersPage(w http.ResponseWriter, r *http.Request) {
	// Redirect old /servers route to new /models page
	http.Redirect(w, r, "/models", http.StatusMovedPermanently)
}

func (s *Server) handleModelsPage(w http.ResponseWriter, r *http.Request) {
	component := templates.Models()
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Error rendering models page: %v", err)
	}
}

func (s *Server) handleTestingPage(w http.ResponseWriter, r *http.Request) {
	component := templates.Testing()
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Error rendering testing page: %v", err)
	}
}

// handleResultDetails returns detailed information for a single test result (HTMX endpoint)
func (s *Server) handleResultDetails(w http.ResponseWriter, r *http.Request) {
	// Extract result ID from URL path
	idStr := r.URL.Path[len("/api/result/"):]
	resultID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid result ID", http.StatusBadRequest)
		return
	}

	// Get the run ID from query parameter
	runIDStr := r.URL.Query().Get("run_id")
	runID, err := strconv.Atoi(runIDStr)
	if err != nil {
		http.Error(w, "Invalid run ID", http.StatusBadRequest)
		return
	}

	results, err := s.store.GetBenchmarkResults(runID)
	if err != nil {
		http.Error(w, "Failed to load results", http.StatusInternalServerError)
		log.Printf("Error loading results: %v", err)
		return
	}

	// Find the specific result by ID
	var result *db.BenchmarkResult
	for i := range results {
		if results[i].ID == resultID {
			result = &results[i]
			break
		}
	}

	if result == nil {
		http.Error(w, "Result not found", http.StatusNotFound)
		return
	}

	// Use Templ template instead of inline HTML
	w.Header().Set("Content-Type", "text/html")
	templates.ResultDetails(result).Render(r.Context(), w)
}

// handleStartRun starts a new benchmark run (HTMX endpoint)
func (s *Server) handleStartRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	modelName := r.FormValue("model_name")
	modelURL := r.FormValue("model_url")

	if modelName == "" || modelURL == "" {
		http.Error(w, "Model name and URL are required", http.StatusBadRequest)
		return
	}

	// TODO: Start benchmark run asynchronously
	// For now, just return a success message
	html := fmt.Sprintf(`
		<div class="p-4 bg-green-900/20 border border-green-500 rounded">
			<p class="text-green-300">Starting benchmark for %s...</p>
			<p class="text-sm text-gray-400 mt-2">This feature is coming soon.</p>
		</div>
	`, modelName)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// ModelInfo represents a model available on a server
type ModelInfo struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	APIType string `json:"api_type"` // "openai", "ollama", "llamacpp"
}

// handleGetModels returns a list of available models from common servers
func (s *Server) handleGetModels(w http.ResponseWriter, r *http.Request) {
	log.Println("API: /api/models called")
	models := []ModelInfo{}

	// Common server URLs to check
	serverURLs := []string{
		"http://localhost:11434", // Ollama default
		"http://localhost:5000",  // vllm/text-generation-inference
	}

	// Scan common llama.cpp ports
	for port := 8081; port <= 8090; port++ {
		serverURLs = append(serverURLs, fmt.Sprintf("http://localhost:%d", port))
	}

	for _, serverURL := range serverURLs {
		log.Printf("Checking server: %s", serverURL)
		// Try to fetch models from this server
		fetchedModels := fetchModelsFromServer(serverURL)
		log.Printf("Found %d models from %s", len(fetchedModels), serverURL)
		models = append(models, fetchedModels...)
	}

	log.Printf("Total models found: %d", len(models))

	// If no models found, return default suggestions
	if len(models) == 0 {
		log.Println("No models found, returning defaults")
		models = []ModelInfo{
			{Name: "Ollama (http://localhost:11434)", URL: "http://localhost:11434", APIType: "ollama"},
			{Name: "llama.cpp (http://localhost:8081)", URL: "http://localhost:8081", APIType: "llamacpp"},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(models); err != nil {
		log.Printf("Error encoding models JSON: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	log.Println("Successfully sent models response")
}

// handleGetModelOptions returns HTML <option> elements for HTMX
func (s *Server) handleGetModelOptions(w http.ResponseWriter, r *http.Request) {
	log.Println("API: /api/models/options called")
	models := []ModelInfo{}

	// Common server URLs to check
	serverURLs := []string{
		"http://localhost:11434", // Ollama default
		"http://localhost:5000",  // vllm/text-generation-inference
	}

	// Scan common llama.cpp ports
	for port := 8081; port <= 8090; port++ {
		serverURLs = append(serverURLs, fmt.Sprintf("http://localhost:%d", port))
	}

	for _, serverURL := range serverURLs {
		log.Printf("Checking server: %s", serverURL)
		fetchedModels := fetchModelsFromServer(serverURL)
		log.Printf("Found %d models from %s", len(fetchedModels), serverURL)
		models = append(models, fetchedModels...)
	}

	log.Printf("Total models found: %d", len(models))

	// Return HTML options
	w.Header().Set("Content-Type", "text/html")

	// Default option
	if len(models) == 0 {
		fmt.Fprint(w, `<option value="">No models found (enter manually)</option>`)
		log.Println("No models found, returning empty option")
		return
	}

	// Start with a placeholder option
	fmt.Fprint(w, `<option value="">Select a model...</option>`)

	// Add option for each model
	for _, model := range models {
		// Store model data as JSON in the value attribute (with API type)
		jsonData := fmt.Sprintf(`{"name":"%s","url":"%s","api_type":"%s"}`, model.Name, model.URL, model.APIType)

		// Display friendly API type name
		apiTypeDisplay := model.APIType
		switch model.APIType {
		case "llamacpp":
			apiTypeDisplay = "llama.cpp"
		case "openai":
			apiTypeDisplay = "OpenAI"
		case "ollama":
			apiTypeDisplay = "Ollama"
		}

		fmt.Fprintf(w, `<option value='%s'>%s - %s (%s)</option>`, jsonData, model.Name, apiTypeDisplay, model.URL)
	}

	log.Printf("Successfully sent %d model options as HTML", len(models))
}

// handleSearchHuggingFace searches HuggingFace for GGUF models and returns HTML datalist options
func (s *Server) handleSearchHuggingFace(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" || len(query) < 2 {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "") // Return empty if query too short
		return
	}

	// Search HuggingFace API for GGUF models
	searchURL := fmt.Sprintf("https://huggingface.co/api/models?search=%s&filter=gguf&sort=downloads&limit=10", query)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(searchURL)
	if err != nil {
		log.Printf("Failed to search HuggingFace: %v", err)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "")
		return
	}
	defer resp.Body.Close()

	var models []struct {
		ID        string   `json:"id"`
		Downloads int      `json:"downloads"`
		Likes     int      `json:"likes"`
		Tags      []string `json:"tags"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		log.Printf("Failed to decode HuggingFace response: %v", err)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "")
		return
	}

	// Return HTML datalist options
	w.Header().Set("Content-Type", "text/html")
	html := ""
	for _, model := range models {
		// Extract useful tags (exclude generic ones)
		relevantTags := []string{}
		for _, tag := range model.Tags {
			// Skip generic tags, keep specific ones
			if strings.HasPrefix(tag, "base_model:") {
				continue
			}
			if tag == "gguf" || tag == "endpoints_compatible" || strings.HasPrefix(tag, "region:") {
				continue
			}
			// Keep architecture, quantization, and other specific tags
			if tag == "transformers" || tag == "text-generation" {
				continue
			}
			relevantTags = append(relevantTags, tag)
			if len(relevantTags) >= 2 {
				break
			}
		}

		// Format: "user/model-name • 123K↓ 45❤ • tag1, tag2"
		parts := []string{model.ID}

		// Downloads and likes
		stats := ""
		if model.Downloads > 1000 {
			stats = fmt.Sprintf("%dK↓", model.Downloads/1000)
		} else {
			stats = fmt.Sprintf("%d↓", model.Downloads)
		}
		if model.Likes > 0 {
			stats += fmt.Sprintf(" %d❤", model.Likes)
		}
		if stats != "" {
			parts = append(parts, stats)
		}

		// Tags
		if len(relevantTags) > 0 {
			parts = append(parts, strings.Join(relevantTags, ", "))
		}

		displayText := strings.Join(parts, " • ")
		html += fmt.Sprintf(`<option value="%s">%s</option>`, model.ID, displayText)
	}
	fmt.Fprint(w, html)
}

// handleGetModelInfo fetches detailed information about a HuggingFace model
func (s *Server) handleGetModelInfo(w http.ResponseWriter, r *http.Request) {
	modelID := r.URL.Query().Get("id")
	if modelID == "" {
		http.Error(w, "model id required", http.StatusBadRequest)
		return
	}

	// Fetch model details from HuggingFace API
	apiURL := fmt.Sprintf("https://huggingface.co/api/models/%s", modelID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Printf("Failed to fetch model info for %s: %v", modelID, err)
		http.Error(w, "Failed to fetch model info", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("HuggingFace API returned status %d for model %s", resp.StatusCode, modelID)
		http.Error(w, fmt.Sprintf("Model not found (status %d)", resp.StatusCode), http.StatusNotFound)
		return
	}

	var modelData struct {
		ID        string   `json:"id"`
		Author    string   `json:"author"`
		Downloads int      `json:"downloads"`
		Likes     int      `json:"likes"`
		Tags      []string `json:"tags"`
		Siblings  []struct {
			Rfilename string `json:"rfilename"`
		} `json:"siblings"`
		CardData *struct {
			BaseModel   interface{} `json:"base_model"` // Can be string or array
			QuantizedBy string      `json:"quantized_by"`
			License     string      `json:"license"`
			PipelineTag string      `json:"pipeline_tag"`
			Tags        []string    `json:"tags"`
		} `json:"cardData"`
		GGUF *struct {
			Total         int64  `json:"total"`
			Architecture  string `json:"architecture"`
			ContextLength int    `json:"context_length"`
		} `json:"gguf"`
		UsedStorage  int64  `json:"usedStorage"`
		LastModified string `json:"lastModified"`
		CreatedAt    string `json:"createdAt"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&modelData); err != nil {
		log.Printf("Failed to decode model info for %s: %v", modelID, err)
		http.Error(w, "Failed to parse model info", http.StatusInternalServerError)
		return
	}

	// Filter GGUF files only
	var ggufFiles []string
	for _, sibling := range modelData.Siblings {
		if strings.HasSuffix(sibling.Rfilename, ".gguf") {
			ggufFiles = append(ggufFiles, sibling.Rfilename)
		}
	}

	// Fetch file sizes from /tree/main endpoint
	type FileInfo struct {
		Path string `json:"path"`
		Size int64  `json:"size"`
	}

	fileSizes := make(map[string]int64)
	treeURL := fmt.Sprintf("https://huggingface.co/api/models/%s/tree/main", modelID)
	treeResp, err := client.Get(treeURL)
	if err == nil {
		defer treeResp.Body.Close()
		var treeData []FileInfo
		if err := json.NewDecoder(treeResp.Body).Decode(&treeData); err == nil {
			for _, file := range treeData {
				if strings.HasSuffix(file.Path, ".gguf") {
					fileSizes[file.Path] = file.Size
				}
			}
		}
	}

	// Build HTML response with model details
	w.Header().Set("Content-Type", "text/html")

	html := `<div id="model_info" style="margin-top: 15px; padding: 15px; background: #0f172a; border: 1px solid #334155; border-radius: 6px;">`

	// Add hover style for link and hide main download button script
	html += `<style>.model-link:hover { text-decoration: underline !important; }</style>`
	html += `<script>
		// Hide main download button when model info is displayed
		const mainBtn = document.getElementById('main_download_btn');
		if (mainBtn) {
			mainBtn.style.display = 'none';
		}
	</script>`

	// Model header with clickable title and stats
	modelURL := fmt.Sprintf("https://huggingface.co/%s", modelData.ID)
	html += fmt.Sprintf(`
		<div style="margin-bottom: 15px; padding-bottom: 12px; border-bottom: 1px solid #334155;">
			<h3 style="margin: 0 0 8px 0; font-size: 16px;">
				<a href="%s" target="_blank" class="model-link" style="color: #60a5fa; text-decoration: none;">
					%s
				</a>
			</h3>
			<div style="display: flex; gap: 15px; font-size: 12px; color: #94a3b8; margin-bottom: 8px;">
				<span>📥 %s downloads</span>
				<span>❤️ %d likes</span>
			</div>
	`, modelURL, modelData.ID, formatNumber(modelData.Downloads), modelData.Likes)

	// Add model metadata from cardData if available
	if modelData.CardData != nil {
		metaItems := []string{}

		// Base model (handle both string and array)
		if modelData.CardData.BaseModel != nil {
			baseModelStr := ""
			switch v := modelData.CardData.BaseModel.(type) {
			case string:
				baseModelStr = v
			case []interface{}:
				if len(v) > 0 {
					if str, ok := v[0].(string); ok {
						baseModelStr = str
					}
				}
			}
			if baseModelStr != "" {
				metaItems = append(metaItems, fmt.Sprintf(`<span>🔗 Base: <code style="font-size: 11px; background: #1e293b; padding: 2px 4px; border-radius: 3px;">%s</code></span>`, baseModelStr))
			}
		}

		// Quantized by
		if modelData.CardData.QuantizedBy != "" {
			metaItems = append(metaItems, fmt.Sprintf(`<span>⚙️ By: <strong>%s</strong></span>`, modelData.CardData.QuantizedBy))
		}

		// License
		if modelData.CardData.License != "" {
			metaItems = append(metaItems, fmt.Sprintf(`<span>📜 %s</span>`, modelData.CardData.License))
		}

		// Tags (show first 3-4 relevant tags)
		if len(modelData.CardData.Tags) > 0 {
			tagBadges := []string{}
			for i, tag := range modelData.CardData.Tags {
				if i >= 4 {
					break
				}
				tagBadges = append(tagBadges, fmt.Sprintf(`<span style="display: inline-block; padding: 2px 6px; background: #1e293b; border-radius: 3px; font-size: 10px;">%s</span>`, tag))
			}
			if len(tagBadges) > 0 {
				metaItems = append(metaItems, fmt.Sprintf(`<span>🏷️ %s</span>`, strings.Join(tagBadges, " ")))
			}
		}

		if len(metaItems) > 0 {
			html += fmt.Sprintf(`
			<div style="display: flex; flex-wrap: wrap; gap: 12px; font-size: 11px; color: #94a3b8;">
				%s
			</div>
			`, strings.Join(metaItems, "\n\t\t\t\t"))
		}
	}

	// Add GGUF metadata if available
	if modelData.GGUF != nil {
		ggufMetaItems := []string{}

		// Architecture
		if modelData.GGUF.Architecture != "" {
			ggufMetaItems = append(ggufMetaItems, fmt.Sprintf(`<span>🏗️ Arch: <code style="font-size: 11px; background: #1e293b; padding: 2px 4px; border-radius: 3px;">%s</code></span>`, modelData.GGUF.Architecture))
		}

		// Context length
		if modelData.GGUF.ContextLength > 0 {
			contextStr := fmt.Sprintf("%d", modelData.GGUF.ContextLength)
			if modelData.GGUF.ContextLength >= 1000 {
				contextStr = fmt.Sprintf("%dK", modelData.GGUF.ContextLength/1024)
			}
			ggufMetaItems = append(ggufMetaItems, fmt.Sprintf(`<span>📏 Context: <strong>%s</strong></span>`, contextStr))
		}

		// Total GGUF size
		if modelData.GGUF.Total > 0 {
			ggufMetaItems = append(ggufMetaItems, fmt.Sprintf(`<span>💾 Total Size: <strong>%s</strong></span>`, formatBytes(modelData.GGUF.Total)))
		}

		// Repository storage
		if modelData.UsedStorage > 0 {
			ggufMetaItems = append(ggufMetaItems, fmt.Sprintf(`<span>📦 Repo Size: %s</span>`, formatBytes(modelData.UsedStorage)))
		}

		// Author
		if modelData.Author != "" {
			ggufMetaItems = append(ggufMetaItems, fmt.Sprintf(`<span>👤 %s</span>`, modelData.Author))
		}

		// Last modified (relative time)
		if modelData.LastModified != "" {
			// Parse the timestamp and show relative time
			if t, err := time.Parse(time.RFC3339, modelData.LastModified); err == nil {
				relativeTime := formatRelativeTime(t)
				ggufMetaItems = append(ggufMetaItems, fmt.Sprintf(`<span>🕒 Updated %s</span>`, relativeTime))
			}
		}

		if len(ggufMetaItems) > 0 {
			html += fmt.Sprintf(`
			<div style="display: flex; flex-wrap: wrap; gap: 12px; font-size: 11px; color: #64748b; margin-top: 8px;">
				%s
			</div>
			`, strings.Join(ggufMetaItems, "\n\t\t\t\t"))
		}
	}

	html += `</div>`

	// Available GGUF files
	if len(ggufFiles) > 0 {
		// Group files by quantization type (handling multi-part files)
		type FileGroup struct {
			Quantization string
			Files        []string
			TotalSize    int64
			IsMultiPart  bool
		}

		quantGroups := make(map[string]*FileGroup)

		for _, filename := range ggufFiles {
			// Extract quantization
			quant := extractQuantization(filename)
			log.Printf("DEBUG: File '%s' -> Quantization '%s'", filename, quant)
			if quant == "" {
				// Try extracting from directory name (e.g., Q2_K/file.gguf)
				if strings.Contains(filename, "/") {
					parts := strings.Split(filename, "/")
					if len(parts) > 0 {
						quant = parts[0]
					}
				}
			}

			// If still no quantization, use filename as key
			if quant == "" {
				quant = filename
			}

			// Check if this is a multi-part file (contains "00001-of-" or similar)
			isMultiPart := strings.Contains(filename, "-of-")

			// Get or create group
			if _, exists := quantGroups[quant]; !exists {
				quantGroups[quant] = &FileGroup{
					Quantization: quant,
					Files:        []string{},
					TotalSize:    0,
					IsMultiPart:  false,
				}
			}

			group := quantGroups[quant]
			group.Files = append(group.Files, filename)
			if isMultiPart {
				group.IsMultiPart = true
			}

			// Add file size
			if size, ok := fileSizes[filename]; ok {
				group.TotalSize += size
			}
		}

		// Convert map to sorted slice
		var groups []*FileGroup
		for _, group := range quantGroups {
			groups = append(groups, group)
		}

		// Sort by quantization name
		sort.Slice(groups, func(i, j int) bool {
			return groups[i].Quantization < groups[j].Quantization
		})

		// Get GPU info for memory compatibility checks
		gpus, gpuErr := gpu.GetGPUInfo()
		var availableMemory int64
		var totalMemory int64
		var memoryType string
		gpuStatusHTML := ""
		if gpuErr == nil && len(gpus) > 0 {
			g := gpus[0] // Use first GPU

			// Detect unified memory architecture (same logic as GPU status)
			isUnifiedMemory := g.GTTTotalBytes > g.VRAMTotalBytes*2

			if isUnifiedMemory && g.GTTTotalBytes > 0 {
				// Use GTT (shared system RAM) for unified memory systems
				availableMemory = g.GTTTotalBytes - g.GTTUsedBytes
				totalMemory = g.GTTTotalBytes
				memoryType = "shared system RAM"
			} else {
				// Use VRAM for dedicated GPUs
				availableMemory = g.VRAMFreeBytes
				totalMemory = g.VRAMTotalBytes
				memoryType = "VRAM"
			}

			gpuStatusHTML = fmt.Sprintf(
				`<p style="color: #94a3b8; font-size: 11px; margin-bottom: 10px; padding: 6px 12px; background: #1e293b; border-left: 3px solid %s; border-radius: 4px;">
			🖥️ %s: %.1f GB free %s (%.1f GB total) - compatibility indicators shown below
		</p>`,
				func() string {
					if availableMemory > 16*1024*1024*1024 {
						return "#10b981" // green
					} else if availableMemory > 8*1024*1024*1024 {
						return "#fbbf24" // yellow
					}
					return "#ef4444" // red
				}(),
				func() string {
					if g.Name != "" {
						return g.Name
					}
					return "GPU"
				}(),
				float64(availableMemory)/(1024*1024*1024),
				memoryType,
				float64(totalMemory)/(1024*1024*1024),
			)
		}

		// Get context length for RAM estimation
		contextLength := 4096 // default
		if modelData.GGUF != nil && modelData.GGUF.ContextLength > 0 {
			contextLength = modelData.GGUF.ContextLength
		}

		html += fmt.Sprintf(`
			<div style="margin-bottom: 12px;">
				<label style="display: block; margin-bottom: 8px; color: #cbd5e1; font-size: 13px; font-weight: 500;">
					Available Quantizations (%d variants):
				</label>
			%s
			<p style="color: #94a3b8; font-size: 12px; margin-bottom: 10px; padding: 8px 12px; background: #1e293b; border-left: 3px solid #3b82f6; border-radius: 4px;">
				💡 <strong>Select a quantization below.</strong> Each shows quality rating (⭐) and bits per weight (bpw). 
				Hover over any quantization for details. <strong>Recommended:</strong> Q4_K_M for best balance, Q5_K_M for better quality, IQ3_M/Q3_K_M for limited RAM.
			</p>
			<details style="margin-bottom: 10px; padding: 8px 12px; background: #1e293b; border-radius: 4px; cursor: pointer;">
				<summary style="color: #cbd5e1; font-size: 11px; font-weight: 500; cursor: pointer;">📊 Quality Rating Guide</summary>
				<div style="margin-top: 8px; padding-left: 8px; font-size: 11px; color: #94a3b8; line-height: 1.6;">
					<div><span style="color: #fbbf24;">⭐⭐⭐⭐⭐⭐ Perfect</span> - Lossless (F16/F32)</div>
					<div><span style="color: #fbbf24;">⭐⭐⭐⭐⭐ Near-Perfect</span> - Minimal loss (Q8, Q6_K)</div>
					<div><span style="color: #fbbf24;">⭐⭐⭐⭐☆ Very High</span> - Best balance (Q5_K_M, Q4_K_M)</div>
					<div><span style="color: #fbbf24;">⭐⭐⭐☆ Medium-High</span> - Good quality (Q4_K_S, IQ4)</div>
					<div><span style="color: #fbbf24;">⭐⭐⭐ Medium</span> - Usable (Q3_K_M, IQ3_M)</div>
					<div><span style="color: #fbbf24;">⭐⭐☆ Low-Medium</span> - RAM constrained (Q2_K, IQ2)</div>
					<div><span style="color: #fbbf24;">⭐⭐ Low</span> - Very small (IQ1)</div>
					<div style="margin-top: 6px; font-style: italic; color: #64748b;">Lower quality = smaller file size, faster loading, less RAM needed</div>
			</div>
		</details>
			<div style="max-height: 400px; overflow-y: auto; border: 1px solid #334155; border-radius: 4px; background: #1e293b;">
	`, len(groups), gpuStatusHTML)

		for i, group := range groups {
			borderBottom := ""
			if i < len(groups)-1 {
				borderBottom = "border-bottom: 1px solid #334155;"
			}

			// Build display text with quantization info
			bitsPerWeight, description, qualityRating, useCase := getQuantizationInfo(group.Quantization)
			displayText := group.Quantization

			// Add quality indicator and bits per weight if available
			if qualityRating != "" && qualityRating != "Unknown" {
				displayText += fmt.Sprintf(` <span style="color: #fbbf24; font-size: 10px;">%s</span>`, qualityRating)
			}
			if bitsPerWeight != "" && bitsPerWeight != "Unknown" {
				displayText += fmt.Sprintf(` <span style="color: #94a3b8; font-size: 10px;">(%s)</span>`, bitsPerWeight)
			}

			if group.IsMultiPart {
				displayText += fmt.Sprintf(` <span style="color: #64748b; font-size: 10px;">- %d parts</span>`, len(group.Files))
			}

			// Build description tooltip (used below in the badge)
			tooltipAttr := ""
			if description != "" && description != "Custom quantization" {
				tooltipAttr = fmt.Sprintf(` title="%s - %s"`, description, useCase)
			}

			// Build file list for multi-part
			fileListHTML := ""
			if group.IsMultiPart && len(group.Files) > 1 {
				fileListHTML = `<div style="font-size: 10px; color: #64748b; margin-top: 4px;">`
				for _, file := range group.Files {
					fileListHTML += fmt.Sprintf(`• %s<br>`, file)
				}
				fileListHTML += `</div>`
			}

			// Total size
			sizeStr := ""
			if group.TotalSize > 0 {
				sizeStr = fmt.Sprintf(`<span style="color: #64748b; font-size: 11px; margin-right: 8px;">%s total</span>`, formatBytes(group.TotalSize))
			}

			// Memory compatibility check
			memoryCompatHTML := ""
			if availableMemory > 0 && group.TotalSize > 0 {
				requiredBytes, _ := estimateRAMRequirement(group.TotalSize, contextLength)
				_, memoryCompatHTML = getMemoryCompatibility(requiredBytes, availableMemory)
			}

			// Build URLs for download (comma-separated for multi-part)
			var urls []string
			for _, file := range group.Files {
				urls = append(urls, fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", modelData.ID, file))
			}
			urlsJSON := strings.Join(urls, ",")

			html += fmt.Sprintf(`
				<div style="padding: 12px; %s">
					<div style="display: flex; align-items: center; justify-content: space-between;">
						<div style="flex: 1;">
							<div style="display: flex; align-items: center; margin-bottom: 4px; flex-wrap: wrap;">
								<span style="display: inline-block; padding: 3px 8px; background: #334155; border-radius: 3px; font-size: 11px; color: #94a3b8; margin-right: 8px; font-weight: 500; cursor: help;"%s>%s</span>
								%s
								%s
							</div>
							%s
						</div>
						<form 
							hx-post="/api/models/download-group"
							hx-target="#download_status"
							hx-swap="innerHTML"
							style="margin: 0;"
						>
							<input type="hidden" name="urls" value="%s" />
							<input type="hidden" name="model_id" value="%s" />
							<input type="hidden" name="quantization" value="%s" />
							<button 
								type="submit"
								class="btn"
								style="padding: 8px 16px; font-size: 11px; white-space: nowrap;"
							>
								%s
							</button>
					</form>
				</div>
			</div>
	`, borderBottom, tooltipAttr, displayText, sizeStr, memoryCompatHTML, fileListHTML, urlsJSON, modelData.ID, group.Quantization,
				func() string {
					if group.IsMultiPart {
						return fmt.Sprintf("FIXED: Download All %d Parts", len(group.Files))
					}
					return "Download Single File"
				}())
		}

		html += `
				</div>
			</div>`
	} else {
		html += `<p style="color: #f87171; font-size: 13px;">⚠️ No GGUF files found in this repository</p>`
	}

	html += `</div>`

	fmt.Fprint(w, html)
}

// formatNumber formats large numbers with K/M suffixes
func formatNumber(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	} else if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// extractQuantization extracts quantization type from GGUF filename
// Examples: "Q4_K_M", "IQ3_M", "f16", etc.
func extractQuantization(filename string) string {
	// Remove .gguf extension
	name := strings.TrimSuffix(filename, ".gguf")

	// Strip multi-part suffix first (e.g., "-00001-of-00003")
	// This prevents the part number from being detected as quantization
	re := regexp.MustCompile(`-\d{5}-of-\d{5}$`)
	name = re.ReplaceAllString(name, "")

	// Try splitting by dot (e.g., llama-2-7b.Q4_K_M.gguf)
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		// Only use this if the last part is PURELY quantization (no other text)
		// Valid quantization starts with Q, IQ, or f and has no dashes
		if (strings.HasPrefix(lastPart, "Q") || strings.HasPrefix(lastPart, "IQ") || strings.HasPrefix(lastPart, "f")) && !strings.Contains(lastPart, "-") {
			return lastPart
		}
	}

	// Try splitting by dash (e.g., Qwen2.5-Coder-7B-Instruct-IQ2_M.gguf)
	parts = strings.Split(name, "-")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		lastPartLower := strings.ToLower(lastPart)
		// Check if it looks like a quantization
		// Common patterns: Q4_K_M, IQ3_M, f16, f32, mxfp4, mxfp8, etc.
		if strings.HasPrefix(lastPart, "Q") ||
			strings.HasPrefix(lastPart, "IQ") ||
			strings.HasPrefix(lastPart, "f") ||
			strings.HasPrefix(lastPartLower, "mxfp") ||
			strings.HasPrefix(lastPartLower, "int") ||
			strings.HasPrefix(lastPartLower, "fp") {
			return strings.ToUpper(lastPart) // Normalize to uppercase
		}
	}

	return ""
}

// formatRelativeTime formats a time as a relative string (e.g., "2 days ago", "3 months ago")
func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 30*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if diff < 365*24*time.Hour {
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
	years := int(diff.Hours() / 24 / 365)
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}

// getQuantizationInfo returns detailed information about a quantization type
func getQuantizationInfo(quant string) (bitsPerWeight string, description string, quality string, useCase string) {
	// Default values
	bitsPerWeight = "Unknown"
	description = "Custom quantization"
	quality = "Unknown"
	useCase = "Experimental"

	// Clean up quantization string (remove directory prefixes)
	if strings.Contains(quant, "/") {
		parts := strings.Split(quant, "/")
		quant = parts[len(parts)-1]
	}

	switch {
	// IQ (Importance Matrix Quantization) - newer, better quality for size
	case quant == "IQ1_S":
		bitsPerWeight = "1.56 bpw"
		description = "Importance Matrix Quant - Super low bits"
		quality = "⭐ Experimental"
		useCase = "Maximum compression, expect quality loss"
	case quant == "IQ1_M":
		bitsPerWeight = "1.75 bpw"
		description = "Importance Matrix Quant - Medium 1-bit"
		quality = "⭐ Experimental"
		useCase = "Extreme compression, quality trade-offs"
	case quant == "IQ2_XXS":
		bitsPerWeight = "2.06 bpw"
		description = "Importance Matrix Quant - Extra Extra Small"
		quality = "⭐⭐ Low"
		useCase = "Very small models, limited RAM"
	case quant == "IQ2_XS":
		bitsPerWeight = "2.31 bpw"
		description = "Importance Matrix Quant - Extra Small"
		quality = "⭐⭐ Low"
		useCase = "Small models, constrained devices"
	case quant == "IQ2_S":
		bitsPerWeight = "2.5 bpw"
		description = "Importance Matrix Quant - Small"
		quality = "⭐⭐ Low"
		useCase = "Small models, better than XXS"
	case quant == "IQ2_M":
		bitsPerWeight = "2.7 bpw"
		description = "Importance Matrix Quant - Medium"
		quality = "⭐⭐☆ Low-Medium"
		useCase = "Balanced small size, decent quality"
	case quant == "IQ3_XXS":
		bitsPerWeight = "3.06 bpw"
		description = "Importance Matrix Quant - Extra Extra Small"
		quality = "⭐⭐☆ Medium-Low"
		useCase = "3-bit with good compression"
	case quant == "IQ3_XS":
		bitsPerWeight = "3.3 bpw"
		description = "Importance Matrix Quant - Extra Small"
		quality = "⭐⭐⭐ Medium"
		useCase = "Good balance, popular choice"
	case quant == "IQ3_S":
		bitsPerWeight = "3.5 bpw"
		description = "Importance Matrix Quant - Small"
		quality = "⭐⭐⭐ Medium"
		useCase = "Better quality 3-bit"
	case quant == "IQ3_M":
		bitsPerWeight = "3.7 bpw"
		description = "Importance Matrix Quant - Medium"
		quality = "⭐⭐⭐☆ Medium-High"
		useCase = "High quality 3-bit, recommended"
	case quant == "IQ4_XS":
		bitsPerWeight = "4.25 bpw"
		description = "Importance Matrix Quant - Extra Small"
		quality = "⭐⭐⭐⭐ High"
		useCase = "Excellent quality/size ratio"
	case quant == "IQ4_NL":
		bitsPerWeight = "4.5 bpw"
		description = "Importance Matrix Quant - Non-Linear"
		quality = "⭐⭐⭐⭐ High"
		useCase = "Very good quality, recommended"

	// Q2 Series - 2-bit quantization
	case quant == "Q2_K":
		bitsPerWeight = "2.63 bpw"
		description = "2-bit K-quant"
		quality = "⭐⭐ Low"
		useCase = "Smallest traditional quant"
	case quant == "Q2_K_S":
		bitsPerWeight = "2.5 bpw"
		description = "2-bit K-quant Small"
		quality = "⭐⭐ Low"
		useCase = "Smaller than Q2_K"
	case quant == "Q2_K_L":
		bitsPerWeight = "2.8 bpw"
		description = "2-bit K-quant Large"
		quality = "⭐⭐☆ Low-Medium"
		useCase = "Better quality than Q2_K"

	// Q3 Series - 3-bit quantization
	case quant == "Q3_K_S":
		bitsPerWeight = "3.5 bpw"
		description = "3-bit K-quant Small"
		quality = "⭐⭐⭐ Medium"
		useCase = "Good size/quality balance"
	case quant == "Q3_K_M":
		bitsPerWeight = "3.91 bpw"
		description = "3-bit K-quant Medium"
		quality = "⭐⭐⭐☆ Medium-High"
		useCase = "Recommended 3-bit option"
	case quant == "Q3_K_L":
		bitsPerWeight = "4.27 bpw"
		description = "3-bit K-quant Large"
		quality = "⭐⭐⭐⭐ High"
		useCase = "Best quality 3-bit"
	case quant == "Q3_K_XL":
		bitsPerWeight = "4.5 bpw"
		description = "3-bit K-quant Extra Large"
		quality = "⭐⭐⭐⭐ High"
		useCase = "Highest quality 3-bit"

	// Q4 Series - 4-bit quantization (most popular)
	case quant == "Q4_0":
		bitsPerWeight = "4.5 bpw"
		description = "4-bit legacy quantization"
		quality = "⭐⭐⭐ Medium"
		useCase = "Legacy, use Q4_K_M instead"
	case quant == "Q4_1":
		bitsPerWeight = "5.0 bpw"
		description = "4-bit legacy with better quality"
		quality = "⭐⭐⭐☆ Medium-High"
		useCase = "Legacy, use Q4_K_M instead"
	case quant == "Q4_K_S":
		bitsPerWeight = "4.58 bpw"
		description = "4-bit K-quant Small"
		quality = "⭐⭐⭐⭐ High"
		useCase = "Good all-around choice"
	case quant == "Q4_K_M":
		bitsPerWeight = "4.85 bpw"
		description = "4-bit K-quant Medium"
		quality = "⭐⭐⭐⭐☆ Very High"
		useCase = "RECOMMENDED - Best balance"
	case quant == "Q4_K_L":
		bitsPerWeight = "5.2 bpw"
		description = "4-bit K-quant Large"
		quality = "⭐⭐⭐⭐⭐ Excellent"
		useCase = "High quality 4-bit"

	// Q5 Series - 5-bit quantization
	case quant == "Q5_0":
		bitsPerWeight = "5.5 bpw"
		description = "5-bit legacy quantization"
		quality = "⭐⭐⭐⭐ High"
		useCase = "Legacy, use Q5_K_M instead"
	case quant == "Q5_1":
		bitsPerWeight = "6.0 bpw"
		description = "5-bit legacy with better quality"
		quality = "⭐⭐⭐⭐☆ Very High"
		useCase = "Legacy, use Q5_K_M instead"
	case quant == "Q5_K_S":
		bitsPerWeight = "5.54 bpw"
		description = "5-bit K-quant Small"
		quality = "⭐⭐⭐⭐☆ Very High"
		useCase = "High quality, smaller size"
	case quant == "Q5_K_M":
		bitsPerWeight = "5.69 bpw"
		description = "5-bit K-quant Medium"
		quality = "⭐⭐⭐⭐⭐ Excellent"
		useCase = "Near-original quality"
	case quant == "Q5_K_L":
		bitsPerWeight = "6.0 bpw"
		description = "5-bit K-quant Large"
		quality = "⭐⭐⭐⭐⭐ Excellent"
		useCase = "Highest quality 5-bit"

	// Q6 Series - 6-bit quantization
	case quant == "Q6_K":
		bitsPerWeight = "6.59 bpw"
		description = "6-bit K-quant"
		quality = "⭐⭐⭐⭐⭐ Excellent"
		useCase = "Very close to original quality"
	case quant == "Q6_K_L":
		bitsPerWeight = "6.8 bpw"
		description = "6-bit K-quant Large"
		quality = "⭐⭐⭐⭐⭐ Excellent"
		useCase = "Highest quality quantization"

	// Q8 Series - 8-bit quantization
	case quant == "Q8_0":
		bitsPerWeight = "8.5 bpw"
		description = "8-bit quantization"
		quality = "⭐⭐⭐⭐⭐ Near-Perfect"
		useCase = "Almost identical to FP16"
	case quant == "Q8_1":
		bitsPerWeight = "9.0 bpw"
		description = "8-bit with better quality"
		quality = "⭐⭐⭐⭐⭐ Near-Perfect"
		useCase = "Highest quality quantization"

	// Special quantization types
	case strings.Contains(quant, "f16") || quant == "F16":
		bitsPerWeight = "16 bpw"
		description = "16-bit floating point (original)"
		quality = "⭐⭐⭐⭐⭐ Perfect"
		useCase = "Original precision, largest size"
	case strings.Contains(quant, "f32") || quant == "F32":
		bitsPerWeight = "32 bpw"
		description = "32-bit floating point (full precision)"
		quality = "⭐⭐⭐⭐⭐ Perfect"
		useCase = "Maximum precision, very large"
	case strings.Contains(strings.ToLower(quant), "mxfp"):
		bitsPerWeight = "~4 bpw"
		description = "Microsoft Mixed Precision Format"
		quality = "⭐⭐⭐⭐ High"
		useCase = "Experimental Microsoft format"
	}

	return bitsPerWeight, description, quality, useCase
}

// estimateRAMRequirement calculates estimated RAM needed for a model
// Formula: (fileSize × 1.2) for model + KV cache overhead
// The 1.2 multiplier accounts for:
// - KV cache (context memory): ~15-20%
// - Runtime overhead: ~5%
// Returns: estimated bytes needed, formatted string
func estimateRAMRequirement(fileSizeBytes int64, contextLength int) (int64, string) {
	if fileSizeBytes == 0 {
		return 0, "Unknown"
	}

	// Base model size + 20% overhead for KV cache and runtime
	baseOverhead := float64(fileSizeBytes) * 1.2

	// Additional KV cache depends on context length
	// Rough estimate: 0.5MB per 1K context for 7B models
	// Scale by apparent model size
	kvCacheBytes := int64(0)
	if contextLength > 0 {
		// Estimate: ~0.5MB per 1K context tokens for 7B models
		// Scale proportionally to model size
		modelSizeGB := float64(fileSizeBytes) / (1024 * 1024 * 1024)
		kvCacheMBPerK := 0.5 * (modelSizeGB / 7.0) // Scale from 7B baseline
		kvCacheBytes = int64(float64(contextLength) / 1000.0 * kvCacheMBPerK * 1024 * 1024)
	}

	totalBytes := int64(baseOverhead) + kvCacheBytes
	return totalBytes, formatBytes(totalBytes)
}

// getMemoryCompatibility checks if a quantization fits in available VRAM
// Returns: canFit bool, availableGB float64, requiredGB float64, statusHTML string
func getMemoryCompatibility(requiredBytes int64, availableBytes int64) (bool, string) {
	if availableBytes == 0 {
		return false, ""
	}

	canFit := requiredBytes <= availableBytes
	requiredGB := float64(requiredBytes) / (1024 * 1024 * 1024)
	availableGB := float64(availableBytes) / (1024 * 1024 * 1024)

	if canFit {
		headroom := availableGB - requiredGB
		return true, fmt.Sprintf(
			`<span style="color: #10b981; font-size: 10px; margin-left: 6px;">✓ Fits (%.1f GB available, %.1f GB headroom)</span>`,
			availableGB, headroom,
		)
	}

	shortage := requiredGB - availableGB
	return false, fmt.Sprintf(
		`<span style="color: #ef4444; font-size: 10px; margin-left: 6px;">✗ Needs %.1f GB more (%.1f GB available, %.1f GB required)</span>`,
		shortage, availableGB, requiredGB,
	)
}

// fetchModelsFromServer attempts to fetch models from a given server URL
func fetchModelsFromServer(serverURL string) []ModelInfo {
	models := []ModelInfo{}

	// Try llama.cpp API format first (/v1/models)
	log.Printf("Fetching models from %s/v1/models", serverURL)
	resp, err := http.Get(serverURL + "/v1/models")
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()

		var result struct {
			Models []struct {
				Name  string `json:"name"`
				Model string `json:"model"`
			} `json:"models"`
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
			log.Printf("Successfully decoded llama.cpp/OpenAI response from %s", serverURL)

			// Try models array first (llama.cpp format)
			if len(result.Models) > 0 {
				for _, model := range result.Models {
					name := model.Name
					if name == "" {
						name = model.Model
					}
					if name != "" {
						models = append(models, ModelInfo{
							Name:    name,
							URL:     serverURL,
							APIType: "llamacpp",
						})
						log.Printf("  - %s (llamacpp)", name)
					}
				}
				return models // Return early, don't check data array
			}

			// Only check data array if models array was empty (pure OpenAI format)
			if len(result.Data) > 0 {
				for _, model := range result.Data {
					if model.ID != "" {
						models = append(models, ModelInfo{
							Name:    model.ID,
							URL:     serverURL,
							APIType: "openai",
						})
						log.Printf("  - %s (openai)", model.ID)
					}
				}
				return models
			}
		}
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Try Ollama API format (/api/tags)
	log.Printf("Fetching models from %s/api/tags", serverURL)
	resp, err = http.Get(serverURL + "/api/tags")
	if err != nil {
		log.Printf("Error fetching from %s: %v", serverURL, err)
		return models
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Server %s returned status %d", serverURL, resp.StatusCode)
		return models
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Error decoding response from %s: %v", serverURL, err)
		return models
	}

	log.Printf("Successfully decoded %d models from %s", len(result.Models), serverURL)

	for _, model := range result.Models {
		models = append(models, ModelInfo{
			Name:    model.Name,
			URL:     serverURL,
			APIType: "ollama",
		})
		log.Printf("  - %s (ollama)", model.Name)
	}

	return models
}

// HandleRunTest handles running a quick test against a running model server
func (s *Server) HandleRunTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body or form data
	var modelPath, prompt string

	if r.Header.Get("Content-Type") == "application/json" {
		var payload struct {
			ModelPath string `json:"model_path"`
			Prompt    string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		modelPath = payload.ModelPath
		prompt = payload.Prompt
	} else {
		modelPath = r.FormValue("model_path")
		prompt = r.FormValue("prompt")
	}

	if modelPath == "" {
		http.Error(w, "model_path required", http.StatusBadRequest)
		return
	}

	if prompt == "" {
		http.Error(w, "prompt required", http.StatusBadRequest)
		return
	}

	// Check if server is running
	server, exists := s.modelManager.GetServerByModelPath(modelPath)
	if !exists {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	if server.Status != "running" {
		http.Error(w, "Server is not running. Please start the server first.", http.StatusConflict)
		return
	}

	// Execute test by sending prompt to model server
	startTime := time.Now()

	// Build request to llama-server
	serverURL := fmt.Sprintf("http://localhost:%d/completion", server.Port)
	requestBody := map[string]interface{}{
		"prompt":      prompt,
		"n_predict":   100, // Max tokens
		"temperature": 0.7,
		"stop":        []string{"\n\n"},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}

	// Send request to model
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(serverURL, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send request to model: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyText, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("Model returned error: %s", bodyText), http.StatusInternalServerError)
		return
	}

	// Parse response
	var llamaResp struct {
		Content string `json:"content"`
		Stop    bool   `json:"stop"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&llamaResp); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse response: %v", err), http.StatusInternalServerError)
		return
	}

	duration := time.Since(startTime)

	// Save test result to database
	quickTest := &db.QuickTest{
		ModelName:  server.ModelName,
		ModelPath:  modelPath,
		Prompt:     prompt,
		Response:   llamaResp.Content,
		DurationMs: duration.Milliseconds(),
		CreatedAt:  time.Now(),
	}
	if err := s.store.SaveQuickTest(quickTest); err != nil {
		log.Printf("Failed to save quick test: %v", err)
		// Don't fail the request, just log the error
	}

	// Return success with model output
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	// Escape HTML in response
	escapedResponse := strings.ReplaceAll(llamaResp.Content, "<", "&lt;")
	escapedResponse = strings.ReplaceAll(escapedResponse, ">", "&gt;")

	fmt.Fprintf(w, `<div style="padding: 12px; background: #10b981; color: white; border-radius: 4px; margin: 10px 0;">
		<strong>✓ Test completed in %dms</strong><br>
		<strong>Model:</strong> %s<br>
		<strong>Prompt:</strong> "%s"<br>
		<strong>Response:</strong> <pre style="background: rgba(0,0,0,0.2); padding: 8px; margin-top: 8px; border-radius: 4px; white-space: pre-wrap;">%s</pre>
	</div>
	<script>
		// Trigger refresh of test results table
		htmx.trigger('#test_results_table', 'reload');
	</script>`, duration.Milliseconds(), server.ModelName, prompt, escapedResponse)
}

// HandleGetTestResults returns the test results as HTML
func (s *Server) HandleGetTestResults(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	// Fetch recent test results
	tests, err := s.store.GetRecentQuickTests(20)
	if err != nil {
		log.Printf("Failed to fetch quick tests: %v", err)
		tests = []db.QuickTest{} // Empty slice on error
	}

	html := `<table style="width: 100%; border-collapse: collapse; margin-top: 20px;">
		<thead>
			<tr style="border-bottom: 1px solid #334155;">
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Model</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Prompt</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Response Preview</th>
				<th style="text-align: left; padding: 12px; color: #94a3b8;">Time</th>
			</tr>
		</thead>
		<tbody>`

	if len(tests) == 0 {
		html += `<tr>
			<td colspan="4" style="padding: 20px; text-align: center; color: #64748b;">No tests run yet</td>
		</tr>`
	} else {
		for _, test := range tests {
			// Truncate response for preview
			preview := test.Response
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}

			// Escape HTML
			preview = strings.ReplaceAll(preview, "<", "&lt;")
			preview = strings.ReplaceAll(preview, ">", "&gt;")
			promptEsc := strings.ReplaceAll(test.Prompt, "<", "&lt;")
			promptEsc = strings.ReplaceAll(promptEsc, ">", "&gt;")

			html += fmt.Sprintf(`
			<tr style="border-bottom: 1px solid #1e293b;">
				<td style="padding: 12px; color: #e2e8f0;">%s</td>
				<td style="padding: 12px; color: #cbd5e1;">%s</td>
				<td style="padding: 12px; color: #94a3b8; font-family: monospace; font-size: 12px;">%s</td>
				<td style="padding: 12px; color: #10b981;">%dms</td>
			</tr>
			`, test.ModelName, promptEsc, preview, test.DurationMs)
		}
	}

	html += `</tbody></table>`

	w.Write([]byte(html))
}

// HandleRunBenchmark runs the full benchmark suite against a model server
func (s *Server) HandleRunBenchmark(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var modelPath string
	if r.Header.Get("Content-Type") == "application/json" {
		var payload struct {
			ModelPath string `json:"model_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		modelPath = payload.ModelPath
	} else {
		modelPath = r.FormValue("model_path")
	}

	if modelPath == "" {
		http.Error(w, "model_path required", http.StatusBadRequest)
		return
	}

	// Check if server is running
	server, exists := s.modelManager.GetServerByModelPath(modelPath)
	if !exists {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	if server.Status != "running" {
		http.Error(w, "Server is not running. Please start the server first.", http.StatusConflict)
		return
	}

	// Perform health check
	healthURL := fmt.Sprintf("http://localhost:%d/health", server.Port)
	if !s.modelManager.checkServerHealth(healthURL) {
		http.Error(w, "Server health check failed", http.StatusServiceUnavailable)
		return
	}

	// Return immediate feedback that benchmark started
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, `<div id="benchmark_status_msg" style="padding: 12px; background: #2563eb; color: white; border-radius: 4px; margin: 10px 0;">
		<strong>🚀 Benchmark started for %s</strong><br>
		<div style="margin-top: 8px;">Running %d agentic tests... This may take a few minutes.</div>
		<div style="margin-top: 8px; font-size: 12px; opacity: 0.8;">
			View results on the <a href="/testing" style="color: white; text-decoration: underline;">Testing &amp; Results</a> tab.
		</div>
	</div>`, server.ModelName, len(llm.AgenticBenchmarkSuite))

	// Run benchmarks asynchronously
	go s.runAgenticBenchmarks(server)
}

// runAgenticBenchmarks runs the agentic benchmark suite asynchronously
func (s *Server) runAgenticBenchmarks(server *ModelServer) {
	startTime := time.Now()

	// Create benchmark run record
	run := &db.AgenticBenchmarkRun{
		ModelName:   server.ModelName,
		ModelPath:   server.ModelPath,
		TotalTests:  len(llm.AgenticBenchmarkSuite),
		PassedTests: 0,
		FailedTests: 0,
		SuccessRate: 0.0,
		StartedAt:   startTime,
	}

	runID, err := s.store.SaveAgenticBenchmarkRun(run)
	if err != nil {
		log.Printf("Failed to save benchmark run: %v", err)
		return
	}
	run.ID = int(runID)

	log.Printf("Starting agentic benchmark run %d for %s", runID, server.ModelName)

	// Run each test
	for _, test := range llm.AgenticBenchmarkSuite {
		testStart := time.Now()

		result := &db.AgenticBenchmarkResult{
			RunID:           int(runID),
			TestName:        test.Name,
			TaskDescription: test.Description,
			Prompt:          test.Prompt,
			ExpectedResult:  "See validator function",
			CreatedAt:       testStart,
		}

		// Send prompt to model to generate code
		code, err := s.sendPromptToModel(server, test.Prompt)
		if err != nil {
			result.Passed = false
			result.ErrorMessage.Valid = true
			result.ErrorMessage.String = fmt.Sprintf("Failed to get response from model: %v", err)
			result.DurationMs = time.Since(testStart).Milliseconds()
			s.store.SaveAgenticBenchmarkResult(result)
			run.FailedTests++
			log.Printf("  ❌ %s: %v", test.Name, err)
			s.broadcastBenchmarkUpdate()
			continue
		}

		result.GeneratedCode.Valid = true
		result.GeneratedCode.String = code

		// Extract code from response (supports bash, python, js, go)
		extractedCode, language := extractCode(code)
		if extractedCode == "" {
			result.Passed = false
			result.ErrorMessage.Valid = true
			result.ErrorMessage.String = "No executable code found in model response"
			result.DurationMs = time.Since(testStart).Milliseconds()
			s.store.SaveAgenticBenchmarkResult(result)
			run.FailedTests++
			log.Printf("  ❌ %s: No code found in response", test.Name)
			s.broadcastBenchmarkUpdate()
			continue
		}

		// Execute the code
		output, execErr := tools.ExecuteCode(language, extractedCode)
		result.ExecutionOutput.Valid = true
		result.ExecutionOutput.String = output

		if execErr != nil {
			result.Passed = false
			result.ErrorMessage.Valid = true
			result.ErrorMessage.String = fmt.Sprintf("Code execution failed: %v", execErr)
			result.DurationMs = time.Since(testStart).Milliseconds()
			s.store.SaveAgenticBenchmarkResult(result)
			run.FailedTests++
			log.Printf("  ❌ %s: Execution failed: %v", test.Name, execErr)
			s.broadcastBenchmarkUpdate()
			continue
		}

		// Validate the output
		passed, reason := test.Validator(output)
		result.Passed = passed
		result.ValidationReason.Valid = true
		result.ValidationReason.String = reason
		result.DurationMs = time.Since(testStart).Milliseconds()

		if passed {
			run.PassedTests++
			log.Printf("  ✅ %s: %s (%dms)", test.Name, reason, result.DurationMs)
		} else {
			run.FailedTests++
			log.Printf("  ❌ %s: %s (%dms)", test.Name, reason, result.DurationMs)
		}

		s.store.SaveAgenticBenchmarkResult(result)

		// Update run stats after each test (for real-time progress)
		run.SuccessRate = float64(run.PassedTests) / float64(run.TotalTests) * 100
		s.store.UpdateAgenticBenchmarkRun(run)

		// Broadcast progress update after each test
		s.broadcastBenchmarkUpdate()
	}

	// Update run with final stats
	run.TotalDurationMs = time.Since(startTime).Milliseconds()
	run.SuccessRate = float64(run.PassedTests) / float64(run.TotalTests) * 100
	run.CompletedAt.Valid = true
	run.CompletedAt.Time = time.Now()

	if err := s.store.UpdateAgenticBenchmarkRun(run); err != nil {
		log.Printf("Failed to update benchmark run: %v", err)
	}

	log.Printf("Benchmark run %d complete: %d/%d passed (%.1f%%) in %dms",
		runID, run.PassedTests, run.TotalTests, run.SuccessRate, run.TotalDurationMs)

	// Broadcast completion event to SSE clients
	s.broadcastBenchmarkUpdate()
}

// sendPromptToModel sends a prompt to the model server with system prompt (agent-style) and returns the response
func (s *Server) sendPromptToModel(server *ModelServer, prompt string) (string, error) {
	serverURL := fmt.Sprintf("http://localhost:%d/v1/chat/completions", server.Port)

	// Use the same system prompt as main CLAI for consistency
	systemPrompt := `You are a free agent AI with full code execution capabilities. You can execute bash, python, and javascript code directly on the system.

**Critical rules:**
1. When you need to write code, wrap it in XML tags:
   <code language="bash">cat /path/to/file</code>
   <code language="go">package main...</code>
   <code language="python">print("Hello")</code>
   <code language="javascript">console.log("Hello")</code>

2. DO NOT use echo/print/console.log to narrate your thinking. Only use them for actual task output.
   ❌ BAD: <code language="bash">echo "Now I will write the program"</code>
   ✅ GOOD: <code language="go">package main...</code>

3. Keep code blocks focused and purposeful.
4. When asked to write a program, provide complete, executable code.

Answer questions clearly and provide code when needed.`

	requestBody := map[string]interface{}{
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": prompt},
		},
		"max_tokens":  1000,
		"temperature": 0.1, // Low temperature for more deterministic code
		"stream":      false,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(serverURL, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyText, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("model returned error (status %d): %s", resp.StatusCode, bodyText)
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response from model")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// extractCode extracts code from a response that may contain XML <code> tags or markdown code blocks
// Returns the code and the language (bash, python, javascript, go, etc.)
func extractCode(response string) (string, string) {
	// Priority order: bash, python, javascript, go (since tasks often use these)
	languages := []string{"bash", "python", "javascript", "js", "go", "golang"}

	// First: Try XML <code language="..."> tags (agent-style)
	for _, lang := range languages {
		tag := fmt.Sprintf(`<code language="%s">`, lang)
		if strings.Contains(response, tag) {
			start := strings.Index(response, tag) + len(tag)
			end := strings.Index(response[start:], "</code>")
			if end > 0 {
				code := strings.TrimSpace(response[start : start+end])
				// Normalize js -> javascript
				if lang == "js" {
					lang = "javascript"
				}
				return code, lang
			}
			// Handle unclosed code tag
			code := strings.TrimSpace(response[start:])
			if len(code) > 0 {
				if lang == "js" {
					lang = "javascript"
				}
				return code, lang
			}
		}
	}

	// Second: Try simplified XML <code language> tags (without quotes) - NEW FORMAT
	for _, lang := range languages {
		tag := fmt.Sprintf(`<code %s>`, lang)
		if strings.Contains(response, tag) {
			start := strings.Index(response, tag) + len(tag)
			end := strings.Index(response[start:], "</code>")
			if end > 0 {
				code := strings.TrimSpace(response[start : start+end])
				// Normalize js -> javascript
				if lang == "js" {
					lang = "javascript"
				} else if lang == "golang" {
					lang = "go"
				}
				return code, lang
			}
			// Handle unclosed code tag
			code := strings.TrimSpace(response[start:])
			if len(code) > 0 {
				if lang == "js" {
					lang = "javascript"
				} else if lang == "golang" {
					lang = "go"
				}
				return code, lang
			}
		}
	}

	// Also check for malformed tags: <code language (missing >)
	for _, lang := range languages {
		tag := fmt.Sprintf(`<code %s`, lang)
		tagIdx := strings.Index(response, tag)
		if tagIdx >= 0 {
			// Check if next character is whitespace or newline (indicating incomplete tag)
			start := tagIdx + len(tag)
			if start < len(response) && (response[start] == ' ' || response[start] == '\n' || response[start] == '\r' || response[start] == '\t') {
				// Skip whitespace
				for start < len(response) && (response[start] == ' ' || response[start] == '\n' || response[start] == '\r' || response[start] == '\t') {
					start++
				}
				// Try to find closing tag
				end := strings.Index(response[start:], "</code>")
				if end > 0 {
					code := strings.TrimSpace(response[start : start+end])
					if lang == "js" {
						lang = "javascript"
					} else if lang == "golang" {
						lang = "go"
					}
					return code, lang
				}
				// No closing tag, take rest of response
				code := strings.TrimSpace(response[start:])
				if len(code) > 0 {
					if lang == "js" {
						lang = "javascript"
					} else if lang == "golang" {
						lang = "go"
					}
					return code, lang
				}
			}
		}
	}

	// Second: Try markdown ```language code blocks
	for _, lang := range languages {
		marker := "```" + lang
		if strings.Contains(response, marker) {
			start := strings.Index(response, marker) + len(marker)
			// Skip to newline if present
			if idx := strings.Index(response[start:], "\n"); idx >= 0 {
				start += idx + 1
			}
			end := strings.Index(response[start:], "```")
			if end > 0 {
				code := strings.TrimSpace(response[start : start+end])
				if lang == "js" {
					lang = "javascript"
				}
				return code, lang
			}
			// Handle unclosed code block
			code := strings.TrimSpace(response[start:])
			if len(code) > 0 {
				if lang == "js" {
					lang = "javascript"
				}
				return code, lang
			}
		}
	}

	// Third: Try generic ``` code blocks - assume bash for shell commands
	if strings.Contains(response, "```") {
		start := strings.Index(response, "```")
		start += 3
		// Skip language identifier if present
		if idx := strings.Index(response[start:], "\n"); idx > 0 {
			start += idx + 1
		}
		end := strings.Index(response[start:], "```")
		if end > 0 {
			code := strings.TrimSpace(response[start : start+end])
			// Heuristic: if it has common bash commands, treat as bash
			if strings.Contains(code, "cat ") || strings.Contains(code, "ls ") ||
				strings.Contains(code, "grep ") || strings.Contains(code, "echo ") ||
				strings.Contains(code, "wc ") || strings.Contains(code, "awk ") {
				return code, "bash"
			}
			// If it starts with python imports/keywords, treat as python
			if strings.HasPrefix(code, "import ") || strings.HasPrefix(code, "from ") ||
				strings.HasPrefix(code, "def ") || strings.Contains(code, "print(") {
				return code, "python"
			}
			// If it has Go package declaration, treat as go
			if strings.Contains(code, "package main") || strings.Contains(code, "func main()") {
				return code, "go"
			}
			// Default to bash for unknown generic blocks
			return code, "bash"
		}
	}

	// Fourth: Check if entire response looks like code
	trimmed := strings.TrimSpace(response)

	// Check for Go code
	if strings.HasPrefix(trimmed, "package main") {
		return trimmed, "go"
	}

	// Check for Python code
	if strings.HasPrefix(trimmed, "#!/usr/bin/env python") ||
		strings.HasPrefix(trimmed, "import ") ||
		strings.HasPrefix(trimmed, "from ") {
		return trimmed, "python"
	}

	// Check for bash/shell commands
	if strings.HasPrefix(trimmed, "#!/bin/bash") ||
		strings.HasPrefix(trimmed, "cat ") ||
		strings.HasPrefix(trimmed, "ls ") ||
		strings.HasPrefix(trimmed, "echo ") {
		return trimmed, "bash"
	}

	return "", ""
}

// HandleGetBenchmarkResults returns the recent agentic benchmark results as HTML
func (s *Server) HandleGetBenchmarkResults(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.GetRecentAgenticBenchmarkRuns(10)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get benchmark runs: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	if len(runs) == 0 {
		fmt.Fprint(w, `<div id="benchmark_results_table"><p style="color: #94a3b8; font-style: italic;">No benchmark runs yet. Click "Run Benchmarks" on a running model to start.</p></div>`)
		return
	}

	// Build HTML table with wrapper div
	html := `<div id="benchmark_results_table"><table style="width: 100%; border-collapse: collapse;">
		<thead>
			<tr style="border-bottom: 1px solid #475569;">
				<th style="padding: 8px; text-align: left;">Model</th>
				<th style="padding: 8px; text-align: center;">Tests</th>
				<th style="padding: 8px; text-align: center;">Passed</th>
				<th style="padding: 8px; text-align: center;">Failed</th>
				<th style="padding: 8px; text-align: center;">Success Rate</th>
				<th style="padding: 8px; text-align: right;">Duration</th>
				<th style="padding: 8px; text-align: right;">Started</th>
			</tr>
		</thead>
		<tbody>`

	for _, run := range runs {
		statusColor := "#10b981" // green
		if run.SuccessRate < 50 {
			statusColor = "#ef4444" // red
		} else if run.SuccessRate < 80 {
			statusColor = "#f59e0b" // yellow
		}

		duration := fmt.Sprintf("%.1fs", float64(run.TotalDurationMs)/1000.0)
		if !run.CompletedAt.Valid {
			duration = "Running..."
		}

		html += fmt.Sprintf(`
			<tr 
				style="border-bottom: 1px solid #334155; cursor: pointer;" 
				hx-get="/api/test/results/detailed?run_id=%d"
				hx-target="#detailed_results"
				hx-swap="morph:outerHTML"
				onmouseover="this.style.backgroundColor='#1e293b'"
				onmouseout="this.style.backgroundColor='transparent'"
			>
				<td style="padding: 8px; color: #e2e8f0;">%s</td>
				<td style="padding: 8px; text-align: center; color: #cbd5e1;">%d</td>
				<td style="padding: 8px; text-align: center; color: #10b981;">%d</td>
				<td style="padding: 8px; text-align: center; color: #ef4444;">%d</td>
				<td style="padding: 8px; text-align: center; color: %s; font-weight: bold;">%.1f%%</td>
				<td style="padding: 8px; text-align: right; color: #94a3b8; font-size: 12px;">%s</td>
				<td style="padding: 8px; text-align: right; color: #94a3b8; font-size: 12px;">%s</td>
			</tr>`,
			run.ID,
			htmlEscape(run.ModelName),
			run.TotalTests,
			run.PassedTests,
			run.FailedTests,
			statusColor,
			run.SuccessRate,
			duration,
			run.StartedAt.Format("Jan 02 15:04"),
		)
	}

	html += `
		</tbody>
	</table></div>`

	fmt.Fprint(w, html)
}

// handleClearBenchmarkResults deletes all benchmark runs and results
func (s *Server) handleClearBenchmarkResults(w http.ResponseWriter, r *http.Request) {
	// Only allow DELETE method
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Delete all benchmark runs and results
	if err := s.store.DeleteAllAgenticBenchmarkRuns(); err != nil {
		log.Printf("Failed to clear benchmark results: %v", err)
		http.Error(w, fmt.Sprintf("Failed to clear results: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("All benchmark runs and results cleared")

	// Return empty table
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<div id="benchmark_results_table"><p style="color: #94a3b8; font-style: italic;">No benchmark runs yet. Click "Run Benchmarks" on a running model to start.</p></div>`)
}

// HandleGPUStatus returns the GPU status dashboard HTML
func (s *Server) HandleGPUStatus(w http.ResponseWriter, r *http.Request) {
	log.Println("HandleGPUStatus: Starting...")
	gpus, err := gpu.GetGPUInfo()
	log.Println("HandleGPUStatus: Got GPU info")

	w.Header().Set("Content-Type", "text/html")

	if err != nil {
		// Show minimal error state with stable ID
		fmt.Fprintf(w, `<div id="gpu_status">
			<div style="background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 12px 20px; margin-bottom: 20px;">
				<span style="color: #ef4444; font-size: 13px;">⚠️ GPU unavailable</span>
			</div>
		</div>`)
		return
	}

	if len(gpus) == 0 {
		fmt.Fprint(w, `<div id="gpu_status">
			<div style="background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 12px 20px; margin-bottom: 20px;">
				<span style="color: #64748b; font-size: 13px;">No GPUs detected</span>
			</div>
		</div>`)
		return
	}

	// Get running servers with VRAM usage
	s.modelManager.UpdateVRAMUsage()
	servers := s.modelManager.GetServerStatus()

	// Calculate total VRAM used by models and other usage
	var totalModelVRAM int64
	var runningServers []*ModelServer
	for _, srv := range servers {
		if srv.Status == "running" && srv.VRAMUsageBytes > 0 {
			totalModelVRAM += srv.VRAMUsageBytes
			runningServers = append(runningServers, srv)
		}
	}

	// Start with stable wrapper ID matching template
	html := `<div id="gpu_status">`

	for _, g := range gpus {
		// Detect unified memory architecture (GTT available and significant)
		isUnifiedMemory := g.GTTTotalBytes > g.VRAMTotalBytes*2

		// Calculate VRAM percentage
		vramUsedPercent := float64(0)
		if g.VRAMTotalBytes > 0 {
			vramUsedPercent = float64(g.VRAMUsedBytes) / float64(g.VRAMTotalBytes) * 100
		}

		// Calculate GTT percentage
		gttUsedPercent := float64(0)
		if g.GTTTotalBytes > 0 {
			gttUsedPercent = float64(g.GTTUsedBytes) / float64(g.GTTTotalBytes) * 100
		}

		// Determine colors for VRAM
		vramColor := "#10b981" // green
		if vramUsedPercent > 90 {
			vramColor = "#ef4444" // red
		} else if vramUsedPercent > 75 {
			vramColor = "#f59e0b" // yellow
		}

		// Determine colors for GTT
		gttColor := "#10b981" // green
		if gttUsedPercent > 90 {
			gttColor = "#ef4444" // red
		} else if gttUsedPercent > 75 {
			gttColor = "#f59e0b" // yellow
		}

		tempColor := "#64748b" // gray default
		if g.Temperature > 80 {
			tempColor = "#ef4444" // red
		} else if g.Temperature > 70 {
			tempColor = "#f59e0b" // yellow
		}

		// Build simple progress bars (no segmentation for now - just solid bars)
		vramBar := fmt.Sprintf(
			`<div style="background: %s; height: 100%%; width: %.1f%%;" title="VRAM: %s / %s"></div>`,
			vramColor,
			vramUsedPercent,
			gpu.FormatBytes(g.VRAMUsedBytes),
			gpu.FormatBytes(g.VRAMTotalBytes),
		)

		gttBar := fmt.Sprintf(
			`<div style="background: %s; height: 100%%; width: %.1f%%;" title="GTT: %s / %s"></div>`,
			gttColor,
			gttUsedPercent,
			gpu.FormatBytes(g.GTTUsedBytes),
			gpu.FormatBytes(g.GTTTotalBytes),
		)

		// Build detailed memory breakdown list (shows process-level VRAM usage)
		segmentColors := []string{"#3b82f6", "#8b5cf6", "#ec4899", "#f97316", "#06b6d4"}
		var memoryBreakdown string

		if len(runningServers) > 0 {
			// Sort servers by memory usage (descending)
			sortedServers := make([]*ModelServer, len(runningServers))
			copy(sortedServers, runningServers)

			// Simple bubble sort by VRAM usage
			for i := 0; i < len(sortedServers)-1; i++ {
				for j := 0; j < len(sortedServers)-i-1; j++ {
					if sortedServers[j].VRAMUsageBytes < sortedServers[j+1].VRAMUsageBytes {
						sortedServers[j], sortedServers[j+1] = sortedServers[j+1], sortedServers[j]
					}
				}
			}

			for i, srv := range sortedServers {
				segmentColor := segmentColors[i%len(segmentColors)]

				// Calculate percentage against total memory (GTT for unified, VRAM for dedicated)
				memoryPercentage := float64(0)
				memoryLabel := "memory"
				if isUnifiedMemory && g.GTTTotalBytes > 0 {
					memoryPercentage = float64(srv.VRAMUsageBytes) / float64(g.GTTTotalBytes) * 100
					memoryLabel = "GPU memory"
				} else if g.VRAMTotalBytes > 0 {
					memoryPercentage = float64(srv.VRAMUsageBytes) / float64(g.VRAMTotalBytes) * 100
					memoryLabel = "VRAM"
				}

				// Truncate long model names
				displayName := srv.ModelName
				if len(displayName) > 45 {
					displayName = displayName[:42] + "..."
				}

				// Use PID as stable ID for idiomorph
				memoryBreakdown += fmt.Sprintf(`
					<div id="model_%d" style="display: flex; align-items: center; gap: 12px; padding: 8px 0; border-bottom: 1px solid #1e293b;">
						<div style="width: 12px; height: 12px; background: %s; border-radius: 2px; flex-shrink: 0;"></div>
						<div style="flex: 1; min-width: 0;">
							<div style="color: #e2e8f0; font-size: 13px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">%s</div>
							<div style="color: #64748b; font-size: 11px; margin-top: 2px;">Port %d • PID %d</div>
						</div>
						<div style="text-align: right; flex-shrink: 0;">
							<div style="color: %s; font-size: 13px; font-weight: 600;">%s</div>
							<div style="color: #64748b; font-size: 11px;">%.1f%% of %s</div>
						</div>
					</div>`,
					srv.PID,
					segmentColor,
					displayName,
					srv.Port,
					srv.PID,
					segmentColor,
					gpu.FormatBytes(srv.VRAMUsageBytes),
					memoryPercentage,
					memoryLabel,
				)
			}
		} else {
			memoryBreakdown = `
				<div id="model_empty" style="padding: 20px; text-align: center; color: #64748b; font-size: 13px;">
					No model servers currently running
				</div>`
		}

		// Build memory section HTML
		var memorySection string
		if isUnifiedMemory {
			// Unified memory: show single GTT bar (system RAM accessible by GPU)
			memorySection = fmt.Sprintf(`
			<div style="display: flex; align-items: center; gap: 10px; margin-bottom: 16px; padding-bottom: 16px; border-bottom: 1px solid #334155;">
				<span style="color: #94a3b8; font-weight: 500; min-width: 90px;">GPU Memory</span>
				<div style="flex: 1; background: #0f172a; border-radius: 4px; height: 10px; overflow: hidden;">
					%s
				</div>
				<span style="color: %s; font-weight: 600; min-width: 100px; text-align: right;">%s / %s</span>
				<span style="color: %s; font-weight: 600; min-width: 50px; text-align: right;">%.0f%%</span>
			</div>
			<div style="color: #64748b; font-size: 11px; margin-bottom: 16px; padding-bottom: 16px; border-bottom: 1px solid #334155;">
				Unified memory architecture - GPU uses system RAM directly
			</div>`,
				gttBar,
				gttColor, gpu.FormatBytes(g.GTTUsedBytes), gpu.FormatBytes(g.GTTTotalBytes),
				gttColor, gttUsedPercent,
			)
		} else {
			// Dedicated VRAM only
			memorySection = fmt.Sprintf(`
			<div style="display: flex; align-items: center; gap: 10px; margin-bottom: 16px; padding-bottom: 16px; border-bottom: 1px solid #334155;">
				<span style="color: #94a3b8; font-weight: 500; min-width: 90px;">VRAM</span>
				<div style="flex: 1; background: #0f172a; border-radius: 4px; height: 10px; overflow: hidden;">
					%s
				</div>
				<span style="color: %s; font-weight: 600; min-width: 100px; text-align: right;">%s / %s</span>
				<span style="color: %s; font-weight: 600; min-width: 50px; text-align: right;">%.0f%%</span>
			</div>`,
				vramBar,
				vramColor, gpu.FormatBytes(g.VRAMUsedBytes), gpu.FormatBytes(g.VRAMTotalBytes),
				vramColor, vramUsedPercent,
			)
		}

		html += fmt.Sprintf(`
		<div style="background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 16px 20px; margin-bottom: 20px;">
			<!-- GPU Stats Header -->
			<div style="display: flex; gap: 20px; color: #cbd5e1; margin-bottom: 16px; padding-bottom: 12px; border-bottom: 1px solid #334155; flex-wrap: wrap;">
				<div style="flex: 1; min-width: 150px;">
					<div style="color: #64748b; font-size: 11px; text-transform: uppercase; margin-bottom: 4px;">GPU</div>
					<div style="color: #e2e8f0; font-size: 14px; font-weight: 600;">%s</div>
				</div>
				<div style="text-align: center; min-width: 70px;">
					<div style="color: #64748b; font-size: 11px; text-transform: uppercase; margin-bottom: 4px;">Temp</div>
					<div style="color: %s; font-size: 14px; font-weight: 600;">%.0f°C</div>
				</div>
				<div style="text-align: center; min-width: 60px;">
					<div style="color: #64748b; font-size: 11px; text-transform: uppercase; margin-bottom: 4px;">Util</div>
					<div style="color: #3b82f6; font-size: 14px; font-weight: 600;">%.0f%%</div>
				</div>
				%s
				%s
			</div>
			
			%s
			
			<!-- Memory breakdown -->
			<div style="background: #0f172a; border-radius: 6px; padding: 12px;">
				<div style="color: #94a3b8; font-size: 12px; font-weight: 600; text-transform: uppercase; margin-bottom: 8px; padding: 0 8px;">Running Models</div>
				<div style="padding: 0 8px;">
					%s
				</div>
			</div>
		</div>`,
			func() string {
				if g.Name != "" {
					return g.Name
				}
				return g.ID
			}(),
			tempColor, g.Temperature,
			g.Utilization,
			func() string {
				if g.PowerUsageW > 0 {
					return fmt.Sprintf(`<div style="text-align: center; min-width: 70px;">
						<div style="color: #64748b; font-size: 11px; text-transform: uppercase; margin-bottom: 4px;">Power</div>
						<div style="color: #10b981; font-size: 14px; font-weight: 600;">%.1fW</div>
					</div>`, g.PowerUsageW)
				}
				return ""
			}(),
			func() string {
				if g.ClockGPUMHz > 0 || g.ClockMemMHz > 0 {
					clocks := ""
					if g.ClockGPUMHz > 0 {
						clocks = fmt.Sprintf("GPU: %dMHz", g.ClockGPUMHz)
					}
					if g.ClockMemMHz > 0 {
						if clocks != "" {
							clocks += " / "
						}
						clocks += fmt.Sprintf("Mem: %dMHz", g.ClockMemMHz)
					}
					return fmt.Sprintf(`<div style="text-align: center; min-width: 140px;">
						<div style="color: #64748b; font-size: 11px; text-transform: uppercase; margin-bottom: 4px;">Clocks</div>
						<div style="color: #a78bfa; font-size: 14px; font-weight: 600;">%s</div>
					</div>`, clocks)
				}
				return ""
			}(),
			memorySection,
			memoryBreakdown,
		)
	}

	html += `</div>`

	fmt.Fprint(w, html)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
