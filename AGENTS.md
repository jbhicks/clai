# AGENTS.md

## Core Operational Rules
This repository is a Go CLI project for local AI agent interaction. These rules take precedence over all others.

### 1. Server Management & Building (ABSOLUTELY FORBIDDEN)
**Agents MUST NEVER start, stop, restart dev servers, or manually run `go build`.**
- **VIOLATION**: Never run `make dev`, `make dev-benchmark`, `./clai benchmark`, or `go build`.
- **REASON**: These commands are either blocking or redundant; a dev server automatically handles rebuilding.
- **CONSEQUENCE**: Session freeze or redundant resource usage.
- **TESTING**: Use the automated dev server; verify logic changes via the UI or `clai-debug`.

### 2. Mandatory Testing Requirement
**NO EXCEPTIONS**: You must verify all UI and functional changes using the `clai-debug` MCP tools before replying.
- See: [Testing Workflow & Standards](docs/guidelines/TESTING_WORKFLOW.md)

### 3. Code Style & Health
- **Go Best Practices**: Stdlib first, explicit types, group imports, `gofmt`.
- **Accuracy**: Code comments MUST match the implementation. Verify function calls with `grep`.
- **Formatting**: Use [TUI Patterns](docs/guidelines/TUI_PATTERNS.md) for all Bubble Tea views.

### 4. Background Processes
- Never run blocking scripts (e.g., `scripts/dev.sh`) in the foreground.
- Use `&` or background redirection if necessary.
- View project status and future plans in **[CLAI Roadmap](docs/ROADMAP.md)**.

### 5. The Ralph Method (Architectural Vision)
CLAI is transitioning into an autonomous orchestrator.
- **Goal**: Automate the PRD -> User Story -> Sub-agent -> Verification loop.
- **Reference**: Follow the patterns in `docs/reference/library_reference/ralph/AGENTS.md`.
- **Memory**: Always respect and update `progress.txt` and `prd.json`.
- **Concurrency**: Use non-blocking goroutines for sub-agent execution.

---

## Technical Guidelines Index
For detailed implementation patterns, refer to these specialized documents:

- **[TUI & Bubble Tea Patterns](docs/guidelines/TUI_PATTERNS.md)**
  - Full-screen height rules (`m.height - 1`).
  - Sectional Layout (Header/Body/Footer).
  - Lipgloss border transparency fixes.
  - Sizing math and clamping.
  - See also: **[UI Layout Guide](docs/guidelines/UI_LAYOUT_GUIDE.md)** for legacy reference.

- **[API & Concurrency](docs/guidelines/API_AND_CONCURRENCY.md)**
  - Mutex locking rules (No locks during I/O).
  - SSE and HTTP interaction rules.
  - llama.cpp server configuration.
  - MCP/Socket handling.

- **[Testing Workflow](docs/guidelines/TESTING_WORKFLOW.md)**
  - Mandatory `clai-debug` verification steps.
  - TDD implementation flow and automated tmux testing.
  - See also: **[Bubble Tea Testing Strategy](docs/guidelines/BUBBLETEA_TESTING.md)** for detailed patterns and mocks.

- **[Web UI (HTMX/Alpine)](docs/guidelines/WEB_UI.md)**
  - HTMX morphing and flickering prevention.
  - Alpine.js integration for loading states.
  - SSE and real-time dashboard patterns.

- **[CLAI Roadmap](docs/ROADMAP.md)**
  - Current status, future phases, and known technical debt.

---

## Build and Run Commands
- Build: `go build -o clai ./cmd/clai`
- Test: `go test ./...`
- Clean: `make clean`
- Debug: Check logs via `tail -f clai.log` or similar if configured.
