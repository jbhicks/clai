.PHONY: dev
# Live reload development
dev:
	@if ! command -v tmux >/dev/null 2>&1; then \
		echo "Error: tmux is not installed. Please install tmux to use the dev workflow."; \
		exit 1; \
	fi
	@if tmux has-session -t clai_dev 2>/dev/null; then \
		echo "Killing previous tmux session 'clai_dev'..."; \
		tmux kill-session -t clai_dev; \
	fi
	@tmux new-session -d -s clai_dev 'bash' \
		\; split-window -v 'tail -f debug.log' \
		\; select-pane -t 0
	@echo "Started tmux session 'clai_dev'."
	@echo "Top pane: shell prompt. Run './clai' manually, or let the watcher do it."
	@echo "Bottom pane: live logs."
	@echo "In a third terminal, run:"
	@echo "  ls internal/**/*.go cmd/clai/*.go | entr -r go build -o clai ./cmd/clai && tmux send-keys -t clai_dev:0.0 C-c './clai' Enter"
	@if [ -z "$$TMUX" ]; then \
		echo "Attaching to tmux session 'clai_dev'..."; \
		tmux attach -t clai_dev; \
	else \
		echo "tmux session 'clai_dev' started. Attach manually if needed."; \
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

all: build

build:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/clai

run:
	$(GORUN) ./cmd/clai

test:
	$(GOTEST) ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)


