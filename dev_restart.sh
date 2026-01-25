#!/bin/bash
# CLAI Development restart script - handles cleanup and startup

echo "Restarting CLAI..."

# Clean up existing processes
pkill -f "go run ./cmd/clai" 2>/dev/null || true
rm -f /tmp/clai.sock 2>/dev/null || true

# Reset logs
truncate -s 0 debug.log benchmark.log 2>/dev/null || true

# Set environment
export CLAI_DEV=1

# Load environment variables from .env file
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
else
    echo "Warning: .env file not found"
fi

# Check if we're in a suitable terminal environment
# Check for TTY (interactive terminal) by trying to open /dev/tty
# Allow tmux sessions and development mode
if [ "$CLAI_DEV" != "1" ] && [ ! -t 0 ] && [ -z "$TMUX" ]; then
    echo "Warning: CLAI requires an interactive terminal (TTY). Consider using tmux:"
    echo "  tmux new-session -d -s clai-dev 'make dev-tmux'"
    echo "  tmux attach -t clai-dev"
    exit 1
fi

# Check if we're in tmux (interactive development) or regular entr (auto-reload)
if [ -n "$TMUX" ]; then
    # In tmux: Run CLAI in foreground
    echo "Running CLAI in tmux (foreground mode)..."
    go run -ldflags "-X main.buildTime=$(date -u +%Y%m%d-%H%M%S) -X main.gitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.buildCount=$(git rev-list --count HEAD 2>/dev/null || echo 0) -X main.buildRand=$RANDOM" ./cmd/clai
else
    # With entr: Run CLAI in foreground (entr handles restarts)
    echo "Running CLAI with entr auto-reload..."
    go run -ldflags "-X main.buildTime=$(date -u +%Y%m%d-%H%M%S) -X main.gitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.buildCount=$(git rev-list --count HEAD 2>/dev/null || echo 0) -X main.buildRand=$RANDOM" ./cmd/clai
fi