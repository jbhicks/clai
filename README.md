# clai
An AI interface, built for local AI, by AI. AlrAIght?

## Features

- **Local AI Integration**: Built for Ollama and OpenAI-compatible local LLM endpoints
- **Interactive TUI**: Modern terminal user interface built with Bubble Tea
- **Tool System**: Extensible tool calling system for LLM function execution
- **Conversation Management**: Persistent conversation history with SQLite
- **Smart Chat Viewport**:
  - Auto-scroll with manual override
  - Mouse wheel and smooth keyboard navigation
  - Visual scroll position indicators with percentage
  - Beautiful chat bubbles (user right-aligned, assistant left-aligned)
  - Dynamic bubble sizing with max 70% width
- **Theme Support**: Multiple color themes (Dracula, Catppuccin Mocha, Tokyo Night Storm)
- **Live Reload**: Developer-friendly hot reload workflow
- **Configurable Logging**: Multi-level logging system (DEBUG, INFO, WARN, ERROR) with environment variable control
- **Autonomous Agent Mode**: Optional ReAct-style agent with embedded JavaScript runtime for complex reasoning and task execution

## Configuration

### Environment Variables

- `OLLAMA_MODEL`: LLM model to use (default: `llama3.1-gpu:latest`)
- `OLLAMA_HOST`: Ollama server URL (default: `http://localhost:11434`)
- `SYSTEM_PROMPT`: Custom system prompt for the LLM
- `AGENT_MODE`: Enable autonomous ReAct agent mode (`true`/`false`, default: `false`)
- `LOG_LEVEL`: Logging verbosity level - `DEBUG`, `INFO`, `WARN`, or `ERROR` (default: `INFO`)

**Log Levels:**
- `DEBUG`: Verbose output including LLM request/response details, UI rendering info, and detailed execution traces
- `INFO`: Standard operational messages (startup, conversation loading, tool execution)
- `WARN`: Warning messages for non-critical issues
- `ERROR`: Error messages only

To reduce log verbosity, set `LOG_LEVEL=WARN` or `LOG_LEVEL=ERROR` in your environment or `.env` file.

## Agent Mode

Enable autonomous agent mode by setting `AGENT_MODE=true`. In this mode, the LLM uses a ReAct-style reasoning loop with:

- **Embedded JavaScript Runtime**: Execute code directly using Goja (pure-Go ECMAScript 5.1+ implementation)
- **Iterative Reasoning**: Think → Code/Delegate → Observe → Repeat
- **Parallel Sub-Agent Delegation**: Spawn concurrent sub-agents for independent subtasks
- **Structured Response Format**: Enforces Thought/Delegation/Code/Final Answer structure

**Example queries for agent mode:**
- `What is 5 + 3?` (simple math via JavaScript)
- `Calculate 15 * 23 + 100 and log the steps`
- Complex multi-step reasoning tasks that benefit from iterative refinement

**How it works:**
1. Agent receives query and enters reasoning loop (max 20 iterations)
2. Each iteration: LLM responds with structured format (Thought/Code/Delegation/Final Answer)
3. If Code block present: Execute JavaScript and feed output back as Observation
4. If Delegation present: Spawn sub-agents in parallel and collect results
5. Continue until Final Answer or max iterations reached

**Enable:**
```sh
export AGENT_MODE=true
./clai
```

## Keyboard Shortcuts

**Global:**
- `Ctrl+T`: Switch between chat and log panes
- `Ctrl+H`: Toggle help
- `Ctrl+D`: Cycle themes
- `Ctrl+Q`: Quit

**Chat Navigation (when input not focused):**
- `↑` or `k`: Scroll up one line
- `↓` or `j`: Scroll down one line
- `Page Up`: Scroll up one page
- `Page Down`: Scroll down one page
- `Home` or `g`: Jump to top
- `End` or `G`: Jump to bottom

**Input:**
- `Ctrl+N`: Focus input (from log pane)
- `↑`/`↓`: Navigate command history

# CLAI Implementation Plan

[...unchanged content above...]

## Live Reload for Bubble Tea TUI

[...unchanged content above...]

This workflow ensures instant reloads and a robust TUI experience.

---

### Recommended: tmux-based Live Reload for Bubble Tea TUI

Bubble Tea TUIs require a real TTY for proper rendering and hot-reload. The best workflow is to use tmux with two panes:

- **Pane 0:** Runs the TUI app
- **Pane 1:** Runs a watcher that rebuilds and restarts the app on code changes

To start this workflow, run:

```sh
make dev
```

This will:
- Check for tmux installation
- Start a tmux session named `clai_dev` with two panes
- Pane 0 runs the TUI
- Pane 1 watches for code changes and restarts the TUI automatically

To attach to the session:
```sh
tmux attach -t clai_dev
```

---

### One-liner Live Reload with entr (legacy)

For a single-terminal live reload workflow using [entr](https://eradman.com/entrproject/), run this command from your project root:

```sh
find . -type f -name '*.go' | grep -v '/_build/' | grep -v '/vendor/' | entr -r ./dev_entr.sh
```

- This will rebuild and restart the app automatically whenever any Go source file changes.
- The TUI will remain visible in your terminal.
- All logs will be written to `debug.log` (overwritten on each run).

---

**Alternative (for simple projects):**

If you want to watch only the Go files in your current directory, you can use:

```sh
ls *.go | entr ./dev_entr.sh
```

> **Note:**  
> Always run these commands in a real terminal window (not in the background or via a tool that captures output), so the TUI appears as expected.

---
