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
	@echo "Cleaning up any existing CLAI processes..."
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
	@tmux new-session -d -P -s clai_dev -x 120 -y 40 'clear && TERM=xterm-256color bash -c "while true; do go run ./cmd/clai; echo '\''CLAI exited, restarting in 2 seconds...'\''; sleep 2; done"'
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
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/clai

run:
	$(GORUN) ./cmd/clai

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


