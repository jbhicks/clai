package main

import (
	"clai/internal/benchmark"
	"clai/internal/db"
	"clai/internal/logger"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func runBenchmarkCommand(args []string) {
	// Initialize logger to file
	logFile, err := os.Create("debug.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open debug.log: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	logger.Init(logFile)

	// Initialize database
	store, err := db.New()
	if err != nil {
		logger.Error("Failed to initialize database: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Create and start server
	server := benchmark.NewServer(store)

	// Check if server was already running (for reloads)
	wasAlreadyRunning := checkServerRunning()
	preferredPort := getPreferredPort()

	// Start server in a goroutine
	go func() {
		if err := server.StartWithPreferredPort(preferredPort); err != nil {
			logger.Error("Server error: %v", err)
			fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait a moment for server to start
	// TODO: Better way to wait for server to be ready
	fmt.Println("Starting benchmark server...")

	// Give server time to bind to port
	port := 0
	for i := 0; i < 10; i++ {
		port = server.GetPort()
		if port != 0 {
			break
		}
		// Simple sleep alternative
		for j := 0; j < 10000000; j++ {
			// busy wait
		}
	}

	if port == 0 {
		fmt.Fprintln(os.Stderr, "Failed to get server port")
		os.Exit(1)
	}

	url := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("Benchmark server running at %s\n", url)

	// Only open browser if this is a fresh start (not a reload)
	if !wasAlreadyRunning {
		fmt.Println("Opening in browser...")
		if err := openBrowser(url); err != nil {
			logger.Warn("Failed to open browser: %v", err)
			fmt.Printf("Please open %s in your browser\n", url)
		}
	} else {
		fmt.Println("Server reloaded - browser tab should auto-refresh")
	}

	// Write lock file so we know server is running
	writeLockFile(port)
	defer removeLockFile()

	fmt.Println("Press Ctrl+C to stop the server")

	// Wait forever (until Ctrl+C)
	select {}
}

// checkServerRunning checks if the server was already running before this start
func checkServerRunning() bool {
	lockFile := getLockFilePath()
	if _, err := os.Stat(lockFile); err == nil {
		// Lock file exists, server was already running
		return true
	}
	return false
}

// writeLockFile writes a lock file with the current port
func writeLockFile(port int) {
	lockFile := getLockFilePath()
	content := fmt.Sprintf("%d", port)
	if err := os.WriteFile(lockFile, []byte(content), 0644); err != nil {
		logger.Warn("Failed to write lock file: %v", err)
	}
}

// removeLockFile removes the lock file
func removeLockFile() {
	lockFile := getLockFilePath()
	os.Remove(lockFile)
}

// getLockFilePath returns the path to the lock file
func getLockFilePath() string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, "clai-benchmark.lock")
}

// getPreferredPort reads the preferred port from lock file, or returns 8080
func getPreferredPort() int {
	lockFile := getLockFilePath()
	data, err := os.ReadFile(lockFile)
	if err != nil {
		return 8080 // Default port
	}

	portStr := strings.TrimSpace(string(data))
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 8080
	}

	return port
}

// openBrowser opens the default browser to the given URL
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
