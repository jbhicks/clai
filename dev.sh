#!/bin/bash

# CLAI Development Script
# Provides live reload functionality for TUI development using inotifywait
# This handles TTY properly for Bubble Tea applications

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check dependencies
check_dependencies() {
    local missing_deps=()

    if ! command -v inotifywait >/dev/null 2>&1; then
        missing_deps+=("inotify-tools")
    fi

    if ! command -v tmux >/dev/null 2>&1; then
        missing_deps+=("tmux")
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        echo -e "${RED}Error: Missing required dependencies: ${missing_deps[*]}${NC}"
        echo -e "${YELLOW}Install with:"
        echo -e "  Ubuntu/Debian: sudo apt-get install ${missing_deps[*]}"
        echo -e "  macOS: brew install ${missing_deps[*]}"
        echo -e "  CentOS/RHEL: sudo yum install ${missing_deps[*]}${NC}"
        exit 1
    fi
}

# Check if already in a tmux session
check_tmux_session() {
    if [ -n "$TMUX" ]; then
        echo -e "${RED}Error: Already inside a tmux session. Run this script outside of tmux.${NC}"
        exit 1
    fi
}

# Build the application
build_app() {
    echo -e "${BLUE}Building CLAI...${NC}"
    if go build -o clai ./cmd/clai; then
        echo -e "${GREEN}✓ Build successful${NC}"
        return 0
    else
        echo -e "${RED}✗ Build failed${NC}"
        return 1
    fi
}

# Start the application in tmux
start_app() {
    local session_name="clai_dev_$$"

    echo -e "${BLUE}Starting tmux session: $session_name${NC}"

    # Create new detached tmux session with the app
    tmux new-session -d -s "$session_name" -x "$(tput cols)" -y "$(tput lines)" ./clai

    # Split window for logs
    tmux split-window -h -p 40 -t "$session_name"
    tmux send-keys -t "$session_name:0.1" "tail -f debug.log" Enter

    # Select the app pane
    tmux select-pane -t "$session_name:0.0"

    echo -e "${GREEN}✓ CLAI started in tmux session: $session_name${NC}"
    echo -e "${YELLOW}Commands:${NC}"
    echo -e "  Attach: ${BLUE}tmux attach -t $session_name${NC}"
    echo -e "  Detach: ${BLUE}Ctrl+b, then D${NC}"
    echo -e "  Kill: ${BLUE}tmux kill-session -t $session_name${NC}"
    echo -e "  Switch panes: ${BLUE}Ctrl+b, then arrow keys${NC}"

    # Attach to the session (always try if not already in tmux)
    if [ -z "$TMUX" ]; then
        echo -e "${GREEN}Attaching to tmux session...${NC}"
        if tmux attach -t "$session_name" 2>/dev/null; then
            # Successfully attached
            exit 0
        else
            echo -e "${RED}Failed to attach to tmux session${NC}"
            echo -e "${YELLOW}This usually means you're not in a proper terminal environment.${NC}"
        fi
    fi

    # If we get here, we couldn't attach - provide manual instructions
    echo -e "${YELLOW}The CLAI development session is running in the background.${NC}"
    echo -e "${YELLOW}To attach manually, run: ${BLUE}tmux attach -t $session_name${NC}"
    echo -e "${YELLOW}"
    echo -e "${YELLOW}The session is running in the background. You can:${NC}"
    echo -e "${YELLOW}  - Attach: tmux attach -t $session_name${NC}"
    echo -e "${YELLOW}  - Check status: tmux ls${NC}"
    echo -e "${YELLOW}  - Kill: tmux kill-session -t $session_name${NC}"
    echo -e "${YELLOW}"
    echo -e "${YELLOW}File changes will automatically rebuild and restart the app.${NC}"
    echo -e "${YELLOW}"
    echo -e "${YELLOW}To stop the development session: ${BLUE}tmux kill-session -t $session_name${NC}"
}

# Watch for file changes and rebuild
watch_files() {
    echo -e "${BLUE}Watching for file changes...${NC}"

    # Initial build
    if ! build_app; then
        echo -e "${RED}Initial build failed. Fix errors and try again.${NC}"
        exit 1
    fi

    # Start the app (this will create tmux session and try to attach)
    start_app

    # Clean up function
    cleanup() {
        echo -e "${YELLOW}Stopping development session...${NC}"
        kill "$app_pid" 2>/dev/null || true
        tmux kill-session -t "clai_dev_$$" 2>/dev/null || true
        exit 0
    }

    # Set up signal handlers
    trap cleanup SIGINT SIGTERM

    # Watch for Go file changes
    while true; do
        echo -e "${BLUE}Ready. Watching for changes...${NC}"

        # Wait for file changes
        if inotifywait -r -e modify,create,delete,move --include '\.go$' . 2>/dev/null; then
            echo -e "${YELLOW}File change detected. Rebuilding...${NC}"

            # Kill current app
            tmux send-keys -t "clai_dev_$$:0.0" C-c 2>/dev/null || true
            sleep 1

            # Rebuild
            if build_app; then
                # Restart app in tmux
                tmux send-keys -t "clai_dev_$$:0.0" "./clai" Enter
                echo -e "${GREEN}✓ App restarted${NC}"
            else
                echo -e "${RED}✗ Build failed. App not restarted.${NC}"
            fi
        fi
    done
}

# Main execution
main() {
    echo -e "${GREEN}CLAI Development Environment${NC}"
    echo -e "${BLUE}==============================${NC}"

    # Show usage if requested
    if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
        echo -e "${YELLOW}Usage: $0 [options]${NC}"
        echo -e "${YELLOW}Options:${NC}"
        echo -e "${YELLOW}  --build-only    Build once and exit${NC}"
        echo -e "${YELLOW}  --attach        Force tmux auto-attachment${NC}"
        echo -e "${YELLOW}  --help          Show this help${NC}"
        exit 0
    fi

    check_dependencies
    check_tmux_session

    # Check if we should just run once or watch
    if [ "$1" = "--build-only" ]; then
        build_app
        exit $?
    elif [ "$1" = "--attach" ]; then
        # Force attachment mode
        FORCE_ATTACH=true
        watch_files
    fi

    watch_files
}

main "$@"