# clai

clai is a local-first AI chat client written in Go. It talks to an OpenAI-compatible llama.cpp server, exposes a Charm Bubble Tea TUI by default, and can also run an HTTP service with a web UI for model management and benchmarking.

## Requirements

- Go 1.24 or later
- A local LLM server at `OLLAMA_HOST` (default `http://localhost:8081`)
- [`templ`](https://github.com/a-h/templ) for the web UI: `go install github.com/a-h/templ/cmd/templ@latest`

## Build and run

```sh
make build          # go build -o clai ./cmd/clai
./clai              # TUI (default)
./clai service      # HTTP service / web UI
go build -o clai ./cmd/clai
go run ./cmd/clai
```

Live-reload development (web UI at http://localhost:8080):

```sh
make dev            # web UI with auto-reload (needs inotifywait)
make dev-tui        # TUI with auto-reload
make run            # CLAI_DEV=1 go run ./cmd/clai
```

## Commands

- `clai` / `clai tui` -- interactive terminal UI
- `clai service` -- HTTP service and web UI
- `clai ask` -- one-shot prompt
- `clai models list` -- list downloaded GGUF models
- `clai benchmark --cli` -- run the local benchmark suite (`make benchmark`)
- `clai stop` -- stop a running instance
- `clai debug` -- inspect TUI state

```sh
go test ./...
```

## Configuration

- `OLLAMA_HOST` -- LLM base URL (default `http://localhost:8081`)
- `OLLAMA_MODEL` -- model name (default `llama3.1-gpu:latest`)
- `MODELS_DIR` -- GGUF directory (default `$HOME/models`)

Env names keep an Ollama-style prefix. The runtime is a llama.cpp-compatible OpenAI chat API, not Ollama itself.

## License

MIT. See [LICENSE](LICENSE).
