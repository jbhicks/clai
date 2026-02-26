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

---

## [2026-02-25 23:35:00] | Story: CORE-003

**Pattern:** Controller Integration in Bubble Tea TUI

**Problem:** The RalphLoopController existed in internal/ralph/loop.go but was never connected to the UI model, so the features were inaccessible to users.

**Solution:** Integrated the controller directly into the UI Model struct and wired up keyboard handlers. The controller manages its own state (StageIdle, StageThinking, etc.) and the UI model delegates to it.

**When to Use:** When implementing background orchestrators or state machines that need to be controlled from the UI (Bubble Tea, TUI frameworks).

**Implementation:**
```go
// In UI Model struct
type Model struct {
    // ... other fields ...
    ralphController *ralph.RalphLoopController
}

// Initialize in Init()
func (m *Model) Init() tea.Cmd {
    m.ralphController = ralph.NewRalphLoopController(".")
    // ... other initializations ...
}

// Keyboard handler
case "r":
    if m.ralphController.IsIdle() {
        err := m.ralphController.Start()
        // Handle error...
    }
case "escape", "s":
    if !m.ralphController.IsIdle() {
        m.ralphController.Stop()
    }

// Use controller's styling functions in renderBriefingRoom()
func (m *Model) renderBriefingRoom() string {
    for _, story := range m.prd.UserStories {
        var style lipgloss.Style
        if m.ralphController.GetActiveStory().ID == story.ID {
            style = ralph.GetActiveStoryStyle()
        } else if story.Passes {
            style = ralph.GetPassedStoryStyle()
        } else {
            style = ralph.GetPendingStoryStyle()
        }
        // Render styled line...
    }
}
```

**Key Points:**
- Controller owns its state and lifecycle
- UI model delegates to controller for behavior
- Use controller's helper methods (GetActiveStory(), IsIdle(), etc.)
- Style functions from controller should be used for consistent theming
- Add keyboard handlers for Start/Stop/Continue operations

**Context:** CORE-003 The Ralph Main Loop & State Machine integration.
