# 🦞 Patterns - System Prompt Addendum

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

## [2026-02-10 00:12:00] | Story: CORE-002

**Pattern:** Channel-based Single-Writer Queue for File Operations

**Problem:** Multiple goroutines trying to write to patterns.md and progress.txt simultaneously caused race conditions and potential data corruption.

**Solution:** Created internal/ralph/memory_writer.go with a MemoryWriter struct that uses channels (patterns and progress) to queue write requests. A single goroutine (writerLoop) processes all writes sequentially, preventing race conditions.

**When to Use:** Any time multiple goroutines need to write to shared files. This pattern ensures thread-safe file operations without mutex locking overhead.

**Implementation:**
```go
type MemoryWriter struct {
    baseDir   string
    patterns  chan string
    progress  chan ProgressEntry
    wg        sync.WaitGroup
}

func (mw *MemoryWriter) Start(ctx context.Context) {
    mw.wg.Add(1)
    go mw.writerLoop(ctx)
}

func (mw *MemoryWriter) writerLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case pattern := <-mw.patterns:
            mw.appendPattern(pattern, patternsPath)
        case entry := <-mw.progress:
            mw.appendProgress(entry, progressPath)
        }
    }
}
```

**Context:** CORE-002 Multi-file Markdown Persistent Memory implementation.
