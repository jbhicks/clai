# AGENTS.md

---

# NOTE FOR AGENTS RUNNING IN PARALLEL THREADS

If you are a buddy agent (running in a parallel thread as a companion/monitor), you have a responsibility to:

- Dynamically update your context and react to other agents' actions and the current repo state, using:
    - LSP diagnostics, build/test status, formatting and lint analysis
    - Results from background explore/librarian agents
    - Best practices for Go project health and cleanup
- Provide real-time assistance during file changes or codebase reorganization
    - Monitor for: errors, warnings, deprecated patterns, formatting issues
    - Suggest/perform modernizations (interface{} → any, deprecated functions, string optimizations, etc)
- Whenever you detect an event or message relevant to the other agent's context, you should log that explicitly:
    - Use: `bash -c 'echo "[AGENT BUDDY MESSAGE] Context updated: ..."'`
    - Pipe relevant monitoring/data to these echo logs when significant state/context events occur
    - This is for human/agent transparency and debugging

(See Ralph/MCP: ALL CONTEXT MODIFICATIONS AND NOTIFICATIONS MUST BE LOGGED THIS WAY)

---


This repository is a Go CLI project for local AI agent interaction. Follow these guidelines for agentic coding:

## Build, Lint, and Test Commands
- Build: `make build` or `go build -o clai ./cmd/clai`
- Run: `make run` or `go run ./cmd/clai`
- Test all: `make test` or `go test ./...`
- Run a single test: `go test -run TestFunctionName ./...`
- Clean: `make clean`
- Install: `make install`
- Dev/debug: `make dev-tmux` (runs with automatic reload in tmux session)
- Benchmark: `make benchmark TEST=N` (runs specific benchmark test N)

## Server Management
**🚨 ABSOLUTELY FORBIDDEN: Agents MUST NEVER start, stop, or restart dev/benchmark servers**
- **VIOLATION**: Agents are PROHIBITED from running `make dev`, `make dev-benchmark`, `./clai benchmark`, or ANY server-starting commands
- **REASON**: These commands block execution indefinitely and completely break the development workflow
- **CONSEQUENCE**: Running these commands will freeze the entire agent session and require manual termination
- The user manages ALL server processes (dev server, benchmark server, etc.) - this is their responsibility only
- Only build code changes and let the user handle server management
- If you need to test functionality, use API calls to running servers, not starting new ones

**EXCEPTION: Benchmark testing is allowed for development verification**
- Agents MAY run `make benchmark TEST=N` or `./clai benchmark --cli --test N` commands for testing specific benchmark tests
- This is permitted because these commands run and return results without blocking indefinitely
- Use this only when explicitly requested by the user for testing changes
- Still prohibited: Running full benchmark suites or starting benchmark servers

**CRITICAL SAFEGUARD: Before any server-related operations**
- ALWAYS check if dev/benchmark servers are already running: `ps aux | grep -E "(make dev|clai.*benchmark)" | grep -v grep`
- If any servers are found running, IMMEDIATELY return an error: "ERROR: Dev/benchmark servers are already running. Please let the user manage server processes - do not run 'make dev' yourself."
- Do NOT proceed with any server operations if instances are detected
- **NEVER** bypass this check or attempt to "help" by starting servers

### Development Environment Cleanup
To prevent zombie processes from old development sessions:
- Run `make dev-clean` to clean up orphaned processes before starting new sessions
- The `dev.sh` script automatically cleans up old sessions on startup
- Check for leftover processes: `ps aux | grep -E "(inotifywait.*clai|dev\.sh)"`

## HTTP API Interactions
**CRITICAL: NEVER attempt to curl/fetch Server-Sent Events (SSE) endpoints in the foreground**
- SSE endpoints (like `/api/servers/events`, `/api/model-benchmark/live`) are designed for real-time streaming
- Attempting to curl SSE endpoints in the foreground will BLOCK THE ENTIRE THREAD INDEFINITELY
- This completely breaks the development workflow and requires manual termination
- For SSE endpoints, use browser inspection, WebSocket clients, or server-side testing only
- Regular HTTP endpoints are fine to test with curl
- If you must test SSE endpoints programmatically, run curl in the background with output redirection: `curl -s <sse_url> > /tmp/output.txt &`

## Model Server Configuration

**CRITICAL: Use the same LLM server configuration as the main clai application**
- The project uses a llama.cpp server (not Ollama) for LLM operations
- CLI benchmarks (`clai benchmark --cli`) use the same configuration as clai
- **Unified Benchmark Suite**: 21 tests total (12 core + 4 advanced + 5 ultra-challenging benchmarks)
- Environment variables (same as clai):
  - `OLLAMA_HOST`: Server URL (default: `http://localhost:8081`)
  - `OLLAMA_MODEL`: Model name (default: `llama3.1-gpu:latest`)
- If `OLLAMA_MODEL` is not explicitly set, CLI benchmarks automatically detect the actual model running on the server
- **DO NOT attempt to install or run Ollama** - use the existing llama.cpp server
- The server supports OpenAI-compatible API format for chat completions

## OpenCode/MCP Socket Connections

**CRITICAL: Handle socket disconnections gracefully with large models**
- Large models (especially pickle/serialized models) can cause socket timeouts during processing
- Error: "The socket connection was closed unexpectedly" indicates MCP connection dropped
- **ALWAYS** handle potential socket errors in fetch operations:

```go
// Use retry logic for socket operations
func fetchWithRetry(url string, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        resp, err := http.Get(url)
        if err != nil {
            if i < maxRetries-1 {
                time.Sleep(time.Duration(i+1) * time.Second)
                continue
            }
            return fmt.Errorf("failed after %d retries: %w", maxRetries, err)
        }
        resp.Body.Close()
        return nil
    }
}
```

**Prevention Strategies:**
- Add timeout handling: `http.Client{Timeout: 30 * time.Second}`
- Use `verbose: true` for debugging socket issues: `fetch(url, {verbose: true})`
  - When MCP/OpenCode tools fail with socket errors, retry with `verbose: true` to get detailed connection information
  - This helps diagnose whether it's a timeout, network issue, or model processing delay
- Monitor model loading progress with heartbeats
- For very large models, consider streaming responses or chunked transfers
- Set appropriate context timeouts: `ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)`

**Debug Socket Issues:**
- Monitor `/tmp/clai.sock` for connection issues
- Check system limits: `ulimit -n` for file descriptors
- Look for memory pressure causing connection drops

## Debugging TUI Issues

**MANDATORY: Always use clai-debug tools when working on TUI/rendering issues**

When debugging Bubble Tea TUI rendering problems, layout issues, or terminal dimension problems:

1. **USE THE TOOLS, NOT THE USER** - Never ask the user to run `clai-debug` commands for you
   - Run `clai-debug_inspect` yourself to see current UI state
   - Run `clai-debug_inspect_styles` yourself to see structured viewport data
   - Run `clai-debug_get_history` yourself to examine conversation state

2. **INSPECT BEFORE REPLYING** - When the user reports TUI issues:
   - Check the current state with debug tools FIRST
   - Analyze the actual dimensions and rendering state
   - Report findings with actual numbers, not guesses

3. **COMMON DEBUG COMMANDS** (use these tools directly):
   - `clai-debug_inspect` - Get full UI inspection including viewport content, dimensions, and state
   - `clai-debug_inspect_styles` - Get structured viewport dimensions and state info (JSON format)
   - `clai-debug_get_history` - Get the conversation history/messages
   - `clai-debug_ping` - Test connectivity to the CLAI debug server
   - `clai-debug_send_key KEY` - Send a keystroke to the TUI (e.g., "enter", "ctrl+h", "up", "down")
   - `clai-debug_type_text TEXT` - Type text into the input field (e.g., "hello world")

4. **WHAT TO LOOK FOR**:
   - Terminal dimensions (should NOT be 0x0)
   - Chat pane width/height
   - Viewport content length vs rendered height
   - Active pane state
   - Scroll position (y_offset)

**Example workflow:**
```
❌ WRONG: "Can you run clai-debug_inspect and tell me what it says?"
✅ RIGHT: *runs clai-debug_inspect* "I see the terminal is reporting 0x0 dimensions..."
```

## Code Style Guidelines
- Use standard Go libraries; minimize dependencies.
- Group imports: stdlib first, then external.
- Format code with `gofmt`.
- Use clear, descriptive names for types, functions, and variables.
- Define tool schemas as Go structs for JSON marshalling.
- Handle errors gracefully (invalid JSON, tool failures, timeouts).
- Use Go doc comments for exported functions/types.
- Prefer explicit types and avoid unnecessary complexity.

### Code Comment Accuracy

**CRITICAL**: Code comments must accurately reflect implementation. Comments that state functions are called or behaviors exist, but aren't actually implemented, lead to bugs and confusion.

**Problem**: Comments suggesting a function is called in a certain place when it's not (e.g., "Viewport.Height is set by chat.updateViewportHeight() in chat.Update()") can hide bugs for months.

**Prevention**:
- When writing comments about function calls or behaviors, immediately verify they exist in the code
- Use grep searches to confirm function calls: `grep -r "functionName" .` 
- For Bubble Tea Update() methods, check that all referenced functions are actually invoked
- Review comments during code review - if a comment says "X is called here" but X isn't called, it's a bug
- Prefer self-documenting code over comments when possible

**Example**:
```go
// ✅ GOOD: Comment matches code
// Note: Viewport.Height is set by chat.updateViewportHeight() in chat.Update()
c.updateViewportHeight()  // Actually called

// ❌ BAD: Comment doesn't match code  
// Note: Viewport.Height is set by chat.updateViewportHeight() in chat.Update()
// ...but updateViewportHeight() is never called!
```

## UI Corruption Prevention

**CRITICAL**: To prevent the UI corruption issues experienced in recent commits (orphaned code blocks, incomplete expressions, duplicate calculations):

- **Always run `go build ./cmd/clai` after changes to UI/layout code** - Verify compilation succeeds before committing. LSP diagnostics may show false positives.
- **Remove orphaned code blocks immediately** - Code outside function bodies (e.g., after Init() but before Update()) can cause compilation errors and runtime corruption.
- **Complete all expressions** - Missing parentheses, incomplete strings, or syntax errors in layout calculations (especially in handleWindowSizeMsg and View methods) will break rendering.
- **Avoid duplicate calculations** - Ensure handleWindowSizeMsg and View() methods have consistent, non-duplicate dimension logic. Use constants for ratios (e.g., 0.6 for chat pane width).
- **Test UI changes** - Use `clai debug inspect` to verify pane dimensions, viewport sizes, and overall layout after changes. Check for 0x0 dimensions or misaligned panes.
- **Verify Bubble Tea calculations** - Frame sizes (GetHorizontalFrameSize/GetVerticalFrameSize) must be subtracted correctly from outer dimensions to get inner component sizes.

## 🚨 CRITICAL: Lipgloss Border Background Transparency

**ABSOLUTELY FORBIDDEN**: When using lipgloss borders, **ALWAYS** set `BorderBackground()` in addition to `BorderForeground()`.

**WHY THIS MATTERS**: Border characters (┌┐└┘─│) do NOT automatically inherit background colors. Without `BorderBackground()`, borders appear with transparent backgrounds, creating ugly black gaps/bars in your UI.

**CORRECT - Always include both:**
```go
style := lipgloss.NewStyle().
    Background(lipgloss.Color("#282a36")).           // Content background
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("#ffb86c")).     // Border color
    BorderBackground(lipgloss.Color("#282a36"))      // ← CRITICAL: Border background
```

**INCORRECT - Causes black bars:**
```go
style := lipgloss.NewStyle().
    Background(lipgloss.Color("#282a36")).
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("#ffb86c"))
    // Missing BorderBackground() - causes transparent gaps!
```

**VERIFICATION**: Use `clai debug inspect` to check for ANSI reset codes (`[0m`) in border areas. If you see resets without background codes following them, you have transparency gaps.

**CONSEQUENCE**: UI corruption with black bars/gaps that ruin the visual experience. This wasted hours of debugging time in recent commits.

## UI Component Guidelines
- Always use Charm Bubble components for all UI pieces.
- Do not write custom UI components unless absolutely necessary (e.g., when no Bubble component exists for your use case).
- If a custom component is required, document the reason in code comments and prefer extending Bubble components when possible.

### Bubble Tea Layout & Sizing Best Practices

**Critical Rules for Proper Layout:**

#### Guard width-derived arithmetic

- Problem: Tests and very narrow terminals can set viewport widths (e.g., v.Width) smaller than the constants used in layout code (for example `v.Width - 4`). If these values are used without guarding they can produce negative values which cause runtime panics (negative counts for `strings.Repeat`, out-of-range slice indices) or corrupt rendering.

- Rule: Always clamp any width-derived value to a non-negative integer before using it in `strings.Repeat`, slice bounds, or as arguments to lipgloss `Width()`/`Height()`.

- Example (safe divider repeat):

```go
padCount := v.Width - 4
if padCount < 0 {
    padCount = 0
}
divider := strings.Repeat("━", padCount)
```

- Example (safe truncation with ellipsis):

```go
maxLen := v.Width - 20
if maxLen < 0 {
    maxLen = 0
}
// Prefer rune-aware truncation (see below) when dealing with user-visible text
if len(s) > maxLen {
    if maxLen > 3 {
        s = s[:maxLen-3] + "..."
    } else {
        s = s[:maxLen]
    }
}
```

#### Rune-aware truncation (Unicode-safe)

- Problem: Slicing strings by byte index (using `len` and `[:n]`) can break multi-byte UTF-8 characters and produce invalid output.

- Rule: When truncating user-visible text, convert the string to `[]rune` first and slice by rune count.

```go
r := []rune(s)
if len(r) > maxLen {
    if maxLen > 3 {
        s = string(r[:maxLen-3]) + "..."
    } else {
        s = string(r[:maxLen])
    }
}
```

#### Narrow-width unit tests

- Add unit tests that exercise narrow terminal widths (0..12) for every view that uses arithmetic from `v.Width`. These tests catch panics and off-by-one errors quickly.

Example test skeleton:

```go
func TestAgentStatusViewNarrowWidths(t *testing.T) {
    theme := NewTestTheme()
    for w := 0; w <= 12; w++ {
        v := NewAgentStatusView(theme)
        v.Width = w
        v.Height = 10
        v.Status = AgentStatus{Active: true, Thought: "short"}
        _ = v.View() // must not panic
    }
}
```

#### Defensive helpers

- Encourage creating and reusing helpers such as `safeWidth(vWidth, sub int) int` and `truncateForWidth(s string, w int) string` to centralize clamping/truncation behavior and keep code consistent.

#### Testing checklist addition (UI changes)

- After UI changes, include narrow-width test cases as part of the CI/dev checklist:
  - Run clai-debug_inspect baseline -> implement -> wait for dev auto-reload -> clai-debug_inspect again
  - Run unit tests including narrow-width tests: `go test ./internal/ui -run Narrow -v`
  - Run integration tests that exercise Bubble Tea rendering (existing suites)


1. **Height Calculation**: Use `lipgloss.Height()` for actual rendered heights, not `style.GetHeight()` (returns 0 for padded styles). Formula: `contentHeight = terminalHeight - statusBarRows - errorBannerRows`.

2. **Width Calculation**: `innerWidth = outerWidth - style.GetHorizontalFrameSize()`. Frame includes borders + padding (e.g., 6 for padding(0,2) + rounded border).

3. **Style Application**: Calculate inner dimensions first, set on components, render with unsized style. Pad to exact outer width if needed:
   ```go
   innerWidth := outerWidth - style.GetHorizontalFrameSize()
   innerHeight := outerHeight - style.GetVerticalFrameSize()
   component.Width, component.Height = innerWidth, innerHeight
   view := style.Render(component.View())
   if lipgloss.Width(view) < outerWidth {
       view = lipgloss.NewStyle().Width(outerWidth).Render(view)
   }
   ```

4. **Layout Joining**: Sum heights/widths must equal terminal dimensions. Split proportionally: `chatWidth = int(float64(terminalWidth) * 0.8)`.

5. **Debugging**: Use `clai-debug_inspect` tool. Log: `lipgloss.Width/Height(view)`. Off-by-one errors indicate frame miscalculation. **NEVER** debug in `View()` methods.

6. **List Components**: Split multi-line content: `strings.Split(text, "\n")`. Set zero padding on list styles.

7. **Background Colors**: Set explicit `.Background()` on wrappers. Pad lines to full width with matching background to avoid artifacts.

8. **Nested Borders**: Apply borders AFTER joining, not before. Account for single border in height calculations.

9. **Background Transparency Prevention**:
   - **Problem**: Border characters (especially rounded borders like `╭╮╰╯`) don't automatically fill their rendered area with background color, causing transparent gaps that show through to content behind
   - **Solution**: Apply background at multiple layers:
     ```go
     // Inner layer: Component-level styling
     chatInput.TextStyle = lipgloss.NewStyle().Background(bgColor)
     chatInput.Cursor.Style = lipgloss.NewStyle().Background(bgColor)
     chatInput.PlaceholderStyle = lipgloss.NewStyle().Background(bgColor)
     chatInput.PromptStyle = lipgloss.NewStyle().Background(bgColor)

     // Outer layer: Lipgloss wrapper style
     InputFocused: lipgloss.NewStyle().
         Background(lipgloss.Color(bgColor)).
         Border(lipgloss.NormalBorder()).  // Use solid over rounded
         Padding(0, 1)  // Add padding to fill gaps
     ```
   - **Always wrap with background**: Ensure background wrapper is applied regardless of content width to guarantee full coverage
   - **Prefer solid borders**: Normal borders are more reliable than rounded corners for background coverage

10. **Chat Bubble Background Transparency**:
   - **Problem**: When message bubbles have borders and backgrounds, but padding is added with plain spaces instead of background-colored text, transparent gaps appear between border and content background
   - **Root Cause**: `padLinesToWidth()` was adding space-only padding: `line + strings.Repeat(" ", paddingNeeded)`. These spaces are transparent and show through to whatever's behind the bubble (like the chat pane)
   - **Solution**: Use `BackgroundWrapper` style with background color for padding instead of plain spaces:
     ```go
     // WRONG - transparent spaces
     paddedLine := line + strings.Repeat(" ", paddingNeeded)

     // CORRECT - background-colored padding
     paddedLine := themeStyles.BackgroundWrapper.Width(width).Render(line)
     ```
   - **Applies to**: User messages, Assistant messages, Tool messages
   - **Pattern**: Always use existing wrapper styles (BackgroundWrapper) for padding to ensure consistency
   - **Result**: No visible transparent gaps, borders frame content properly with matching background

**Common Pitfalls:**
- Using `style.GetHeight()` for padded styles (returns 0)
- Setting `.Width()` on styles with borders/padding (double-counts)
- Forgetting frame size subtraction
- Adding multi-line content as single list items
- Creating wrappers without explicit backgrounds
- Applying borders before joining
- Debug logging in `View()` methods

### Bubble Tea External Message Injection Patterns

**CRITICAL**: When injecting messages from external goroutines or async operations into Bubble Tea programs, follow these patterns from the official examples:

1. **Use `p.Send(msg)` for External Messages**:
   ```go
   // ✅ CORRECT - From send-msg example
   go func() {
       // ... async work ...
       p.Send(resultMsg{data: "result"})
   }()
   ```

2. **Commands Handle Async I/O**:
   ```go
   // ✅ CORRECT - Commands perform I/O and return messages
   func checkServer() tea.Msg {
       res, err := http.Get(url)
       if err != nil {
           return errMsg{err}
       }
       return statusMsg(res.StatusCode)
   }
   ```

3. **Channel-based Communication**:
   ```go
   // ✅ CORRECT - From realtime example
   type model struct {
       sub chan struct{}  // channel for external events
   }

   func waitForActivity(sub chan struct{}) tea.Cmd {
       return func() tea.Msg {
           return responseMsg(<-sub)
       }
   }
   ```

4. **Avoid Direct UI State Manipulation**:
   ```go
   // ❌ WRONG - Direct manipulation from goroutines
   go func() {
       m.Chat.Streaming = true  // Never do this!
   }()

   // ✅ CORRECT - Send message to Update loop
   go func() {
       p.Send(startStreamingMsg{})
   }()
   ```

**Why This Matters**:
- All UI updates must happen in the `Update()` method in response to messages
- External goroutines should never directly modify model state
- Use `p.Send()` to inject messages into the Bubble Tea event loop
- Commands provide the bridge between async I/O and the UI event loop

**Streaming Implementation Pattern**:
When implementing streaming responses (like AI text generation), use:
```go
// Agent callback sends chunks via p.Send()
callback := func(chunk string) {
    p.Send(streamChunkMsg{chunk: chunk})
}

// UI Update method handles streaming
case streamChunkMsg:
    m.Chat.Messages[lastIdx].Content += msg.chunk
    m.Chat.ContentDirty = true
```

## UI Component Research Guidelines

### Always Use Bubbles Library Components First

This project uses [Bubble Tea](https://github.com/charmbracelet/bubbletea) for TUI development. The [bubbles](https://github.com/charmbracelet/bubbles) library provides pre-built, well-tested components.

**Before implementing ANY UI component:**

1. **Check if a bubbles component exists**:
   - `textinput.Model` - for text input fields
   - `textarea.Model` - for multi-line text input
   - `list.Model` - for selectable lists
   - `spinner.Model` - for loading indicators
   - `table.Model` - for tabular data
   - `viewport.Model` - for scrollable content
   - `progress.Model` - for progress bars
   - `key.Binding` - for keyboard shortcuts
   - `help.Model` - for help screens

2. **Research proper usage**:
   - Consult the official documentation at `reference/bubbletea` and `reference/lipgloss`
   - Check the examples in `reference/bubbletea/examples/`
   - Review `reference/bubbletea/tutorials/` for guided learning
   - Look at `reference/lipgloss/examples/` for styling patterns

3. **When adding UI features**:
   - Use the `call_omo_agent` tool with `librarian` subagent to fetch documentation from official sources
   - Search for existing implementations in the codebase first
   - Review test patterns in `reference/bubbletea/` and `reference/lipgloss/`

**Reference Repositories (as submodules)**:
- `reference/bubbletea/` - Main Bubble Tea framework
- `reference/lipgloss/` - Styling library for terminal UI

**Example Workflow for New UI Feature:**

```go
// Before implementing an autocomplete menu:
// 1. Check bubbles for existing components: list, textinput
// 2. Review reference/bubbletea/examples/ for similar patterns
// 3. Check reference/lipgloss/ for styling autocomplete dropdowns
// 4. Search codebase for "autocomplete" or "suggestion" patterns
// 5. Use librarian agent to fetch official documentation

// Correct approach: Extend existing bubbles components
type autocompleteModel struct {
    textinput.Model
    suggestions []string
    selected    int
}

// NOT: Building a custom rendering engine from scratch
```

**Never build from scratch when bubbles has a solution:**
- Bubbles components handle keyboard navigation, focus, accessibility
- They integrate properly with Bubble Tea's Update/View loop
- They work seamlessly with lipgloss styling

## Testing

### Test-Driven Development (TDD) - REQUIRED for New Features

**CRITICAL**: Always write tests BEFORE implementing new features, especially API endpoints and handlers.

**TDD Workflow**:
1. **Write the test first** - Define what the feature should do
2. **Run the test** - It should fail (red)
3. **Implement the feature** - Make the test pass (green)
4. **Refactor** - Clean up code while keeping tests passing
5. **Verify** - Run all tests before declaring done

**Example: Adding a new API endpoint**

```go
// Step 1: Write the test FIRST
func TestHandleNewEndpoint(t *testing.T) {
    server := &Server{modelManager: NewModelManager()}
    
    req := httptest.NewRequest("GET", "/api/new-endpoint?param=value", nil)
    w := httptest.NewRecorder()
    
    server.handleNewEndpoint(w, req)
    
    if w.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", w.Code)
    }
    
    // Assert expected behavior
    body := w.Body.String()
    if !strings.Contains(body, "expected content") {
        t.Errorf("Response missing expected content")
    }
}

// Step 2: Run the test - it will fail (handleNewEndpoint doesn't exist)

// Step 3: Implement the handler
func (s *Server) handleNewEndpoint(w http.ResponseWriter, r *http.Request) {
    // Implementation here
}

// Step 4: Run the test again - it should pass
```

**When TDD is REQUIRED**:
- ✅ New API endpoints (HTTP handlers)
- ✅ New functions with complex logic
- ✅ Bug fixes (write a failing test that reproduces the bug, then fix it)
- ✅ Database operations
- ✅ External API integrations

**When TDD is optional** (but still recommended):
- Simple getters/setters
- UI templates (use manual testing)
- One-liner utility functions

**Benefits**:
- Catches bugs before they reach users
- Defines expected behavior upfront
- Prevents regressions
- Documents how code should be used

### General Testing Guidelines

- Use Go's `testing` package for unit/integration tests.
- Mock external dependencies (e.g., Ollama, HuggingFace API) in tests.
- For Bubble Tea projects, follow the [Bubble Tea Agent Testing Strategy](BUBBLETEA_TESTING_STRATEGY.md) for all test creation. This document provides detailed guidelines and examples for unit, integration, and UI testing specific to Bubble Tea applications.
- For UI/feature testing of the running application, use tmux as documented in "Automated Testing with tmux" below.

No Cursor or Copilot rules detected.

## Development Workflow
- **ASSUME** the user is running `make dev-tmux` in a tmux session with automatic file watching and reload enabled.
- **🚨 ABSOLUTELY FORBIDDEN**: Agents MUST NEVER run the application themselves (e.g., `./clai` or `make run`).
- **🚨 ABSOLUTELY FORBIDDEN**: Agents MUST NEVER run `make dev` or `make dev-tmux` themselves - they require interactive terminals and block execution indefinitely.
- After making code changes, **ASSUME** the automatic reload has already occurred.
- To verify your changes:
   1. Check `tui.log` for runtime logs: `tail -f tui.log` or `cat tui.log`
  2. Look for errors, warnings, or debug output related to your changes
  3. **MANDATORY**: Use clai-debug MCP tools to verify UI/functionality changes
- The dev watcher uses `entr` to automatically rebuild and restart on `.go` file changes.
- **Exception:** You may check if processes are running (`ps aux | grep`), check listening ports (`ss -tlnp`), or read log files to diagnose issues, but do not start/stop/kill processes.

## Automated Testing with tmux

Agents can spawn tmux windows for quick automated testing of the running application:

```bash
# Spawn a detached tmux session running clai
tmux new-session -d -s clai-test 'go run ./cmd/clai'

# Wait for app to start
sleep 3

# Capture the current pane content to verify UI
tmux capture-pane -t clai-test -p

# Clean up when done
tmux kill-session -t clai-test
```

**Workflow for automated testing:**
1. Spawn tmux session with `new-session -d` (detached)
2. Wait for app to start: `sleep 3`
3. Capture pane output to verify rendering
4. Kill session when done

**Example - Testing UI rendering:**
```bash
tmux new-session -d -s clai-test 'go run ./cmd/clai'
sleep 3
tmux capture-pane -t clai-test -p
tmux kill-session -t clai-test
```

**Note:** Sending keystrokes to tmux sessions works best interactively (via `attach-session`). Detached sessions may not receive input reliably.

## Debugging UI Issues
- **ALWAYS** use the debug server to inspect UI rendering when working on layout or display issues
- The app runs a Unix socket server at `/tmp/clai.sock` that provides real-time UI inspection
- Use `clai debug inspect` to see:
  - Actual rendered viewport content (with ANSI codes preserved)
  - Terminal and pane dimensions
  - Viewport state (scroll position, total lines)
  - Message count and active pane
- This is the ONLY reliable way to see what's actually rendering in the terminal
- See [docs/DEBUG_SERVER.md](docs/DEBUG_SERVER.md) for full protocol documentation
- **Workflow for UI fixes**:
  1. Run `clai debug inspect` to capture current state
  2. Make code changes
  3. Wait for `make dev` auto-reload
  4. Run `clai debug inspect` again to verify fix
  5. Compare before/after output
- Other debug commands: `clai debug ping`, `clai debug switch_pane`, `clai debug get_history`

## ⚠️ **MANDATORY TESTING REQUIREMENT - NO EXCEPTIONS** ⚠️

**🚨 CRITICAL: Agents MUST use clai-debug MCP tools to verify ALL changes that affect user interaction, UI, or functionality**

**FAILURE TO TEST THROUGH CLAI-DEBUG TOOLS = AUTOMATIC FAILURE - NO CODE MAY BE COMMITTED WITHOUT VERIFICATION**

This requirement applies to ANY changes that could impact how users see or interact with the application, including (but not limited to):
- UI layout, rendering, or styling changes
- New features or user interactions
- Message handling or conversation flow
- Tool execution and results display
- Status indicators, loading states, or error messages
- Keyboard shortcuts, navigation, or input handling
- Any modifications to `internal/ui/` files or Bubble Tea models

**❌ FORBIDDEN**: Agents may NOT ask users to test changes or report UI issues. Agents MUST verify functionality themselves.

### Development Environment Assumption

**ASSUME DEV MODE IS ACTIVE**: All testing assumes `make dev` or equivalent hot-reload development environment is running. Changes to `.go` files trigger automatic rebuild and restart with proper build identifiers. After making changes, wait for the auto-restart before testing with clai-debug tools.

### Required Testing Workflow

**BEFORE making ANY UI/affecting changes:**
1. Use `clai-debug_inspect` to capture baseline application state
2. Use `clai-debug_inspect_styles` to document current layout and dimensions
3. Use `clai-debug_get_history` to verify conversation state if relevant

**AFTER making changes:**
1. **ASSUME DEV MODE IS RUNNING** - Changes to `.go` files trigger automatic rebuild and restart
2. **VERIFY AUTO-RESTART**: Check that CLAI process restarted (new PID) and debug socket reconnected
3. **WAIT FOR STABILIZATION**: Allow 2-3 seconds for UI to fully render after restart
4. Use `clai-debug_inspect` to verify the change is visible and correct
5. Use `clai-debug_inspect_styles` to confirm layout/dimensions are proper
6. Use `clai-debug_send_key` to test user interactions (e.g., keyboard navigation, inputs)
7. Use `clai-debug_get_history` to verify conversation flow if affected

**DEV MODE VERIFICATION:**
- Check `ps aux | grep "make run"` shows running process with recent start time
- Confirm `/tmp/clai.sock` exists and is accessible
- Use `clai-debug_ping` to verify debug server connectivity
- **Check Build ID in status bar** changes after code edits (proves auto-restart)
- Use `clai-debug_inspect` to see current Build ID (shows as 🔄 Build ID: ...)
- **Build ID format**: `YYYYMMDD-HHMMSS-COMMIT-bCOUNT-RAND` (guaranteed unique per build)
- If auto-restart fails, manually restart with `make dev` or `./clai`

**For tool/functionality testing:**
1. Use `clai-debug_send_key` to simulate user input that triggers the functionality
2. Use `clai-debug_inspect` to verify tool execution status and results appear correctly
3. Use `clai-debug_get_history` to confirm tool results are added to conversation

### **WHEN TOOLS ARE INSUFFICIENT** 🚨

**If clai-debug tools cannot properly test a change:**
1. **IMMEDIATELY STOP** and document the limitation
2. **INVESTIGATE** what additional debugging capabilities are needed
3. **ADD NEW TOOLS** to the clai-debug MCP server to fill the gap
4. **TEST AGAIN** with the new tools before proceeding

**Examples of when new tools are needed:**
- Visual rendering issues not captured by text inspection
- Animation/timing problems
- Color/styling verification beyond dimensions
- Interactive behavior testing
- Performance profiling

**Process for adding new tools:**
1. Identify the specific debugging need
2. Implement new MCP tool in `scripts/mcp/clai-mcp-server.js`
3. Add corresponding handler in CLAI's debug server
4. Document the new tool in this section
5. Test the implementation thoroughly

### Documentation Requirements

After testing, agents MUST document:
- What was tested using which clai-debug commands
- Expected vs. actual results
- Any issues found and how they were resolved
- Screenshots or detailed descriptions of UI state

**Example Workflow:**
```
Agent: Making changes to message rendering in internal/ui/model.go
- BEFORE: clai-debug_inspect shows current chat pane layout
- Change: Modify View() method for better message formatting
- AFTER: clai-debug_inspect confirms messages render correctly
- Test: clai-debug_send_key "test message<enter>" verifies input works
- Result: Messages display properly, no layout issues
```

### Common clai-debug Commands (MUST BE USED BY AGENTS)

**❌ WRONG**: "Can you test this and tell me if the UI looks right?"
**✅ RIGHT**: Agent runs clai-debug_inspect themselves and reports findings

Available tools (accessed via MCP protocol to clai-debug server):
- `inspect` - Get full UI inspection including viewport content, dimensions, and state
- `inspect_styles` - Get structured viewport dimensions and state info (JSON format)
- `get_history` - Get the conversation history/messages
- `ping` - Test connectivity to the CLAI debug server
- `send_key` - Send a keystroke to the TUI (e.g., "enter", "ctrl+h", "up", "down")
- `type_text` - Type text into the input field
- `send_message` - Inject a message into the conversation
- `switch_pane` - Switch between chat and log panes

**Important Distinction:**
- **skill_mcp tool**: For MCP servers embedded within Oh My OpenCode skills
- **clai-debug tools**: Standalone MCP server accessed via direct MCP protocol calls

The clai-debug server is configured in `.mcp.json` but accessed directly via MCP protocol, not through the skill_mcp tool.

**Workflow Enforcement:**
- Agents MUST include clai-debug testing results in their responses
- If clai-debug tools are unavailable or fail, agents MUST report this and cannot proceed with changes
- All UI changes are considered incomplete until verified with clai-debug tools
- This ensures agents are fully responsible for change validation, not users

**Implementation Notes:**
- Server runs as stdio transport MCP server located at `scripts/mcp/clai-mcp-server.js`
- Communicates with CLAI via Unix socket at `/tmp/clai.sock`
- Tools are invoked using MCP JSON-RPC protocol (not skill_mcp tool)
- Example: `{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "inspect", "arguments": {}}}`

### Why This Is Mandatory

Without agent-driven testing:
- Users become the primary testers, slowing development
- UI issues go undetected until user reports
- Agents cannot iterate quickly on UI problems
- Quality suffers from lack of immediate feedback

With mandatory clai-debug testing:
- Agents verify changes immediately and accurately via hot-reload dev environment
- Issues are caught and fixed in real-time during development
- Development velocity increases through rapid iteration with auto-restart
- Users only see working, tested features
- No manual testing cycles required - dev environment handles everything

**VIOLATION**: Making UI changes without clai-debug verification = incomplete work that must be reverted.

**🚨 ZERO TOLERANCE POLICY: ANY UI/AFFECTING CHANGE WITHOUT CLAI-DEBUG VERIFICATION WILL BE REJECTED AND MUST BE REVERTED IMMEDIATELY**

## Running Blocking Scripts
- Do **not** run blocking scripts (such as `dev_run.sh` or `dev_watch.sh`) directly in the foreground, as this will block the thread and prevent further interaction.
- If you need to run a blocking script, run it in the background (e.g., with `&` or in a separate terminal) and monitor log output separately (for example, using `tail -f debug.log`).
- Recommended workflow: Start the script in the background, then use a separate process or terminal to watch log output and interact with the application as needed.

## HTMX and Idiomorph (Web UI)

### Preventing Flickering with Morph Swaps

When using HTMX with the idiomorph extension for smooth DOM updates, follow this pattern to avoid flickering:

**Critical Rule**: Server responses MUST match the swap strategy.

#### ✅ Correct Pattern (morph:outerHTML)

**Template:**
```html
<div 
    id="servers_list" 
    hx-get="/api/servers/list" 
    hx-trigger="load, every 3s"
    hx-swap="morph:outerHTML"
    hx-ext="morph"
>
    <p>Loading...</p>
</div>
```

**Server Handler:**
```go
func HandleServersList(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    
    // Return complete element with SAME ID as template
    html := `<div id="servers_list">
        <table>...</table>
    </div>`
    
    fmt.Fprint(w, html)
}
```

**Why it works:**
- `morph:outerHTML` replaces the entire element
- Server returns the complete element with the same ID
- Idiomorph can match elements by ID and morph smoothly
- Only changed content updates, structure is preserved

#### ❌ Wrong Pattern (causes flickering)

**Template:**
```html
<div 
    id="wrapper"
    hx-get="/api/content"
    hx-trigger="every 3s"
    hx-swap="morph:innerHTML"
    hx-target="#content"
    hx-ext="morph"
>
    <div id="content">Loading...</div>
</div>
```

**Server Handler:**
```go
// Returns full element when innerHTML is expected
html := `<div id="content">...</div>`  // ❌ WRONG
```

**Problem:**
- Template uses `morph:innerHTML` targeting `#content`
- Server returns `<div id="content">` (full element, not inner content)
- Mismatch: HTMX tries to morph *children* but receives a replacement element
- Idiomorph cannot match properly, causes full DOM replacement = flicker

#### Recommendations

1. **Prefer `morph:outerHTML`** - Simpler pattern, server returns complete elements
2. **Always wrap responses with the target ID** - Enables idiomorph to match and morph
3. **Use stable IDs on dynamic content** - Helps idiomorph track elements:
   ```go
   for _, item := range items {
       html += fmt.Sprintf(`<div id="item_%d">...</div>`, item.ID)
   }
   ```
4. **Use SSE for real-time updates** - For streaming data, use Server-Sent Events instead of polling

#### SSE Usage Patterns

**Pattern 1: Native sse-swap (PREFERRED)** - Use this for most real-time updates

The canonical HTMX SSE pattern. SSE sends the content directly, replacing the element's innerHTML:

```html
<div hx-ext="sse" sse-connect="/api/updates" sse-swap="update">
  <span class="counter">Connecting...</span>
</div>
```

Server sends (incrementing counter every 2 seconds):
```
event: update
data: 42
```

**Why Pattern 1 is preferred:**
- Simpler server (single endpoint handles both connection and content)
- Lower latency (no extra HTTP round-trip per update)
- Less complexity to debug
- HTMX docs use this as the canonical example

**Go server implementation:**
```go
http.HandleFunc("/api/updates", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")

    flusher := w.(http.Flusher)
    counter := 0

    for {
        counter++
        fmt.Fprintf(w, "event: update\n")
        fmt.Fprintf(w, "data: %d\n\n", counter)
        flusher.Flush()
        time.Sleep(2 * time.Second)

        if r.Context().Err() != nil {
            break
        }
    }
})
```

**Pattern 2: SSE with hx-trigger** - Use when you need separate content endpoints

When SSE should trigger an hx-get to fetch content from a different endpoint:

```html
<div hx-ext="sse" sse-connect="/api/events">
  <span hx-get="/api/counter" hx-trigger="sse:refresh" hx-swap="innerHTML" class="counter">
    Connecting...
  </span>
</div>
```

Server sends events (separate endpoint):
```
event: refresh
data: (empty - triggers hx-get)

GET /api/counter returns:
<span class="counter">43</span>
```

**When to use Pattern 2:**
- Content comes from a different API/service
- Requires authentication/session data not available in SSE endpoint
- You need to transform/process data before rendering
- Event signaling and content fetching should be decoupled

#### HTMX Action Handlers Must Return Complete Replacements

**CRITICAL**: When HTMX actions (like button clicks) specify `hx-target` and `hx-swap="morph:outerHTML"`, the server handler MUST return the complete replacement element, not just status text.

**❌ WRONG - Handler returns status text:**
```go
func HandleStopServer(w http.ResponseWriter, r *http.Request) {
    // ... stop the server ...
    
    w.WriteHeader(http.StatusOK)
    fmt.Fprint(w, "Server stopped")  // ❌ BREAKS THE UI!
}
```

**Template:**
```html
<button 
    hx-post="/api/servers/stop"
    hx-target="#servers_list"
    hx-swap="morph:outerHTML"
>Stop</button>
```

**Problem:** The button specifies `hx-target="#servers_list"` and expects to replace that div, but the handler returns plain text. HTMX will try to replace `#servers_list` with "Server stopped", causing the entire list to disappear!

**✅ CORRECT - Handler returns complete element:**
```go
func (s *Server) HandleStopServer(w http.ResponseWriter, r *http.Request) {
    modelPath := r.FormValue("model_path")
    
    if err := s.modelManager.StopServer(modelPath); err != nil {
        http.Error(w, fmt.Sprintf("Failed: %v", err), http.StatusInternalServerError)
        return
    }
    
    // Return the updated server list (calls the list handler)
    s.HandleListModels(w, r)
}
```

**Why this works:**
- The handler calls `HandleListModels`, which returns the complete `<div id="servers_list">...</div>` element
- HTMX receives the full replacement element and can properly morph it
- The UI updates smoothly with the server now showing "stopped" status

**Best Practice:**
- Action handlers (Start, Stop, Delete, etc.) should delegate to the GET handler that returns the full element
- This ensures consistency: the same rendering logic is used for initial load and updates
- Errors should still return HTTP errors, not partial HTML

#### HTMX Dynamic Content Processing

**CRITICAL**: HTMX does not automatically process content added via morphing/swapping.

**Problem**: When using `morph:innerHTML` or similar swaps, buttons and forms in the swapped content won't have HTMX attributes active until explicitly processed.

**Solution**: Add global event listener to process swapped content:

```javascript
document.body.addEventListener('htmx:afterSwap', function(evt) {
    htmx.process(evt.detail.target);
});
```

**When Needed**:
- Any time you use `hx-swap="morph:innerHTML"` or `hx-swap="innerHTML"`
- Content that contains HTMX-enabled buttons/forms dynamically loaded
- SSE-triggered content updates with HTMX elements

**Alternative Patterns**:
- Use `morph:outerHTML` with complete elements (includes HTMX processing)
- Trigger HTMX requests from parent elements (they're already processed)

#### CSS Selectors in HTMX Targets

**Issue**: Complex CSS selectors (like `closest div[hx-get]`) are unreliable in HTMX 2.x.

**Best Practice**: Always use direct ID selectors:
```html
<!-- ✅ GOOD -->
<div id="content"></div>
<button hx-target="#content">Click</button>

<!-- ❌ AVOID -->
<button hx-target="closest div[hx-get]">Click</button>
```

#### Go Format String Gotcha

**Issue**: Backticks and commas on the same line can cause format string errors.

**Symptom**: `%!(EXTRA type=value)` appears in output

**Fix**: Always put closing backtick on separate line from arguments:
```go
// ❌ BAD
html := fmt.Sprintf(`<div>%s</div>`,
    value, extraValue) // ERROR: extra argument

// ✅ GOOD
html := fmt.Sprintf(`<div>%s</div>
`,
    value, extraValue) // Clear separation
```

### HTMX-First Development

**CRITICAL**: Always prefer HTMX attributes over custom JavaScript for web UI interactions.

**✅ CORRECT - Use HTMX attributes:**
```html
<!-- Simple HTMX request on button click -->
<button 
    hx-post="/api/servers/stop"
    hx-vals='{"model": "llama-7b"}'
    hx-target="#status"
    hx-swap="innerHTML"
>Stop Server</button>

<!-- HTMX with custom events for complex interactions -->
<div 
    hx-get="/api/models/info"
    hx-trigger="modelSelected from:body"
    hx-target="#model_info"
    hx-vals='js:{id: document.getElementById("model_id").value}'
></div>

<script>
// Minimal JS only to trigger HTMX event
document.getElementById('model_select').addEventListener('change', function() {
    htmx.trigger(document.body, 'modelSelected');
});
</script>
```

**❌ WRONG - Custom JavaScript doing AJAX:**
```html
<button onclick="stopServer()">Stop Server</button>

<script>
function stopServer() {
    fetch('/api/servers/stop', {
        method: 'POST',
        body: JSON.stringify({model: 'llama-7b'})
    })
    .then(resp => resp.text())
    .then(html => {
        document.getElementById('status').innerHTML = html;
    });
}
</script>
```

**When JavaScript is acceptable:**
- Detecting events that HTMX can't natively handle (like datalist selection)
- Triggering custom HTMX events via `htmx.trigger()`
- Reading/setting form values that HTMX will use
- Client-side validation before HTMX submission

**When JavaScript is NOT acceptable:**
- Making HTTP requests (use HTMX attributes)
- Manually updating DOM (use HTMX swaps)
- Event handlers that could be HTMX triggers
- Building query strings or form data (use `hx-vals` or `hx-include`)

**Best Practice:**
- Action handlers (Start, Stop, Delete, etc.) should delegate to the GET handler that returns the full element
- This ensures consistency: the same rendering logic is used for initial load and updates
- Errors should still return HTTP errors, not partial HTML

#### Libraries Required

**CRITICAL**: SSE extension requires specific HTMX version and proper loading order.

```html
<!-- HTMX 2.0.8 - Works correctly with SSE extension 2.2.4 -->
<!-- Use full path /dist/htmx.min.js - bare @version URL may redirect incorrectly -->
<script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.8/dist/htmx.min.js" integrity="sha384-/TgkGk7p307TH7EXJDuUlgG3Ce1UVolAOFopFekQkkXihi5u/6OCvVKyz1W+idaz" crossorigin="anonymous"></script>

<!-- SSE extension 2.2.4 - depends on htmx.org ^2.0.x - NOTE: must include /sse.js -->
<script src="https://cdn.jsdelivr.net/npm/htmx-ext-sse@2.2.4/sse.js" integrity="sha384-QA9wXqexhwzXTuTvuF5QP82pddm3R2hy81UzXi7ioNTqNF2b75hlkkSGjafohhL3" crossorigin="anonymous"></script>

<!-- Idiomorph for morph swaps -->
<script src="https://unpkg.com/idiomorph@0.3.0/dist/idiomorph-ext.min.js"></script>
```

**Version Requirements:**
| Package | Version | Notes |
|---------|---------|-------|
| htmx.org | **2.0.8** | Works with SSE 2.2.4 |
| htmx-ext-sse | **2.2.4** | Correct version for HTMX 2.0.x |

**Common mistake - Wrong HTMX version:**
```html
<!-- ❌ WRONG - Old versions without integrity -->
<script src="https://unpkg.com/htmx.org@2.0.2"></script>
<script src="https://cdn.jsdelivr.net/npm/htmx-ext-sse@2.2.3/sse.js"></script>

<!-- ✅ CORRECT - Version 2.0.8 with integrity hash -->
<script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.8/dist/htmx.min.js" integrity="sha384-/TgkGk7p307TH7EXJDuUlgG3Ce1UVolAOFopFekQkkXihi5u/6OCvVKyz1W+idaz" crossorigin="anonymous"></script>
<script src="https://cdn.jsdelivr.net/npm/htmx-ext-sse@2.2.4" integrity="sha384-QA9wXqexhwzXTuTvuF5QP82pddm3R2hy81UzXi7ioNTqNF2b75hlkkSGjafohhL3" crossorigin="anonymous"></script>
```

**Checking if SSE Extension Loaded:**
`htmx.ext` doesn't exist in HTMX 2.0.x - extensions are stored internally. Use this pattern:

```javascript
// Track registered extensions
const registeredExtensions = {};
const originalDefineExtension = htmx.defineExtension.bind(htmx);
htmx.defineExtension = function(name, extension) {
    const result = originalDefineExtension(name, extension);
    registeredExtensions[name] = true;
    console.log('Extension "' + name + '" registered');
    return result;
};

// Later check:
if (registeredExtensions.sse) {
    console.log('✓ SSE extension loaded');
}
```

**SSE Event Format:**
```go
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")
w.Header().Set("Access-Control-Allow-Origin", "*")

flusher := w.(http.Flusher)

// Send named event with HTML content
fmt.Fprintf(w, "event: download_update\n")
fmt.Fprintf(w, "data: <span class=\"counter\">%d</span>\n\n", counter)
flusher.Flush()
```

**Listening for SSE Events:**
```javascript
document.body.addEventListener('htmx:sseOpen', function(e) {
    console.log('SSE Connection opened');
});
document.body.addEventListener('htmx:sseError', function(e) {
    console.log('SSE Error:', e.detail);
});
document.body.addEventListener('htmx:sseClose', function(e) {
    console.log('SSE Connection closed:', e.detail.type);
});
document.body.addEventListener('htmx:sseMessage', function(e) {
    console.log('SSE Message received');
});
```

#### Troubleshooting HTMX Extensions

**1. Integrity Hashes Must Be Correct**

When loading HTMX extensions from CDN, always use valid `integrity` hashes. An incorrect hash will silently block the script from executing. The console will show:

```
Failed to find a valid digest in the 'integrity' attribute for resource '...'
The resource has been blocked.
```

The error message includes the *computed* SHA-384 hash - use that value for the correct `integrity` attribute.

**2. Script Loading Order for Extension Wrapping**

If you need to intercept extension registration (e.g., for diagnostics), the wrapper script must load BEFORE the extension:

```html
<!-- ✅ CORRECT - Wrapper loads first -->
<script>
const registeredExtensions = {};
const originalDefineExtension = htmx.defineExtension.bind(htmx);
htmx.defineExtension = function(name, extension) {
    registeredExtensions[name] = true;
    return originalDefineExtension(name, extension);
};
</script>
<script src="htmx-ext-sse@2.2.4/sse.js"></script>

<!-- ❌ WRONG - Wrapper loads after extension -->
<script src="htmx-ext-sse@2.2.4/sse.js"></script>
<script>
const registeredExtensions = {};
// SSE already registered, this won't catch it!
</script>
```

**3. Browser Cache After Code Changes**

After fixing HTMX/extension issues, force-clear the browser cache:
```javascript
// In Chrome DevTools console
window.location.reload(true)
// Or use the MCP tool with ignoreCache: true
```

**4. Don't Use JSON.stringify on Event Detail Objects**

HTMX event `detail` objects contain circular references (e.g., element references that point back to themselves). Using `JSON.stringify(e.detail)` will throw "Converting circular structure to JSON" errors.

**❌ WRONG:**
```javascript
document.body.addEventListener('htmx:sseMessage', function(e) {
    logEvent('SSE: ' + JSON.stringify(e.detail));  // ERROR!
});
```

**✅ CORRECT:**
```javascript
document.body.addEventListener('htmx:sseMessage', function(e) {
    logEvent('SSE Message received');
});
// Or extract specific safe properties:
document.body.addEventListener('htmx:sseError', function(e) {
    logEvent('Error: ' + (e.detail.error ? e.detail.error.message : 'unknown'));
});
```

### Alpine.js + HTMX Integration

**Alpine.js and HTMX work perfectly together.** Use Alpine.js for local UI state (loading indicators, show/hide, form validation) and HTMX for server communication (fetching HTML, form submissions, real-time updates).

#### When to Use Alpine.js

**✅ DO use Alpine.js for:**
- Loading spinners on async requests
- Client-side form validation before submission
- Show/hide UI elements (dropdowns, modals, collapsible sections)
- Local search/filtering (before sending to server)
- Tab/accordion UI state
- Ephemeral UI state that resets on page navigation

**❌ DON'T use Alpine.js for:**
- Making HTTP requests (use HTMX)
- Replacing HTMX functionality
- Complex state management (use a framework like React/Vue instead)
- Server-side data fetching

#### Loading Indicators Pattern

**Setup (in `<head>`):**
```html
<script defer src="https://unpkg.com/alpinejs@3.x.x/dist/cdn.min.js"></script>

<style>
.spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-radius: 50%;
    border-top-color: white;
    animation: spin 0.6s linear infinite;
}

@keyframes spin {
    to { transform: rotate(360deg); }
}

.loading .btn-text { display: none; }
.loading .spinner { display: inline-block; }
button:not(.loading) .spinner { display: none; }
</style>
```

**Button with loading spinner:**
```html
<form 
    hx-post="/api/servers/start" 
    hx-target="#servers_list" 
    hx-swap="innerHTML" 
    x-data="{ loading: false }"
>
    <input type="hidden" name="model_path" value="..." />
    <button 
        type="submit"
        @click="loading = true"
        :disabled="loading"
        :class="{ 'loading': loading }"
    >
        <span class="spinner"></span>
        <span class="btn-text">Start</span>
    </button>
</form>
```

**How it works:**
1. User clicks button
2. `@click="loading = true"` sets Alpine.js state
3. `:class="{ 'loading': loading }"` adds `.loading` class
4. CSS shows spinner, hides text
5. `:disabled="loading"` prevents double-clicks
6. HTMX sends request and swaps content
7. Content swap resets state (new elements rendered)

#### Alpine.js + HTMX Events

HTMX emits events that Alpine.js can listen to for global loading states:

```html
<div 
    x-data="{ loading: false }"
    @htmx:before-request.window="loading = true"
    @htmx:after-swap.window="loading = false"
>
    <div x-show="loading" class="loading-overlay">
        <div class="spinner-large"></div>
    </div>
</div>
```

**Available HTMX events:**
- `htmx:before-request` - Before request is sent
- `htmx:after-request` - After request completes
- `htmx:before-swap` - Before content is swapped
- `htmx:after-swap` - After content is swapped
- `htmx:response-error` - On HTTP error

**Best Practice:**
- Use button-level loading state (first pattern) for individual actions
- Use global loading overlay for full-page loads
- Always disable buttons during loading to prevent double-clicks
- Let HTMX content swaps naturally reset Alpine.js state

## Concurrency and Mutex Guidelines

### Never Hold Locks During I/O Operations

**CRITICAL**: Holding mutex locks while performing I/O operations (HTTP requests, file operations, external commands) can cause deadlocks and severe performance issues.

**Problem Pattern:**
```go
func (mm *ModelManager) RefreshServerStatus() error {
    mm.mu.Lock()
    defer mm.mu.Unlock()
    
    // ❌ BAD: Holding lock while doing HTTP requests
    for port := 8081; port <= 8090; port++ {
        resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
        // ... process response ...
    }
    
    // Update shared state
    server.Status = "running"
}
```

**Why this is bad:**
1. Other goroutines trying to read the same data are blocked during ALL I/O operations
2. If you have 10 ports to check at 200ms timeout each = 2 seconds of blocking
3. Can cause deadlocks if the I/O operation indirectly tries to acquire the same lock
4. Severely degrades application responsiveness

**✅ CORRECT Pattern - Gather Data First, Lock Briefly:**
```go
func (mm *ModelManager) RefreshServerStatus() error {
    // Step 1: Do ALL I/O WITHOUT holding any locks
    type portInfo struct {
        port   int
        status string
        // ... other fields
    }
    
    var portsData []portInfo
    for port := 8081; port <= 8090; port++ {
        resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
        if err == nil {
            portsData = append(portsData, portInfo{port: port, status: "running"})
        }
        resp.Body.Close()
    }
    
    // Step 2: Acquire lock ONLY to update shared state (fast)
    mm.mu.Lock()
    defer mm.mu.Unlock()
    
    for _, data := range portsData {
        if server, exists := mm.servers[data.port]; exists {
            server.Status = data.status
        }
    }
    
    return nil
}
```

**Benefits:**
- Lock is only held for microseconds (memory updates), not seconds (I/O)
- Other goroutines can read data while I/O is happening
- No risk of deadlock from nested lock attempts
- Much better application responsiveness

**Examples of I/O that should NOT be done under locks:**
- `http.Client.Get()` / `http.Client.Post()` - Network requests
- `exec.Command().Output()` - External process execution (e.g., `lsof`, `rocm-smi`)
- `os.Open()` / `ioutil.ReadFile()` - File operations
- `time.Sleep()` - Deliberate delays
- Database queries

**General Rule:**
1. Do all I/O operations first (without locks)
2. Collect results in local variables
3. Acquire lock briefly
4. Update shared state from local variables
5. Release lock

This pattern prevents deadlocks and keeps your application responsive.

## API Usage Patterns

### Use bufio.Scanner for efficient line-by-line file reading

Use bufio.Scanner for efficient line-by-line file reading

*Source: cli-command | Confidence: 0.8 | Importance: 3* *Tags: api_usage*
