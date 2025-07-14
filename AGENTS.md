# AGENTS.md

This repository is a Go CLI project for local AI agent interaction. Follow these guidelines for agentic coding:

## Build, Lint, and Test Commands
- Build: `make build` or `go build -o clai ./cmd/clai`
- Run: `make run` or `go run ./cmd/clai`
- Test all: `make test` or `go test ./...`
- Run a single test: `go test -run TestFunctionName ./...`
- Clean: `make clean`
- Install: `make install`
- Dev/debug: `make dev` (runs with DEBUG=true)

## Code Style Guidelines
- Use standard Go libraries; minimize dependencies.
- Group imports: stdlib first, then external.
- Format code with `gofmt`.
- Use clear, descriptive names for types, functions, and variables.
- Define tool schemas as Go structs for JSON marshalling.
- Handle errors gracefully (invalid JSON, tool failures, timeouts).
- Use Go doc comments for exported functions/types.
- Prefer explicit types and avoid unnecessary complexity.

## UI Component Guidelines
- Always use Charm Bubble components for all UI pieces.
- Do not write custom UI components unless absolutely necessary (e.g., when no Bubble component exists for your use case).
- If a custom component is required, document the reason in code comments and prefer extending Bubble components when possible.

## Testing
- Use Go's `testing` package for unit/integration tests.
- Mock external dependencies (e.g., Ollama) in tests.
- For Bubble Tea projects, follow the [Bubble Tea Agent Testing Strategy](BUBBLETEA_TESTING_STRATEGY.md) for all test creation. This document provides detailed guidelines and examples for unit, integration, and UI testing specific to Bubble Tea applications.

No Cursor or Copilot rules detected.

## Running Blocking Scripts
- Do **not** run blocking scripts (such as `dev_run.sh` or `dev_watch.sh`) directly in the foreground, as this will block the thread and prevent further interaction.
- If you need to run a blocking script, run it in the background (e.g., with `&` or in a separate terminal) and monitor log output separately (for example, using `tail -f debug.log`).
- Recommended workflow: Start the script in the background, then use a separate process or terminal to watch log output and interact with the application as needed.
