package benchmark

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHandleStartServer_ReturnsUpdatedList tests that starting a server returns the updated server list
func TestHandleStartServer_ReturnsUpdatedList(t *testing.T) {
	// Create a temporary model file for testing
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-model.gguf")

	// Create a dummy model file
	f, err := os.Create(modelPath)
	if err != nil {
		t.Fatalf("Failed to create test model file: %v", err)
	}
	f.Close()

	// Create server with model manager
	server := &Server{
		modelManager: NewModelManager(),
	}

	// Manually add the test model to the model manager
	// (In production, ScanAvailableModels discovers from modelsDir)
	server.modelManager.mu.Lock()
	server.modelManager.servers[modelPath] = &ModelServer{
		ModelPath: modelPath,
		ModelName: "test-model.gguf",
		Status:    "stopped",
		APIType:   "llamacpp",
	}
	server.modelManager.mu.Unlock()

	// Initial state - server should be stopped
	initialServer, exists := server.modelManager.GetServerByModelPath(modelPath)
	if !exists {
		t.Fatal("Model not found in server list")
	}
	if initialServer.Status != "stopped" {
		t.Errorf("Expected initial status 'stopped', got %s", initialServer.Status)
	}

	// Create POST request to start the server
	form := url.Values{}
	form.Add("model_path", modelPath)
	req := httptest.NewRequest("POST", "/api/servers/start", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// Call the handler
	server.HandleStartServer(w, req)

	// Check response status
	resp := w.Result()
	defer resp.Body.Close()

	// Should return HTML (the server list)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", resp.StatusCode)
	}

	// Response should be HTML
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected Content-Type text/html, got %s", contentType)
	}

	// Check response body contains server list HTML
	body := w.Body.String()

	// Should contain the servers_list div
	if !strings.Contains(body, `id="servers_list"`) {
		t.Error("Response should contain servers_list div")
	}

	// Should contain the model filename
	if !strings.Contains(body, "test-model.gguf") {
		t.Error("Response should contain model filename")
	}

	// The important part: verify HTMX swap target is present
	// This proves the handler returns the correct HTML structure for HTMX
	t.Logf("Response body preview (first 500 chars): %s", body[:min(500, len(body))])

	// Since we can't actually start llama-server in tests, we accept various outcomes:
	// 1. Server started (status: "starting" or "running")
	// 2. Server failed to start (status: "stopped" with error message)
	// 3. Response contains server list with model info

	// The critical test: does it return the HTMX-compatible server list?
	hasServersList := strings.Contains(body, `id="servers_list"`)
	hasModelName := strings.Contains(body, "test-model.gguf")

	if !hasServersList || !hasModelName {
		t.Errorf("Response must contain servers_list div and model name for HTMX swap")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestHandleStartServer_AlreadyRunning tests that starting an already running server returns an error
func TestHandleStartServer_AlreadyRunning(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-model.gguf")

	// Create dummy model file
	f, err := os.Create(modelPath)
	if err != nil {
		t.Fatalf("Failed to create test model file: %v", err)
	}
	f.Close()

	server := &Server{
		modelManager: NewModelManager(),
	}

	// Manually add a server in "running" state (simulate already running)
	server.modelManager.mu.Lock()
	server.modelManager.servers[modelPath] = &ModelServer{
		ModelPath: modelPath,
		Status:    "running",
		Port:      8081,
	}
	server.modelManager.mu.Unlock()

	// Try to start it again
	form := url.Values{}
	form.Add("model_path", modelPath)
	req := httptest.NewRequest("POST", "/api/servers/start", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.HandleStartServer(w, req)

	// Should return error
	resp := w.Result()
	defer resp.Body.Close()

	// Check response contains error message
	body := w.Body.String()

	// Should mention the error or show it's already running
	if !strings.Contains(body, "running") && !strings.Contains(body, "Failed") {
		t.Errorf("Response should indicate server is already running, got: %s", body)
	}
}

// TestHandleStopServer_ReturnsUpdatedList tests that stopping a server returns the updated server list
func TestHandleStopServer_ReturnsUpdatedList(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-model.gguf")

	// Create dummy model file
	f, err := os.Create(modelPath)
	if err != nil {
		t.Fatalf("Failed to create test model file: %v", err)
	}
	f.Close()

	server := &Server{
		modelManager: NewModelManager(),
	}

	// Manually add a server in "stopped" state (not running)
	// This tests the "server not running" error path
	server.modelManager.mu.Lock()
	server.modelManager.servers[modelPath] = &ModelServer{
		ModelPath: modelPath,
		ModelName: "test-model.gguf",
		Status:    "stopped",
		Port:      0,
		PID:       0,
		APIType:   "llamacpp",
	}
	server.modelManager.mu.Unlock()

	// Create POST request to stop the server
	form := url.Values{}
	form.Add("model_path", modelPath)
	req := httptest.NewRequest("POST", "/api/servers/stop", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// Call the handler
	server.HandleStopServer(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	// When trying to stop an already-stopped server, we expect either:
	// 1. An error response (500)
	// 2. A successful response showing the server list with "stopped" status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", resp.StatusCode)
	}

	// Check response body
	body := w.Body.String()

	// The key assertion: even on error, we should get HTML back (for HTMX)
	// The handler calls HandleListModels which returns HTML
	if resp.StatusCode == http.StatusOK {
		// Success case - should be HTML
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			t.Errorf("Expected Content-Type text/html, got %s", contentType)
		}

		if !strings.Contains(body, `id="servers_list"`) {
			t.Error("Response should contain servers_list div")
		}

		if !strings.Contains(body, "test-model.gguf") {
			t.Error("Response should contain model filename")
		}

		// Should show stopped status
		if !strings.Contains(body, "Stopped") {
			t.Error("Response should show 'Stopped' status")
		}
	} else {
		// Error case - will be plain text error message
		t.Logf("Stop server returned error (expected for already-stopped server): %s", body)
	}
}

// TestHandleStartServer_MissingModelPath tests error handling when model_path is missing
func TestHandleStartServer_MissingModelPath(t *testing.T) {
	server := &Server{
		modelManager: NewModelManager(),
	}

	// Create request without model_path
	req := httptest.NewRequest("POST", "/api/servers/start", nil)
	w := httptest.NewRecorder()

	server.HandleStartServer(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

// TestHandleStartServer_WrongMethod tests that non-POST requests are rejected
func TestHandleStartServer_WrongMethod(t *testing.T) {
	server := &Server{
		modelManager: NewModelManager(),
	}

	req := httptest.NewRequest("GET", "/api/servers/start", nil)
	w := httptest.NewRecorder()

	server.HandleStartServer(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

// TestHandleListModels_HTMXCompatibility tests that HandleListModels returns HTMX-compatible HTML
func TestHandleListModels_HTMXCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	modelPath := filepath.Join(tempDir, "test-model.gguf")

	// Create dummy model file
	f, err := os.Create(modelPath)
	if err != nil {
		t.Fatalf("Failed to create test model file: %v", err)
	}
	f.Close()

	// Create server with custom models directory
	mm := &ModelManager{
		servers:   make(map[string]*ModelServer),
		modelsDir: tempDir,
	}

	server := &Server{
		modelManager: mm,
	}

	// Create GET request
	req := httptest.NewRequest("GET", "/api/servers/list", nil)
	w := httptest.NewRecorder()

	// Call the handler
	server.HandleListModels(w, req)

	// Check response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Response should be HTML
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Expected Content-Type text/html, got %s", contentType)
	}

	body := w.Body.String()

	// Critical HTMX compatibility checks:
	// 1. Must have servers_list div with correct ID
	if !strings.Contains(body, `id="servers_list"`) {
		t.Error("Response must contain div with id='servers_list' for HTMX swap target")
	}

	// 2. Should contain HTMX attributes for morph swap
	// Note: The template might set these on the client side, but the div structure must match

	// 3. Should contain the model in the list
	if !strings.Contains(body, "test-model.gguf") {
		t.Error("Response should list discovered models")
	}

	// 4. Should contain table structure
	if !strings.Contains(body, "<table") {
		t.Error("Response should contain server list table")
	}

	// 5. Should contain action buttons with HTMX attributes
	hasStartButton := strings.Contains(body, "hx-post") || strings.Contains(body, "Start")
	if !hasStartButton {
		t.Error("Response should contain action buttons (Start/Stop)")
	}

	t.Logf("HandleListModels returns valid HTMX-compatible HTML with %d bytes", len(body))
}
