# clai
An AI interface, built for local AI, by AI. AlrAIght?

...

## Command-Line Benchmarking

Run the full benchmark suite against your current model directly from the command line:

```sh
clai benchmark
```

**Details:**
- Uses the same LLM server configuration as the main clai application
- **Unified Benchmark Suite** (29 tests total):
  - **Model Benchmarks** (12 tests): Structured tests with predefined expectations
  - **Agentic Benchmarks** (17 tests): Natural language prompts testing autonomous tool usage
  - **Advanced Benchmarks** (5 tests): Challenging multi-step reasoning and safety scenarios
- Environment variables (same as clai):
    - `OLLAMA_HOST` (default: `http://localhost:8081`)
- CLI Options:
    - `--test N`: Run specific benchmark by index (0-23), shows detailed results
    - `--sequential`: Run tests sequentially (more stable)
    - No flags: Run all 24 benchmarks with summary output
- **Safety Features**:
    - Checks API health before starting
    - Prevents multiple benchmark processes from running simultaneously
    - Graceful shutdown on Ctrl+C
- **Important**: Stop the dev server (`go run ./cmd/clai`) before running benchmarks to avoid conflicts
- Runs benchmark tests and prints human-readable results.
- Results are also saved to the local database.

### Example
```
$ clai benchmark
================================================================================
MODEL BENCHMARK SUMMARY
================================================================================
✓ Extract Specific Value from File      0.78s  2 iter
✓ Count Files by Extension             1.12s  1 iter
✗ Mathematical Calculation             0.46s  1 iter  Wrong answer: expected 392
...
--------------------------------------------------------------------------------
TOTAL: 12 tests, 9 passed, 3 failed
Total time: 15.20s, Avg time: 1.27s
Total iterations: 18, Avg iterations: 1.5
Success rate: 75.0%
================================================================================
```

This feature does NOT launch the TUI or web server. If you want to run the interactive web server, use the appropriate TUI/serve command instead.

...
