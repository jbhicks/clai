package ralph

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type UserStory struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	Description        string    `json:"description"`
	AcceptanceCriteria []string  `json:"acceptanceCriteria"`
	Priority           string    `json:"priority"`
	Passes             bool      `json:"passes"`
	Phase              string    `json:"phase"`
	Created            time.Time `json:"created"`
	Updated            time.Time `json:"updated"`
}

type Stories struct {
	Version          string      `json:"version"`
	Stories          []UserStory `json:"stories"`
	Completed        int         `json:"completed"`
	Total            int         `json:"total"`
	CurrentIteration int         `json:"currentIteration"`
	StartTime        time.Time   `json:"startTime"`
	LastUpdate       time.Time   `json:"lastUpdate"`
}

type PRD struct {
	Project     string      `json:"project"`
	BranchName  string      `json:"branchName"`
	Description string      `json:"description"`
	UserStories []UserStory `json:"userStories"`
}

// LoadStories reads and validates the stories.json file from the given path.
func LoadStories(path string) (*Stories, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read stories file: %w", err)
	}

	var stories Stories
	if err := json.Unmarshal(data, &stories); err != nil {
		return nil, fmt.Errorf("invalid stories schema: %w", err)
	}

	// Validation
	ids := make(map[string]bool)
	for i, story := range stories.Stories {
		if story.ID == "" {
			return nil, fmt.Errorf("story at index %d missing 'id'", i)
		}
		if ids[story.ID] {
			return nil, fmt.Errorf("duplicate story ID: %s", story.ID)
		}
		ids[story.ID] = true
		if story.Title == "" {
			return nil, fmt.Errorf("story %s missing 'title'", story.ID)
		}
	}

	// Sort stories by priority (high first, then medium, then low), then by creation time
	sort.SliceStable(stories.Stories, func(i, j int) bool {
		priorityOrder := map[string]int{"P0": 0, "high": 1, "medium": 2, "low": 3, "P1": 4, "P2": 5, "P3": 6}
		pi := priorityOrder[stories.Stories[i].Priority]
		pj := priorityOrder[stories.Stories[j].Priority]
		if pi != pj {
			return pi < pj
		}
		return stories.Stories[i].Created.Before(stories.Stories[j].Created)
	})

	return &stories, nil
}

// LoadPRD reads and validates the prd.json file from the given path (legacy support).
func LoadPRD(path string) (*PRD, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PRD file: %w", err)
	}

	var prd PRD
	if err := json.Unmarshal(data, &prd); err != nil {
		return nil, fmt.Errorf("invalid PRD schema: %w", err)
	}

	// Validation
	if prd.Project == "" {
		return nil, fmt.Errorf("PRD missing 'project' field")
	}
	if prd.BranchName == "" {
		return nil, fmt.Errorf("PRD missing 'branchName' field")
	}

	ids := make(map[string]bool)
	for i, story := range prd.UserStories {
		if story.ID == "" {
			return nil, fmt.Errorf("story at index %d missing 'id'", i)
		}
		if ids[story.ID] {
			return nil, fmt.Errorf("duplicate story ID: %s", story.ID)
		}
		ids[story.ID] = true
		if story.Title == "" {
			return nil, fmt.Errorf("story %s missing 'title'", story.ID)
		}
	}

	// Sort stories: priority (ASC), then original order (implicit in slice)
	sort.SliceStable(prd.UserStories, func(i, j int) bool {
		return prd.UserStories[i].Priority < prd.UserStories[j].Priority
	})

	return &prd, nil
}

// AppendLog adds a timestamped entry to the progress.txt file.
func AppendLog(storyID, content, gitHash string) error {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	header := fmt.Sprintf("## [%s] | Story: %s\n", timestamp, storyID)
	body := fmt.Sprintf("Git Hash: %s\n\n%s\n---\n\n", gitHash, content)

	f, err := os.OpenFile(".clai/progress.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(header + body); err != nil {
		return err
	}
	return nil
}

// UpdatePatterns appends a new pattern to patterns.md.
func UpdatePatterns(pattern string) error {
	f, err := os.OpenFile(".clai/patterns.md", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString("- " + pattern + "\n"); err != nil {
		return err
	}
	return nil
}

// Verify runs 'go test ./...' and 'go build' to check project health.
func Verify() (string, error) {
	// Run 'go build' to verify the code compiles
	buildCmd := exec.Command("go", "build", "./...")
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build failed: %w", err)
	}

	// Run 'go test ./...' to verify tests pass
	testCmd := exec.Command("go", "test", "./...")
	testOutput, err := testCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tests failed: %w", err)
	}

	return fmt.Sprintf("Build output:\n%s\n\nTests output:\n%s", string(buildOutput), string(testOutput)), nil
}

// CommitStory creates a git commit for the completed story.
func CommitStory(storyID, title, branchName string) error {
	// Get current git hash for audit trail
	hash, err := execCommand("git", "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get git hash: %w", err)
	}

	// Get current branch
	currentBranch, err := execCommand("git", "branch", "--show-current")
	if err != nil {
		return fmt.Errorf("failed to get branch: %w", err)
	}

	// Get parent commit for the commit message
	parentHash, err := execCommand("git", "rev-parse", "HEAD^")
	if err != nil {
		return fmt.Errorf("failed to get parent hash: %w", err)
	}

	// Get recent commit message for context
	recentMsg, err := execCommand("git", "log", "-1", "--pretty=%B")
	if err != nil {
		return fmt.Errorf("failed to get recent message: %w", err)
	}

	// Create commit message
	commitMessage := fmt.Sprintf("feat: %s - %s\n\nSummary: %s\n\nCommit: %s\nParent: %s\n\nGit Hash: %s",
		storyID, title, recentMsg, parentHash, currentBranch, strings.TrimSpace(hash))

	// Add all files and commit
	_, err = execCommand("git", "add", "-A")
	if err != nil {
		return fmt.Errorf("failed to add files: %w", err)
	}

	_, err = execCommand("git", "commit", "-m", commitMessage)
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}

// SavePRD writes the PRD back to disk.
func SavePRD(path string, prd *PRD) error {
	data, err := json.MarshalIndent(prd, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// FindLineNumber returns the 1-indexed line number where a story ID is defined.
func FindLineNumber(path, storyID string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 1
	}

	lines := splitLines(string(data))
	searchStr := fmt.Sprintf("\"id\": \"%s\"", storyID)

	for i, line := range lines {
		if contains(line, searchStr) {
			return i + 1
		}
	}
	return 1
}

func splitLines(s string) []string {
	var lines []string
	var start int
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
