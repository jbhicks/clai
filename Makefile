dev:
	@if ! command -v entr >/dev/null 2>&1; then \
		echo "Error: entr is not installed."; \
		echo "Install with: sudo pacman -S entr (or your package manager)"; \
		exit 1; \
	fi
	@echo "Starting CLAI with entr auto-reload..."
	@echo "Logs will be in debug.log and benchmark.log"
	@echo "This provides auto-reload without tmux complexity"
	@echo "Press Ctrl+C to stop"
	@echo ""
	@echo "Watching Go files for changes..."
	@find . -name "*.go" -not -path "./vendor/*" | entr ./dev_restart.sh

 dev-clean:
	@echo "Cleaning up old development processes..."
	@pkill -f "inotifywait.*clai" 2>/dev/null || echo "No inotifywait processes found"
	@pkill -f "dev\.sh" 2>/dev/null || echo "No dev.sh processes found"
	@pkill -f "clai benchmark" 2>/dev/null || echo "No benchmark processes found"
	@for pid in $$(ps aux | grep "/clai" | grep -v "make" | grep -v "grep" | awk '{print $$2}'); do \
		kill -9 $$pid 2>/dev/null || true; \
	done
	@for session in $$(tmux ls -F "#{session_name}" 2>/dev/null | grep "^clai_dev" | head -10 || true); do \
		echo "Killing tmux session: $$session"; \
		tmux kill-session -t "$$session" 2>/dev/null || true; \
	done
	@rm -f /tmp/clai.sock 2>/dev/null || echo "No socket file found"
	@echo "Cleanup complete"

attach:
	@if ! command -v tmux >/dev/null 2>&1; then \
		echo "Error: tmux is not installed."; \
		exit 1; \
	fi
	@if tmux has-session -t clai_dev 2>/dev/null; then \
		echo "✓ Attaching to existing CLAI dev session..."; \
		tmux attach -t clai_dev 2>/dev/null || echo "Could not attach (not in interactive terminal)"; \
	else \
		echo "No CLAI dev session found. Start with: make dev"; \
		exit 1; \
	fi

#
# The benchmark server now runs automatically whenever CLAI starts.
# The web UI provides model management, benchmarking, and GPU monitoring.
# Use 'make dev' for development with live reload.

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GORUN=$(GOCMD) run
GOINSTALL=$(GOCMD) install
BINARY_NAME=clai
BINARY_UNIX=$(BINARY_NAME)

all: test lint build

build:
	$(eval BUILD_TIME := $(shell date -u +"%Y%m%d-%H%M%S"))
	$(eval BUILD_RAND := $(shell echo $$RANDOM))
	$(eval GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown"))
	$(eval BUILD_COUNT := $(shell git rev-list --count HEAD 2>/dev/null || echo "0"))
	$(GOBUILD) -ldflags "-X 'main.buildTime=$(BUILD_TIME)' -X 'main.gitCommit=$(GIT_COMMIT)' -X 'main.buildCount=$(BUILD_COUNT)' -X 'main.buildRand=$(BUILD_RAND)'" -o $(BINARY_NAME) ./cmd/clai

run:
	$(eval BUILD_TIME := $(shell date -u +"%Y%m%d-%H%M%S"))
	$(eval BUILD_RAND := $(shell echo $$RANDOM))
	$(eval GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown"))
	$(eval BUILD_COUNT := $(shell git rev-list --count HEAD 2>/dev/null || echo "0"))
	CLAI_DEV=1 $(GORUN) -ldflags "-X 'main.buildTime=$(BUILD_TIME)' -X 'main.gitCommit=$(GIT_COMMIT)' -X 'main.buildCount=$(BUILD_COUNT)' -X 'main.buildRand=$(BUILD_RAND)'" ./cmd/clai

run-simple:
	@echo "Running CLAI directly (no auto-reload)..."
	@echo "Logs will be in debug.log and benchmark.log"
	@echo "Press Ctrl+C to stop"
	@CLAI_DEV=1 make run

