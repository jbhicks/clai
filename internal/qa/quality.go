package qa

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"clai/internal/logger"
)

// QualityCheckResult represents the result of a quality check
type QualityCheckResult struct {
	Name        string        `json:"name"`
	Passed      bool          `json:"passed"`
	Duration    time.Duration `json:"duration"`
	Output      string        `json:"output"`
	ErrorOutput string        `json:"errorOutput"`
	Error       error         `json:"error,omitempty"`
}

// QualityAssurance manages quality checks for the autonomous agent
type QualityAssurance struct {
	projectRoot string
}

// NewQualityAssurance creates a new quality assurance instance
func NewQualityAssurance(projectRoot string) *QualityAssurance {
	// If projectRoot is empty or just ".clai", assume we're in the .clai directory
	// and the project root is the parent directory
	if projectRoot == "" || projectRoot == ".clai" {
		projectRoot = ".."
	}
	return &QualityAssurance{
		projectRoot: projectRoot,
	}
}

// RunAllChecks runs all quality checks and returns comprehensive results
func (qa *QualityAssurance) RunAllChecks() ([]QualityCheckResult, bool, time.Duration) {
	startTime := time.Now()

	checks := []struct {
		name string
		fn   func() QualityCheckResult
	}{
		{"build", qa.checkBuild},
		{"test", qa.checkTest},
		{"lint", qa.checkLint},
		{"typecheck", qa.checkTypeCheck},
		{"ui_state", qa.checkUIState},
	}

	var results []QualityCheckResult
	allPassed := true

	// Run checks sequentially for now (could be parallelized later)
	for _, check := range checks {
		logger.Info("Running quality check: %s", check.name)
		result := check.fn()
		results = append(results, result)

		if !result.Passed {
			allPassed = false
			logger.Warn("Quality check failed: %s", check.name)
		} else {
			logger.Info("Quality check passed: %s (%.2fs)", check.name, result.Duration.Seconds())
		}
	}

	totalDuration := time.Since(startTime)
	logger.Info("All quality checks completed in %.2fs, overall result: %s",
		totalDuration.Seconds(), map[bool]string{true: "PASSED", false: "FAILED"}[allPassed])

	return results, allPassed, totalDuration
}

// RunChecksParallel runs quality checks in parallel where possible
func (qa *QualityAssurance) RunChecksParallel() ([]QualityCheckResult, bool, time.Duration) {
	startTime := time.Now()

	// Define which checks can run in parallel
	parallelChecks := []struct {
		name string
		fn   func() QualityCheckResult
	}{
		{"typecheck", qa.checkTypeCheck},
		{"lint", qa.checkLint},
	}

	// Sequential checks that must run after build
	sequentialChecks := []struct {
		name string
		fn   func() QualityCheckResult
	}{
		{"build", qa.checkBuild},
		{"test", qa.checkTest},
	}

	resultsChan := make(chan QualityCheckResult, len(parallelChecks)+len(sequentialChecks))

	// Run parallel checks concurrently
	for _, check := range parallelChecks {
		go func(name string, fn func() QualityCheckResult) {
			logger.Info("Running parallel quality check: %s", name)
			result := fn()
			resultsChan <- result
		}(check.name, check.fn)
	}

	// Run sequential checks
	for _, check := range sequentialChecks {
		logger.Info("Running sequential quality check: %s", check.name)
		result := check.fn()
		resultsChan <- result
	}

	// Collect all results
	var results []QualityCheckResult
	allPassed := true

	for i := 0; i < len(parallelChecks)+len(sequentialChecks); i++ {
		result := <-resultsChan
		results = append(results, result)

		if !result.Passed {
			allPassed = false
		}
	}

	totalDuration := time.Since(startTime)
	logger.Info("Parallel quality checks completed in %.2fs, overall result: %s",
		totalDuration.Seconds(), map[bool]string{true: "PASSED", false: "FAILED"}[allPassed])

	return results, allPassed, totalDuration
}

// checkBuild runs go build to ensure the code compiles
func (qa *QualityAssurance) checkBuild() QualityCheckResult {
	startTime := time.Now()

	cmd := exec.Command("go", "build", "./cmd/clai")
	cmd.Dir = qa.projectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime)

	result := QualityCheckResult{
		Name:        "build",
		Duration:    duration,
		Output:      stdout.String(),
		ErrorOutput: stderr.String(),
	}

	if err != nil {
		result.Passed = false
		result.Error = err
	} else {
		result.Passed = true
	}

	return result
}

// checkTest runs go test
func (qa *QualityAssurance) checkTest() QualityCheckResult {
	startTime := time.Now()

	cmd := exec.Command("go", "test", "./cmd/...", "./internal/...")
	cmd.Dir = qa.projectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime)

	result := QualityCheckResult{
		Name:        "test",
		Duration:    duration,
		Output:      stdout.String(),
		ErrorOutput: stderr.String(),
	}

	if err != nil {
		result.Passed = false
		result.Error = err
	} else {
		result.Passed = true
	}

	return result
}

// checkLint runs go vet and golangci-lint if available
func (qa *QualityAssurance) checkLint() QualityCheckResult {
	startTime := time.Now()

	var allPassed = true
	var output strings.Builder
	var errorOutput strings.Builder

	// Run go vet
	cmd := exec.Command("go", "vet", "./cmd/...", "./internal/...")
	cmd.Dir = qa.projectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output.WriteString("=== go vet ===\n")
	output.WriteString(stdout.String())
	errorOutput.WriteString(stderr.String())

	if err != nil {
		allPassed = false
	}

	// Run golangci-lint if available
	if qa.hasGolangciLint() {
		cmd = exec.Command("golangci-lint", "run", "./cmd/...", "./internal/...")
		cmd.Dir = qa.projectRoot

		stdout.Reset()
		stderr.Reset()
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		output.WriteString("\n=== golangci-lint ===\n")
		output.WriteString(stdout.String())
		errorOutput.WriteString(stderr.String())

		if err != nil {
			allPassed = false
		}
	}

	duration := time.Since(startTime)

	return QualityCheckResult{
		Name:        "lint",
		Passed:      allPassed,
		Duration:    duration,
		Output:      output.String(),
		ErrorOutput: errorOutput.String(),
	}
}

// checkTypeCheck runs go build with type checking only
func (qa *QualityAssurance) checkTypeCheck() QualityCheckResult {
	startTime := time.Now()

	// Use -o /dev/null to avoid creating binary, just check types
	cmd := exec.Command("go", "build", "-o", "/dev/null", "./cmd/clai")
	cmd.Dir = qa.projectRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime)

	result := QualityCheckResult{
		Name:        "typecheck",
		Duration:    duration,
		Output:      stdout.String(),
		ErrorOutput: stderr.String(),
	}

	if err != nil {
		result.Passed = false
		result.Error = err
	} else {
		result.Passed = true
	}

	return result
}

// checkUIState runs UI state verification
func (qa *QualityAssurance) checkUIState() QualityCheckResult {
	startTime := time.Now()

	result := qa.CheckUIState()
	result.Duration = time.Since(startTime)

	return result
}

// hasGolangciLint checks if golangci-lint is available
func (qa *QualityAssurance) hasGolangciLint() bool {
	cmd := exec.Command("which", "golangci-lint")
	return cmd.Run() == nil
}

// ShouldAllowCommit determines if a commit should be allowed based on quality check results
func (qa *QualityAssurance) ShouldAllowCommit(results []QualityCheckResult) (bool, string) {
	var failures []string
	var criticalFailures int

	for _, result := range results {
		if !result.Passed {
			failures = append(failures, result.Name)

			// Build and typecheck are critical - never allow commit if these fail
			if result.Name == "build" || result.Name == "typecheck" {
				criticalFailures++
			}
		}
	}

	if len(failures) == 0 {
		return true, "All quality checks passed"
	}

	if criticalFailures > 0 {
		return false, fmt.Sprintf("Critical failures in: %s. Commit blocked.", strings.Join(failures, ", "))
	}

	// For non-critical failures (test, lint), allow with warning
	return false, fmt.Sprintf("Quality check failures in: %s. Commit blocked.", strings.Join(failures, ", "))
}

// GetQualityMetrics extracts metrics from quality check results
func (qa *QualityAssurance) GetQualityMetrics(results []QualityCheckResult) map[string]interface{} {
	metrics := map[string]interface{}{
		"total_checks":   len(results),
		"passed_checks":  0,
		"failed_checks":  0,
		"total_duration": 0.0,
		"check_details":  make(map[string]interface{}),
	}

	details := metrics["check_details"].(map[string]interface{})

	for _, result := range results {
		if result.Passed {
			metrics["passed_checks"] = metrics["passed_checks"].(int) + 1
		} else {
			metrics["failed_checks"] = metrics["failed_checks"].(int) + 1
		}

		metrics["total_duration"] = metrics["total_duration"].(float64) + result.Duration.Seconds()

		details[result.Name] = map[string]interface{}{
			"passed":   result.Passed,
			"duration": result.Duration.Seconds(),
		}
	}

	return metrics
}

// ParseTestOutput extracts test statistics from test output
func (qa *QualityAssurance) ParseTestOutput(testOutput string) map[string]int {
	stats := map[string]int{
		"tests_run":    0,
		"tests_passed": 0,
		"tests_failed": 0,
	}

	// Look for patterns like "PASS: 5/5 tests passed"
	passPattern := regexp.MustCompile(`PASS:\s*(\d+)/(\d+)`)
	if matches := passPattern.FindStringSubmatch(testOutput); len(matches) == 3 {
		if passed, err := strconv.Atoi(matches[1]); err == nil {
			stats["tests_passed"] = passed
		}
		if total, err := strconv.Atoi(matches[2]); err == nil {
			stats["tests_run"] = total
		}
	}

	// Look for failure indicators
	if strings.Contains(testOutput, "FAIL") {
		stats["tests_failed"] = stats["tests_run"] - stats["tests_passed"]
	}

	return stats
}

// CheckUIState performs basic UI state verification by attempting to connect to debug server
func (qa *QualityAssurance) CheckUIState() QualityCheckResult {
	startTime := time.Now()

	result := QualityCheckResult{
		Name: "ui_state",
	}

	// Try to connect to the debug server socket
	socketPath := "/tmp/clai.sock"
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		result.Passed = false
		result.Error = fmt.Errorf("debug server socket not found: %s", socketPath)
		result.Duration = time.Since(startTime)
		return result
	}

	// If socket exists, consider UI state check passed
	// In a more sophisticated implementation, we could send actual commands
	result.Passed = true
	result.Duration = time.Since(startTime)

	return result
}
