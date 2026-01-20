#!/bin/bash
# CLAI Simple Development Runner
# Runs CLAI directly without tmux - blocks terminal but is simpler

set -e

echo "Starting CLAI in development mode..."
echo "Press Ctrl+C to stop"
echo ""

# Set terminal for proper display
export TERM=xterm-256color

# Run CLAI - will restart automatically if it exits
while true; do
    echo "Starting CLAI..."
    go run ./cmd/clai || {
        echo "CLAI exited with error. Restarting in 2 seconds..."
        sleep 2
    }
done