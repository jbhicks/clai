package patterns

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"clai/internal/logger"
)

// Learning represents a discovered pattern or insight
type Learning struct {
	ID         string     `json:"id"`
	Category   string     `json:"category"`   // "api_usage", "bug_fix", "performance", "architecture", etc.
	Pattern    string     `json:"pattern"`    // The actual pattern/insight
	Context    string     `json:"context"`    // When/where this applies
	Confidence float64    `json:"confidence"` // 0.0 to 1.0
	Source     string     `json:"source"`     // Which task/component this came from
	Timestamp  time.Time  `json:"timestamp"`
	Importance int        `json:"importance"` // 1-5, how important this learning is
	Tags       []string   `json:"tags"`       // Related tags for search
	Frequency  int        `json:"frequency"`  // How often this pattern has been observed
	LastUsed   *time.Time `json:"lastUsed"`   // When this was last applied
}

// PatternManager handles pattern discovery and AGENTS.md updates
type PatternManager struct {
	projectRoot         string
	patternsFile        string
	agentsMdFile        string
	learnings           []Learning
	maxLearnings        int
	importanceThreshold int
}

// NewPatternManager creates a new pattern manager
func NewPatternManager(projectRoot string) (*PatternManager, error) {
	patternsDir := filepath.Join(projectRoot, ".clai")
	patternsFile := filepath.Join(patternsDir, "patterns.log")
	agentsMdFile := filepath.Join(projectRoot, "AGENTS.md")

	return &PatternManager{
		projectRoot:         projectRoot,
		patternsFile:        patternsFile,
		agentsMdFile:        agentsMdFile,
		learnings:           []Learning{},
		maxLearnings:        1000, // Keep last 1000 learnings
		importanceThreshold: 3,    // Only add learnings with importance >= 3 to AGENTS.md
	}, nil
}

// RecordLearning captures a new pattern or insight
func (pm *PatternManager) RecordLearning(category, pattern, context, source string, confidence float64, importance int, tags []string) error {
	logger.Debug("RecordLearning: category=%s, pattern_len=%d, importance=%d", category, len(pattern), importance)

	// Safely truncate pattern for ID generation
	safePattern := strings.ReplaceAll(pattern, " ", "_")
	if len(safePattern) == 0 {
		safePattern = "empty"
		logger.Warn("RecordLearning: empty pattern received, using fallback ID")
	}
	idPattern := safePattern
	if len(idPattern) > 20 {
		idPattern = idPattern[:20]
	}

	learning := Learning{
		ID:         fmt.Sprintf("learn_%d_%s", time.Now().Unix(), idPattern),
		Category:   category,
		Pattern:    pattern,
		Context:    context,
		Confidence: confidence,
		Source:     source,
		Timestamp:  time.Now(),
		Importance: importance,
		Tags:       tags,
		Frequency:  1,
	}

	// Check if this pattern already exists
	if existing := pm.findExistingPattern(pattern, category); existing != nil {
		existing.Frequency++
		existing.LastUsed = &learning.Timestamp
		existing.Confidence = (existing.Confidence + confidence) / 2
		if existing.Importance < importance {
			existing.Importance = importance
		}
		logger.Info("Updated existing pattern: %s (frequency: %d)", pattern, existing.Frequency)
	} else {
		pm.learnings = append(pm.learnings, learning)
		logger.Info("Recorded new learning: %s (importance: %d)", pattern, importance)
	}

	// Keep only the most recent learnings
	if len(pm.learnings) > pm.maxLearnings {
		pm.learnings = pm.learnings[len(pm.learnings)-pm.maxLearnings:]
	}

	// Auto-update AGENTS.md if this is an important learning
	if importance >= pm.importanceThreshold {
		return pm.updateAgentsMd(learning)
	}

	return pm.saveLearningsToFile()
}

// findExistingPattern looks for an existing pattern with similar content
func (pm *PatternManager) findExistingPattern(pattern, category string) *Learning {
	// Simple similarity check - could be made more sophisticated
	// Safely truncate pattern for comparison
	compareLen := min(20, len(pattern))
	if compareLen == 0 {
		logger.Warn("findExistingPattern: empty pattern received")
		return nil
	}
	comparePrefix := pattern[:compareLen]
	logger.Debug("findExistingPattern: looking for pattern prefix=%s (len=%d)", comparePrefix, compareLen)

	for i := range pm.learnings {
		if pm.learnings[i].Category == category &&
			strings.Contains(pm.learnings[i].Pattern, comparePrefix) {
			logger.Debug("findExistingPattern: found matching pattern at index %d", i)
			return &pm.learnings[i]
		}
	}
	return nil
}

// updateAgentsMd adds important learnings to AGENTS.md
func (pm *PatternManager) updateAgentsMd(learning Learning) error {
	// Read existing AGENTS.md
	content, err := os.ReadFile(pm.agentsMdFile)
	if err != nil {
		if os.IsNotExist(err) {
			content = []byte("# CLAI Agent Guidelines\n\n")
		} else {
			return fmt.Errorf("failed to read AGENTS.md: %w", err)
		}
	}

	// Check if this learning is already documented
	// Safely truncate pattern for comparison
	patternLen := len(learning.Pattern)
	searchLen := min(50, patternLen)
	var searchPrefix string
	if searchLen > 0 {
		searchPrefix = learning.Pattern[:searchLen]
	} else {
		searchPrefix = ""
		logger.Warn("updateAgentsMd: empty pattern received")
	}
	logger.Debug("updateAgentsMd: checking if pattern is documented, prefix=%s (len=%d)", searchPrefix, searchLen)

	if strings.Contains(string(content), searchPrefix) {
		logger.Debug("Learning already documented in AGENTS.md: %s", learning.Pattern)
		return nil
	}

	// Find the appropriate section or create one
	sectionName := pm.getSectionName(learning.Category)
	sectionPattern := regexp.MustCompile(fmt.Sprintf(`(?m)^## %s$`, regexp.QuoteMeta(sectionName)))

	newEntry := pm.formatLearningEntry(learning)

	if sectionPattern.Match(content) {
		// Section exists, add to it
		sectionWithEntry := fmt.Sprintf("## %s\n\n%s\n\n", sectionName, newEntry)
		content = sectionPattern.ReplaceAllLiteral(content, []byte(sectionWithEntry))
	} else {
		// Create new section at the end
		newSection := fmt.Sprintf("\n## %s\n\n%s\n", sectionName, newEntry)
		content = append(content, []byte(newSection)...)
	}

	// Write back to file
	if err := os.WriteFile(pm.agentsMdFile, content, 0644); err != nil {
		return fmt.Errorf("failed to update AGENTS.md: %w", err)
	}

	logger.Info("Updated AGENTS.md with new learning: %s", learning.Pattern)
	return nil
}

// getSectionName maps categories to AGENTS.md section names
func (pm *PatternManager) getSectionName(category string) string {
	switch category {
	case "api_usage":
		return "API Usage Patterns"
	case "bug_fix":
		return "Common Bug Fixes"
	case "performance":
		return "Performance Optimizations"
	case "architecture":
		return "Architecture Decisions"
	case "testing":
		return "Testing Patterns"
	case "deployment":
		return "Deployment Guidelines"
	default:
		return "General Patterns"
	}
}

// formatLearningEntry formats a learning for AGENTS.md
func (pm *PatternManager) formatLearningEntry(learning Learning) string {
	tags := ""
	if len(learning.Tags) > 0 {
		tags = fmt.Sprintf(" *Tags: %s*", strings.Join(learning.Tags, ", "))
	}

	context := ""
	if learning.Context != "" {
		context = fmt.Sprintf("\n\n*Context: %s*", learning.Context)
	}

	return fmt.Sprintf("### %s\n\n%s%s\n\n*Source: %s | Confidence: %.1f | Importance: %d*%s",
		learning.Pattern,
		learning.Pattern,
		context,
		learning.Source,
		learning.Confidence,
		learning.Importance,
		tags)
}

// SearchLearnings searches for patterns matching the query
func (pm *PatternManager) SearchLearnings(query string, categoryFilter string, minImportance int) []Learning {
	var results []Learning

	query = strings.ToLower(query)

	for _, learning := range pm.learnings {
		if learning.Importance < minImportance {
			continue
		}

		if categoryFilter != "" && learning.Category != categoryFilter {
			continue
		}

		// Search in pattern, context, and tags
		searchable := strings.ToLower(learning.Pattern + " " + learning.Context + " " + strings.Join(learning.Tags, " "))
		if strings.Contains(searchable, query) {
			results = append(results, learning)
		}
	}

	// Sort by relevance (importance, then recency)
	sort.Slice(results, func(i, j int) bool {
		if results[i].Importance != results[j].Importance {
			return results[i].Importance > results[j].Importance
		}
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	return results
}

// GetLearningsByCategory returns all learnings in a specific category
func (pm *PatternManager) GetLearningsByCategory(category string) []Learning {
	var results []Learning
	for _, learning := range pm.learnings {
		if learning.Category == category {
			results = append(results, learning)
		}
	}
	return results
}

// GetTopLearnings returns the most important learnings
func (pm *PatternManager) GetTopLearnings(limit int) []Learning {
	if limit <= 0 {
		limit = 10
	}

	// Sort by importance and frequency
	sorted := make([]Learning, len(pm.learnings))
	copy(sorted, pm.learnings)

	sort.Slice(sorted, func(i, j int) bool {
		// Primary sort: importance
		if sorted[i].Importance != sorted[j].Importance {
			return sorted[i].Importance > sorted[j].Importance
		}
		// Secondary sort: frequency
		if sorted[i].Frequency != sorted[j].Frequency {
			return sorted[i].Frequency > sorted[j].Frequency
		}
		// Tertiary sort: recency
		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})

	if len(sorted) < limit {
		limit = len(sorted)
	}

	return sorted[:limit]
}

// CompactLearnings reduces memory usage by removing less important learnings
func (pm *PatternManager) CompactLearnings() {
	logger.Info("Starting pattern compaction...")

	before := len(pm.learnings)

	// Keep only learnings with importance >= 2 and frequency >= 1
	// Or learnings from the last 30 days
	cutoff := time.Now().AddDate(0, 0, -30)

	var compacted []Learning
	for _, learning := range pm.learnings {
		if learning.Importance >= 2 || learning.Frequency >= 2 || learning.Timestamp.After(cutoff) {
			compacted = append(compacted, learning)
		}
	}

	pm.learnings = compacted
	after := len(pm.learnings)

	logger.Info("Pattern compaction completed: %d -> %d learnings", before, after)
}

// saveLearningsToFile persists learnings to the patterns.log file
func (pm *PatternManager) saveLearningsToFile() error {
	file, err := os.OpenFile(pm.patternsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open patterns file: %w", err)
	}
	defer file.Close()

	// Find the latest learning that hasn't been saved yet
	scanner := bufio.NewScanner(file)
	lastSavedID := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID:") {
			lastSavedID = strings.TrimPrefix(line, "ID: ")
		}
	}

	// Append only new learnings
	for _, learning := range pm.learnings {
		if learning.ID > lastSavedID {
			entry := pm.formatLearningForFile(learning)
			if _, err := file.WriteString(entry); err != nil {
				return fmt.Errorf("failed to write learning: %w", err)
			}
		}
	}

	return nil
}

// formatLearningForFile formats a learning for the patterns.log file
func (pm *PatternManager) formatLearningForFile(learning Learning) string {
	return fmt.Sprintf(`ID: %s
Timestamp: %s
Category: %s
Importance: %d
Confidence: %.2f
Frequency: %d
Pattern: %s
Context: %s
Source: %s
Tags: %s

`,
		learning.ID,
		learning.Timestamp.Format(time.RFC3339),
		learning.Category,
		learning.Importance,
		learning.Confidence,
		learning.Frequency,
		learning.Pattern,
		learning.Context,
		learning.Source,
		strings.Join(learning.Tags, ", "))
}

// LoadLearningsFromFile loads persisted learnings from file
func (pm *PatternManager) LoadLearningsFromFile() error {
	file, err := os.Open(pm.patternsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's okay
		}
		return fmt.Errorf("failed to open patterns file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentLearning *Learning

	for scanner.Scan() {
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			if currentLearning != nil {
				pm.learnings = append(pm.learnings, *currentLearning)
				currentLearning = nil
			}
			continue
		}

		if strings.HasPrefix(line, "ID: ") {
			currentLearning = &Learning{}
			currentLearning.ID = strings.TrimPrefix(line, "ID: ")
		} else if currentLearning != nil {
			pm.parseLearningField(currentLearning, line)
		}
	}

	if currentLearning != nil {
		pm.learnings = append(pm.learnings, *currentLearning)
	}

	logger.Info("Loaded %d learnings from file", len(pm.learnings))
	return nil
}

// parseLearningField parses a single field from the learning file
func (pm *PatternManager) parseLearningField(learning *Learning, line string) {
	parts := strings.SplitN(line, ": ", 2)
	if len(parts) != 2 {
		return
	}

	key, value := parts[0], parts[1]

	switch key {
	case "Timestamp":
		if t, err := time.Parse(time.RFC3339, value); err == nil {
			learning.Timestamp = t
		}
	case "Category":
		learning.Category = value
	case "Importance":
		if i, err := strconv.Atoi(value); err == nil {
			learning.Importance = i
		}
	case "Confidence":
		if c, err := strconv.ParseFloat(value, 64); err == nil {
			learning.Confidence = c
		}
	case "Frequency":
		if f, err := strconv.Atoi(value); err == nil {
			learning.Frequency = f
		}
	case "Pattern":
		learning.Pattern = value
	case "Context":
		learning.Context = value
	case "Source":
		learning.Source = value
	case "Tags":
		if value != "" {
			learning.Tags = strings.Split(value, ", ")
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
