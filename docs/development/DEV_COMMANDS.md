# Development Commands Reference

This document explains the different development workflows available for clai.

## Quick Reference

| Command | Description | Best For |
|---------|-------------|----------|
| `make dev` | Custom inotifywait script | **Main TUI app development** (handles TTY properly) |
| `make dev-benchmark` | Air auto-restart with benchmark server | **Benchmark web interface development** |
| `make dev-air` | Alias for dev-benchmark | Same as above |
| `make dev-tmux` | Tmux split with air + logs | Multi-pane terminal workflow |

---

## `make dev` (Main TUI App)

**Best for:** Main clai TUI application development

⚠️ **Important:** The TUI (terminal user interface) has special TTY requirements. The custom `dev.sh` script handles these properly, unlike generic tools like air.

Uses a custom bash script (`dev.sh`) with `inotifywait`.

### Features:
- ✅ Proper TTY handling for Bubble Tea TUI
- ✅ Sets `AGENT_MODE=true` and `LOG_LEVEL=DEBUG`
- ✅ Redirects I/O to `/dev/tty` correctly
- ✅ Fast rebuild on file changes
- ✅ Graceful process cleanup

### Usage:
```bash
make dev
```

### What it does:
1. Watches for `.go` and `.env` file changes using `inotifywait`
2. On change: rebuilds `./clai`
3. Restarts with proper TTY connections: `./clai < /dev/tty > /dev/tty 2>&1`
4. Tracks PID for clean shutdown
5. Outputs to your current terminal

### Why not air for TUI?
The main clai app is a full-screen TUI built with Bubble Tea. It requires:
- Direct TTY access for keyboard input
- Proper terminal control for rendering
- Alt-screen buffer management
- Signal handling for cleanup

The custom `dev.sh` script ensures all of this works correctly during development.

---

## `make dev-benchmark` (Benchmark Server)

**Best for:** Developing the benchmark web interface

This is specifically configured to run `./clai benchmark` with auto-reload.

Uses [Air](https://github.com/cosmtrek/air) - a live-reload tool for Go applications.

### Features:
- ✅ Automatically runs `./clai benchmark` on start
- ✅ Auto-rebuilds on `.go`, `.templ`, `.html` file changes
- ✅ Automatic server restart after successful build
- ✅ Colored output (build/run/error states)
- ✅ Browser stays connected (just refresh after reload)
- ✅ Fast rebuild with configurable delay (1s)
- ✅ Excludes generated files (`*_templ.go`)

### Usage:
```bash
make dev-benchmark
# or
make dev-air
```

### What it does:
1. Watches all `.go` and `.templ` files
2. On file change: rebuilds `./tmp/clai`
3. Automatically restarts: `./tmp/clai benchmark`
4. Opens browser to the benchmark interface (usually http://localhost:8080)
5. Shows colored build/run status in terminal
6. Logs to `air.log`

### Example workflow:
```bash
# Terminal 1: Start benchmark server with auto-reload
make dev-benchmark

# Your browser opens to http://localhost:8080
# Edit files in internal/benchmark/*
# Air detects changes → rebuilds → restarts server
# Refresh browser to see changes
```

### Typical development cycle:
1. `make dev-benchmark` - starts server
2. Edit `internal/benchmark/templates/dashboard.templ`
3. Run `templ generate` (generates `*_templ.go` files)
4. Air detects `.templ` change → rebuilds
5. Server restarts automatically
6. Refresh browser to see changes

---

## `make dev-air`

Alias for `make dev-benchmark`. Same functionality.

---

## `make dev-tmux`

**Best for:** Multi-pane terminal workflow with logs visible

Creates a tmux session with:
- Left pane: App running under air
- Right pane: Live logs (`tail -f debug.log`)

### Features:
- ✅ Split-screen view (app + logs)
- ✅ Air auto-reload in left pane
- ✅ Live debug.log in right pane
- ✅ Session named `clai_dev`

### Usage:
```bash
make dev-tmux
```

### Tmux Controls:
- `Ctrl+b` then `arrow keys` - Switch panes
- `Ctrl+b` then `z` - Zoom current pane
- `Ctrl+b` then `d` - Detach session
- `tmux attach -t clai_dev` - Re-attach

---

## Other Development Commands

### Build
```bash
make build          # Build binary to ./clai
go build -o clai ./cmd/clai
```

### Run (no auto-reload)
```bash
make run            # Run without building binary
./clai              # Run pre-built binary
```

### Test
```bash
make test           # Run all tests
go test ./...       # Alternative

# Run specific test
go test -run TestFunctionName ./...

# Run with verbose output
go test -v ./...
```

### Clean
```bash
make clean          # Remove binary and build artifacts
```

### Benchmark Web Interface
```bash
./clai benchmark    # Start web-based benchmark interface
```

---

## File Watching Behavior

### What triggers a rebuild?

| Tool | Watches | Excludes |
|------|---------|----------|
| **dev.sh** (inotifywait) | `.go`, `.env` | None |
| **air** | `.go`, `.templ`, `.html` | `*_templ.go`, `tmp/`, `_build/`, `model_test_results/` |

### Templ Files
If you edit `.templ` files, run `templ generate` manually or air will detect the generated `*_templ.go` changes (but we exclude those to avoid double-rebuilds).

**Recommended workflow:**
1. Edit `.templ` file
2. Run `templ generate`
3. Air detects `.templ` change → rebuilds
4. Or manually rebuild if needed

---

## Debugging

### View logs
```bash
tail -f debug.log      # Live debug logs
cat debug.log          # Full debug log
```

### Debug server (while app is running)
```bash
./clai debug inspect         # Inspect UI state
./clai debug ping            # Test connectivity
./clai debug get_history     # Get conversation
```

See `docs/DEBUG_SERVER.md` for full debug protocol.

---

## Troubleshooting

### inotify-tools not installed (for dev.sh)
```bash
# Debian/Ubuntu
sudo apt install inotify-tools

# Arch
sudo pacman -S inotify-tools
```

### Air not installed
```bash
go install github.com/cosmtrek/air@latest
```

### tmux not installed
```bash
# Debian/Ubuntu
sudo apt install tmux

# Arch
sudo pacman -S tmux
```

### Air not restarting properly
- Check `air.log` for errors
- Increase `kill_delay` in `.air.toml`
- Ensure `send_interrupt = true` for graceful shutdown

### Build failing
```bash
# Check build manually
go build -o tmp/clai ./cmd/clai

# Check for syntax errors
go vet ./...

# Format code
go fmt ./...
```

### TUI not rendering correctly in air
Don't use `make dev-air` for the main TUI app. Use `make dev` instead, which properly handles TTY connections.

---

## Recommended Workflows

**For main TUI app development:**
```bash
make dev
```

**For benchmark web interface development:**
```bash
make dev-benchmark

# Edit templates:
# 1. Edit *.templ files in internal/benchmark/templates/
# 2. Run: templ generate
# 3. Air auto-detects change → rebuilds → restarts
# 4. Refresh browser
```

**For multi-pane development with logs:**
```bash
make dev-tmux
```

**If you need custom environment variables:**
Edit `dev.sh` or `.air.toml` to set them.

---

## See Also
- `AGENTS.md` - General development guidelines
- `docs/DEBUG_SERVER.md` - Debug server protocol
- `UI_GUIDE.md` - UI development guide
- `.air.toml` - Air configuration
- `dev.sh` - Custom dev script source

---

## Quick Start Examples

### Scenario 1: Working on the main TUI chat interface
```bash
# Terminal 1
make dev

# Edit files in internal/ui/, internal/llm/, etc.
# App auto-rebuilds and restarts
# Test your changes in the TUI
```

### Scenario 2: Building the benchmark web interface
```bash
# Terminal 1
make dev-benchmark

# Browser opens to http://localhost:8080 (or 8081, 8082... if 8080 is taken)
# Edit files in internal/benchmark/
# Edit templates in internal/benchmark/templates/*.templ
# Run: templ generate (to compile templates)
# Air detects changes → rebuilds → restarts server
# Refresh browser to see changes
```

### Scenario 3: Debugging with logs visible
```bash
# Creates split screen: app on left, logs on right
make dev-tmux

# Ctrl+b then arrow keys to switch panes
# Ctrl+b then z to zoom a pane
# Edit code and watch logs update in real-time
```

### Scenario 4: Just testing the benchmark server (no auto-reload)
```bash
# Build once
make build

# Run benchmark server
./clai benchmark

# Browser opens, manually restart when needed
```

---

## Port Management

The benchmark server automatically finds an available port in the 8080-8089 range:
- First tries 8080
- If busy (e.g., llama-server on 8081), tries 8082
- Continues until it finds a free port
- Prints the URL it's using (e.g., "Starting benchmark server on http://localhost:8083")

When using `make dev-benchmark`, the server will bind to a new port on each restart if the old one is still occupied.

