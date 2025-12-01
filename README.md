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
