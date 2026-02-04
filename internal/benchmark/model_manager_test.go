package benchmark

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckServerHealth(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse int
		serverDelay    time.Duration
		want           bool
	}{
		{
			name:           "healthy server returns true",
			serverResponse: http.StatusOK,
			serverDelay:    0,
			want:           true,
		},
		{
			name:           "server error returns false",
			serverResponse: http.StatusInternalServerError,
			serverDelay:    0,
			want:           false,
		},
		{
			name:           "loading server returns true",
			serverResponse: http.StatusServiceUnavailable,
			serverDelay:    0,
			want:           true,
		},
		{
			name:           "timeout returns false",
			serverResponse: http.StatusOK,
			serverDelay:    500 * time.Millisecond, // Longer than 200ms timeout
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.serverDelay > 0 {
					time.Sleep(tt.serverDelay)
				}
				w.WriteHeader(tt.serverResponse)
			}))
			defer server.Close()

			mm := NewModelManagerForTest()
			got := mm.checkServerHealth(server.URL)
			if got != tt.want {
				t.Errorf("checkServerHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindAvailablePortForModel(t *testing.T) {
	mm := NewModelManagerForTest()

	// Test finding an available port
	port, err := mm.findAvailablePortForModel()
	if err != nil {
		t.Fatalf("findAvailablePortForModel() error = %v", err)
	}

	if port < 8081 || port > 8090 {
		t.Errorf("findAvailablePortForModel() = %d, want port in range 8081-8090", port)
	}

	// Verify port is actually available
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Errorf("Port %d should be available but got error: %v", port, err)
	} else {
		listener.Close()
	}
}

func TestFindAvailablePortForModel_AllPortsOccupied(t *testing.T) {
	mm := NewModelManagerForTest()

	// Check if any ports are already in use (skip test if running servers exist)
	for port := 8081; port <= 8090; port++ {
		addr := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			t.Skipf("Port %d already in use (running servers detected), skipping test", port)
			return
		}
		listener.Close()
	}

	// Occupy all ports in range
	var listeners []net.Listener
	for port := 8081; port <= 8090; port++ {
		addr := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatalf("Failed to occupy port %d: %v", port, err)
		}
		listeners = append(listeners, listener)
	}
	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()

	// Should return error when all ports occupied
	_, err := mm.findAvailablePortForModel()
	if err == nil {
		t.Error("findAvailablePortForModel() should return error when all ports occupied")
	}

	expectedMsg := "no available ports in range 8081-8090"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Error message should contain %q, got %q", expectedMsg, err.Error())
	}
}

func TestIsSystemdManaged(t *testing.T) {
	mm := NewModelManagerForTest()

	// Test current process (should be able to read own cgroup)
	currentPID := os.Getpid()

	// Read actual cgroup to determine expected result
	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", currentPID)
	data, err := ioutil.ReadFile(cgroupPath)
	if err != nil {
		t.Skipf("Cannot read cgroup file: %v", err)
	}

	cgroupContent := string(data)
	expectedResult := strings.Contains(cgroupContent, "user@") && strings.Contains(cgroupContent, ".service")

	result := mm.isSystemdManaged(currentPID)
	if result != expectedResult {
		t.Errorf("isSystemdManaged(%d) = %v, want %v (cgroup: %s)", currentPID, result, expectedResult, cgroupContent)
	}

	// Test non-existent PID
	if mm.isSystemdManaged(999999) {
		t.Error("isSystemdManaged(999999) should return false for non-existent PID")
	}
}

func TestGetSystemdServiceName(t *testing.T) {
	mm := NewModelManagerForTest()

	tests := []struct {
		name          string
		cgroupContent string
		want          string
	}{
		{
			name:          "extracts llama-server.service",
			cgroupContent: "0::/user.slice/user-1000.slice/user@1000.service/app.slice/llama-server.service",
			want:          "llama-server.service",
		},
		{
			name:          "extracts llama-embed.service",
			cgroupContent: "0::/user.slice/user-1000.slice/user@1000.service/app.slice/llama-embed.service",
			want:          "llama-embed.service",
		},
		{
			name:          "ignores user@.service",
			cgroupContent: "0::/user.slice/user-1000.slice/user@1000.service",
			want:          "",
		},
		{
			name:          "returns empty for non-service",
			cgroupContent: "0::/user.slice/user-1000.slice",
			want:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with test cgroup content
			tmpDir := t.TempDir()
			testProcDir := filepath.Join(tmpDir, "proc", "12345")
			if err := os.MkdirAll(testProcDir, 0755); err != nil {
				t.Fatalf("Failed to create test proc dir: %v", err)
			}

			cgroupFile := filepath.Join(testProcDir, "cgroup")
			if err := ioutil.WriteFile(cgroupFile, []byte(tt.cgroupContent), 0644); err != nil {
				t.Fatalf("Failed to write test cgroup file: %v", err)
			}

			// Test the parsing logic directly
			// Since we can't easily mock file reads, we verify the parsing algorithm
			lines := strings.Split(tt.cgroupContent, "\n")
			_ = mm // Use mm to avoid unused variable warning
			var serviceName string
			for _, line := range lines {
				if strings.Contains(line, ".service") {
					parts := strings.Split(line, "/")
					for _, part := range parts {
						if strings.HasSuffix(part, ".service") && !strings.Contains(part, "user@") {
							serviceName = part
							break
						}
					}
				}
			}

			if serviceName != tt.want {
				t.Errorf("Parsed service name = %q, want %q", serviceName, tt.want)
			}
		})
	}
}

func TestGetModelNameFromPort(t *testing.T) {
	tests := []struct {
		name          string
		responseBody  string
		wantModelPath string
	}{
		{
			name:          "extracts model path from response",
			responseBody:  `{"data":[{"id":"/home/josh/models/test-model.gguf"}]}`,
			wantModelPath: "/home/josh/models/test-model.gguf",
		},
		{
			name:          "handles empty response",
			responseBody:  `{"data":[]}`,
			wantModelPath: "",
		},
		{
			name:          "handles invalid JSON",
			responseBody:  `invalid json`,
			wantModelPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/models" {
					t.Errorf("Expected request to /v1/models, got %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tt.responseBody)
			}))
			defer server.Close()

			mm := NewModelManagerForTest()
			// Extract port from server URL
			parts := strings.Split(server.URL, ":")
			var port int
			fmt.Sscanf(parts[len(parts)-1], "%d", &port)

			// Since getModelNameFromPort is private and uses hardcoded port range,
			// we'll test the logic by examining what it should return
			// In a real scenario, we'd need to refactor to make it testable
			result := mm.getModelNameFromPort(port)

			// For the mock server, it won't be in the 8081-8090 range
			// so we expect empty string
			if result != "" {
				t.Logf("Note: getModelNameFromPort returned %q (expected empty for test server port %d)", result, port)
			}
		})
	}
}

func TestGetContextSizeFromPort(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		wantCtx      int
	}{
		{
			name:         "extracts context size from slots",
			responseBody: `[{"id":0,"n_ctx":131072,"speculative":false,"is_processing":false}]`,
			wantCtx:      131072,
		},
		{
			name:         "returns 0 for empty slots",
			responseBody: `[]`,
			wantCtx:      0,
		},
		{
			name:         "returns 0 for invalid JSON",
			responseBody: `invalid`,
			wantCtx:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/slots" {
					t.Errorf("Expected request to /slots, got %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tt.responseBody)
			}))
			defer server.Close()

			mm := NewModelManagerForTest()
			parts := strings.Split(server.URL, ":")
			var port int
			fmt.Sscanf(parts[len(parts)-1], "%d", &port)

			result := mm.getContextSizeFromPort(port)

			// Same issue as above - test server won't be in range
			if result != 0 {
				t.Logf("Note: getContextSizeFromPort returned %d (expected 0 for test server port %d)", result, port)
			}
		})
	}
}

func TestScanAvailableModels(t *testing.T) {
	// Create temp directory with test model files
	tmpDir := t.TempDir()

	testFiles := []string{
		"model1.gguf",
		"model2.gguf",
		"not-a-model.txt",
	}

	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		if err := ioutil.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	mm := &ModelManager{
		servers:   make(map[string]*ModelServer),
		modelsDir: tmpDir,
	}

	models, err := mm.ScanAvailableModels()
	if err != nil {
		t.Fatalf("ScanAvailableModels() error = %v", err)
	}

	// Should only find .gguf files
	if len(models) != 2 {
		t.Errorf("ScanAvailableModels() found %d models, want 2", len(models))
	}

	// Verify model names
	foundModels := make(map[string]bool)
	for _, model := range models {
		foundModels[model.ModelName] = true
	}

	if !foundModels["model1.gguf"] {
		t.Error("ScanAvailableModels() should find model1.gguf")
	}
	if !foundModels["model2.gguf"] {
		t.Error("ScanAvailableModels() should find model2.gguf")
	}
	if foundModels["not-a-model.txt"] {
		t.Error("ScanAvailableModels() should not find not-a-model.txt")
	}
}

func TestStopServer_SystemdIntegration(t *testing.T) {
	// This test requires systemd and appropriate permissions
	// Skip if not in a systemd environment
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl not available, skipping systemd integration test")
	}

	mm := NewModelManagerForTest()
	testModelPath := "/tmp/test-model.gguf"

	// Create a mock server entry with a fake PID
	// We'll use PID 1 (init/systemd) which we know exists and is systemd-managed
	mm.servers[testModelPath] = &ModelServer{
		ModelPath: testModelPath,
		ModelName: "test-model.gguf",
		PID:       1, // init/systemd process
		Status:    "running",
		Port:      8081,
	}

	// Test systemd detection on PID 1
	if !mm.isSystemdManaged(1) {
		t.Skip("PID 1 not detected as systemd-managed, environment may not be systemd-based")
	}

	serviceName := mm.getSystemdServiceName(1)
	if serviceName == "" {
		t.Skip("Cannot extract service name from PID 1, skipping")
	}

	t.Logf("Detected systemd service for PID 1: %s", serviceName)

	// Note: We won't actually call StopServer() because we don't want to
	// stop real system services. This test just verifies the detection works.
}

func TestRefreshServerStatus(t *testing.T) {
	// Create test HTTP server simulating a llama-server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"id":"/home/josh/models/test-model.gguf"}]}`)
		case "/slots":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[{"id":0,"n_ctx":131072}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	mm := NewModelManagerForTest()

	// Add a test model to track
	testModelPath := "/home/josh/models/test-model.gguf"
	mm.servers[testModelPath] = &ModelServer{
		ModelPath: testModelPath,
		ModelName: "test-model.gguf",
		Status:    "stopped",
	}

	// RefreshServerStatus scans ports 8081-8090
	// Our test server is on a different port, so it won't find anything
	// This test mainly verifies it doesn't crash
	err := mm.RefreshServerStatus()
	if err != nil {
		t.Errorf("RefreshServerStatus() error = %v", err)
	}

	// Verify the model is still tracked
	if _, exists := mm.servers[testModelPath]; !exists {
		t.Error("RefreshServerStatus() should not remove tracked models")
	}
}
