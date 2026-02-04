#!/bin/bash

export MODELS_PATH=/mnt/media/models

BINARY="clai"
PID_FILE="/tmp/clai_dev.pid"
BUILD_TRIGGER="/tmp/clai_rebuild_trigger"

cleanup() {
    echo "Cleaning up..." > /dev/tty
    if [ -f "$PID_FILE" ]; then
        kill $(cat "$PID_FILE") 2>/dev/null || true
        rm -f "$PID_FILE"
    fi
    rm -f "$BUILD_TRIGGER"
    # Kill file watcher
    [ -n "$WATCHER_PID" ] && kill $WATCHER_PID 2>/dev/null || true
    # Kill any remaining clai processes
    pkill -f "./clai" 2>/dev/null || true
    exit 0
}

trap cleanup INT TERM

rebuild_and_restart() {
    if [ -f "$PID_FILE" ]; then
        kill $(cat "$PID_FILE") 2>/dev/null || true
        sleep 0.2
    fi
    
    clear
    echo "Rebuilding..."
    if go build -o $BINARY ./cmd/clai 2>&1; then
        clear
        AGENT_MODE=true LOG_LEVEL=DEBUG MODELS_PATH=$MODELS_PATH ./$BINARY 2>&1 &
        local app_pid=$!
        echo $app_pid > "$PID_FILE"
    else
        echo "Build failed!"
        sleep 2
    fi
}

echo "=== clai development mode ==="

# Start file watcher in background before starting app
echo "Watching for changes..." > /dev/null
inotifywait -r -m -e modify,create,delete --include '\.go$|\.env$' . 2>/dev/null | while read path action file; do
    touch "$BUILD_TRIGGER"
done &
WATCHER_PID=$!

# Initial build and start
rebuild_and_restart

# Monitor for rebuild triggers
while true; do
    if [ -f "$BUILD_TRIGGER" ]; then
        rm -f "$BUILD_TRIGGER"
        sleep 0.2  # Debounce rapid file changes
        rebuild_and_restart
    fi
    sleep 0.5
done
