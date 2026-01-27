package main

import (
	"clai/internal/db"
	"clai/internal/llm"
	"clai/internal/logger"
	"clai/internal/ralph"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// runOrchestrateCommand starts Ralph autonomous development loop
func runOrchestrateCommand(args []string) {
	storiesFile := ".clai/stories.json"
	maxIterations := 50
	model := "opencode/claude-opus-4-5"
	watch := false
	singleIteration := false

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--stories":
			if i+1 < len(args) {
				storiesFile = args[i+1]
				i++
			}
		case "--max-iterations":
			if i+1 < len(args) {
				if val, err := strconv.Atoi(args[i+1]); err == nil {
					maxIterations = val
				}
				i++
			}
		case "--model":
			if i+1 < len(args) {
				model = args[i+1]
				i++
			}
		case "--verbose", "-v":
			// Verbose logging would be implemented here
		case "--watch":
			watch = true
		case "--single":
			singleIteration = true
		case "--help", "-h":
			fmt.Println("clai orchestrate - Run Ralph autonomous development loop")
			fmt.Println("")
			fmt.Println("Usage: clai orchestrate [options]")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --stories FILE         Path to stories.json (default: .clai/stories.json)")
			fmt.Println("  --max-iterations N     Maximum iterations (default: 50)")
			fmt.Println("  --model MODEL          Model to use (default: opencode/claude-opus-4-5)")
			fmt.Println("  --verbose, -v         Enable verbose logging")
			fmt.Println("  --watch               Watch mode with live updates")
			fmt.Println("  --single              Run single iteration and exit")
			fmt.Println("  --help, -h           Show this help")
			os.Exit(0)
		default:
			fmt.Printf("Unknown option: %s\n", args[i])
			fmt.Println("Use --help for usage information")
			os.Exit(1)
		}
	}

	fmt.Printf("🏔️  Starting Ralph autonomous development loop...\n")
	fmt.Printf("   Stories file: %s\n", storiesFile)
	fmt.Printf("   Max iterations: %d\n", maxIterations)
	fmt.Printf("   Model: %s\n", model)
	if watch {
		fmt.Printf("   Watch mode: enabled\n")
	}
	fmt.Println("")

	// Load stories using existing Ralph library
	stories, err := ralph.LoadStories(storiesFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load stories: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📋 Loaded %d stories (%d completed)\n", len(stories.Stories), stories.Completed)
	fmt.Println("")

	// Filter incomplete stories
	var incompleteStories []ralph.UserStory
	for _, story := range stories.Stories {
		if !story.Passes {
			incompleteStories = append(incompleteStories, story)
		}
	}

	if len(incompleteStories) == 0 {
		fmt.Println("✅ All stories completed!")
		os.Exit(0)
	}

	// Run Ralph loop
	startTime := time.Now()
	iteration := 0
	allComplete := false

	// If single iteration mode, set maxIterations to 1
	if singleIteration {
		maxIterations = 1
		fmt.Println("🔄 Single iteration mode - will run one task and exit")
		fmt.Println("")
	}

	for iteration < maxIterations && !allComplete {
		iteration++
		fmt.Printf("🔄 Iteration %d/%d\n", iteration, maxIterations)

		// Find next incomplete story
		var nextStory *ralph.UserStory
		for i, story := range incompleteStories {
			if !story.Passes {
				nextStory = &incompleteStories[i]
				break
			}
		}

		if nextStory == nil {
			fmt.Println("✅ All stories completed!")
			allComplete = true
			break
		}

		fmt.Printf("📝 Executing: %s\n", nextStory.Title)

		// Execute task using real LLM agent
		if err := executeTaskWithAgentOrchestrator(nextStory, model); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Task execution failed: %v\n", err)
			// Don't mark as completed if execution failed
			continue
		}

		// Mark as completed (successful execution)
		for i, story := range incompleteStories {
			if story.ID == nextStory.ID {
				incompleteStories[i].Passes = true
				incompleteStories[i].Updated = time.Now()
				break
			}
		}

		fmt.Printf("✅ Completed: %s\n", nextStory.Title)
		time.Sleep(1 * time.Second) // Brief pause between iterations
	}

	duration := time.Since(startTime)
	completed := stories.Completed
	for _, story := range incompleteStories {
		if story.Passes {
			completed++
		}
	}

	fmt.Println("")
	fmt.Printf("📊 Final Results:\n")
	fmt.Printf("   Total iterations: %d\n", iteration)
	fmt.Printf("   Stories completed: %d/%d\n", completed, len(stories.Stories))
	fmt.Printf("   Duration: %v\n", duration.Round(time.Second))
	fmt.Println("")

	// Save updated stories
	stories.Completed = completed
	stories.CurrentIteration = iteration
	stories.LastUpdate = time.Now()

	// Save stories back to file
	storiesData, err := json.MarshalIndent(stories, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save stories: %v\n", err)
	}
	os.WriteFile(storiesFile, storiesData, 0644)

	if allComplete {
		fmt.Println("🎉 <COMPLETE>")
		os.Exit(0)
	} else {
		fmt.Printf("⚠️  %d stories remain incomplete\n", len(stories.Stories)-completed)
		os.Exit(1)
	}
}

// executeTaskWithAgentOrchestrator executes a single story using the LLM agent
func executeTaskWithAgentOrchestrator(story *ralph.UserStory, model string) error {
	// Load environment variables
	_ = godotenv.Load()

	// Set up LLM client
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:8081"
	}

	llmModel := os.Getenv("OLLAMA_MODEL")
	if llmModel == "" {
		llmModel = "llama3.1-gpu:latest"
	}
	if model != "" {
		llmModel = model
	}

	systemPrompt := os.Getenv("SYSTEM_PROMPT")
	if systemPrompt == "" {
		systemPrompt = "You are an expert software development assistant. Execute tasks efficiently and provide working code solutions."
	}

	// Initialize LLM client and agent
	llmClient := llm.NewClient(host, llmModel, systemPrompt)
	agent := llm.NewAgent(llmClient)

	// Initialize database for conversation storage (optional for CLI mode)
	store, err := db.New()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer store.Close()

	// Load latest conversation for context
	conv, err := store.GetLatestConversation()
	if err != nil {
		logger.Warn("Failed to load conversation: %v", err)
		conv = &db.Conversation{
			Messages: []llm.Message{},
		}
	}

	// Add task context to agent
	taskPrompt := fmt.Sprintf(`Execute the following task:

Title: %s
Description: %s
Phase: %s
Priority: %s

Acceptance Criteria:
%s

Please execute this task step by step. Use appropriate tools to read, write, and test code as needed. Provide working solutions that meet all acceptance criteria.`,
		story.Title,
		story.Description,
		story.Phase,
		story.Priority,
		strings.Join(story.AcceptanceCriteria, "\n"))

	// Add task to agent's message history
	agent.AddMessage("user", taskPrompt)

	// Execute the task using the agent
	response, err := agent.Run(taskPrompt)
	if err != nil {
		return fmt.Errorf("agent execution failed: %w", err)
	}

	// Display execution results
	fmt.Printf("🤖 Agent response:\n%s\n", response)

	// Save the conversation
	conv.Messages = append(conv.Messages, llm.Message{Role: "user", Content: taskPrompt})
	conv.Messages = append(conv.Messages, llm.Message{Role: "assistant", Content: response})

	if err := store.SaveConversation(conv); err != nil {
		logger.Warn("Failed to save conversation: %v", err)
	}

	return nil
}
