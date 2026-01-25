package persistence

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"clai/internal/archival"
	"clai/internal/logger"
	"clai/internal/qa"
)

// ProgressEntry represents a single iteration's progress
type ProgressEntry struct {
	Iteration   int                    `json:"iteration"`
	StartTime   time.Time              `json:"startTime"`
	EndTime     *time.Time             `json:"endTime,omitempty"`
	TaskID      string                 `json:"taskId"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"` // "started", "completed", "failed"
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
	Learnings   []string               `json:"learnings,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// QualityCheck represents the results of quality checks
type QualityCheck struct {
	LastRun   *time.Time     `json:"lastRun"`
	Passed    bool           `json:"passed"`
	Failures  []string       `json:"failures"`
	BuildTime *time.Duration `json:"buildTime,omitempty"`
	TestCount int            `json:"testCount,omitempty"`
}

// SystemInfo contains system metadata
type SystemInfo struct {
	CLaiVersion string    `json:"claiVersion"`
	GoVersion   string    `json:"goVersion"`
	LastRestart time.Time `json:"lastRestart"`
}

// ProgressData represents the complete progress state
type ProgressData struct {
	Version              string          `json:"version"`
	Iterations           []ProgressEntry `json:"iterations"`
	CurrentTask          *ProgressEntry  `json:"currentTask,omitempty"`
	QualityChecks        QualityCheck    `json:"qualityChecks"`
	PatternsLearned      int             `json:"patternsLearned"`
	TotalIterationsTime  int64           `json:"totalIterationsTime"`  // in seconds
	AverageIterationTime float64         `json:"averageIterationTime"` // in seconds
	SuccessRate          float64         `json:"successRate"`          // 0.0 to 1.0
	LastUpdate           time.Time       `json:"lastUpdate"`
	SystemInfo           SystemInfo      `json:"systemInfo"`
}

// PatternEntry represents a learned pattern
type PatternEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	Category    string    `json:"category"` // "code_pattern", "error_recovery", "performance", etc.
	Pattern     string    `json:"pattern"`
	Context     string    `json:"context"`
	Confidence  float64   `json:"confidence"` // 0.0 to 1.0
	SourceTask  string    `json:"sourceTask"`
	Description string    `json:"description"`
}

// PersistenceManager handles all memory persistence operations
type PersistenceManager struct {
	claiDir      string
	progressFile string
	patternsFile string
	storiesFile  string
	configFile   string
	mu           sync.RWMutex
}

// NewPersistenceManager creates a new persistence manager
func NewPersistenceManager() (*PersistenceManager, error) {
	// Find the project root (directory containing go.mod)
	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find project root: %w", err)
	}

	claiDir := filepath.Join(projectRoot, ".clai")
	if err := os.MkdirAll(claiDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .clai directory: %w", err)
	}

	return &PersistenceManager{
		claiDir:      projectRoot, // Use project root for QA commands
		progressFile: filepath.Join(claiDir, "progress.json"),
		patternsFile: filepath.Join(claiDir, "patterns.log"),
		storiesFile:  filepath.Join(claiDir, "stories.json"),
		configFile:   filepath.Join(claiDir, "config.json"),
	}, nil
}

// findProjectRoot finds the project root directory (containing go.mod)
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("go.mod file not found in current directory or any parent directory")
}

// LoadProgress loads the current progress state
func (pm *PersistenceManager) LoadProgress() (*ProgressData, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	data, err := pm.loadProgressData()
	if err != nil {
		// If file doesn't exist, create default progress data
		if os.IsNotExist(err) {
			return pm.createDefaultProgressData(), nil
		}
		return nil, err
	}
	return data, nil
}

// SaveProgress saves the progress state atomically
func (pm *PersistenceManager) SaveProgress(data *ProgressData) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	data.LastUpdate = time.Now()
	return pm.saveProgressData(data)
}

// StartIteration begins tracking a new iteration
func (pm *PersistenceManager) StartIteration(iteration int, taskID, description string) error {
	entry := ProgressEntry{
		Iteration:   iteration,
		StartTime:   time.Now(),
		TaskID:      taskID,
		Description: description,
		Status:      "started",
		Metrics:     make(map[string]interface{}),
		Learnings:   []string{},
	}

	data, err := pm.LoadProgress()
	if err != nil {
		return err
	}

	data.Iterations = append(data.Iterations, entry)
	data.CurrentTask = &entry

	return pm.SaveProgress(data)
}

// CompleteIteration marks an iteration as completed
func (pm *PersistenceManager) CompleteIteration(iteration int, learnings []string, metrics map[string]interface{}) error {
	data, err := pm.LoadProgress()
	if err != nil {
		return err
	}

	for i := len(data.Iterations) - 1; i >= 0; i-- {
		if data.Iterations[i].Iteration == iteration {
			now := time.Now()
			data.Iterations[i].EndTime = &now
			data.Iterations[i].Status = "completed"
			data.Iterations[i].Learnings = learnings
			data.Iterations[i].Metrics = metrics
			break
		}
	}

	if data.CurrentTask != nil && data.CurrentTask.Iteration == iteration {
		data.CurrentTask = nil
	}

	return pm.SaveProgress(data)
}

// FailIteration marks an iteration as failed
func (pm *PersistenceManager) FailIteration(iteration int, errorMsg string) error {
	data, err := pm.LoadProgress()
	if err != nil {
		return err
	}

	for i := len(data.Iterations) - 1; i >= 0; i-- {
		if data.Iterations[i].Iteration == iteration {
			now := time.Now()
			data.Iterations[i].EndTime = &now
			data.Iterations[i].Status = "failed"
			data.Iterations[i].Error = errorMsg
			break
		}
	}

	if data.CurrentTask != nil && data.CurrentTask.Iteration == iteration {
		data.CurrentTask = nil
	}

	return pm.SaveProgress(data)
}

// RecordPattern logs a discovered pattern
func (pm *PersistenceManager) RecordPattern(category, pattern, context, sourceTask, description string, confidence float64) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	entry := PatternEntry{
		Timestamp:   time.Now(),
		Category:    category,
		Pattern:     pattern,
		Context:     context,
		Confidence:  confidence,
		SourceTask:  sourceTask,
		Description: description,
	}

	return pm.appendPatternEntry(entry)
}

// UpdateQualityCheck updates the quality check results
func (pm *PersistenceManager) UpdateQualityCheck(passed bool, failures []string, buildTime *time.Duration, testCount int) error {
	data, err := pm.LoadProgress()
	if err != nil {
		return err
	}

	now := time.Now()
	data.QualityChecks.LastRun = &now
	data.QualityChecks.Passed = passed
	data.QualityChecks.Failures = failures
	if buildTime != nil {
		data.QualityChecks.BuildTime = buildTime
	}
	data.QualityChecks.TestCount = testCount

	return pm.SaveProgress(data)
}

// BackupToGit creates a git commit with current progress
func (pm *PersistenceManager) BackupToGit(message string) error {
	// Add .clai files to git
	if err := pm.runGitCommand("add", ".clai/"); err != nil {
		logger.Warn("Failed to add .clai files to git: %v", err)
		// Don't fail the whole operation for git issues
		return nil
	}

	// Commit with the provided message
	if err := pm.runGitCommand("commit", "-m", message); err != nil {
		logger.Warn("Failed to commit progress: %v", err)
		return nil
	}

	logger.Info("Progress backed up to git")
	return nil
}

// GetCurrentBranch returns the current git branch name
func (pm *PersistenceManager) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// ArchiveOnBranchChange checks for branch changes and triggers archiving
func (pm *PersistenceManager) ArchiveOnBranchChange(oldBranchFile string) error {
	currentBranch, err := pm.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Read the previously stored branch using bufio.Scanner for efficiency
	oldBranch := ""
	if file, err := os.Open(oldBranchFile); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			oldBranch = strings.TrimSpace(scanner.Text())
		}
	}

	// If branch changed, archive the old branch
	if oldBranch != "" && oldBranch != currentBranch {
		logger.Info("Branch change detected: %s -> %s", oldBranch, currentBranch)

		// Archive the old branch's work
		archiver := archival.NewArchiver(pm.claiDir)
		if err := archiver.ArchiveOnBranchChange(oldBranch, currentBranch); err != nil {
			logger.Warn("Failed to archive on branch change: %v", err)
		}
	}

	if err := os.WriteFile(oldBranchFile, []byte(currentBranch), 0644); err != nil {
		logger.Warn("Failed to update branch file: %v", err)
	}

	return nil
}

// CommitWithQualityCheck attempts to commit changes but only if quality checks pass
func (pm *PersistenceManager) CommitWithQualityCheck(message string) error {
	// Run quality checks first
	allowCommit, reason, err := pm.ShouldAllowCommit()
	if err != nil {
		return fmt.Errorf("failed to run quality checks: %w", err)
	}

	if !allowCommit {
		return fmt.Errorf("commit blocked by quality checks: %s", reason)
	}

	logger.Info("Quality checks passed, proceeding with commit")

	// Add all changes to git
	if err := pm.runGitCommand("add", "."); err != nil {
		return fmt.Errorf("failed to add files to git: %w", err)
	}

	// Commit with the provided message
	if err := pm.runGitCommand("commit", "-m", message); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	logger.Info("Successfully committed changes after quality checks")
	return nil
}

// SaveStateCommit creates a save-state commit without full quality checks
func (pm *PersistenceManager) SaveStateCommit(message string) error {
	logger.Info("Creating save-state commit without full quality checks")

	// Add .clai directory and any work-in-progress files
	if err := pm.runGitCommand("add", ".clai/"); err != nil {
		logger.Warn("Failed to add .clai files: %v", err)
	}

	// Add any modified files that are safe to commit
	if err := pm.runGitCommand("add", "-u"); err != nil {
		logger.Warn("Failed to add modified files: %v", err)
	}

	// Commit as a save state
	commitMessage := fmt.Sprintf("[SAVE STATE] %s", message)
	if err := pm.runGitCommand("commit", "-m", commitMessage); err != nil {
		// If nothing to commit, that's okay for save states
		if strings.Contains(err.Error(), "nothing to commit") {
			logger.Info("No changes to commit for save state")
			return nil
		}
		return fmt.Errorf("failed to create save state commit: %w", err)
	}

	logger.Info("Save state committed successfully")
	return nil
}

// RollbackToLastGoodState performs a git rollback to the last good commit
func (pm *PersistenceManager) RollbackToLastGoodState() error {
	logger.Info("Attempting rollback to last good state")

	// Check if there are uncommitted changes
	cmd := pm.runGitCommand("status", "--porcelain")
	if cmd == nil { // Git command succeeded
		// If there are uncommitted changes, reset them
		resetErr := pm.runGitCommand("reset", "--hard", "HEAD")
		if resetErr != nil {
			logger.Warn("Failed to reset uncommitted changes: %v", resetErr)
		}

		// Clean untracked files
		cleanErr := pm.runGitCommand("clean", "-fd")
		if cleanErr != nil {
			logger.Warn("Failed to clean untracked files: %v", cleanErr)
		}
	}

	// Find the last commit before the current one
	// This is a simplified rollback - in practice you'd want more sophisticated logic
	lastCommitCmd := exec.Command("git", "rev-parse", "HEAD~1")
	lastCommitOutput, err := lastCommitCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get last commit: %w", err)
	}

	lastCommit := strings.TrimSpace(string(lastCommitOutput))

	// Reset to the previous commit
	if err := pm.runGitCommand("reset", "--hard", lastCommit); err != nil {
		return fmt.Errorf("failed to rollback to commit %s: %w", lastCommit, err)
	}

	logger.Info("Successfully rolled back to commit: %s", lastCommit[:8])
	return nil
}

// GetRollbackStatus provides information about rollback capability
func (pm *PersistenceManager) GetRollbackStatus() map[string]interface{} {
	status := map[string]interface{}{
		"can_rollback": false,
		"reason":       "",
	}

	// Check if git repository exists
	if err := pm.runGitCommand("rev-parse", "--git-dir"); err != nil {
		status["reason"] = "not a git repository"
		return status
	}

	// Check if there are commits to rollback to
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		status["reason"] = "cannot determine commit history"
		return status
	}

	commitCount := strings.TrimSpace(string(output))
	if commitCount == "1" {
		status["reason"] = "only one commit exists, cannot rollback further"
		return status
	}

	status["can_rollback"] = true
	status["commits_available"] = commitCount
	return status
}

// GetIterationStats returns statistics about completed iterations
func (pm *PersistenceManager) GetIterationStats() map[string]interface{} {
	data, err := pm.LoadProgress()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	stats := map[string]interface{}{
		"total_iterations":     len(data.Iterations),
		"completed_iterations": 0,
		"failed_iterations":    0,
		"average_duration":     0,
		"total_learnings":      0,
	}

	var totalDuration time.Duration
	for _, iteration := range data.Iterations {
		switch iteration.Status {
		case "completed":
			stats["completed_iterations"] = stats["completed_iterations"].(int) + 1
			if iteration.EndTime != nil {
				totalDuration += iteration.EndTime.Sub(iteration.StartTime)
			}
			stats["total_learnings"] = stats["total_learnings"].(int) + len(iteration.Learnings)
		case "failed":
			stats["failed_iterations"] = stats["failed_iterations"].(int) + 1
		}
	}

	if completed := stats["completed_iterations"].(int); completed > 0 {
		stats["average_duration"] = int(totalDuration.Seconds() / float64(completed))
	}

	return stats
}

// RunQualityChecks executes all quality checks and updates progress
func (pm *PersistenceManager) RunQualityChecks() ([]qa.QualityCheckResult, bool, error) {
	qaSystem := qa.NewQualityAssurance(pm.claiDir)

	results, allPassed, totalDuration := qaSystem.RunAllChecks()

	// Update quality check results in progress
	err := pm.UpdateQualityCheck(allPassed, pm.extractFailures(results), &totalDuration, pm.extractTestCount(results))
	if err != nil {
		logger.Warn("Failed to update quality check results: %v", err)
	}

	return results, allPassed, nil
}

// RunQualityChecksParallel executes quality checks in parallel and updates progress
func (pm *PersistenceManager) RunQualityChecksParallel() ([]qa.QualityCheckResult, bool, error) {
	qaSystem := qa.NewQualityAssurance(pm.claiDir)

	results, allPassed, totalDuration := qaSystem.RunChecksParallel()

	// Update quality check results in progress
	err := pm.UpdateQualityCheck(allPassed, pm.extractFailures(results), &totalDuration, pm.extractTestCount(results))
	if err != nil {
		logger.Warn("Failed to update quality check results: %v", err)
	}

	return results, allPassed, nil
}

// ShouldAllowCommit determines if commits should be allowed based on quality checks
func (pm *PersistenceManager) ShouldAllowCommit() (bool, string, error) {
	data, err := pm.LoadProgress()
	if err != nil {
		return false, "Failed to load progress data", err
	}

	if data.QualityChecks.LastRun == nil {
		return false, "No quality checks have been run", nil
	}

	qaSystem := qa.NewQualityAssurance(pm.claiDir)

	// Run fresh quality checks
	results, _, _ := qaSystem.RunAllChecks()

	allowCommit, reason := qaSystem.ShouldAllowCommit(results)
	return allowCommit, reason, nil
}

// extractFailures extracts failure messages from quality check results
func (pm *PersistenceManager) extractFailures(results []qa.QualityCheckResult) []string {
	var failures []string
	for _, result := range results {
		if !result.Passed && result.Error != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", result.Name, result.Error))
		}
	}
	return failures
}

// extractTestCount extracts the number of tests run from test results
func (pm *PersistenceManager) extractTestCount(results []qa.QualityCheckResult) int {
	for _, result := range results {
		if result.Name == "test" {
			// Parse test count from output (simplified)
			if strings.Contains(result.Output, "PASS") {
				// Try to extract test count - this is a simplified implementation
				return 1 // Placeholder - would need better parsing
			}
		}
	}
	return 0
}

// Private methods

func (pm *PersistenceManager) createDefaultProgressData() *ProgressData {
	return &ProgressData{
		Version:    "1.0",
		Iterations: []ProgressEntry{},
		QualityChecks: QualityCheck{
			Passed:   true,
			Failures: []string{},
		},
		PatternsLearned:      0,
		TotalIterationsTime:  0,
		AverageIterationTime: 0,
		SuccessRate:          1.0,
		LastUpdate:           time.Now(),
		SystemInfo: SystemInfo{
			CLaiVersion: "1.0.0",
			GoVersion:   "1.24.5",
			LastRestart: time.Now(),
		},
	}
}

func (pm *PersistenceManager) loadProgressData() (*ProgressData, error) {
	file, err := os.Open(pm.progressFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data ProgressData
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode progress.json: %w", err)
	}

	return &data, nil
}

func (pm *PersistenceManager) saveProgressData(data *ProgressData) error {
	// Write to temporary file first for atomicity
	tempFile := pm.progressFile + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		file.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to encode progress data: %w", err)
	}

	file.Close()

	// Atomic move
	if err := os.Rename(tempFile, pm.progressFile); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to move temp file: %w", err)
	}

	return nil
}

func (pm *PersistenceManager) appendPatternEntry(entry PatternEntry) error {
	file, err := os.OpenFile(pm.patternsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open patterns.log: %w", err)
	}
	defer file.Close()

	// Write in a structured format
	line := fmt.Sprintf("[%s] %s: %s\n",
		entry.Timestamp.Format("2006-01-02 15:04:05"),
		entry.Category,
		entry.Pattern)

	if entry.Context != "" {
		line += fmt.Sprintf("  Context: %s\n", entry.Context)
	}

	if entry.Description != "" {
		line += fmt.Sprintf("  Description: %s\n", entry.Description)
	}

	line += fmt.Sprintf("  Confidence: %.2f, Source: %s\n\n", entry.Confidence, entry.SourceTask)

	_, err = file.WriteString(line)
	return err
}

func (pm *PersistenceManager) runGitCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = "." // Run from project root

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git command failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	logger.Debug("Git command successful: git %v", args)
	return nil
}
