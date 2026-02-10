package ralph

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// RalphStage represents the current state of the Ralph loop
type RalphStage int

const (
	StageIdle RalphStage = iota
	StageThinking
	StageWorking
	StageCompleted
	StagePaused
	StageError
)

// String returns a string representation of the stage
func (s RalphStage) String() string {
	switch s {
	case StageIdle:
		return "Idle"
	case StageThinking:
		return "Thinking"
	case StageWorking:
		return "Working"
	case StageCompleted:
		return "Completed"
	case StagePaused:
		return "Paused"
	case StageError:
		return "Error"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

// RalphLoopController manages the Ralph development loop state machine
type RalphLoopController struct {
	baseDir     string
	stage       RalphStage
	activeStory *UserStory
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewRalphLoopController creates a new loop controller
func NewRalphLoopController(baseDir string) *RalphLoopController {
	ctx, cancel := context.WithCancel(context.Background())
	return &RalphLoopController{
		baseDir: baseDir,
		stage:   StageIdle,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start checks if we can start the loop and initiates it
func (lc *RalphLoopController) Start() error {
	// Guard state: 'R' key only starts the loop if m.Stage == StageIdle
	if lc.stage != StageIdle {
		return fmt.Errorf("cannot start loop: current stage is %s, must be Idle", lc.stage.String())
	}

	// Load PRD to find first story where passes=false
	prdPath := filepath.Join(lc.baseDir, ".clai", "prd.json")
	prd, err := LoadPRD(prdPath)
	if err != nil {
		return fmt.Errorf("failed to load PRD: %w", err)
	}

	// Find first story where passes=false (sorted by priority)
	var firstPending *UserStory
	for i := range prd.UserStories {
		if !prd.UserStories[i].Passes {
			firstPending = &prd.UserStories[i]
			break
		}
	}

	if firstPending == nil {
		// All stories passed - nothing to do
		lc.stage = StageCompleted
		return nil
	}

	// Set active story and start working
	lc.activeStory = firstPending
	lc.stage = StageThinking

	// Append progress log
	if err := AppendLog(firstPending.ID, fmt.Sprintf("Starting Ralph loop - Stage: %s", lc.stage.String()), lc.getGitHash()); err != nil {
		return fmt.Errorf("failed to log: %w", err)
	}

	return nil
}

// Stop cancels the current loop execution
func (lc *RalphLoopController) Stop() error {
	// Interruption: 'ESC' or 'S' key triggers context cancellation
	lc.cancel()
	lc.stage = StagePaused
	return AppendLog("", fmt.Sprintf("Loop stopped - Stage: %s", lc.stage.String()), lc.getGitHash())
}

// Pause pauses the loop execution
func (lc *RalphLoopController) Pause() error {
	lc.cancel()
	lc.stage = StagePaused
	return AppendLog("", fmt.Sprintf("Loop paused - Stage: %s", lc.stage.String()), lc.getGitHash())
}

// Continue resumes the loop from paused state
func (lc *RalphLoopController) Continue() error {
	if lc.stage != StagePaused {
		return fmt.Errorf("cannot continue: current stage is %s", lc.stage.String())
	}

	lc.ctx, lc.cancel = context.WithCancel(context.Background())
	lc.stage = StageThinking
	return nil
}

// GetStage returns the current stage
func (lc *RalphLoopController) GetStage() RalphStage {
	return lc.stage
}

// GetActiveStory returns the currently active story
func (lc *RalphLoopController) GetActiveStory() *UserStory {
	return lc.activeStory
}

// IsIdle returns true if the loop is idle
func (lc *RalphLoopController) IsIdle() bool {
	return lc.stage == StageIdle
}

// IsThinking returns true if the loop is in thinking phase
func (lc *RalphLoopController) IsThinking() bool {
	return lc.stage == StageThinking
}

// IsWorking returns true if the loop is in working phase
func (lc *RalphLoopController) IsWorking() bool {
	return lc.stage == StageWorking
}

// IsCompleted returns true if all stories are complete
func (lc *RalphLoopController) IsCompleted() bool {
	return lc.stage == StageCompleted
}

// IsError returns true if the loop is in error state
func (lc *RalphLoopController) IsError() bool {
	return lc.stage == StageError
}

// CompleteStory marks the current story as passed
func (lc *RalphLoopController) CompleteStory() error {
	if lc.activeStory == nil {
		return fmt.Errorf("no active story to complete")
	}

	// Mark story as passed in PRD
	prdPath := filepath.Join(lc.baseDir, ".clai", "prd.json")
	prd, err := LoadPRD(prdPath)
	if err != nil {
		return fmt.Errorf("failed to load PRD: %w", err)
	}

	for i := range prd.UserStories {
		if prd.UserStories[i].ID == lc.activeStory.ID {
			prd.UserStories[i].Passes = true
			prd.UserStories[i].Updated = time.Now()
			break
		}
	}

	// Save updated PRD
	if err := SavePRD(prdPath, prd); err != nil {
		return fmt.Errorf("failed to save PRD: %w", err)
	}

	// Update progress log
	if err := AppendLog(lc.activeStory.ID, fmt.Sprintf("Story completed - %s", lc.activeStory.Title), lc.getGitHash()); err != nil {
		return fmt.Errorf("failed to log: %w", err)
	}

	// Check if all stories are complete
	allPassed := true
	for i := range prd.UserStories {
		if !prd.UserStories[i].Passes {
			allPassed = false
			break
		}
	}

	if allPassed {
		lc.stage = StageCompleted
		lc.activeStory = nil
		return AppendLog("", "All stories complete - Nothing to do", lc.getGitHash())
	}

	// Find next pending story
	for i := range prd.UserStories {
		if !prd.UserStories[i].Passes {
			lc.activeStory = &prd.UserStories[i]
			lc.stage = StageThinking
			return AppendLog(lc.activeStory.ID, fmt.Sprintf("Next story selected - %s", lc.activeStory.Title), lc.getGitHash())
		}
	}

	return nil
}

// FailStory marks the current story as failed
func (lc *RalphLoopController) FailStory(err error) error {
	if lc.activeStory == nil {
		return fmt.Errorf("no active story to fail")
	}

	lc.stage = StageError
	return AppendLog(lc.activeStory.ID, fmt.Sprintf("Story failed - %v", err), lc.getGitHash())
}

// ContextGathering performs pre-flight stage that reads patterns, history, and workspace files
func (lc *RalphLoopController) ContextGathering() error {
	if lc.stage != StageThinking {
		return fmt.Errorf("can only gather context in Thinking stage, current: %s", lc.stage.String())
	}

	// Read patterns from patterns.md
	patternsPath := filepath.Join(lc.baseDir, ".clai", "patterns.md")
	patternsData, err := os.ReadFile(patternsPath)
	if err != nil {
		return fmt.Errorf("failed to read patterns: %w", err)
	}

	// Read history from progress.txt
	progressPath := filepath.Join(lc.baseDir, ".clai", "progress.txt")
	progressData, err := os.ReadFile(progressPath)
	if err != nil {
		return fmt.Errorf("failed to read progress: %w", err)
	}

	// Extract relevant workspace files for the active story
	workspaceFiles := lc.getRelevantWorkspaceFiles()

	// Append context gathering log
	contextInfo := fmt.Sprintf("Patterns loaded: %d bytes\nProgress loaded: %d bytes\nWorkspace files: %d", len(patternsData), len(progressData), len(workspaceFiles))
	if err := AppendLog(lc.activeStory.ID, fmt.Sprintf("Context gathering complete:\n%s", contextInfo), lc.getGitHash()); err != nil {
		return fmt.Errorf("failed to log: %w", err)
	}

	lc.stage = StageWorking
	return nil
}

// getRelevantWorkspaceFiles returns files relevant to the active story
func (lc *RalphLoopController) getRelevantWorkspaceFiles() []string {
	var files []string

	// Check if story involves UI changes
	if strings.Contains(strings.ToLower(lc.activeStory.Title), "ui") ||
		strings.Contains(strings.ToLower(lc.activeStory.Title), "style") ||
		strings.Contains(strings.ToLower(lc.activeStory.Title), "theme") {
		files = append(files, "views/*.go", "internal/ui/*.go")
	}

	// Check if story involves data structures
	if strings.Contains(strings.ToLower(lc.activeStory.Title), "struct") ||
		strings.Contains(strings.ToLower(lc.activeStory.Title), "data") ||
		strings.Contains(strings.ToLower(lc.activeStory.Title), "memory") {
		files = append(files, "internal/ralph/*.go")
	}

	// Check if story involves testing
	if strings.Contains(strings.ToLower(lc.activeStory.Title), "test") ||
		strings.Contains(strings.ToLower(lc.activeStory.Title), "verify") ||
		strings.Contains(strings.ToLower(lc.activeStory.Title), "quality") {
		files = append(files, "*_test.go")
	}

	return files
}

// getGitHash returns the current git hash
func (lc *RalphLoopController) getGitHash() string {
	hash, err := execCommand("git", "rev-parse", "HEAD")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(hash)
}

// execCommand executes a shell command and returns output
func execCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// GetCompletionBanner returns the "Nothing to do" message when all stories pass
func GetCompletionBanner() string {
	return lipgloss.NewStyle().
		Width(60).
		Height(5).
		Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("2")).
		Background(lipgloss.Color("240")).
		Render("Nothing to do")
}

// GetActiveStoryStyle returns Lipgloss style for highlighting active story
func GetActiveStoryStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("3")).
		Background(lipgloss.Color("236")).
		Padding(1, 2)
}

// GetThinkingStoryStyle returns Lipgloss style for "Thinking" state
func GetThinkingStoryStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")).
		Bold(true).
		Background(lipgloss.Color("236"))
}

// GetPassedStoryStyle returns Lipgloss style for "Passed" state
func GetPassedStoryStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("2")).
		Background(lipgloss.Color("236"))
}

// GetPendingStoryStyle returns Lipgloss style for "Pending" state
func GetPendingStoryStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).
		Background(lipgloss.Color("236"))
}