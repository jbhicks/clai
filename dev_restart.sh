#!/bin/bash
# CLAI Development Watch Script using entr
# Simple version that entr calls to restart CLAI

# Clean up any existing processes
pkill -TERM -f "go run ./cmd/clai" 2>/dev/null || true
pkill -TERM -f "make run" 2>/dev/null || true
pkill -TERM -f "./clai benchmark" 2>/dev/null || true
for pid in $(ps aux | grep "/clai" | grep -v "grep" | awk '{print $2}'); do
    kill -TERM $pid 2>/dev/null || true
done

# Clean up socket
rm -f /tmp/clai.sock 2>/dev/null || true

# Truncate logs
truncate -s 0 debug.log 2>/dev/null || true
truncate -s 0 benchmark.log 2>/dev/null || true

# Reset terminal (just in case)
# Note: CLAI handles its own terminal setup, so minimal reset here
stty sane 2>/dev/null || true

echo "Rebuilding templates..."
templ generate || echo "Warning: templ generate failed, continuing..."

echo "Rebuilding and restarting CLAI..."

# Check if we should run web-only mode (for web UI development)
if [ "$CLAI_WEB_DEV" = "1" ]; then
    echo "Running in web-only development mode..."
    # Build first
    make build
    # Run benchmark server (web UI only, no TUI)
    ./clai benchmark &
else
    # Run full CLAI with TUI (default)
    make run-simple &
fi

CLAI_PID=$!

# Wait a moment for CLAI to start
sleep 3

# Verify server is running
if ! ss -tlnp | grep -q ":8080"; then
    echo "WARNING: Server may not have started on port 8080"
    echo "Check debug.log for errors"
fi

# Exit so entr can wait for next file change
# CLAI continues running in background
exit 0