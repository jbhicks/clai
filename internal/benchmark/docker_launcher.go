package benchmark

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DockerImageInfo holds information about available Docker images for model serving
type DockerImageInfo struct {
	Name        string `json:"name"`
	Tag         string `json:"tag"`
	FullImage   string `json:"full_image"`
	Backend     string `json:"backend"` // "rocm" or "vulkan"
	Description string `json:"description"`
}

// DockerLauncher manages Docker-based model servers
type DockerLauncher struct {
	images map[string]*DockerImageInfo
}

// NewDockerLauncher creates a new Docker launcher with predefined images
func NewDockerLauncher() *DockerLauncher {
	return &DockerLauncher{
		images: map[string]*DockerImageInfo{
			"rocm-7.2": {
				Name:        "rocm-7.2",
				Tag:         "rocm-7.2",
				FullImage:   "docker.io/kyuz0/amd-strix-halo-toolboxes:rocm-7.2",
				Backend:     "rocm",
				Description: "ROCm 7.2 - Latest stable ROCm build for Strix Halo",
			},
			"rocm6_4_4": {
				Name:        "rocm-6.4.4",
				Tag:         "rocm-6.4.4",
				FullImage:   "docker.io/kyuz0/amd-strix-halo-toolboxes:rocm-6.4.4",
				Backend:     "rocm",
				Description: "ROCm 6.4.4 - Stable 6.x build",
			},
			"rocm7-nightlies": {
				Name:        "rocm7-nightlies",
				Tag:         "rocm7-nightlies",
				FullImage:   "docker.io/kyuz0/amd-strix-halo-toolboxes:rocm7-nightlies",
				Backend:     "rocm",
				Description: "ROCm 7 Nightlies - Latest nightly builds",
			},
			"vulkan-radv": {
				Name:        "vulkan-radv",
				Tag:         "vulkan-radv",
				FullImage:   "docker.io/kyuz0/amd-strix-halo-toolboxes:vulkan-radv",
				Backend:     "vulkan",
				Description: "Vulkan RADV - Most stable and compatible",
			},
			"vulkan-amdvlk": {
				Name:        "vulkan-amdvlk",
				Tag:         "vulkan-amdvlk",
				FullImage:   "docker.io/kyuz0/amd-strix-halo-toolboxes:vulkan-amdvlk",
				Backend:     "vulkan",
				Description: "Vulkan AMDVLK - Fastest backend (2GiB buffer limit)",
			},
		},
	}
}

// GetAvailableImages returns all available Docker images
func (dl *DockerLauncher) GetAvailableImages() map[string]*DockerImageInfo {
	return dl.images
}

// PullImage pulls a Docker image from the registry
func (dl *DockerLauncher) PullImage(imageTag string) error {
	imageInfo, exists := dl.images[imageTag]
	if !exists {
		return fmt.Errorf("unknown image: %s", imageTag)
	}

	cmd := exec.Command("docker", "pull", imageInfo.FullImage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w\nOutput: %s", imageInfo.FullImage, err, string(output))
	}

	return nil
}

// StartContainer starts a Docker container with llama-server
func (dl *DockerLauncher) StartContainer(
	modelPath string,
	modelName string,
	port int,
	contextSize int,
	ngl int,
	imageTag string,
) (string, error) {
	imageInfo, exists := dl.images[imageTag]
	if !exists {
		return "", fmt.Errorf("unknown image: %s", imageTag)
	}

	// Generate container name
	containerName := fmt.Sprintf("clai-model-%s", sanitizeContainerName(modelName))

	// Get models directory path
	modelsDir := os.Getenv("MODELS_PATH")
	if modelsDir == "" {
		modelsDir = "/home/josh/models" // default fallback
	}

	// Get user info for proper permissions
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/home/josh"
	}

	// Create logs directory
	logsDir := fmt.Sprintf("%s/.local/share/clai/logs", homeDir)
	os.MkdirAll(logsDir, 0755)

	// Build docker run command
	args := []string{
		"run",
		"-d", // detached mode
		"--name", containerName,
		"--rm", // auto-remove when stopped
		"-p", fmt.Sprintf("%d:8080", port),
		"-v", fmt.Sprintf("%s:/models:ro", modelsDir),
		"-v", fmt.Sprintf("%s:/logs", logsDir),
		"--device", "/dev/dri",
		"--security-opt", "seccomp=unconfined",
	}

	// Add backend-specific device and group options
	if imageInfo.Backend == "rocm" {
		args = append(args,
			"--device", "/dev/kfd",
			"--group-add", "video",
			"--group-add", "render",
		)
		// On Arch, 'wheel' is used instead of 'sudo'
		if _, err := exec.Command("getent", "group", "wheel").Output(); err == nil {
			args = append(args, "--group-add", "wheel")
		} else if _, err := exec.Command("getent", "group", "sudo").Output(); err == nil {
			args = append(args, "--group-add", "sudo")
		}
	} else {
		// Vulkan
		args = append(args,
			"--group-add", "video",
		)
	}

	// Environment variables for ROCm
	envVars := []string{
		"-e", "ROCBLAS_USE_HIPBLASLT=1",
	}
	args = append(args, envVars...)

	// Add the image
	args = append(args, imageInfo.FullImage)

	// Add llama-server command with arguments
	// Note: Container has llama-server at /usr/local/bin/llama-server
	modelPathInContainer := fmt.Sprintf("/models/%s", modelName)
	logFileName := fmt.Sprintf("llama-server-%d.log", port)

	// Check if it's an embedding model
	isEmbeddingModel := strings.Contains(strings.ToLower(modelName), "embed")
	extraArgs := "--jinja"
	if isEmbeddingModel {
		extraArgs = "--embedding"
	}

	// We wrap the command in sh -c to allow redirection to the mounted /logs volume
	// This ensures parity with native servers which write to the same host log files
	shCommand := fmt.Sprintf("llama-server -m %s --host 0.0.0.0 --port 8080 -c %d -ngl %d -fa on --no-mmap -b 2048 -ub 512 --verbose %s > /logs/%s 2>&1",
		modelPathInContainer, contextSize, ngl, extraArgs, logFileName)

	args = append(args, "sh", "-c", shCommand)

	// Execute docker run
	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to start container: %w\nOutput: %s", err, string(output))
	}

	// Get container ID from output
	containerID := strings.TrimSpace(string(output))

	return containerID, nil
}

// StopContainer stops and removes a Docker container
func (dl *DockerLauncher) StopContainer(containerName string) error {
	cmd := exec.Command("docker", "stop", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Container might already be stopped
		if !strings.Contains(string(output), "No such container") {
			return fmt.Errorf("failed to stop container: %w\nOutput: %s", err, string(output))
		}
	}
	return nil
}

// GetContainerLogs retrieves logs from a container
func (dl *DockerLauncher) GetContainerLogs(containerName string, tail int) (string, error) {
	args := []string{"logs"}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	args = append(args, containerName)

	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}

	return string(output), nil
}

// GetContainerStatus checks if a container is running
func (dl *DockerLauncher) GetContainerStatus(containerName string) (bool, error) {
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Status}}", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, nil // Container doesn't exist
	}

	status := strings.TrimSpace(string(output))
	return status == "running", nil
}

// GetContainerPID retrieves the host PID of a container
func (dl *DockerLauncher) GetContainerPID(containerName string) (int, error) {
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Pid}}", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to get container PID: %w\nOutput: %s", err, string(output))
	}

	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &pid); err != nil {
		return 0, fmt.Errorf("failed to parse container PID: %w", err)
	}

	return pid, nil
}

// ListContainers lists all CLAI model containers
func (dl *DockerLauncher) ListContainers() ([]string, error) {
	cmd := exec.Command("docker", "ps", "-a", "--filter", "name=clai-model-", "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	names := strings.TrimSpace(string(output))
	if names == "" {
		return []string{}, nil
	}

	return strings.Split(names, "\n"), nil
}

// sanitizeContainerName converts a model filename into a valid Docker container name
func sanitizeContainerName(name string) string {
	// Remove .gguf extension
	name = strings.TrimSuffix(name, ".gguf")
	// Replace non-alphanumeric characters with underscores
	name = strings.NewReplacer(
		" ", "_",
		"-", "_",
		".", "_",
		"/", "_",
		"\\", "_",
		":", "_",
	).Replace(name)
	// Ensure it starts with a letter
	if len(name) > 0 && !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) {
		name = "model_" + name
	}
	// Truncate if too long (Docker has 64 char limit)
	if len(name) > 60 {
		name = name[:60]
	}
	return strings.ToLower(name)
}

// ImageExistsLocally checks if an image exists locally
func (dl *DockerLauncher) ImageExistsLocally(imageTag string) bool {
	imageInfo, exists := dl.images[imageTag]
	if !exists {
		return false
	}

	// Try with FullImage first (may include docker.io/ prefix)
	cmd := exec.Command("docker", "images", "-q", imageInfo.FullImage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	if strings.TrimSpace(string(output)) != "" {
		return true
	}

	// Fallback: try without docker.io/ prefix
	imageRef := strings.TrimPrefix(imageInfo.FullImage, "docker.io/")
	if imageRef != imageInfo.FullImage {
		cmd = exec.Command("docker", "images", "-q", imageRef)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return false
		}
		return strings.TrimSpace(string(output)) != ""
	}

	return false
}

// UpdateImage pulls the latest version of an image
func (dl *DockerLauncher) UpdateImage(imageTag string) error {
	return dl.PullImage(imageTag)
}
