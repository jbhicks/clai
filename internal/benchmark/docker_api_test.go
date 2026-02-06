package benchmark

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// TestHandleListDockerImages tests the Docker images list API endpoint
func TestHandleListDockerImages(t *testing.T) {
	// Create a test server
	store := createTestStore(t)
	defer store.Close()

	server := NewServer(store)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/docker/images", nil)
	w := httptest.NewRecorder()

	// Call handler
	server.HandleListDockerImages(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Parse response
	var images map[string]*DockerImageInfo
	if err := json.Unmarshal(w.Body.Bytes(), &images); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify we got the expected images
	if len(images) != 5 {
		t.Errorf("Expected 5 images, got %d", len(images))
	}

	// Check that rocm-7.2 exists with correct fields
	if img, exists := images["rocm-7.2"]; exists {
		if img.Name != "rocm-7.2" {
			t.Errorf("Expected image name 'rocm-7.2', got '%s'", img.Name)
		}
		if img.Backend != "rocm" {
			t.Errorf("Expected backend 'rocm', got '%s'", img.Backend)
		}
		if !strings.Contains(img.FullImage, "kyuz0/amd-strix-halo-toolboxes") {
			t.Errorf("Expected image to contain kyuz0/amd-strix-halo-toolboxes, got %s", img.FullImage)
		}
	} else {
		t.Error("rocm-7.2 image not found in response")
	}
}

// TestHandleListDockerImagesMethodNotAllowed tests that only GET is allowed
func TestHandleListDockerImagesMethodNotAllowed(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	server := NewServer(store)

	// Test POST request (should fail)
	req := httptest.NewRequest(http.MethodPost, "/api/docker/images", nil)
	w := httptest.NewRecorder()

	server.HandleListDockerImages(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestHandlePullDockerImageValidation tests the image pull endpoint validation
func TestHandlePullDockerImageValidation(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	server := NewServer(store)

	// Test with missing image_tag
	form := url.Values{}
	req := httptest.NewRequest(http.MethodPost, "/api/docker/pull", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	server.HandlePullDockerImage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing image_tag, got %d", w.Code)
	}

	// Test with unknown image
	form = url.Values{}
	form.Set("image_tag", "unknown-image")
	req = httptest.NewRequest(http.MethodPost, "/api/docker/pull", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()

	server.HandlePullDockerImage(w, req)

	// Should return error since image doesn't exist
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for unknown image, got %d", w.Code)
	}
}

// TestHandlePullDockerImageMethodNotAllowed tests that only POST is allowed
func TestHandlePullDockerImageMethodNotAllowed(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	server := NewServer(store)

	req := httptest.NewRequest(http.MethodGet, "/api/docker/pull", nil)
	w := httptest.NewRecorder()

	server.HandlePullDockerImage(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestHandleDockerContainerStatusValidation tests the container status endpoint
func TestHandleDockerContainerStatusValidation(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	server := NewServer(store)

	// Test with missing container parameter
	req := httptest.NewRequest(http.MethodGet, "/api/docker/container/status", nil)
	w := httptest.NewRecorder()

	server.HandleDockerContainerStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing container, got %d", w.Code)
	}

	// Test with container parameter (will check non-existent container)
	req = httptest.NewRequest(http.MethodGet, "/api/docker/container/status?container=test-container", nil)
	w = httptest.NewRecorder()

	server.HandleDockerContainerStatus(w, req)

	// Should return 200 with status "stopped" for non-existent container
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["container"] != "test-container" {
		t.Errorf("Expected container 'test-container', got '%s'", response["container"])
	}

	if response["status"] != "stopped" {
		t.Errorf("Expected status 'stopped' for non-existent container, got '%s'", response["status"])
	}
}

// TestHandleDockerContainerStatusMethodNotAllowed tests that only GET is allowed
func TestHandleDockerContainerStatusMethodNotAllowed(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	server := NewServer(store)

	req := httptest.NewRequest(http.MethodPost, "/api/docker/container/status?container=test", nil)
	w := httptest.NewRecorder()

	server.HandleDockerContainerStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestModelManagerDockerMethods tests ModelManager Docker integration methods
func TestModelManagerDockerMethods(t *testing.T) {
	mm := NewModelManagerForTest()

	// Test GetDockerImages
	images := mm.GetDockerImages()
	if len(images) != 5 {
		t.Errorf("Expected 5 Docker images, got %d", len(images))
	}

	// Verify image properties
	for name, img := range images {
		if img == nil {
			t.Errorf("Image %s is nil", name)
			continue
		}
		if img.Name == "" {
			t.Errorf("Image %s has empty Name", name)
		}
		if img.FullImage == "" {
			t.Errorf("Image %s has empty FullImage", name)
		}
		if img.Backend == "" {
			t.Errorf("Image %s has empty Backend", name)
		}
		if !strings.Contains(img.FullImage, ":") {
			t.Errorf("Image %s FullImage should contain tag separator: %s", name, img.FullImage)
		}
	}
}

// TestStartServerWithDockerValidation tests validation in StartServerWithDocker
func TestStartServerWithDockerValidation(t *testing.T) {
	mm := NewModelManagerForTest()

	// Test with non-existent model
	err := mm.StartServerWithDocker("/nonexistent/model.gguf", 131072, 999, "rocm-7.2")
	if err == nil {
		t.Error("Expected error for non-existent model")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("Expected 'model not found' error, got: %v", err)
	}

	// Test with unknown image
	// Note: Can't easily test this without a valid model in the manager
}

// TestDockerLauncherPullImageUnknown tests pulling unknown image
func TestDockerLauncherPullImageUnknown(t *testing.T) {
	dl := NewDockerLauncher()

	err := dl.PullImage("unknown-image-tag")
	if err == nil {
		t.Error("Expected error for unknown image")
	}
	if !strings.Contains(err.Error(), "unknown image") {
		t.Errorf("Expected 'unknown image' error, got: %v", err)
	}
}

// TestDockerLauncherContainerLifecycle is an integration test for container lifecycle
// Note: This requires Docker to be running
func TestDockerLauncherContainerLifecycle(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	if os.Getenv("RUN_DOCKER_TESTS") != "1" {
		t.Skip("Set RUN_DOCKER_TESTS=1 to run Docker integration tests")
	}

	dl := NewDockerLauncher()

	// Test listing containers
	containers, err := dl.ListContainers()
	if err != nil {
		t.Errorf("ListContainers failed: %v", err)
	}
	if containers == nil {
		t.Error("ListContainers returned nil")
	}

	// Test checking status of non-existent container
	running, err := dl.GetContainerStatus("non-existent-test-container")
	if err != nil {
		t.Errorf("GetContainerStatus should not error for non-existent: %v", err)
	}
	if running {
		t.Error("Non-existent container should not be running")
	}

	// Test stopping non-existent container (should not error)
	err = dl.StopContainer("non-existent-test-container")
	if err != nil {
		t.Errorf("StopContainer should not error for non-existent: %v", err)
	}
}

// TestModelManagerDockerLauncherInitialized tests that DockerLauncher is properly initialized
func TestModelManagerDockerLauncherInitialized(t *testing.T) {
	mm := NewModelManagerForTest()

	if mm.dockerLauncher == nil {
		t.Fatal("ModelManager.dockerLauncher is nil")
	}

	// Verify it has images
	images := mm.dockerLauncher.GetAvailableImages()
	if len(images) == 0 {
		t.Error("DockerLauncher has no images")
	}
}

// TestDockerImageTypes tests that Docker images have correct types
func TestDockerImageTypes(t *testing.T) {
	dl := NewDockerLauncher()
	images := dl.GetAvailableImages()

	rocmCount := 0
	vulkanCount := 0

	for name, img := range images {
		switch img.Backend {
		case "rocm":
			rocmCount++
			if !strings.Contains(name, "rocm") {
				t.Errorf("ROCm image %s should have 'rocm' in name", name)
			}
		case "vulkan":
			vulkanCount++
			if !strings.Contains(name, "vulkan") {
				t.Errorf("Vulkan image %s should have 'vulkan' in name", name)
			}
		default:
			t.Errorf("Unknown backend type '%s' for image %s", img.Backend, name)
		}
	}

	if rocmCount != 3 {
		t.Errorf("Expected 3 ROCm images, got %d", rocmCount)
	}

	if vulkanCount != 2 {
		t.Errorf("Expected 2 Vulkan images, got %d", vulkanCount)
	}
}

// TestDockerImageDescriptions tests that all images have descriptions
func TestDockerImageDescriptions(t *testing.T) {
	dl := NewDockerLauncher()
	images := dl.GetAvailableImages()

	for name, img := range images {
		if img.Description == "" {
			t.Errorf("Image %s has no description", name)
		}
	}
}

// BenchmarkDockerLauncherGetAvailableImages benchmarks image listing
func BenchmarkDockerLauncherGetAvailableImages(b *testing.B) {
	dl := NewDockerLauncher()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dl.GetAvailableImages()
	}
}

// BenchmarkModelManagerGetDockerImages benchmarks ModelManager image retrieval
func BenchmarkModelManagerGetDockerImages(b *testing.B) {
	mm := NewModelManagerForTest()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mm.GetDockerImages()
	}
}
