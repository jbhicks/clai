.PHONY: dev
# Live reload development with inotifywait (handles TTY properly for TUI)
dev:
	@./dev.sh

.PHONY: dev-benchmark
# Live reload for benchmark server development with Templ templates
# Air automatically runs 'templ generate' before each build (see .air.toml pre_cmd)
dev-benchmark:
	@if ! command -v air >/dev/null 2>&1; then \
		echo "Error: air is not installed."; \
		echo "Install with: go install github.com/cosmtrek/air@latest"; \
		exit 1; \
	fi
	@if ! command -v templ >/dev/null 2>&1; then \
		echo "Error: templ is not installed."; \
		echo "Install with: go install github.com/a-h/templ/cmd/templ@latest"; \
		exit 1; \
	fi
	@echo "Starting benchmark server with live reload..."
	@echo "Air will auto-restart when you edit .go or .templ files."
	@echo "Templates are auto-compiled via 'templ generate' before each build."
	@echo "Press Ctrl+C to stop."
	@air

.PHONY: dev-air
# Live reload development with air (for non-TUI commands like benchmark server)
# Alias for dev-benchmark
dev-air: dev-benchmark

.PHONY: dev-tmux
# Alternative: tmux-based development (old method)
dev-tmux:
	@if ! command -v tmux >/dev/null 2>&1; then \
		echo "Error: tmux is not installed."; \
		exit 1; \
	fi
	@if tmux has-session -t clai_dev 2>/dev/null; then \
		tmux kill-session -t clai_dev; \
	fi
	@echo "Starting tmux session with air..."
	@tmux new-session -d -s clai_dev -x $$(tput cols) -y $$(tput lines) 'air' \
		\; split-window -h -p 40 'tail -f debug.log' \
		\; select-pane -t 0
	@echo "✓ 2-pane tmux session started"
	@echo "  Left: App running under air (auto-reload)"
	@echo "  Right: Live logs"
	@echo ""
	@echo "Tmux commands:"
	@echo "  Ctrl+b then arrow keys - Switch panes"
	@echo "  Ctrl+b then z - Toggle zoom"
	@echo "  Ctrl+b then d - Detach"
	@if [ -z "$$TMUX" ]; then \
		tmux attach -t clai_dev; \
	else \
		echo "Attach with: tmux attach -t clai_dev"; \
	fi
# Makefile for the clai project
#
# Additional targets:
# minimal_testing_air: Runs minimal_testing.go with air for live prototyping

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GORUN=$(GOCMD) run
GOINSTALL=$(GOCMD) install
BINARY_NAME=clai
BINARY_UNIX=$(BINARY_NAME)

all: test build

build:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/clai

run:
	$(GORUN) ./cmd/clai

test:
	$(GOTEST) ./cmd/... ./internal/...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)


