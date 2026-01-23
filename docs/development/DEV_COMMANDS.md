# Development Commands Reference

This document explains the different development workflows available for clai.

## Quick Reference

| Command | Description | Best For |
|---------|-------------|----------|
| `make dev` | entr auto-reload (simple) | **Main TUI app development** (no tmux required) |
| `make dev-benchmark` | Air auto-restart with benchmark server | **Benchmark web interface development** |
| `make dev-air` | Alias for dev-benchmark | Same as above |
| `make dev-tmux` | Tmux split with air + logs | Multi-pane terminal workflow |

---

## `make dev` (Main TUI Auto-Reload)

**Best for:** Main clai TUI application development

This is the primary development command that uses `entr` (event notify test runner) to provide auto-reload functionality. When you save any Go file, CLAI automatically rebuilds and restarts.

### Features
- ✅ **Simple setup**: No tmux required, works in any terminal
- ✅ **Proper TTY handling**: Unlike air, entr correctly forwards terminal dimensions to Bubble Tea
- ✅ **Fast restarts**: Uses `entr` for reliable file watching and process management
- ✅ **Automatic cleanup**: Properly terminates old processes before starting new ones

### Usage
```bash
make dev
```

### What it does
1. Finds all `.go` files in the project (excluding vendor/)
2. Uses `entr` to watch for file changes
3. When a Go file changes, runs `dev_restart.sh` which:
   - Kills any existing CLAI processes
   - Cleans up stale sockets and logs
   - Rebuilds and starts CLAI in the background
4. `entr` waits for the next file change

### Requirements
- `entr` package must be installed (`sudo pacman -S entr` on Arch)

### When to use
- When you want the simplest auto-reload experience
- When working in IDEs or editors that don't play well with tmux
- For most CLAI development scenarios

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

## `make dev-entr` (Simple TUI Auto-Reload)

**Best for:** Main clai TUI application development without tmux complexity

This target uses `entr` (a simple file-watching utility) to provide auto-reload functionality without the tmux setup required by `make dev`. It's ideal when you want auto-reloading but prefer to work in a single terminal pane.

### Features
- ✅ **Proper TTY handling**: Unlike air, entr properly forwards terminal dimensions to Bubble Tea
- ✅ **Simple setup**: No tmux required, works in any terminal
- ✅ **Fast restarts**: Uses `entr -r` for clean process replacement
- ✅ **Automatic cleanup**: Properly terminates old processes before starting new ones

### Usage
```bash
make dev-entr
```

### What it does
1. Finds all `.go` files in the project (excluding vendor/)
2. Uses `entr` to watch for file changes
3. When a Go file changes, runs `dev_restart.sh` which:
   - Kills any existing CLAI processes
   - Cleans up stale sockets and logs
   - Resets terminal state
   - Starts CLAI with `make run-simple`

### Requirements
- `entr` package must be installed (`sudo pacman -S entr` on Arch)
- Same terminal environment as `make run-simple`

### When to use
- When you want auto-reload but don't need the tmux split with logs
- When working in IDEs or editors that don't play well with tmux
- When you prefer simpler terminal workflows

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

