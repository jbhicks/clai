dev:
	@if [ ! -t 0 ]; then \
		echo "Error: make dev must be run from an interactive terminal."; \
		exit 1; \
	fi
	@if ! command -v tmux >/dev/null 2>&1; then \
		echo "Error: tmux is not installed."; \
		exit 1; \
	fi
	@if ! command -v inotifywait >/dev/null 2>&1; then \
		echo "Error: inotifywait not installed."; \
		echo "Install with: sudo apt-get install inotify-tools"; \
		exit 1; \
	fi
	@echo "Checking for existing CLAI session..."
	@if tmux has-session -t clai_dev 2>/dev/null; then \
		echo "✓ Session 'clai_dev' already running - attaching..."; \
		tmux attach -t clai_dev 2>/dev/null || echo "Could not attach (not in interactive terminal)"; \
		echo "To restart: tmux kill-session -t clai_dev && make dev"; \
		exit 0; \
	fi
	@echo "No existing session found. Starting new dev environment..."
	@for pid in $$(ps aux | grep "[g]o run ./cmd/clai" | awk '{print $$2}'); do \
		kill -9 $$pid 2>/dev/null || true; \
	done
	@for pid in $$(ps aux | grep "[d]ev_watch.sh" | awk '{print $$2}'); do \
		kill -9 $$pid 2>/dev/null || true; \
	done
	@for pid in $$(ps aux | grep "[c]lai benchmark" | awk '{print $$2}'); do \
		kill -9 $$pid 2>/dev/null || true; \
	done
	@for pid in $$(ps aux | grep "/clai" | grep -v "make" | grep -v "grep" | awk '{print $$2}'); do \
		kill -9 $$pid 2>/dev/null || true; \
	done
	@tmux kill-session -t clai_dev 2>/dev/null || true
	@sleep 0.5
	@truncate -s 0 debug.log
	@truncate -s 0 benchmark.log
	@tmux new-session -s clai_dev -x 120 -y 40 \; split-window -h \; send-keys -t clai_dev.0 'TERM=xterm-256color ./dev_watch.sh' C-m \; send-keys -t clai_dev.1 'tail -f debug.log' C-m \; select-pane -t clai_dev.0 \; resize-pane -t clai_dev.1 -x 36
	@echo "✓ tmux session 'clai_dev' started with vertical split (120x40)"
	@echo "  - CLAI TUI (left pane, 70% width)"
	@echo "  - Debug logs (right pane, 30% width)"
	@echo "  - Benchmark web UI (background, available on port 8080+)"
	@echo "  - Logs: debug.log (TUI), benchmark.log (web UI)"
	@echo ""
	@echo "Manual controls:"
	@echo "  Ctrl+b then d              # Detach from tmux"
	@echo "  Ctrl+b then left/right     # Switch panes"
	@echo "  Ctrl+b then z              # Zoom current pane"
	@echo "  Ctrl+b then :kill-session  # Kill entire session (recommended)"
	@echo "  tmux kill-session -t clai_dev  # Kill from outside tmux"
	@echo "  tail -f benchmark.log      # View benchmark server logs"
	@echo ""
	@echo "Alternative: ./dev.sh       # Run directly (blocks terminal)"

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

test:
	$(GOTEST) ./cmd/... ./internal/...

lint:
	$(GOCMD) vet ./cmd/... ./internal/...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./cmd/... ./internal/...; \
	else \
		echo "golangci-lint not found, using go vet only"; \
	fi

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

benchmark:
	$(eval TEST ?= 0)
	make build
	./clai benchmark --cli --test $(TEST)


dev-tmux:
	@if ! command -v tmux >/dev/null 2>&1; then \
		echo "Error: tmux is not installed."; \
		echo "Install with: sudo apt-get install tmux"; \
		exit 1; \
	fi
	@if ! command -v air >/dev/null 2>&1; then \
		echo "Error: air is not installed."; \
		echo "Install with: go install github.com/cosmtrek/air@latest"; \
		exit 1; \
	fi
	@tmux new-session -d -s clai_dev_air -x 120 -y 40
	@tmux split-window -h -t clai_dev_air
	@tmux send-keys -t clai_dev_air.0 'air' C-m
	@tmux send-keys -t clai_dev_air.1 'tail -f debug.log' C-m
	@tmux select-pane -t clai_dev_air.0
	@echo "✓ tmux session 'clai_dev_air' started with air on left, logs on right (120x40)"
	@echo ""
	@echo "Controls:"
	@echo "  Ctrl+b then arrow keys - Switch panes"
	@echo "  Ctrl+b then z          - Zoom current pane"
	@echo "  Ctrl+b then d          - Detach session"
	@echo "  tmux attach -t clai_dev_air - Re-attach"
	@echo "  tmux kill-session -t clai_dev_air - Stop"

