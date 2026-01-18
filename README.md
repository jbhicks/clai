# clai
An AI interface, built for local AI, by AI. AlrAIght?

## Features

- **Local AI Integration**: Built for Ollama and OpenAI-compatible local LLM endpoints
- **Interactive TUI**: Modern terminal user interface built with Bubble Tea
- **Tool System**: Extensible tool calling system for LLM function execution
- **MCP Server**: Built-in Model Context Protocol server for external tool integration
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

## MCP Server

CLAI includes a built-in Model Context Protocol (MCP) server that allows external tools to inspect and interact with the running application. This enables integration with MCP-compatible clients and tools.

### Available Commands

- `clai debug ping` - Test connectivity to the debug server
- `clai debug inspect` - Capture current UI state and viewport content  
- `clai debug inspect_styles` - Get structured UI layout dimensions
- `clai debug get_history` - Retrieve conversation history
- `clai debug switch_pane` - Switch between chat and log panes

### Usage

The MCP server runs automatically when CLAI starts and listens on Unix socket `/tmp/clai.sock`. It's designed for debugging UI issues and understanding application state in real-time.

See [docs/DEBUG_SERVER.md](docs/DEBUG_SERVER.md) for detailed documentation.

## Development Viewport

CLAI is developed with a live-reload workflow. See **[AGENTS.md](AGENTS.md)** for the primary developer guidelines.

### Live Reload with make dev

The recommended workflow uses `make dev` which handles rebuilding and restarting the TUI automatically.

```sh
make dev
```

This uses `scripts/dev.sh` to watch for changes and restart the application in your terminal. Ensure you have `entr` or `inotify-tools` installed depending on your OS.

### tmux-based Development (Pane Layout)

Alternatively, you can run a tmux session with the app in one pane and logs in another:

```sh
make dev-tmux
```

### Logs & Debugging

Logs are generally written to the internal log pane (toggle with `Ctrl+T`). Advanced debugging can be done via the built-in MCP server.

---

For more details on implementation patterns, layout rules, and testing, see the **[Technical Guidelines Index](AGENTS.md#technical-guidelines-index)**.
