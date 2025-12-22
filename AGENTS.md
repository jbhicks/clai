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

**Common Pitfalls:**
- ❌ Using `style.GetHeight()` when the style has no explicit height (returns 0)
- ❌ Setting `.Width()` on a style that already has padding/borders (double-counts frame)
- ❌ Forgetting to subtract frame sizes when calculating inner dimensions
- ❌ Not accounting for status bars, borders, or other UI chrome in height calculations
- ❌ Adding multi-line content (`\n` newlines) as single list items - causes height overflow
- ❌ Creating width wrappers without explicit backgrounds - causes background color mismatches
- ❌ **NEVER** add debug logging inside `View()` methods - they're called in render loops and will cause infinite spam/lockups

## Testing
- Use Go's `testing` package for unit/integration tests.
- Mock external dependencies (e.g., Ollama) in tests.
- For Bubble Tea projects, follow the [Bubble Tea Agent Testing Strategy](BUBBLETEA_TESTING_STRATEGY.md) for all test creation. This document provides detailed guidelines and examples for unit, integration, and UI testing specific to Bubble Tea applications.

No Cursor or Copilot rules detected.

## Development Workflow
- **ASSUME** the user is running `make dev` in another terminal/thread with automatic file watching and reload enabled.
- **NEVER** run the application yourself (e.g., `./clai` or `make run`).
- After making code changes, **ASSUME** the automatic reload has already occurred.
- To verify your changes:
  1. Check `debug.log` for runtime logs: `tail -f debug.log` or `cat debug.log`
  2. Look for errors, warnings, or debug output related to your changes
  3. If needed, ask the user to test specific functionality in the running app
- The dev watcher uses `air` to automatically rebuild and restart on `.go` file changes.

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
