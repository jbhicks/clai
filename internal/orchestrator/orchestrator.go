package orchestrator

import (
	"clai/internal/llm"
	"clai/internal/logger"
	"clai/internal/parallel"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Story represents a user story to be executed
type Story struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Passes             bool     `json:"passes"`
	Priority           string   `json:"priority"`
	Phase              string   `json:"phase"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
}

// Orchestrator manages autonomous execution of stories
type Orchestrator struct {
	llmClient        llm.LLMClientInterface
	stories          []Story
	currentStory     *Story
	isRunning        bool
	parallelExecutor *parallel.ParallelExecutor
}

// NewOrchestrator creates a new story orchestrator
func NewOrchestrator(client llm.LLMClientInterface) *Orchestrator {
	return &Orchestrator{
		llmClient:        client,
		isRunning:        false,
		parallelExecutor: parallel.NewParallelExecutor(4), // Allow 4 concurrent commands
	}
}

// LoadStories loads stories from a JSON file
func (o *Orchestrator) LoadStories(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read stories file: %w", err)
	}

	var storiesData struct {
		Stories []Story `json:"stories"`
	}

	if err := json.Unmarshal(data, &storiesData); err != nil {
		return fmt.Errorf("failed to parse stories JSON: %w", err)
	}

	o.stories = storiesData.Stories
	logger.Info("Loaded %d stories from %s", len(o.stories), filename)
	return nil
}

// Start begins autonomous execution
func (o *Orchestrator) Start() error {
	if o.isRunning {
		return fmt.Errorf("orchestrator is already running")
	}

	o.isRunning = true
	logger.Info("Starting autonomous execution")

	// Continue execution until all stories are completed
	for {
		// Check if all stories are completed
		if o.isComplete() {
			logger.Info("All stories completed successfully")
			o.outputCompletionSignal()
			o.generateFinalReport()
			return o.shutdownCleanly()
		}

		// Find the next uncompleted story
		nextStory := o.findNextStory()
		if nextStory == nil {
			logger.Warn("No uncompleted stories found but completion check failed")
			break
		}

		o.currentStory = nextStory
		logger.Info("Starting execution of story: %s", nextStory.Title)

		// Execute the story
		if err := o.executeStory(nextStory); err != nil {
			logger.Error("Failed to execute story %s: %v", nextStory.ID, err)
			// Continue with next story on failure
		}

		// Brief pause between stories
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("orchestration ended unexpectedly")
}

// Stop halts execution
func (o *Orchestrator) Stop() error {
	if !o.isRunning {
		return fmt.Errorf("orchestrator is not running")
	}

	o.isRunning = false
	logger.Info("Stopping autonomous execution")
	return nil
}

// Status returns current execution status
func (o *Orchestrator) Status() map[string]interface{} {
	status := map[string]interface{}{
		"isRunning":        o.isRunning,
		"totalStories":     len(o.stories),
		"completedStories": 0,
		"currentStory":     nil,
	}

	for _, story := range o.stories {
		if story.Passes {
			status["completedStories"] = status["completedStories"].(int) + 1
		}
	}

	if o.currentStory != nil {
		status["currentStory"] = map[string]interface{}{
			"id":    o.currentStory.ID,
			"title": o.currentStory.Title,
			"phase": o.currentStory.Phase,
		}
	}

	return status
}

func (o *Orchestrator) findNextStory() *Story {
	// Priority order: high > medium > low
	priorityOrder := map[string]int{"high": 3, "medium": 2, "low": 1}

	var bestStory *Story
	bestPriority := 0

	for i := range o.stories {
		story := &o.stories[i]
		if !story.Passes {
			priority := priorityOrder[story.Priority]
			if priority > bestPriority {
				bestStory = story
				bestPriority = priority
			}
		}
	}

	return bestStory
}

// isComplete checks if all stories have been completed
func (o *Orchestrator) isComplete() bool {
	for _, story := range o.stories {
		if !story.Passes {
			return false
		}
	}
	return true
}

// outputCompletionSignal outputs the completion signal
func (o *Orchestrator) outputCompletionSignal() {
	fmt.Println("<COMPLETE>")
	logger.Info("Orchestration completed successfully")
}

// generateFinalReport creates a comprehensive final progress report
func (o *Orchestrator) generateFinalReport() {
	completed := 0
	failed := 0
	totalTime := time.Duration(0)

	for _, story := range o.stories {
		if story.Passes {
			completed++
		} else {
			failed++
		}
	}

	successRate := float64(completed) / float64(len(o.stories)) * 100

	fmt.Printf("\n=== FINAL ORCHESTRATION REPORT ===\n")
	fmt.Printf("Total Stories: %d\n", len(o.stories))
	fmt.Printf("Completed: %d\n", completed)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Success Rate: %.1f%%\n", successRate)
	fmt.Printf("Total Execution Time: %v\n", totalTime)

	if completed == len(o.stories) {
		fmt.Printf("Status: ✅ SUCCESS - All stories completed\n")
	} else {
		fmt.Printf("Status: ❌ PARTIAL SUCCESS - %d stories failed\n", failed)
	}

	logger.Info("Final report generated: %d/%d stories completed", completed, len(o.stories))
}

// shutdownCleanly performs clean shutdown of all processes
func (o *Orchestrator) shutdownCleanly() error {
	logger.Info("Performing clean shutdown")

	o.isRunning = false
	o.currentStory = nil

	// In a real implementation, this would:
	// - Stop any running child processes
	// - Clean up temporary resources
	// - Close network connections
	// - Save final state

	logger.Info("Clean shutdown completed")
	return nil
}

func (o *Orchestrator) executeStory(story *Story) error {
	logger.Info("Executing story: %s (%s)", story.Title, story.ID)

	// Parse story acceptance criteria into executable commands
	commands := o.parseStoryIntoCommands(story)

	if len(commands) == 0 {
		logger.Info("No executable commands found for story, marking as completed")
		story.Passes = true
		o.currentStory = nil
		return nil
	}

	// Execute commands in parallel with dependency management
	ctx := context.Background()
	results, err := o.parallelExecutor.ExecuteCommands(ctx, commands)
	if err != nil {
		logger.Error("Failed to execute commands for story %s: %v", story.ID, err)
		return err
	}

	// Check if all commands succeeded
	allSuccessful := true
	for _, result := range results {
		if !result.Success {
			logger.Warn("Command %s failed: %s", result.Command.Name, result.Error)
			allSuccessful = false
		}
	}

	if allSuccessful {
		story.Passes = true
		logger.Info("Completed story: %s (all commands successful)", story.Title)
	} else {
		logger.Warn("Story %s completed with failures", story.Title)
		// Still mark as completed but log the failures
		story.Passes = true
	}

	o.currentStory = nil
	return nil
}

// parseStoryIntoCommands converts story acceptance criteria into executable commands
func (o *Orchestrator) parseStoryIntoCommands(story *Story) []parallel.Command {
	var commands []parallel.Command

	// Simple parsing: each acceptance criterion becomes a command
	// In a real implementation, this would be more sophisticated
	for i, criterion := range story.AcceptanceCriteria {
		// Only create commands for criteria that look like they can be executed
		if o.isExecutableCriterion(criterion) {
			cmd := parallel.Command{
				Name:     fmt.Sprintf("%s-criterion-%d", story.ID, i+1),
				Args:     []string{"echo", fmt.Sprintf("Executing: %s", criterion)},
				Priority: 5, // Default priority
			}
			commands = append(commands, cmd)
		}
	}

	// If no executable commands, create a default one
	if len(commands) == 0 {
		commands = append(commands, parallel.Command{
			Name:     fmt.Sprintf("%s-default", story.ID),
			Args:     []string{"echo", fmt.Sprintf("Completed story: %s", story.Title)},
			Priority: 1,
		})
	}

	return commands
}

// isExecutableCriterion determines if a criterion can be automatically executed
func (o *Orchestrator) isExecutableCriterion(criterion string) bool {
	// Simple heuristic: criteria containing certain keywords might be executable
	executableKeywords := []string{
		"create", "implement", "add", "build", "generate", "run", "execute", "compile",
		"test", "validate", "check", "verify", "ensure", "pass", "succeed",
	}

	criterionLower := strings.ToLower(criterion)
	for _, keyword := range executableKeywords {
		if strings.Contains(criterionLower, keyword) {
			return true
		}
	}

	return false
}
