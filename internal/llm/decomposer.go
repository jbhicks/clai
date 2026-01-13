package llm

import (
	"clai/internal/logger"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// DecomposedTask represents a single task in a decomposed feature
type DecomposedTask struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	Priority           string    `json:"priority"` // "high", "medium", "low"
	Phase              string    `json:"phase"`    // "schema", "backend", "ui", etc.
	Dependencies       []string  `json:"dependencies,omitempty"`
	AcceptanceCriteria []string  `json:"acceptanceCriteria"`
	EstimatedTokens    int       `json:"estimatedTokens"`
	Created            time.Time `json:"created"`
}

// TaskDecomposer handles feature decomposition into manageable tasks
type TaskDecomposer struct {
	llmClient         LLMClientInterface
	contextWindowSize int // tokens
}

// NewTaskDecomposer creates a new task decomposer
func NewTaskDecomposer(client LLMClientInterface) *TaskDecomposer {
	return &TaskDecomposer{
		llmClient:         client,
		contextWindowSize: 4096, // Default context window, could be configurable
	}
}

// DecomposeFeature breaks down a feature description into manageable tasks
func (td *TaskDecomposer) DecomposeFeature(featureDescription string) ([]DecomposedTask, error) {
	logger.Info("Decomposing feature: %s", featureDescription)

	// Try LLM-based decomposition first
	tasks, err := td.decomposeWithLLM(featureDescription)
	if err != nil {
		logger.Warn("LLM decomposition failed, falling back to rule-based decomposition: %v", err)
		tasks = td.decomposeWithRules(featureDescription)
	}

	// Validate and adjust task sizes
	tasks = td.validateAndAdjustTasks(tasks)

	logger.Info("Successfully decomposed into %d tasks", len(tasks))
	return tasks, nil
}

// decomposeWithLLM attempts decomposition using the LLM
func (td *TaskDecomposer) decomposeWithLLM(featureDescription string) ([]DecomposedTask, error) {
	prompt := td.buildDecompositionPrompt(featureDescription)

	messages := []Message{
		{Role: "system", Content: "You are an expert software architect who breaks down complex features into small, executable tasks. Focus on creating tasks that fit within LLM context windows and follow proper dependency ordering."},
		{Role: "user", Content: prompt},
	}

	// Try non-streaming first for better compatibility
	streamChan := make(chan string, 100)
	response, err := td.llmClient.SendMessageStreamNoTools(messages, streamChan, true)
	if err != nil {
		return nil, fmt.Errorf("LLM decomposition failed: %w", err)
	}

	// Collect any streaming content
	var streamContent strings.Builder
	timeout := time.After(5 * time.Second)
	for {
		select {
		case chunk, ok := <-streamChan:
			if !ok {
				goto collectDone
			}
			streamContent.WriteString(chunk)
		case <-timeout:
			logger.Warn("Timeout waiting for streaming response")
			goto collectDone
		}
	}
collectDone:

	// Use streaming content if available, otherwise use response content
	content := streamContent.String()
	if content == "" && response.Message.Content != "" {
		content = response.Message.Content
	}

	if content == "" {
		return nil, fmt.Errorf("empty LLM response")
	}

	logger.Debug("LLM Response: %s", response.Message.Content)

	tasks, err := td.parseDecompositionResponse(response.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse decomposition response: %w", err)
	}

	return tasks, nil
}

// decomposeWithRules provides fallback decomposition using predefined rules
func (td *TaskDecomposer) decomposeWithRules(featureDescription string) []DecomposedTask {
	logger.Info("Using rule-based decomposition for: %s", featureDescription)

	// Simple rule-based decomposition for authentication feature
	if strings.Contains(strings.ToLower(featureDescription), "authentication") ||
		strings.Contains(strings.ToLower(featureDescription), "auth") {

		return []DecomposedTask{
			{
				ID:           "TASK-001",
				Title:        "Create user authentication database schema",
				Description:  "Design and implement database tables for user accounts, sessions, and authentication tokens",
				Priority:     "high",
				Phase:        "schema",
				Dependencies: []string{},
				AcceptanceCriteria: []string{
					"Users table with email, password hash, created_at",
					"Sessions table with user_id, token, expires_at",
					"Database migration script created",
					"Typecheck passes",
				},
				EstimatedTokens: 800,
				Created:         time.Now(),
			},
			{
				ID:           "TASK-002",
				Title:        "Implement JWT token generation and validation",
				Description:  "Create JWT service for generating access tokens and validating incoming requests",
				Priority:     "high",
				Phase:        "backend",
				Dependencies: []string{"TASK-001"},
				AcceptanceCriteria: []string{
					"JWT service with sign and verify methods",
					"Token expiration handling",
					"Environment-based secret key configuration",
					"Unit tests for token operations",
					"Typecheck passes",
				},
				EstimatedTokens: 1200,
				Created:         time.Now(),
			},
			{
				ID:           "TASK-003",
				Title:        "Create user registration endpoint",
				Description:  "Implement POST /api/auth/register endpoint with input validation and password hashing",
				Priority:     "high",
				Phase:        "backend",
				Dependencies: []string{"TASK-001", "TASK-002"},
				AcceptanceCriteria: []string{
					"POST /api/auth/register endpoint",
					"Email validation and uniqueness checking",
					"Password hashing with bcrypt",
					"Success response with user data",
					"Error handling for validation failures",
					"Typecheck passes",
				},
				EstimatedTokens: 1000,
				Created:         time.Now(),
			},
			{
				ID:           "TASK-004",
				Title:        "Create user login endpoint",
				Description:  "Implement POST /api/auth/login endpoint that validates credentials and returns JWT token",
				Priority:     "high",
				Phase:        "backend",
				Dependencies: []string{"TASK-001", "TASK-002", "TASK-003"},
				AcceptanceCriteria: []string{
					"POST /api/auth/login endpoint",
					"Password verification against stored hash",
					"JWT token generation on successful login",
					"Session storage in database",
					"Error responses for invalid credentials",
					"Typecheck passes",
				},
				EstimatedTokens: 1100,
				Created:         time.Now(),
			},
			{
				ID:           "TASK-005",
				Title:        "Add JWT authentication middleware",
				Description:  "Create middleware to validate JWT tokens on protected routes",
				Priority:     "medium",
				Phase:        "backend",
				Dependencies: []string{"TASK-002"},
				AcceptanceCriteria: []string{
					"JWT validation middleware function",
					"Extract user ID from valid tokens",
					"Attach user context to request",
					"401 responses for invalid/missing tokens",
					"Typecheck passes",
				},
				EstimatedTokens: 900,
				Created:         time.Now(),
			},
			{
				ID:           "TASK-006",
				Title:        "Create login/register UI components",
				Description:  "Build React/HTML forms for user registration and login",
				Priority:     "medium",
				Phase:        "ui",
				Dependencies: []string{"TASK-003", "TASK-004"},
				AcceptanceCriteria: []string{
					"Registration form with email/password fields",
					"Login form with email/password fields",
					"Form validation and error display",
					"Success feedback and redirect",
					"Responsive design",
					"Typecheck passes",
				},
				EstimatedTokens: 1500,
				Created:         time.Now(),
			},
		}
	}

	// Generic fallback for unknown features
	return []DecomposedTask{
		{
			ID:           "TASK-001",
			Title:        "Analyze feature requirements",
			Description:  fmt.Sprintf("Analyze the requirements for: %s", featureDescription),
			Priority:     "high",
			Phase:        "backend",
			Dependencies: []string{},
			AcceptanceCriteria: []string{
				"Requirements documented",
				"Technical approach defined",
				"Acceptance criteria written",
			},
			EstimatedTokens: 800,
			Created:         time.Now(),
		},
		{
			ID:           "TASK-002",
			Title:        "Implement core functionality",
			Description:  fmt.Sprintf("Implement the core functionality for: %s", featureDescription),
			Priority:     "high",
			Phase:        "backend",
			Dependencies: []string{"TASK-001"},
			AcceptanceCriteria: []string{
				"Core logic implemented",
				"Unit tests written",
				"Typecheck passes",
			},
			EstimatedTokens: 1500,
			Created:         time.Now(),
		},
	}
}

// buildDecompositionPrompt creates the prompt for task decomposition
func (td *TaskDecomposer) buildDecompositionPrompt(featureDescription string) string {
	return fmt.Sprintf(`Please decompose the following feature into small, executable tasks:

Feature: %s

Requirements:
1. Each task must be completable within a single LLM context window (max ~4000 tokens)
2. Tasks should follow dependency ordering: schema → backend → UI
3. Include clear acceptance criteria for each task
4. Assign appropriate priorities (high, medium, low)
5. Estimate token usage for each task
6. Focus on incremental, testable changes

Output Format (JSON):
{
  "tasks": [
    {
      "id": "TASK-001",
      "title": "Short descriptive title",
      "description": "Detailed description of what to implement",
      "priority": "high|medium|low",
      "phase": "schema|backend|ui|testing",
      "dependencies": ["TASK-XXX"],
      "acceptanceCriteria": [
        "Clear, testable criteria",
        "Another criteria"
      ],
      "estimatedTokens": 1500
    }
  ]
}

Ensure tasks are small enough to complete in one focused session and include proper validation steps.`, featureDescription)
}

// parseDecompositionResponse extracts tasks from LLM response
func (td *TaskDecomposer) parseDecompositionResponse(response string) ([]DecomposedTask, error) {
	// Try to extract JSON from response
	jsonPattern := regexp.MustCompile(`(?s)\{.*\}`)
	jsonMatch := jsonPattern.FindString(response)

	if jsonMatch == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var result struct {
		Tasks []struct {
			ID                 string   `json:"id"`
			Title              string   `json:"title"`
			Description        string   `json:"description"`
			Priority           string   `json:"priority"`
			Phase              string   `json:"phase"`
			Dependencies       []string `json:"dependencies"`
			AcceptanceCriteria []string `json:"acceptanceCriteria"`
			EstimatedTokens    int      `json:"estimatedTokens"`
		} `json:"tasks"`
	}

	if err := json.Unmarshal([]byte(jsonMatch), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	tasks := make([]DecomposedTask, len(result.Tasks))
	for i, t := range result.Tasks {
		tasks[i] = DecomposedTask{
			ID:                 t.ID,
			Title:              t.Title,
			Description:        t.Description,
			Priority:           t.Priority,
			Phase:              t.Phase,
			Dependencies:       t.Dependencies,
			AcceptanceCriteria: t.AcceptanceCriteria,
			EstimatedTokens:    t.EstimatedTokens,
			Created:            time.Now(),
		}
	}

	return tasks, nil
}

// validateAndAdjustTasks ensures tasks meet size and dependency requirements
func (td *TaskDecomposer) validateAndAdjustTasks(tasks []DecomposedTask) []DecomposedTask {
	validatedTasks := make([]DecomposedTask, 0, len(tasks))

	for _, task := range tasks {
		// Adjust estimated tokens if too high
		if task.EstimatedTokens > td.contextWindowSize {
			task.EstimatedTokens = td.contextWindowSize - 500 // Leave buffer
			logger.Warn("Adjusted task %s token estimate to %d", task.ID, task.EstimatedTokens)
		}

		// Validate priority
		if task.Priority != "high" && task.Priority != "medium" && task.Priority != "low" {
			task.Priority = "medium"
		}

		// Ensure ID format
		if !strings.HasPrefix(task.ID, "TASK-") {
			task.ID = fmt.Sprintf("TASK-%03d", len(validatedTasks)+1)
		}

		validatedTasks = append(validatedTasks, task)
	}

	// Sort by dependency order (basic implementation)
	validatedTasks = td.sortByDependencies(validatedTasks)

	return validatedTasks
}

// sortByDependencies performs basic topological sort for dependencies
func (td *TaskDecomposer) sortByDependencies(tasks []DecomposedTask) []DecomposedTask {
	// Simple phase-based sorting for now
	phaseOrder := map[string]int{
		"schema":  1,
		"backend": 2,
		"ui":      3,
		"testing": 4,
	}

	// Sort by phase order, then by priority
	sorted := make([]DecomposedTask, len(tasks))
	copy(sorted, tasks)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			phaseI := phaseOrder[sorted[i].Phase]
			phaseJ := phaseOrder[sorted[j].Phase]

			shouldSwap := false
			if phaseI > phaseJ {
				shouldSwap = true
			} else if phaseI == phaseJ {
				// Same phase, sort by priority
				priorityOrder := map[string]int{"high": 3, "medium": 2, "low": 1}
				priI := priorityOrder[sorted[i].Priority]
				priJ := priorityOrder[sorted[j].Priority]
				if priI < priJ {
					shouldSwap = true
				}
			}

			if shouldSwap {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// ExportToStoriesJSON exports decomposed tasks to the stories.json format
func (td *TaskDecomposer) ExportToStoriesJSON(tasks []DecomposedTask) error {
	claiDir := ".clai"
	storiesFile := filepath.Join(claiDir, "stories.json")

	// Read existing stories
	existingContent, err := os.ReadFile(storiesFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing stories: %w", err)
	}

	var stories struct {
		Version          string        `json:"version"`
		Stories          []interface{} `json:"stories"`
		Completed        int           `json:"completed"`
		Total            int           `json:"total"`
		CurrentIteration int           `json:"currentIteration"`
		StartTime        interface{}   `json:"startTime"`
		LastUpdate       time.Time     `json:"lastUpdate"`
	}

	if len(existingContent) > 0 {
		if err := json.Unmarshal(existingContent, &stories); err != nil {
			return fmt.Errorf("failed to parse existing stories: %w", err)
		}
	} else {
		// Initialize default structure
		stories.Version = "1.0"
		stories.Stories = []interface{}{}
		stories.Completed = 0
		stories.Total = 0
		stories.CurrentIteration = 0
		stories.LastUpdate = time.Now()
	}

	// Convert decomposed tasks to story format
	for _, task := range tasks {
		story := map[string]interface{}{
			"id":                 task.ID,
			"title":              task.Title,
			"description":        task.Description,
			"acceptanceCriteria": task.AcceptanceCriteria,
			"passes":             false,
			"priority":           task.Priority,
			"phase":              task.Phase,
			"created":            task.Created.Format(time.RFC3339),
			"updated":            time.Now().Format(time.RFC3339),
		}
		stories.Stories = append(stories.Stories, story)
		stories.Total++
	}

	// Write back to file
	data, err := json.MarshalIndent(stories, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stories: %w", err)
	}

	if err := os.WriteFile(storiesFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write stories file: %w", err)
	}

	logger.Info("Exported %d tasks to stories.json", len(tasks))
	return nil
}

// SetContextWindowSize allows configuring the context window size
func (td *TaskDecomposer) SetContextWindowSize(tokens int) {
	td.contextWindowSize = tokens
	logger.Info("Set context window size to %d tokens", tokens)
}
