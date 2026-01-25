# Benchmark Testing

This directory contains all benchmark-related functionality for the CLAI project.

## Structure

```
internal/benchmark/
├── assets/              # UI screenshots and visual assets
├── scripts/             # Helper scripts for running benchmarks
├── templates/           # HTML templates for web UI
├── server.go            # Main benchmark web server
├── runner.go            # Benchmark execution logic
├── model_manager.go      # Model download and management
├── download_manager.go  # File download utilities
└── *_test.go           # Test files for each component
```

## Usage

### Web Interface
```bash
clai benchmark
```

### Command Line Interface
```bash
# Run all benchmarks
clai benchmark --cli

# Run specific test by index
clai benchmark --cli --test 5

# List all available tests
clai benchmark --list
```

### Development Scripts

#### Interactive Test Runner
Located at `scripts/run_benchmarks_interactive.sh`
- Runs through all 21 benchmark tests interactively
- Allows skipping, rerunning, or stopping at any point
- Uses `make benchmark TEST=N` for each test

#### Development Watch Mode
Located at `scripts/dev_watch_benchmark.sh`
- Auto-restarts benchmark server on file changes
- Uses inotifywait for efficient file watching
- Handles graceful shutdowns and cleanup

## Test Suite

The unified benchmark suite contains 21 tests across three categories:

- **Core Tests (12)**: Basic functionality, file operations, math calculations
- **Advanced Tests (4)**: Multi-step analysis, error handling, data processing
- **Ultra-Challenging Tests (5)**: Complex algorithms, mathematical reasoning, architecture design

## Test Data

Test files are located in `internal/llm/`:
- `sample.txt` - Sample text file for file reading tests
- `test_data.json` - JSON data for data extraction tests

## Results

Benchmark results are stored in `model_test_results/`:
- HTML reports with detailed analysis
- Historical tracking of model performance
- Success rates and timing metrics

## Cleanup

Old benchmark results are automatically pruned to keep only the most recent test runs.