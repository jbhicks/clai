#!/bin/bash
# CLAI Benchmark Development Watch Script
# Uses inotifywait to watch for Go file changes and auto-restart the benchmark command

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
    echo "Stopping CLAI benchmark processes..."

    # Try graceful shutdown first (SIGTERM)
    pkill -TERM -f "go run ./cmd/clai benchmark" 2>/dev/null || true
    pkill -TERM -f "make run-benchmark" 2>/dev/null || true
    for pid in $(ps aux | grep "/clai.*benchmark" | grep -v "grep" | awk '{print $2}'); do
        kill -TERM $pid 2>/dev/null || true
    done

    # Wait for graceful shutdown
    sleep 1

    # Force kill any remaining processes (SIGKILL)
    pkill -9 -f "go run ./cmd/clai benchmark" 2>/dev/null || true
    pkill -9 -f "make run-benchmark" 2>/dev/null || true
    for pid in $(ps aux | grep "/clai.*benchmark" | grep -v "grep" | awk '{print $2}'); do
        kill -9 $pid 2>/dev/null || true
    done

    # Clean up any stale sockets and lock files
    rm -f /tmp/clai.sock 2>/dev/null || true
    rm -f /tmp/clai-benchmark.lock 2>/dev/null || true

    sleep 0.5

    # Start clai benchmark using make run (includes build ldflags)
    echo "Starting CLAI benchmark..."
    TERM=xterm-256color make run ARGS="benchmark" &
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
    if ! pgrep -f "make run.*benchmark" > /dev/null 2>&1 && ! pgrep -f "/clai.*benchmark" > /dev/null 2>&1; then
        restart_app
    fi
done