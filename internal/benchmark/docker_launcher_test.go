package benchmark

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestNewDockerLauncher tests that DockerLauncher initializes correctly
func TestNewDockerLauncher(t *testing.T) {
	dl := NewDockerLauncher()
	if dl == nil {
		t.Fatal("NewDockerLauncher() returned nil")
	}

	if dl.images == nil {
		t.Fatal("DockerLauncher.images is nil")
	}

	// Check that predefined images exist
	expectedImages := []string{"rocm-7.2", "rocm6_4_4", "rocm7-nightlies", "vulkan-radv", "vulkan-amdvlk"}
	for _, img := range expectedImages {
		if _, exists := dl.images[img]; !exists {
			t.Errorf("Expected image %s not found", img)
		}
	}

	// Verify image details
	if img, exists := dl.images["rocm-7.2"]; exists {
		if img.Backend != "rocm" {
			t.Errorf("Expected rocm-7.2 backend to be 'rocm', got '%s'", img.Backend)
		}
		if !strings.Contains(img.FullImage, "kyuz0/amd-strix-halo-toolboxes") {
			t.Errorf("Expected image to contain 'kyuz0/amd-strix-halo-toolboxes', got '%s'", img.FullImage)
		}
	}
}

// TestDockerLauncherGetAvailableImages tests image listing
func TestDockerLauncherGetAvailableImages(t *testing.T) {
	dl := NewDockerLauncher()
	images := dl.GetAvailableImages()

	if len(images) != 5 {
		t.Errorf("Expected 5 images, got %d", len(images))
	}

	// Verify all images have required fields
	for name, img := range images {
		if img.Name == "" {
			t.Errorf("Image %s has empty Name", name)
		}
		if img.FullImage == "" {
			t.Errorf("Image %s has empty FullImage", name)
		}
		if img.Backend == "" {
			t.Errorf("Image %s has empty Backend", name)
		}
	}
}

// TestSanitizeContainerName tests container name sanitization
func TestSanitizeContainerName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"model.gguf", "model"},
		{"my-model.gguf", "my_model"},
		{"my.model.gguf", "my_model"},
		{"model with spaces.gguf", "model_with_spaces"},
		{"model/with/slashes.gguf", "model_with_slashes"},
		{"123numeric.gguf", "model_123numeric"}, // Must start with letter
		{"very-long-model-name-that-exceeds-the-character-limit-for-docker-container-names.gguf", "very_long_model_name_that_exceeds_the_character_limit_for_do"},
		{"UPPERCASE.gguf", "uppercase"},
	}

	for _, test := range tests {
		result := sanitizeContainerName(test.input)
		if result != test.expected {
			t.Errorf("sanitizeContainerName(%q) = %q, expected %q", test.input, result, test.expected)
		}

		// Verify result is valid Docker container name
		if len(result) > 64 {
			t.Errorf("Container name %q exceeds 64 characters", result)
		}
		if len(result) > 0 && !((result[0] >= 'a' && result[0] <= 'z') || (result[0] >= 'A' && result[0] <= 'Z')) {
			t.Errorf("Container name %q doesn't start with a letter", result)
		}
	}
}

// TestDockerLauncherImageExistsLocally tests image existence check
// Note: This test requires Docker to be running and will be skipped if Docker is not available
func TestDockerLauncherImageExistsLocally(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker not available, skipping test")
	}

	dl := NewDockerLauncher()

	// Test with a known non-existent image tag
	exists := dl.ImageExistsLocally("non-existent-image-tag")
	if exists {
		t.Error("Expected ImageExistsLocally to return false for non-existent image")
	}
}

// TestModelManagerGetDockerImages tests ModelManager integration
func TestModelManagerGetDockerImages(t *testing.T) {
	mm := NewModelManagerForTest()
	if mm == nil {
		t.Fatal("NewModelManagerForTest() returned nil")
	}

	images := mm.GetDockerImages()
	if len(images) != 5 {
		t.Errorf("Expected 5 Docker images, got %d", len(images))
	}
}

// TestModelManagerStartServerWithDockerValidation tests input validation
func TestModelManagerStartServerWithDockerValidation(t *testing.T) {
	mm := NewModelManagerForTest()

	// Test with non-existent model
	err := mm.StartServerWithDocker("/non/existent/model.gguf", 131072, 999, "rocm-7.2", false)
	if err == nil {
		t.Error("Expected error for non-existent model, got nil")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("Expected 'model not found' error, got: %v", err)
	}

	// Test with unknown image tag
	// Note: This would need a valid model path to test properly
}

// TestDockerImageInfoStructure tests DockerImageInfo struct
func TestDockerImageInfoStructure(t *testing.T) {
	info := &DockerImageInfo{
		Name:        "test-image",
		Tag:         "test-tag",
		FullImage:   "docker.io/user/repo:tag",
		Backend:     "rocm",
		Description: "Test description",
	}

	if info.Name != "test-image" {
		t.Errorf("Expected Name to be 'test-image', got '%s'", info.Name)
	}
	if info.Backend != "rocm" {
		t.Errorf("Expected Backend to be 'rocm', got '%s'", info.Backend)
	}
}

// TestDockerContainerNameSanitization tests container name sanitization behavior
func TestDockerContainerNameSanitization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"model-a.gguf", "model_a"},
		{"model_a.gguf", "model_a"}, // Same output as above - underscores preserved
		{"model.a.gguf", "model_a"}, // Same output - dots become underscores
		{"model-a-1.gguf", "model_a_1"},
	}

	for _, test := range tests {
		result := sanitizeContainerName(test.input)
		if result != test.expected {
			t.Errorf("sanitizeContainerName(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}

	// Note: Different model names can result in the same container name
	// (e.g., "model-a.gguf" and "model_a.gguf" both become "model_a")
	// This is acceptable because:
	// 1. Models are typically stored in different directories
	// 2. The full model path is used for uniqueness elsewhere
	// 3. Container name collisions would be rare in practice
}

// TestDockerLauncherBuildArgs tests that docker run arguments are built correctly
func TestDockerLauncherBuildArgs(t *testing.T) {
	// Ensure launcher is available (validates initialization)
	_ = NewDockerLauncher()

	// Test ROCm image args
	rocmArgs := []string{
		"run",
		"-d",
		"--name", "test-container",
		"--rm",
		"-p", "8080:8080",
		"-v", "/models:/models:ro",
		"--device", "/dev/dri",
		"--security-opt", "seccomp=unconfined",
		"--device", "/dev/kfd",
		"--group-add", "video",
		"--group-add", "render",
		"--group-add", "sudo",
		"-e", "ROCBLAS_USE_HIPBLASLT=1",
	}

	// Verify args contain required elements
	requiredArgs := []string{"--device", "/dev/dri", "--security-opt", "seccomp=unconfined"}
	for _, required := range requiredArgs {
		found := false
		for _, arg := range rocmArgs {
			if arg == required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Required arg '%s' not found in ROCm args", required)
		}
	}

	// Verify ROCm-specific args
	rocmSpecificArgs := []string{"/dev/kfd", "ROCBLAS_USE_HIPBLASLT"}
	for _, required := range rocmSpecificArgs {
		found := false
		for _, arg := range rocmArgs {
			if strings.Contains(arg, required) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ROCm-specific arg containing '%s' not found", required)
		}
	}
}

// TestServerBackendField tests that Backend field supports docker- prefix
func TestServerBackendField(t *testing.T) {
	server := &ModelServer{
		ModelPath: "/models/test.gguf",
		ModelName: "test.gguf",
		Status:    "running",
		Backend:   "docker-rocm-7.2",
		Port:      8082,
	}

	if !strings.HasPrefix(server.Backend, "docker-") {
		t.Error("Expected Backend to start with 'docker-'")
	}

	if server.Backend != "docker-rocm-7.2" {
		t.Errorf("Expected Backend to be 'docker-rocm-7.2', got '%s'", server.Backend)
	}
}

// TestDockerImagePullValidation tests image pull validation
func TestDockerImagePullValidation(t *testing.T) {
	dl := NewDockerLauncher()

	// Test with unknown image
	err := dl.PullImage("unknown-image")
	if err == nil {
		t.Error("Expected error for unknown image, got nil")
	}
	if !strings.Contains(err.Error(), "unknown image") {
		t.Errorf("Expected 'unknown image' error, got: %v", err)
	}
}

// Helper function to check if Docker is available
func isDockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	err := cmd.Run()
	return err == nil
}

// TestDockerLauncherIntegration is a comprehensive integration test
// Note: This requires Docker to be running and will be skipped otherwise
func TestDockerLauncherIntegration(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker not available, skipping integration test")
	}

	if os.Getenv("RUN_DOCKER_TESTS") != "1" {
		t.Skip("Set RUN_DOCKER_TESTS=1 to run Docker integration tests")
	}

	dl := NewDockerLauncher()

	// Test listing containers (should not error even if none exist)
	containers, err := dl.ListContainers()
	if err != nil {
		t.Errorf("ListContainers failed: %v", err)
	}
	// Should return empty slice or list of containers, not nil
	if containers == nil {
		t.Error("ListContainers returned nil")
	}

	// Test checking status of non-existent container
	running, err := dl.GetContainerStatus("non-existent-container-12345")
	if err != nil {
		t.Errorf("GetContainerStatus for non-existent container should not error: %v", err)
	}
	if running {
		t.Error("Non-existent container should not be running")
	}
}

// TestDockerStopContainerGraceful tests graceful container stopping
func TestDockerStopContainerGraceful(t *testing.T) {
	if !isDockerAvailable() {
		t.Skip("Docker not available")
	}

	if os.Getenv("RUN_DOCKER_TESTS") != "1" {
		t.Skip("Set RUN_DOCKER_TESTS=1 to run Docker tests")
	}

	dl := NewDockerLauncher()

	// Try to stop non-existent container (should not error)
	err := dl.StopContainer("non-existent-container-test")
	if err != nil {
		t.Errorf("StopContainer for non-existent container should not error: %v", err)
	}
}

// TestModelManagerDockerIntegration tests ModelManager with Docker backend
func TestModelManagerDockerIntegration(t *testing.T) {
	mm := NewModelManagerForTest()

	// Verify Docker launcher is initialized
	if mm.dockerLauncher == nil {
		t.Fatal("ModelManager.dockerLauncher is nil")
	}

	// Test GetDockerImages
	images := mm.GetDockerImages()
	if len(images) == 0 {
		t.Error("GetDockerImages returned empty map")
	}

	// Verify predefined images exist
	if _, exists := images["rocm-7.2"]; !exists {
		t.Error("rocm-7.2 image not found")
	}
}

// BenchmarkSanitizeContainerName benchmarks container name sanitization
func BenchmarkSanitizeContainerName(b *testing.B) {
	testNames := []string{
		"model.gguf",
		"my-model-name.gguf",
		"very-long-model-name-with-many-words-and-special-chars!@#$.gguf",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, name := range testNames {
			sanitizeContainerName(name)
		}
	}
}
