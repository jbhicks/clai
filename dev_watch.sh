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
    # Kill existing processes gracefully first, then forcefully
    echo "Stopping CLAI processes..."

    # Try graceful shutdown first (SIGTERM)
    pkill -TERM -f "go run ./cmd/clai" 2>/dev/null || true
    pkill -TERM -f "make run" 2>/dev/null || true
    for pid in $(ps aux | grep "/clai" | grep -v "grep" | awk '{print $2}'); do
        kill -TERM $pid 2>/dev/null || true
    done

    # Wait for graceful shutdown
    sleep 1

    # Force kill any remaining processes (SIGKILL)
    pkill -9 -f "go run ./cmd/clai" 2>/dev/null || true
    pkill -9 -f "make run" 2>/dev/null || true
    for pid in $(ps aux | grep "/clai" | grep -v "grep" | awk '{print $2}'); do
        kill -9 $pid 2>/dev/null || true
    done

    # Clean up any stale sockets
    rm -f /tmp/clai.sock 2>/dev/null || true

    # Reset terminal state (just in case)
    printf '\x1b[2J\x1b[H\x1b[?1049l' 2>/dev/null || true  # Clear screen, reset cursor, exit alt screen
    tput reset 2>/dev/null || true
    stty sane 2>/dev/null || true

    sleep 0.5

    # Start clai using make run (includes build ldflags)
    echo "Starting CLAI..."
    TERM=xterm-256color make run &
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
    
    # Check if make run is still running
    if ! pgrep -f "make run" > /dev/null 2>&1 && ! pgrep -f "/clai" > /dev/null 2>&1; then
        restart_app
    fi
done
