package service

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GetLockFilePath returns the path to the service lock file
func GetLockFilePath() string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, "clai-service.lock")
}

// ReadLockFile reads port and pid from the lock file
func ReadLockFile() (int, int) {
	lockFile := GetLockFilePath()
	data, err := os.ReadFile(lockFile)
	if err != nil {
		return 0, 0
	}

	content := strings.TrimSpace(string(data))
	parts := strings.Split(content, ":")
	if len(parts) == 0 {
		return 0, 0
	}

	port, _ := strconv.Atoi(parts[0])
	pid := 0
	if len(parts) > 1 {
		pid, _ = strconv.Atoi(parts[1])
	}

	return port, pid
}

// GetServicePort reads the service port from the lock file
func GetServicePort() int {
	port, _ := ReadLockFile()
	return port
}
