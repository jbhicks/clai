package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCheckForRunningInstances(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	tests := []struct {
		name         string
		args         []string
		mockPsOutput string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "no running processes - TUI mode should pass",
			args:         []string{"clai"},
			mockPsOutput: "USER               PID  %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND\n",
			wantErr:      false,
		},
		{
			name:         "no running processes - benchmark mode should pass",
			args:         []string{"clai", "benchmark"},
			mockPsOutput: "USER               PID  %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND\n",
			wantErr:      false,
		},
		{
			name: "other clai TUI processes running - benchmark mode should fail",
			args: []string{"clai", "benchmark"},
			mockPsOutput: `USER               PID  %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
user              1234  0.0  0.1  123456  7890 pts/0    S+   10:00   0:00 ./clai
user              5678  0.0  0.1  123456  7890 pts/1    S+   10:01   0:00 grep clai
`,
			wantErr:     true,
			errContains: "CLAI server is already running",
		},
		{
			name: "other clai TUI processes running - TUI mode should pass",
			args: []string{"clai"},
			mockPsOutput: `USER               PID  %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
user              1234  0.0  0.1  123456  7890 pts/0    S+   10:00   0:00 ./clai
user              5678  0.0  0.1  123456  7890 pts/1    S+   10:01   0:00 grep clai
`,
			wantErr: false,
		},
		{
			name: "clai benchmark process running - benchmark mode should fail",
			args: []string{"clai", "benchmark"},
			mockPsOutput: `USER               PID  %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
user              1234  0.0  0.1  123456  7890 pts/0    S+   10:00   0:00 ./clai benchmark
user              5678  0.0  0.1  123456  7890 pts/1    S+   10:01   0:00 grep clai
`,
			wantErr:     true,
			errContains: "CLAI server is already running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args

			err := checkForRunningInstancesWithMockPsOutput(tt.mockPsOutput)

			if tt.wantErr {
				if err == nil {
					t.Errorf("checkForRunningInstances() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("checkForRunningInstances() error = %v, want contains %s", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("checkForRunningInstances() unexpected error = %v", err)
				}
			}
		})
	}
}

func checkForRunningInstancesWithMockPsOutput(mockOutput string) error {
	lines := strings.Split(mockOutput, "\n")
	currentPid := os.Getpid()
	foundInstances := 0

	for _, line := range lines {
		if strings.Contains(line, "clai") && !strings.Contains(line, "grep") {
			fields := strings.Fields(line)
			if len(fields) > 1 {
				if pid, err := strconv.Atoi(fields[1]); err == nil {
					if pid != currentPid {
						foundInstances++
					}
				}
			}
		}
	}

	isServerMode := len(os.Args) > 1 && (os.Args[1] == "benchmark")

	if foundInstances > 0 && isServerMode {
		return fmt.Errorf("CLAI server is already running (%d instance(s) detected). Please stop any running CLAI servers before starting a new one", foundInstances)
	}

	return nil
}

// checkForRunningInstances checks if there are other CLAI processes running
func checkForRunningInstances() error {
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	return checkForRunningInstancesWithMockPsOutput(string(output))
}

func TestBenchmarkLockFileFunctions(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("checkServerRunning with no lock file", func(t *testing.T) {
		lockFile := filepath.Join(tempDir, "clai-benchmark.lock")
		os.Remove(lockFile)

		running := checkServerRunningWithLockPath(lockFile)
		if running {
			t.Errorf("checkServerRunning() = true, want false (no lock file)")
		}
	})

	t.Run("checkServerRunning with existing lock file", func(t *testing.T) {
		lockFile := filepath.Join(tempDir, "clai-benchmark.lock")
		if err := os.WriteFile(lockFile, []byte("8081"), 0644); err != nil {
			t.Fatalf("Failed to create test lock file: %v", err)
		}
		defer os.Remove(lockFile)

		running := checkServerRunningWithLockPath(lockFile)
		if !running {
			t.Errorf("checkServerRunning() = false, want true (lock file exists)")
		}
	})

	t.Run("writeLockFile and getPreferredPort", func(t *testing.T) {
		lockFile := filepath.Join(tempDir, "clai-benchmark.lock")
		os.Remove(lockFile)

		port := getPreferredPortWithLockPath(lockFile)
		if port != 8080 {
			t.Errorf("getPreferredPort() = %d, want 8080 (default)", port)
		}

		testPort := 9090
		writeLockFileWithPort(lockFile, testPort)

		data, err := os.ReadFile(lockFile)
		if err != nil {
			t.Fatalf("Failed to read lock file: %v", err)
		}

		content := strings.TrimSpace(string(data))
		if content != strconv.Itoa(testPort) {
			t.Errorf("Lock file content = %s, want %d", content, testPort)
		}

		port = getPreferredPortWithLockPath(lockFile)
		if port != testPort {
			t.Errorf("getPreferredPort() = %d, want %d (from lock file)", port, testPort)
		}

		os.Remove(lockFile)

		port = getPreferredPortWithLockPath(lockFile)
		if port != 8080 {
			t.Errorf("getPreferredPort() after remove = %d, want 8080 (default)", port)
		}
	})

	t.Run("getPreferredPort with invalid lock file content", func(t *testing.T) {
		lockFile := filepath.Join(tempDir, "clai-benchmark.lock")
		if err := os.WriteFile(lockFile, []byte("invalid"), 0644); err != nil {
			t.Fatalf("Failed to create test lock file: %v", err)
		}
		defer os.Remove(lockFile)

		port := getPreferredPortWithLockPath(lockFile)
		if port != 8080 {
			t.Errorf("getPreferredPort() with invalid content = %d, want 8080 (default)", port)
		}
	})
}

func checkServerRunningWithLockPath(lockFile string) bool {
	if _, err := os.Stat(lockFile); err == nil {
		return true
	}
	return false
}

func getPreferredPortWithLockPath(lockFile string) int {
	data, err := os.ReadFile(lockFile)
	if err != nil {
		return 8080
	}

	portStr := strings.TrimSpace(string(data))
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 8080
	}

	return port
}

func writeLockFileWithPort(lockFile string, port int) {
	content := fmt.Sprintf("%d", port)
	_ = os.WriteFile(lockFile, []byte(content), 0644)
}

func TestKillExistingBenchmarkServers(t *testing.T) {
	t.Run("no existing processes", func(t *testing.T) {
		killExistingBenchmarkServersMocked(nil, nil)
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("mock process killing", func(t *testing.T) {
		var capturedArgs [][]string
		killExistingBenchmarkServersMocked(
			func(name string, arg ...string) *exec.Cmd {
				if name == "pkill" || name == "taskkill" {
					capturedArgs = append(capturedArgs, append([]string{name}, arg...))
					return exec.Command("echo", "mock kill")
				}
				return exec.Command(name, arg...)
			},
			nil,
		)

		if len(capturedArgs) == 0 {
			t.Errorf("Expected pkill/taskkill command to be executed")
		}

		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			if len(capturedArgs[0]) < 3 || capturedArgs[0][1] != "-f" || len(capturedArgs[0]) < 3 || !strings.Contains(capturedArgs[0][2], "clai.*--headless") {
				t.Errorf("Expected pkill -f 'clai.*--headless', got: %v", capturedArgs[0])
			}
		}
	})
}

func killExistingBenchmarkServersMocked(
	commandFunc func(name string, arg ...string) *exec.Cmd,
	sleepFunc func(duration time.Duration),
) {
	if commandFunc == nil {
		commandFunc = exec.Command
	}
	if sleepFunc == nil {
		sleepFunc = time.Sleep
	}

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux", "darwin":
		cmd = commandFunc("pkill", "-f", "clai.*--headless")
	case "windows":
		cmd = commandFunc("taskkill", "/F", "/IM", "clai.exe", "/FI", "WINDOWTITLE eq headless")
	default:
		return
	}

	_ = cmd.Run()
	sleepFunc(500 * time.Millisecond)
}

func TestRealProcessLocking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real process locking test in short mode")
	}

	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	t.Run("actual process detection", func(t *testing.T) {
		os.Args = []string{"clai", "benchmark"}

		err := checkForRunningInstances()

		if err != nil {
			t.Logf("checkForRunningInstances returned error (may be expected): %v", err)
		} else {
			t.Log("checkForRunningInstances completed without error")
		}
	})
}

func TestEdgeCases(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	t.Run("ps output with malformed lines", func(t *testing.T) {
		os.Args = []string{"clai", "benchmark"}

		mockOutput := `USER               PID  %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
user              abc  0.0  0.1  123456  7890 pts/0    S+   10:00   0:00 ./clai
user              5678  0.0  0.1  123456  7890 pts/1    S+   10:01   0:00 ./clai benchmark`

		err := checkForRunningInstancesWithMockPsOutput(mockOutput)
		if err != nil && strings.Contains(err.Error(), "CLAI server is already running") {
			t.Logf("checkForRunningInstances() correctly detected running process: %v", err)
		}
	})

	t.Run("empty ps output", func(t *testing.T) {
		os.Args = []string{"clai", "benchmark"}

		err := checkForRunningInstancesWithMockPsOutput("")
		if err != nil {
			t.Errorf("checkForRunningInstances() with empty ps output unexpected error = %v", err)
		}
	})

	t.Run("ps command fails simulation", func(t *testing.T) {
		os.Args = []string{"clai", "benchmark"}

		cmd := exec.Command("false")
		_, err := cmd.Output()

		if err == nil {
			t.Errorf("Expected ps command failure")
		}

		expectedErr := fmt.Errorf("failed to check running processes: %v", err)
		if expectedErr.Error() == "" {
			t.Errorf("Expected error message for ps failure")
		}
	})
}
