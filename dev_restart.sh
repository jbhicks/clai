#!/bin/bash
# CLAI Development Watch Script - Simple and Reliable
# Does NOT use templ --watch to avoid temp file issues

echo "=================================================="
echo "CLAI Development Mode"
echo "=================================================="

# Cleanup function
cleanup() {
    echo ""
    echo "Shutting down..."
    pkill -TERM -f "./clai service" 2>/dev/null || true
    pkill -TERM -f "./clai" 2>/dev/null || true
    rm -f /tmp/clai.sock 2>/dev/null || true
    exit 0
}

trap cleanup SIGINT SIGTERM

# Check if we should run TUI mode
if [ "$CLAI_TUI_DEV" = "1" ]; then
    echo "Mode: TUI (screen will clear)"
    echo "Press Ctrl+C to stop"
    echo "=================================================="
    echo ""
    
    while true; do
        # Generate templates (NO --watch to avoid temp file issues)
        templ generate 2>&1 | grep -v "Processing file\|File not updated" || true
        
        # Build
        echo "Building..."
        if go build -o clai ./cmd/clai 2>&1; then
            echo "Starting TUI..."
            CLAI_DEV=1 ./clai
            echo ""
            echo "TUI exited. Restarting in 2s..."
        else
            echo "Build failed. Fix errors and save to retry."
        fi
        
        # Wait for file changes using inotifywait
        echo "Waiting for changes..."
        inotifywait -e modify,move,create,delete -r . --include '.*\.(go|templ)$' 2>/dev/null || sleep 2
        echo ""
        echo "Changes detected, rebuilding..."
        echo "=================================================="
    done
else
    echo "Mode: Service (Web UI on http://localhost:8080)"
    echo "Logs will appear below"
    echo "Press Ctrl+C to stop"
    echo "=================================================="
    echo ""
    
    while true; do
        # Generate templates (NO --watch to avoid temp file issues)
        templ generate 2>&1 | grep -v "Processing file\|File not updated" || true
        
        # Build
        echo "Building..."
        if go build -o clai ./cmd/clai 2>&1; then
            echo "Starting service..."
            echo ""
            ./clai service &
            SERVICE_PID=$!
            
            # Wait for file changes or service exit
            inotifywait -e modify,move,create,delete -r . --include '.*\.(go|templ)$' 2>/dev/null
            
            # Kill service
            kill $SERVICE_PID 2>/dev/null || true
            wait $SERVICE_PID 2>/dev/null || true
            
            echo ""
            echo "Changes detected, restarting..."
            echo "=================================================="
            echo ""
        else
            echo "Build failed. Fix errors and save to retry."
            inotifywait -e modify,move,create,delete -r . --include '.*\.(go|templ)$' 2>/dev/null
        fi
    done
fi
