#!/bin/bash
# CLAI Development Watch Script
# Uses inotifywait to watch for Go file changes and auto-restart the app

APP_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$APP_DIR"

# Check for inotify-tools
if ! command -v inotifywait >/dev/null 2>&1; then
    echo "Error: inotifywait not installed" >&2
    echo "Install with: sudo apt-get install inotify-tools" >&2
    exit 1
fi

restart_app() {
    # Kill any existing clai processes
    pkill -9 -f "go run ./cmd/clai" 2>/dev/null || true
    sleep 0.2
    
    # Start clai in background (within the tmux session which has TTY)
    TERM=xterm-256color go run ./cmd/clai &
}

# Initial start
restart_app

# Main loop - check for file changes
while true; do
    inotifywait -t 2 -e modify,create,delete,move \
        --exclude '(\.git|tmp|_build|vendor|node_modules|\.opencode|\.clai|model_test_results|archive|debug\.log)' \
        -r . >/dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        restart_app
    fi
    
    # Check if go is still running
    if ! pgrep -f "go run ./cmd/clai" > /dev/null 2>&1; then
        restart_app
    fi
done
