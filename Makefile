dev:
	@if ! command -v tmux >/dev/null 2>&1; then \
		echo "Error: tmux is not installed."; \
		exit 1; \
	fi
	@if tmux has-session -t clai_dev 2>/dev/null; then \
		echo "Existing clai_dev session found. Killing it..."; \
		tmux kill-session -t clai_dev 2>/dev/null; \
	fi
	@echo "Starting CLAI in tmux..."
	@echo "Note: After making code changes, press Ctrl+C then Up+Enter to rebuild"
	@echo ""
	@truncate -s 0 debug.log
	@tmux new-session -d -s clai_dev -x $$(tput cols) -y $$(tput lines) 'clear && TERM=xterm-256color go run ./cmd/clai'
	@echo "✓ tmux session 'clai_dev' started"
	@echo ""
	@echo "Controls:"
	@echo "  Ctrl+b then d - Detach"
	@echo "  Ctrl+C then up-arrow + enter - Rebuild and restart"
	@echo ""
	@if [ -z "$$TMUX" ]; then \
		echo "Attaching to tmux session..."; \
		tmux attach -t clai_dev; \
	else \
		echo "Already in tmux. Attach with: tmux attach -t clai_dev"; \
	fi

dev-clean:
	@echo "Cleaning up old development processes..."
	@pkill -f "inotifywait.*clai" 2>/dev/null || echo "No inotifywait processes found"
	@pkill -f "dev\.sh" 2>/dev/null || echo "No dev.sh processes found"
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


