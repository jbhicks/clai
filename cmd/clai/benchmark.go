package main

import (
	"clai/internal/benchmark"
	"clai/internal/db"
	"clai/internal/llm"
	"clai/internal/logger"
	"context"
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func runBenchmarkCommand(args []string) {
	_ = godotenv.Load()
	// Parse CLI flags
	var cliMode bool
	var testIndex int
	var sequential bool
	flag.BoolVar(&cliMode, "cli", false, "Run benchmarks in command-line mode (no web server)")
	flag.IntVar(&testIndex, "test", -1, "Run specific test by index (1-based), use -1 for all tests")
	flag.BoolVar(&sequential, "sequential", false, "Run tests sequentially instead of parallel (slower but more stable)")

	// Parse the provided args instead of os.Args
	flag.CommandLine.Parse(args)

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

	// Handle CLI mode
	if cliMode {
		runBenchmarkCLI(store, testIndex, sequential)
		return
	}

	// Create and start server
	server := benchmark.NewServer(store)

	// Check if server was already running (for reloads)
	wasAlreadyRunning := checkServerRunning()
	preferredPort := getPreferredPort()

	// Start server in a goroutine
	go func() {
		if _, err := server.StartWithPreferredPort(preferredPort); err != nil {
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

// runBenchmarkCLI runs benchmarks in command-line mode
func runBenchmarkCLI(store *db.Store, testIndex int, sequential bool) {
	// Check for existing benchmark processes
	// if checkExistingBenchmarks() {
	// 	fmt.Fprintf(os.Stderr, "Another benchmark process is already running.\n")
	// 	fmt.Fprintf(os.Stderr, "If you have the dev server running, stop it first: Ctrl+C in the dev server terminal\n")
	// 	fmt.Fprintf(os.Stderr, "Or kill all benchmarks: pkill -f 'clai benchmark'\n")
	// 	os.Exit(1)
	// }

	// Check API health before starting
	if !checkAPIHealth() {
		fmt.Fprintf(os.Stderr, "LLM API server is not responding. Please ensure the server is running.\n")
		os.Exit(1)
	}

	// Use the same host configuration as the main clai application
	host := forcedLLMHost
	if envHost := os.Getenv("OLLAMA_HOST"); envHost != "" {
		host = envHost
	}
	if envHost := os.Getenv("OLLAMA_HOST"); envHost != "" {
		host = envHost
	}

	// Extract port from host URL for model validation
	port := 11434 // default
	if strings.Contains(host, ":") {
		parts := strings.Split(host, ":")
		if len(parts) >= 3 {
			if p, err := strconv.Atoi(parts[2]); err == nil {
				port = p
			}
		}
	}

	// Use the same model configuration as the main clai application
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "llama3.1-gpu:latest"
	}

	// If no explicit model is configured, query the actual model running on the server
	if os.Getenv("OLLAMA_MODEL") == "" {
		detectedModel := getModelNameFromPort(port)
		if detectedModel != "" {
			modelName = detectedModel
		}
	}

	systemPrompt := os.Getenv("SYSTEM_PROMPT")

	// Create LLM client with the actual running model
	llmClient := llm.NewClient(host, modelName, systemPrompt)

	// Create benchmark runner
	runner := benchmark.NewRunner(store, llmClient)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n🛑 Benchmark interrupted. Cleaning up...")
		os.Exit(1)
	}()

	// Store original test data for display purposes
	var originalTest llm.ModelBenchmarkTest
	if testIndex >= 0 {
		if testIndex == 0 {
			fmt.Fprintln(os.Stderr, "Test index starts at 1. Use -1 for all tests.")
			os.Exit(1)
		}
		adjustedIndex := testIndex - 1
		if adjustedIndex >= len(llm.ModelBenchmarkSuite) {
			fmt.Fprintf(os.Stderr, "Test index %d out of range (1-%d)\n", testIndex, len(llm.ModelBenchmarkSuite))
			os.Exit(1)
		}
		originalTest = llm.ModelBenchmarkSuite[adjustedIndex]
	}

	// Determine which tests to run
	var tests []llm.ModelBenchmarkTest
	if testIndex >= 0 {
		// Run specific test by index (1-based)
		if testIndex == 0 {
			fmt.Fprintln(os.Stderr, "Test index starts at 1. Use -1 for all tests.")
			os.Exit(1)
		}
		adjustedIndex := testIndex - 1
		if adjustedIndex >= len(llm.ModelBenchmarkSuite) {
			fmt.Fprintf(os.Stderr, "Test index %d out of range (1-%d)\n", testIndex, len(llm.ModelBenchmarkSuite))
			os.Exit(1)
		}
		originalTest = llm.ModelBenchmarkSuite[adjustedIndex]
		tests = []llm.ModelBenchmarkTest{originalTest}
	} else {
		tests = llm.ModelBenchmarkSuite
	}

	if len(tests) == 0 {
		fmt.Fprintf(os.Stderr, "No benchmark tests found\n")
		os.Exit(1)
	}

	fmt.Printf("Running %d benchmark test(s) against %s...\n", len(tests), modelName)

	// Create a modified benchmark suite for testing
	originalSuite := llm.ModelBenchmarkSuite
	llm.ModelBenchmarkSuite = tests
	defer func() { llm.ModelBenchmarkSuite = originalSuite }()

	// Run benchmarks
	runID, err := runner.RunBenchmark(context.Background(), modelName, host)
	if err != nil {
		logger.Error("Failed to run benchmarks: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to run benchmarks: %v\n", err)
		os.Exit(1)
	}

	// Get and display results
	run, err := store.GetBenchmarkRun(int(runID))
	if err != nil {
		logger.Error("Failed to get benchmark run: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to get benchmark results: %v\n", err)
		os.Exit(1)
	}

	results, err := store.GetBenchmarkResults(int(runID))
	if err != nil {
		logger.Error("Failed to get benchmark results: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to get benchmark results: %v\n", err)
		os.Exit(1)
	}

	// Print results
	printBenchmarkResults(run, results, testIndex, originalTest)
}

// printBenchmarkResults prints formatted benchmark results to stdout
func printBenchmarkResults(run *db.BenchmarkRun, results []db.BenchmarkResult, testIndex int, originalTest llm.ModelBenchmarkTest) {
	// If running a single test, show detailed information
	if testIndex >= 0 && len(results) == 1 {
		result := results[0]
		test := originalTest

		fmt.Println("=================================================================================")
		fmt.Printf("TEST DETAILS: %s\n", result.TestName)
		fmt.Println("=================================================================================")

		fmt.Printf("📝 Query: %s\n\n", test.Query)

		fmt.Printf("🎯 Expected Behavior: %s\n", test.ExpectedBehavior)

		if len(test.ShouldContain) > 0 {
			fmt.Printf("✅ Should Contain: %s\n", strings.Join(test.ShouldContain, " OR "))
		}

		if len(test.ShouldNotContain) > 0 {
			fmt.Printf("❌ Should NOT Contain: %s\n", strings.Join(test.ShouldNotContain, " OR "))
		}

		fmt.Println("\n📊 Performance:")
		fmt.Printf("   Time: %.2fs\n", result.TimeSeconds)
		fmt.Printf("   Tokens Generated: %d\n", result.TokensGenerated)
		fmt.Printf("   Token Speed: %.1f tokens/second\n", result.TokensPerSecond)

		fmt.Println("\n🤖 Model Response:")
		fmt.Println("   ──────────────────────────────────────────────────────────────────────────")
		if result.Response != "" {
			// Indent each line of the response
			lines := strings.Split(result.Response, "\n")
			for _, line := range lines {
				fmt.Printf("   %s\n", line)
			}
		} else {
			fmt.Println("   (No response)")
		}
		fmt.Println("   ──────────────────────────────────────────────────────────────────────────")

		status := "✅ PASSED"
		if !result.Passed {
			status = "❌ FAILED"
		}
		fmt.Printf("\n📋 Result: %s\n", status)

		if !result.Passed && result.FailureReason != "" {
			fmt.Printf("💥 Failure Reason: %s\n", result.FailureReason)
		}

		fmt.Println("=================================================================================")

	} else {
		// Show summary for multiple tests
		fmt.Println("=================================================================================")
		fmt.Println("MODEL BENCHMARK SUMMARY")
		fmt.Println("=================================================================================")

		for _, result := range results {
			status := "✓"
			if !result.Passed {
				status = "✗"
			}

			fmt.Printf("%s %-35s %.2fs  %d iter  %d tokens  %.1f t/s\n",
				status,
				result.TestName,
				result.TimeSeconds,
				result.Iterations,
				result.TokensGenerated,
				result.TokensPerSecond)

			if !result.Passed && result.FailureReason != "" {
				fmt.Printf("    %s\n", result.FailureReason)
			}
		}

		fmt.Println("---------------------------------------------------------------------------------")
		fmt.Printf("TOTAL: %d tests, %d passed, %d failed\n",
			run.TotalTests,
			run.PassedTests,
			run.FailedTests)
		fmt.Printf("Total time: %.2fs, Avg time: %.2fs\n",
			run.TotalTimeSeconds,
			run.TotalTimeSeconds/float64(run.TotalTests))
		fmt.Printf("Total iterations: %d, Avg iterations: %.1f\n",
			int(run.AvgIterations*float64(run.TotalTests)),
			run.AvgIterations)
		fmt.Printf("Success rate: %.1f%%\n", run.SuccessRate)
		fmt.Println("=================================================================================")
	}
}

// getTestByIndex returns the test at the specified index
func getTestByIndex(index int) llm.ModelBenchmarkTest {
	if index >= 0 && index < len(llm.ModelBenchmarkSuite) {
		return llm.ModelBenchmarkSuite[index]
	}
	return llm.ModelBenchmarkTest{}
}

// checkExistingBenchmarks checks if other benchmark processes are running
func checkExistingBenchmarks() bool {
	// Get current process PID
	currentPID := os.Getpid()

	// Use pgrep to find clai processes (excluding dev servers)
	cmd := exec.Command("pgrep", "-f", "clai benchmark")
	output, err := cmd.Output()
	if err != nil {
		// pgrep returns exit code 1 when no processes found
		return false
	}

	// Parse PIDs and check if any are not our current process
	pids := strings.Fields(string(output))
	for _, pidStr := range pids {
		if pid, err := strconv.Atoi(pidStr); err == nil && pid != currentPID {
			return true
		}
	}

	return false
}

// checkAPIHealth verifies the LLM API server is responsive
func checkAPIHealth() bool {
	host := forcedLLMHost

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(host + "/health")
	if err == nil {
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}

	resp, err = client.Get(host + "/v1/models")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// getModelNameFromPort queries a llama.cpp server to get the actual model name
func getModelNameFromPort(port int) string {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	url := fmt.Sprintf("http://localhost:%d/v1/models", port)
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	bodyStr := string(body)

	// Extract model filename from "id" field
	// Example: {"id":"Devstral-Small-2-24B-Instruct-2512-UD-Q8_K_XL.gguf",...}
	if idStart := strings.Index(bodyStr, `"id":"`); idStart >= 0 {
		idStart += len(`"id":"`)
		if idEnd := strings.Index(bodyStr[idStart:], `"`); idEnd >= 0 {
			modelName := bodyStr[idStart : idStart+idEnd]
			return modelName
		}
	}

	return ""
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
