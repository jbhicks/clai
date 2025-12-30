# AGENTS.md

This repository is a Go CLI project for local AI agent interaction. Follow these guidelines for agentic coding:

## Build, Lint, and Test Commands
- Build: `make build` or `go build -o clai ./cmd/clai`
- Run: `make run` or `go run ./cmd/clai`
- Test all: `make test` or `go test ./...`
- Run a single test: `go test -run TestFunctionName ./...`
- Clean: `make clean`
- Install: `make install`
- Dev/debug: `make dev` (runs with DEBUG=true)

## Code Style Guidelines
- Use standard Go libraries; minimize dependencies.
- Group imports: stdlib first, then external.
- Format code with `gofmt`.
- Use clear, descriptive names for types, functions, and variables.
- Define tool schemas as Go structs for JSON marshalling.
- Handle errors gracefully (invalid JSON, tool failures, timeouts).
- Use Go doc comments for exported functions/types.
- Prefer explicit types and avoid unnecessary complexity.

## UI Component Guidelines
- Always use Charm Bubble components for all UI pieces.
- Do not write custom UI components unless absolutely necessary (e.g., when no Bubble component exists for your use case).
- If a custom component is required, document the reason in code comments and prefer extending Bubble components when possible.

### Bubble Tea Layout & Sizing Best Practices

**Critical Rules for Proper Layout:**

1. **Height Calculation - Account for Actual Rendered Heights**
   - **DON'T** use `style.GetHeight()` for styles that rely on padding/content - it returns 0
   - **DO** use actual rendered heights: e.g., status bar with padding is always 1 row
   - Formula: `contentHeight = terminalHeight - statusBarRows - errorBannerRows`
   - Example: For 84-row terminal with 1-row status bar: `contentHeight = 84 - 1 = 83`

2. **Width Calculation - Respect Frame Sizes**
   - Calculate inner content widths: `innerWidth = outerWidth - style.GetHorizontalFrameSize()`
   - `GetHorizontalFrameSize()` returns: left border (1) + left padding + right padding + right border (1)
   - Example: For 70% of 139 cols = 97 cols, with padding(0,2) + rounded border:
     - `GetHorizontalFrameSize()` = 6 (1 + 2 + 2 + 1)
     - `innerWidth` = 97 - 6 = 91

3. **Applying Styles Without Breaking Dimensions**
   - **DON'T** use `style.Width(outerWidth).Render(content)` - this adds frame size on top
   - **DO** calculate inner dimensions, set those on components, then render with unsized style
   - **DO** pad the result if needed to reach exact outer width
   ```go
   // Calculate inner dimensions
   innerWidth := outerWidth - style.GetHorizontalFrameSize()
   innerHeight := outerHeight - style.GetVerticalFrameSize()
   
   // Set dimensions on inner content/components
   component.Width = innerWidth
   component.Height = innerHeight
   
   // Render with style (no width set on style)
   view := style.Render(component.View())
   
   // Pad to exact width if needed
   if lipgloss.Width(view) < outerWidth {
       view = lipgloss.NewStyle().Width(outerWidth).Render(view)
   }
   ```

4. **Vertical Layout - Join Carefully**
   - When using `lipgloss.JoinVertical()`, ensure sum of heights equals terminal height
   - Each component's rendered height (via `lipgloss.Height()`) must be accounted for
   - Example: `mainView (83) + statusBar (1) = 84 (terminal height)`

5. **Horizontal Layout - Join Carefully**
   - When using `lipgloss.JoinHorizontal()`, ensure sum of widths equals terminal width
   - Each pane's rendered width (via `lipgloss.Width()`) must be accounted for
   - Example: `chatPane (97) + logPane (42) = 139 (terminal width)`
   - **CRITICAL**: When splitting terminal width between panes, calculate each pane's width FIRST, then set component dimensions based on those pane widths
   - Example:
     ```go
     // Calculate pane widths first
     chatPaneWidth := int(float64(terminalWidth) * 0.8)
     logPaneWidth := terminalWidth - chatPaneWidth
     
     // Then set component inner widths by subtracting frame sizes
     chatComponent.Width = chatPaneWidth - style.GetHorizontalFrameSize()
     logComponent.Width = logPaneWidth - style.GetHorizontalFrameSize()
     ```
   - **DON'T** set all component widths to full terminal width minus frame - this makes all panes the same size

6. **Debugging Layout Issues**
   - **CRITICAL**: NEVER add `logger.Debug()` calls inside `View()` methods - they're called in tight render loops
   - Use the debug server (`clai debug inspect`) instead for inspecting rendered output
   - If you must add temporary debug logging, only add it in `Update()` methods or initialization functions
   - Log actual rendered dimensions: `lipgloss.Height(view)`, `lipgloss.Width(view)`
   - Log component dimensions before rendering: `component.Width`, `component.Height`
   - Log frame sizes: `style.GetHorizontalFrameSize()`, `style.GetVerticalFrameSize()`
   - Compare: terminal size → calculated sizes → rendered sizes
   - Off-by-one errors usually indicate frame size or padding miscalculation

7. **List Component - Multi-Line Content Handling**
   - **CRITICAL**: List delegates report each item as 1 row height, but multi-line content will render taller
   - **DON'T** add multi-line strings (with `\n`) as single list items - this causes overflow
   - **DO** split multi-line content into separate items: `strings.Split(text, "\n")`
   - Example problem: Adding "Line1\nLine2\nLine3" as 1 item renders 3 rows but list thinks it's 1 row
   - Example solution: Add each line separately: `for _, line := range strings.Split(text, "\n") { list.InsertItem(...) }`
   - Set list styles with zero padding to prevent additional height: `list.Styles.PaginationStyle = list.Styles.PaginationStyle.Padding(0)`

8. **Background Colors and Content Wrapping**
   - **CRITICAL**: When rendering styled content (borders/padding) inside a container with a background, empty space must be explicitly filled
   - **Problem**: `lipgloss.NewStyle().Width(w).Render(styledContent)` creates transparent space, showing mismatched backgrounds
   - **Solution**: Always set `.Background()` on wrapper styles to match the parent container:
     ```go
     bubble := messageStyle.Render(content)
     wrapper := lipgloss.NewStyle().
         Width(containerWidth).
         Background(lipgloss.Color(parentBackground)).
         Align(lipgloss.Left)
     rendered := wrapper.Render(bubble)
     ```
   - This ensures the entire line has consistent background color with no visible "boxes" or artifacts
   - **CRITICAL**: When rendering content that will be concatenated/joined, ensure ALL message styles have explicit backgrounds:
     ```go
     // In theme styles, ALWAYS set backgrounds
     UserMessage: lipgloss.NewStyle().
         Border(lipgloss.RoundedBorder()).
         Background(lipgloss.Color(ui.Theme.Primary.Background)).  // REQUIRED
         Foreground(lipgloss.Color(ui.Theme.Bright.White)).
         Padding(0, 1)
     ```
   - **CRITICAL**: After rendering styled bubbles, pad each line to full container width with matching background:
     ```go
     // Render the bubble with style
     bubble := messageStyle.Render(content)
     
     // Pad each line to full width with background color
     bg := lipgloss.Color(theme.Primary.Background)
     lines := strings.Split(bubble, "\n")
     for i, line := range lines {
         if lipgloss.Width(line) < containerWidth {
             padStyle := lipgloss.NewStyle().Width(containerWidth).Background(bg)
             lines[i] = padStyle.Render(line)
         }
     }
     result := strings.Join(lines, "\n")
     ```
   - Without line padding, you'll see different colored backgrounds where content doesn't reach full width

9. **Nested Borders - Apply ONE Border Per Visual Pane**
   - **CRITICAL**: When stacking multiple components in a single visual pane, apply borders AFTER joining, not before
   - **Problem**: Wrapping each component with a border style before joining causes border duplication
   - **DON'T**:
     ```go
     comp1View := borderStyle.Render(comp1.View())  // adds 4 rows (border + padding)
     comp2View := borderStyle.Render(comp2.View())  // adds 4 rows (border + padding)
     combined := lipgloss.JoinVertical(lipgloss.Left, comp1View, comp2View)  // 8 extra rows!
     ```
   - **DO**:
     ```go
     comp1Inner := comp1.View()
     comp2Inner := comp2.View()
     combined := lipgloss.JoinVertical(lipgloss.Left, comp1Inner, comp2Inner)
     finalView := borderStyle.Render(combined)  // only 4 extra rows total
     ```
   - When calculating inner heights, account for the SINGLE border that wraps the combined content
   - Example: For agent status (8 rows) + log (71 rows) in one pane:
     - Inner total: 8 + 71 = 79 rows
     - Subtract ONE border frame: `contentHeight - style.GetVerticalFrameSize()` = 83 - 4 = 79 ✓
     - NOT per-component: `(8 + 4) + (71 + 4)` = 87 ✗

**Common Pitfalls:**
- ❌ Using `style.GetHeight()` when the style has no explicit height (returns 0)
- ❌ Setting `.Width()` on a style that already has padding/borders (double-counts frame)
- ❌ Forgetting to subtract frame sizes when calculating inner dimensions
- ❌ Not accounting for status bars, borders, or other UI chrome in height calculations
- ❌ Adding multi-line content (`\n` newlines) as single list items - causes height overflow
- ❌ Creating width wrappers without explicit backgrounds - causes background color mismatches
- ❌ **Forgetting to set explicit backgrounds on message/bubble styles** - causes transparent backgrounds showing terminal default color
- ❌ **Not padding rendered lines to full container width** - causes visible background color boxes/artifacts where content doesn't fill the line
- ❌ Applying borders to components BEFORE joining them vertically/horizontally - causes border duplication
- ❌ **NEVER** add debug logging inside `View()` methods - they're called in render loops and will cause infinite spam/lockups

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

No Cursor or Copilot rules detected.

## Development Workflow
- **ASSUME** the user is running `make dev` in another terminal/thread with automatic file watching and reload enabled.
- **NEVER** run the application yourself (e.g., `./clai` or `make run`).
- **NEVER** start, stop, or restart servers (main app or benchmark server) - the user will handle this.
- After making code changes, **ASSUME** the automatic reload has already occurred.
- To verify your changes:
  1. Check `debug.log` for runtime logs: `tail -f debug.log` or `cat debug.log`
  2. Look for errors, warnings, or debug output related to your changes
  3. If needed, ask the user to test specific functionality in the running app
- The dev watcher uses `air` to automatically rebuild and restart on `.go` file changes.
- **Exception:** You may check if processes are running (`ps aux | grep`), check listening ports (`ss -tlnp`), or read log files to diagnose issues, but do not start/stop/kill processes.

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

**Pattern 1: Native sse-swap (Direct Content Swap)**
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

**Pattern 2: SSE with hx-trigger**
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
