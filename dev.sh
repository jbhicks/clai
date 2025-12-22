#!/bin/bash

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
    # Kill any remaining clai processes
    pkill -f "./clai" 2>/dev/null || true
    exit 0
}

trap cleanup INT TERM

rebuild_and_restart() {
    if [ -f "$PID_FILE" ]; then
        kill $(cat "$PID_FILE") 2>/dev/null || true
        sleep 0.1
    fi
    
    clear
    echo "Rebuilding..."
    if go build -o $BINARY ./cmd/clai 2>&1; then
        clear
        AGENT_MODE=true LOG_LEVEL=DEBUG ./$BINARY < /dev/tty > /dev/tty 2>&1 &
        local app_pid=$!
        echo $app_pid > "$PID_FILE"
        
        # Wait for app to exit
        wait $app_pid 2>/dev/null
        local exit_code=$?
        
        # If app exited normally (0) and not killed by us, user quit intentionally
        if [ $exit_code -eq 0 ]; then
            echo "App exited normally. Press Ctrl+C to stop watching, or save a file to restart."
        fi
    else
        echo "Build failed!"
        sleep 2
    fi
}

echo "=== clai development mode ==="
rebuild_and_restart

echo "Watching for changes..." > /dev/tty

# Watch for changes using inotifywait
while true; do
    inotifywait -r -e modify,create,delete --include '\.go$|\.env$' . >/dev/null 2>&1
    rebuild_and_restart
done
