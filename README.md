# clai
An AI interface, built for local AI, by AI. AlrAIght?

# CLAI Implementation Plan

## Project Overview
CLAI is a simple Go CLI application that provides a terminal-based interface to a locally-run LLM AI agent powered by Meta's Llama 3.1 model. The app focuses on tool-calling capabilities, allowing users to interact with the LLM, which can invoke predefined tools via structured JSON outputs. Key principles:
- **Simplicity**: Minimize dependencies, avoid unnecessary complexity. Use standard Go libraries where possible.
- **Local Execution**: Assume Llama 3.1 runs via Ollama (a lightweight local LLM server) for easy integration and OpenAI-compatible API.
- **UI Framework**: Use Bubble Tea for the terminal UI, as it provides a clean, reactive TUI model in Go without excessive overhead. This enables a chat-like interface with input handling and output rendering.
- **Tool Calling**: Based on Llama 3.1's JSON-structured tool calling spec (tool schemas in prompts, parsed outputs with `tool_calls` array).
- **MCP Server**: Not required. MCP adds standardization for external tool ecosystems but introduces unnecessary complexity for a simple CLI. Instead, handle tool calling directly in CLAI: Parse LLM outputs, execute tools in Go code, and loop back results to the LLM. This keeps everything in one binary.

**Target Model**: Llama 3.1 8B Instruct (for simplicity and lower resource needs; scalable to 70B/405B later).

**Assumptions**:
- User has Ollama installed and Llama 3.1 pulled (`ollama run llama3.1`).
- Ollama runs on `http://localhost:11434`.
- Initial tools: A few examples like `web_search` (mocked or real via API) and `calculator` (simple math eval in Go).

## Architecture
High-level architecture:
```
User <-> CLAI CLI (Bubble Tea TUI) <-> Tool Handler (Go funcs) <-> Ollama API (Local Llama 3.1)
```
- **CLAI CLI**: Bubble Tea-based TUI for chat input/output. Handles user messages, displays responses, and manages conversation state.
- **LLM Interface**: HTTP client to Ollama's `/api/chat` endpoint (OpenAI-compatible). Sends messages with tool schemas, receives responses.
- **Tool Handler**: Parses `tool_calls` from LLM output, executes tools (e.g., via Go functions or external APIs), formats results as "tool" role messages, and re-queries LLM.
- **Data Flow**:
  1. User inputs message.
  2. CLAI constructs chat payload with system prompt, tools, and history.
  3. Send to Ollama; parse response.
  4. If tool_calls present, execute in parallel if possible, append results, repeat query.
  5. Display final synthesized response.

No separate MCP server; tool execution is embedded in CLAI for simplicity.

## Dependencies
- Go 1.22+ (standard library for HTTP/JSON).
- Bubble Tea: `github.com/charmbracelet/bubbletea` (for TUI).
- Glamour: `github.com/charmbracelet/glamour` (optional for markdown rendering in responses).
- Ollama: External (not a Go dep; assume running locally).
- Testing: Go's `testing` package; no external test frameworks.

---

## UI Prototyping with minimal_testing.go

`minimal_testing.go` is a generic Bubble Tea harness for rapid prototyping and testing of UI components. It allows you to quickly implement, run, and visually inspect any Bubble Tea component in isolation.

### How It Works
- The file defines a `UIComponent` interface (with `Init`, `Update`, and `View` methods).
- You implement your own component by satisfying this interface.
- Swap your component into the `main()` function to run and interact with it in the terminal.

### Workflow
1. **Implement your component**: Create a struct and methods that satisfy `UIComponent`.
2. **Plug it into minimal_testing.go**: Replace the `ExampleComponent()` function in `main()` with your own.
3. **Run the harness**: Execute `go run minimal_testing.go` to launch your component in the terminal.
4. **Interact and inspect**: Use the keyboard to interact with your component and observe its output.

### Example
Here's a minimal counter component:
```go
// Define your component
func MyCounterComponent() UIComponent {
    return &counterModel{count: 0}
}

type counterModel struct {
    count int
}

func (c *counterModel) Init() tea.Cmd { return nil }
func (c *counterModel) Update(msg tea.Msg) (UIComponent, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        if msg.String() == "space" { c.count++ }
    }
    return c, nil
}
func (c *counterModel) View() string {
    return fmt.Sprintf("Count: %d (press space)", c.count)
}

// In main(), swap in your component:
func main() {
    p := tea.NewProgram(initialModel(MyCounterComponent()))
    if err := p.Start(); err != nil { panic(err) }
}
```

### Tips
- Use this harness to quickly iterate on UI ideas before integrating into the main app.
- You can prototype text inputs, lists, forms, or any Bubble Tea model.
- Press `q` or `ctrl+c` to quit the harness.

---

## Implementation Phases
Phases are sequential for simplicity, with checkpoints for testing.

1. **Setup and Boilerplate** (1-2 days)
   - Initialize Go module: `go mod init github.com/user/clai`.
   - Install dependencies: `go get github.com/charmbracelet/bubbletea`.
   - Create basic CLI skeleton with Bubble Tea model (init, update, view funcs).
   - Configure Ollama client (HTTP POST to `/api/chat`).

2. **Basic Chat Interface** (2-3 days)
   - Implement TUI: Text input, message history display, scrolling.
   - Send plain messages to LLM without tools.
   - Handle streaming responses if Ollama supports (via SSE).

3. **Tool Calling Integration** (3-5 days)
   - Define tool schemas in Go (structs for JSON marshalling).
   - Add tools to system prompt/chat payload.
   - Parse LLM output for `tool_calls`.
   - Implement execution loop: Execute tools, append results, re-query.
   - Example tools: `calculator` (use Go's `math` or `go-eval`), `echo` (simple string return).

4. **Advanced Features** (2-4 days)
   - Support multiple/parallel tool calls.
   - Add error handling (e.g., invalid JSON, tool failures).
   - Persist conversation history (JSON file).
   - Customizable system prompt via flags.

5. **Testing and Polish** (2-3 days)
   - Write unit/integration tests.
   - Add CLI flags (e.g., `--model=llama3.1`, `--port=11434`).
   - Build binary: `go build -o clai`.

Total Estimated Time: 10-17 days for MVP.

## Feature Tracking

Use these checkboxes to track implementation progress.

- [x] **Project Setup**: Init Go module, deps, basic main.go with Bubble Tea skeleton.
- [x] **TUI Input/Output**: Bubble Tea model for user input, message rendering, history scroll.
- [x] **LLM Client**: HTTP client to Ollama `/api/chat`; handle JSON payloads/responses.
- [x] **Basic Chat**: Send/receive plain messages; display in TUI.
- [x] **Tool Schema Definition**: Go structs for tool JSON schemas (name, desc, params).
- [x] **System Prompt**: Configurable prompt instructing LLM on tool use.
- [x] **Output Parsing**: Parse assistant message for `tool_calls` array.
- [x] **Tool Execution Loop**: If tools called, execute, append "tool" role messages, re-query LLM.
- [x] **Example Tool: Calculator**: Simple math eval tool.
- [x] **Example Tool: Echo**: Return input string for testing.
- [x] **Error Handling**: Graceful failures for bad JSON, tool errors, LLM timeouts.
- [x] **Conversation Persistence**: Save/load chat history to a local file.
- [x] **CLI Flags**: Support for `--model`, `--host`, `--system-prompt`.
- [x] **Streaming Responses**: Stream LLM output to the TUI for better UX.

## Testing Plan
All tests use Go's `testing` package. Aim for 80%+ coverage. Run with `go test ./...`.

- **Unit Tests**:
  - LLM Client: Mock HTTP server; test payload marshalling, response unmarshalling.
  - Output Parsing: Test valid/invalid JSON with tool_calls.
  - Tool Execution: Test individual tools (e.g., calculator with inputs like "2+2").
  - Schema Generation: Test JSON output for tool definitions.

- **Integration Tests**:
  - Full Chat Loop: Mock Ollama responses; simulate tool call -> exec -> re-query.
  - TUI Simulation: Use Bubble Tea's testing utils (if available) or manual runs.

- **Tool Calling Specific Tests**:
  - Zero Tools: Normal response without calls.
  - Single Tool Call: Parse, exec, feed back result.
  - Multiple Calls: Parallel exec, ensure all results appended.
  - Loop Termination: Ensure stops when no more calls.
  - Error in Tool: Handle and propagate to user.
  - Llama 3.1 Format Compliance: Test with sample prompts from spec (e.g., weather tool example).

Mock Ollama with a test server (e.g., httptest) to avoid real LLM dependency in tests.

## Next Steps
- Start with Phase 1: Setup.
- Update this plan's table as features complete (e.g., via agent self-tracking).
- If MCP is reconsidered (e.g., for external tools), it could be added as a separate Go server binary, but defer unless needed.

## Live Reload for Bubble Tea TUI

For reliable live reload during development, use two terminals:

**Terminal 1:**
```
bash dev_run.sh
```
This keeps the TUI app running in the foreground. If the binary is killed, it restarts immediately.

**Terminal 2:**
```
bash dev_watch.sh
```
This watches for changes to Go files, rebuilds the binary, and kills the running process to trigger a restart.

**Why not use Air, wgo, or Makefile wrappers?**
Bubble Tea TUIs require a real terminal (TTY/PTY). Watcher tools and Makefile wrappers do not reliably pass through the terminal context, so interactive UIs may not display or update correctly.

**Scripts:**
- `dev_run.sh`: Runs the app in a loop.
- `dev_watch.sh`: Watches for changes, rebuilds, and restarts the app.

**Usage:**
1. Open two terminals in the project root.
2. In one, run `bash dev_run.sh`.
3. In the other, run `bash dev_watch.sh`.

This workflow ensures instant reloads and a robust TUI experience.
