dev:
	@exec go run ./cmd/clai

clean:
	@echo "Cleaning up CLAI processes and debug artifacts..."
	@pkill -f "go run ./cmd/clai" 2>/dev/null || true
	@pkill -f chrome 2>/dev/null || true
	@pkill -f chromium 2>/dev/null || true
	@rm -f /tmp/clai.sock 2>/dev/null || true
	@echo "✓ Cleanup complete"
	$(GOCLEAN) -testcache

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=clai
BINARY_UNIX=$(BINARY_NAME)

all: test build

benchmark:
	@if [ -z "$(TEST)" ]; then \
		echo "Usage: make benchmark TEST=<test_number>"; \
		echo "Example: make benchmark TEST=1"; \
	else \
		go run ./cmd/clai benchmark --cli --test $(TEST); \
	fi
