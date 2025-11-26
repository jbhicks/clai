.PHONY: dev
# Live reload development with entr (separate build/run processes)
dev:
	@if ! command -v entr >/dev/null 2>&1; then \
		echo "Error: entr is not installed."; \
		echo "Install with: sudo apt install entr (Debian/Ubuntu) or brew install entr (macOS)"; \
		exit 1; \
	fi
	@./dev.sh

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
	$(GOTEST) ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)


