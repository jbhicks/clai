package ralph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MemoryWriter handles concurrent writes to .clai/ files with a single-writer queue
type MemoryWriter struct {
	baseDir   string
	patterns  chan string
	progress  chan ProgressEntry
	wg        sync.WaitGroup
	ctx       context.Context
}

// ProgressEntry represents a log entry to append to progress.txt
type ProgressEntry struct {
	StoryID   string
	Content   string
	GitHash   string
	Timestamp time.Time
}

// NewMemoryWriter creates a new MemoryWriter instance
func NewMemoryWriter(baseDir string) *MemoryWriter {
	mw := &MemoryWriter{
		baseDir:   baseDir,
		patterns:  make(chan string, 100),
		progress:  make(chan ProgressEntry, 100),
	}
	// Ensure patterns.md has initial content immediately
	if err := mw.ensurePatternsHeader(filepath.Join(baseDir, ".clai", "patterns.md")); err != nil {
		fmt.Printf("Warning: Failed to ensure patterns header: %v\n", err)
	}
	return mw
}

// Start begins the single-writer goroutine
func (mw *MemoryWriter) Start(ctx context.Context) {
	mw.wg.Add(1)
	go mw.writerLoop(ctx)
}

// writerLoop is the single goroutine that handles all file writes
func (mw *MemoryWriter) writerLoop(ctx context.Context) {
	defer mw.wg.Done()

	patternsPath := filepath.Join(mw.baseDir, ".clai", "patterns.md")
	progressPath := filepath.Join(mw.baseDir, ".clai", "progress.txt")

	// Ensure patterns.md has initial content (System Prompt Addendum header)
	if err := mw.ensurePatternsHeader(patternsPath); err != nil {
		fmt.Printf("Warning: Failed to ensure patterns header: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case pattern := <-mw.patterns:
			if err := mw.appendPattern(pattern, patternsPath); err != nil {
				fmt.Printf("Warning: Failed to write pattern: %v\n", err)
			}
		case entry := <-mw.progress:
			if err := mw.appendProgress(entry, progressPath); err != nil {
				fmt.Printf("Warning: Failed to write progress: %v\n", err)
			}
		}
	}
}

// ensurePatternsHeader writes the System Prompt Addendum header if file is empty
func (mw *MemoryWriter) ensurePatternsHeader(path string) error {
	// Check if file exists and has content
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create file with header
		return os.WriteFile(path, []byte(patternsHeader), 0644)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return os.WriteFile(path, []byte(patternsHeader), 0644)
	}

	return nil
}

// patternsHeader is the System Prompt Addendum header for patterns.md
const patternsHeader = `# 🦞 Patterns - System Prompt Addendum

This file contains reusable patterns discovered during development iterations.
Each pattern is a reusable solution to a common problem encountered in CLAI's autonomous development loop.

## Pattern Format

- **Pattern Name:** Brief descriptive title
- **Problem:** What issue was encountered
- **Solution:** How it was resolved
- **When to Use:** Context where this pattern applies
- **Implementation:** Code example or approach

## How to Use

When implementing new features, check this file for relevant patterns before writing new code.
Patterns are automatically discovered and added by the Ralph orchestrator.

---

`

// appendPattern adds a new pattern to patterns.md
func (mw *MemoryWriter) appendPattern(pattern string, path string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Add pattern with timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	content := fmt.Sprintf("## [%s]\n%s\n\n", timestamp, pattern)

	if _, err := f.WriteString(content); err != nil {
		return err
	}
	return nil
}

// AppendProgress sends a progress entry to the write queue (non-blocking)
func (mw *MemoryWriter) AppendProgress(entry ProgressEntry) error {
	select {
	case mw.progress <- entry:
		return nil
	default:
		return fmt.Errorf("progress queue full")
	}
}

// appendProgress writes a timestamped progress entry to progress.txt
func (mw *MemoryWriter) appendProgress(entry ProgressEntry, path string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	header := fmt.Sprintf("## [%s] | Story: %s\n", timestamp, entry.StoryID)
	body := fmt.Sprintf("Git Hash: %s\n\n%s\n---\n\n", entry.GitHash, entry.Content)

	if _, err := f.WriteString(header + body); err != nil {
		return err
	}
	return nil
}

// WritePattern sends a pattern to the write queue (non-blocking)
func (mw *MemoryWriter) WritePattern(pattern string) error {
	select {
	case mw.patterns <- pattern:
		return nil
	default:
		return fmt.Errorf("patterns queue full")
	}
}

// Shutdown waits for all pending writes to complete
func (mw *MemoryWriter) Shutdown() {
	mw.wg.Wait()
}

// TruncateContextIfTooLarge checks if log files exceed size limits and truncates if needed
func TruncateContextIfTooLarge(baseDir string, maxSizeMB int) error {
	patternsPath := filepath.Join(baseDir, ".clai", "patterns.md")
	progressPath := filepath.Join(baseDir, ".clai", "progress.txt")

	// Check patterns.md
	if err := checkAndTruncate(patternsPath, maxSizeMB); err != nil {
		return err
	}

	// Check progress.txt
	return checkAndTruncate(progressPath, maxSizeMB)
}

// checkAndTruncate checks file size and truncates if exceeded
func checkAndTruncate(path string, maxSizeMB int) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	fileSizeMB := float64(info.Size()) / (1024 * 1024)
	if fileSizeMB > float64(maxSizeMB) {
		// Read file content
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Keep last 80% of content (simple truncation)
		truncatePoint := int(float64(len(data)) * 0.8)
		truncatedData := data[truncatePoint:]

		// Write truncated content back
		return os.WriteFile(path, truncatedData, 0644)
	}

	return nil
}