package main

import (
	"clai/internal/ralph"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// runDecomposeCommand breaks large features into smaller tasks
func runDecomposeCommand(args []string) {
	storiesFile := ".clai/stories.json"
	model := "opencode/claude-opus-4-5"

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--stories":
			if i+1 < len(args) {
				storiesFile = args[i+1]
				i++
			}
		case "--model":
			if i+1 < len(args) {
				model = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Println("clai decompose - Break large features into smaller tasks")
			fmt.Println("")
			fmt.Println("Usage: clai decompose <feature-description> [options]")
			fmt.Println("")
			fmt.Println("Arguments:")
			fmt.Println("  feature-description    Description of feature to decompose")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --stories FILE       Path to stories.json (default: .clai/stories.json)")
			fmt.Println("  --model MODEL         Model to use (default: opencode/claude-opus-4-5)")
			fmt.Println("  --help, -h           Show this help")
			os.Exit(0)
		}
	}

	// Get feature description from args
	var featureDesc string
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		featureDesc = strings.Join(args, " ")
	}

	if featureDesc == "" {
		fmt.Fprintf(os.Stderr, "Error: Feature description required\n")
		fmt.Println("Use --help for usage information")
		os.Exit(1)
	}

	fmt.Printf("🔍 Decomosing feature: %s\n", featureDesc)
	fmt.Printf("📝 Using model: %s\n", model)
	fmt.Printf("💾 Stories file: %s\n", storiesFile)
	fmt.Println()

	// Load existing stories
	stories, err := ralph.LoadStories(storiesFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load stories: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📋 Current stories: %d\n", len(stories.Stories))

	// For now, simulate task decomposition
	// TODO: Integrate with internal/llm/agent.go for actual decomposition

	// Generate new story ID
	newID := fmt.Sprintf("TASK-%03d", len(stories.Stories)+1)

	// Create new story from feature description
	newStory := ralph.UserStory{
		ID:          newID,
		Title:       fmt.Sprintf("Implement %s", extractFeatureTitle(featureDesc)),
		Phase:       "decomposition",
		Priority:    "high",
		Created:     time.Now(),
		Updated:     time.Now(),
		Passes:      false,
		Description: fmt.Sprintf("Break down and implement the following feature: %s", featureDesc),
		AcceptanceCriteria: []string{
			"Analyze feature requirements and break into executable tasks",
			"Ensure tasks are sized for CLAI context window",
			"Create dependency ordering between tasks",
			"Export tasks to stories.json with proper prioritization",
			"Validate task breakdown for completeness and feasibility",
			"Typecheck passes",
		},
	}

	// Add new story to existing stories
	stories.Stories = append(stories.Stories, newStory)
	stories.Total = len(stories.Stories)

	fmt.Printf("✨ Created new story: %s\n", newID)
	fmt.Printf("📋 Title: %s\n", newStory.Title)
	fmt.Println("📋 Acceptance Criteria:")
	for i, criterion := range newStory.AcceptanceCriteria {
		fmt.Printf("   %d. [ ] %s\n", i+1, criterion)
	}
	fmt.Println()

	// Save updated stories
	storiesData, err := json.MarshalIndent(stories, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal stories: %v\n", err)
		os.Exit(1)
	}

	err = os.WriteFile(storiesFile, storiesData, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save stories: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("💾 Saved %d stories to %s\n", len(stories.Stories), storiesFile)
	fmt.Printf("🎯 New story %s ready for execution\n", newID)
}

// extractFeatureTitle extracts a short title from feature description
func extractFeatureTitle(desc string) string {
	// Simple heuristics to extract main feature
	words := strings.Fields(desc)
	if len(words) == 0 {
		return "New Feature"
	}

	// Use first few words as title, limit length
	title := words[0]
	if len(words) > 1 && len(title)+len(words[1]) < 30 {
		title += " " + words[1]
	}

	// Capitalize first letter
	if len(title) > 0 {
		title = strings.ToUpper(title[:1]) + title[1:]
	}

	return title
}

// SaveStories saves stories back to file (helper function)
func SaveStories(path string, stories *ralph.Stories) error {
	data, err := json.MarshalIndent(stories, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
