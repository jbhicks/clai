dev:
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
	@for pid in $$(ps aux | grep "/clai" | grep -v "make" | grep -v "grep" | awk '{print $$2}'); do \
		kill -9 $$pid 2>/dev/null || true; \
	done
	@tmux kill-session -t clai_dev 2>/dev/null || true
	@sleep 0.5
	@truncate -s 0 debug.log
	@tmux new-session -d -P -s clai_dev -x 120 -y 40 'clear && TERM=xterm-256color ./dev_watch.sh'
	@echo "✓ tmux session 'clai_dev' started with auto-reload (120x40)"
	@echo ""
	@echo "Trying to attach to tmux session..."
	@-tmux attach -t clai_dev 2>/dev/null || echo "Could not attach automatically (not in interactive terminal)"
	@echo ""
	@echo "Manual controls:"
	@echo "  tmux attach -t clai_dev   # Attach to session"
	@echo "  Ctrl+b then d              # Detach"
	@echo "  Ctrl+C                     # Stop"
	@echo ""
	@echo "Alternative: ./dev.sh       # Run directly (blocks terminal)"

 dev-clean:
	@echo "Cleaning up old development processes..."
	@pkill -f "inotifywait.*clai" 2>/dev/null || echo "No inotifywait processes found"
	@pkill -f "dev\.sh" 2>/dev/null || echo "No dev.sh processes found"
	@for pid in $$(ps aux | grep "/clai" | grep -v "make" | grep -v "grep" | awk '{print $$2}'); do \
		kill -9 $$pid 2>/dev/null || true; \
	done
	@for session in $$(tmux ls -F "#{session_name}" 2>/dev/null | grep "^clai_dev" | head -10 || true); do \
		echo "Killing tmux session: $$session"; \
		tmux kill-session -t "$$session" 2>/dev/null || true; \
	done
	@rm -f /tmp/clai.sock 2>/dev/null || echo "No socket file found"
	@echo "Cleanup complete"

#
# The benchmark server runs automatically whenever CLAI starts.
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
	$(GORUN) -ldflags "-X 'main.buildTime=$(BUILD_TIME)' -X 'main.gitCommit=$(GIT_COMMIT)' -X 'main.buildCount=$(BUILD_COUNT)' -X 'main.buildRand=$(BUILD_RAND)'" ./cmd/clai

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


