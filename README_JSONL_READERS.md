# Benchmark JSONL Reader Tools

This directory contains tools for efficiently reading and analyzing benchmark conclusion data stored in JSONL (JSON Lines) format.

## Files

### 1. `read_conclusions.go` (Simple Version)
A lightweight, single-threaded JSONL reader perfect for small to medium files.

**Features:**
- Line-by-line streaming using `bufio.Scanner`
- Memory-efficient processing
- Basic statistics and analysis
- JSON export capability
- Simple error handling

**Usage:**
```bash
# Basic reading
go run read_conclusions.go benchmark_results/conclusions.jsonl

# With export
go run read_conclusions.go benchmark_results/conclusions.jsonl --export
```

### 2. `tools/advanced_conclusions_reader.go` (Advanced Version)
A high-performance, multi-threaded JSONL processor for large files with advanced analysis capabilities.

**Features:**
- Parallel processing with configurable workers
- Streaming architecture with context support
- Advanced filtering capabilities
- HTML and JSON report generation
- Detailed error tracking
- Progress monitoring for large files
- Memory-efficient processing with configurable buffers

**Usage:**
```bash
# Basic processing with 4 workers (default)
go run tools/advanced_conclusions_reader.go benchmark_results/conclusions.jsonl

# Export detailed reports
go run tools/advanced_conclusions_reader.go benchmark_results/conclusions.jsonl --export

# Filter by category and minimum score
go run tools/advanced_conclusions_reader.go benchmark_results/conclusions.jsonl category=basic min_score=90

# Use more workers for large files
go run tools/advanced_conclusions_reader.go large_file.jsonl --workers 8 --export
```

## JSONL File Format

The tools expect JSONL files with the following structure (one JSON object per line):

```json
{"test_id": "model_001", "test_name": "Basic Arithmetic Operations", "model_name": "llama3.1-gpu", "category": "basic", "performance": "excellent", "score": 95.5, "duration": "2.3s", "timestamp": "2026-01-24T10:30:00Z", "status": "passed", "conclusion": "Model performed exceptionally well..."}
```

### Required Fields:
- `test_id` (string): Unique identifier for the test
- `test_name` (string): Human-readable test name
- `model_name` (string): Name of the model tested
- `category` (string): Test category (basic, reasoning, coding, etc.)
- `performance` (string): Performance rating
- `score` (float64): Numerical score (0-100)
- `duration` (string): Test execution time
- `timestamp` (string): ISO 8601 timestamp
- `status` (string): Test status (passed, failed, etc.)
- `conclusion` (string): Text description of results

## Performance Characteristics

### Simple Reader (`read_conclusions.go`)
- **Memory Usage**: O(1) for streaming, O(n) if storing all records
- **Processing**: Single-threaded, line-by-line
- **Best For**: Files < 100MB, quick analysis
- **Speed**: ~10MB/s on typical hardware

### Advanced Reader (`tools/advanced_conclusions_reader.go`)
- **Memory Usage**: O(1) streaming, O(batch_size) for processing
- **Processing**: Multi-threaded with configurable workers
- **Best For**: Large files (>100MB), complex analysis, batch processing
- **Speed**: ~50MB/s with 4 workers on multi-core systems

## Filtering Options (Advanced Reader)

The advanced reader supports filtering with these parameters:

- `category=<string>`: Filter by test category
- `status=<string>`: Filter by test status
- `min_score=<float>`: Minimum score threshold
- `model_name=<string>`: Filter by model name

**Examples:**
```bash
# Filter for reasoning tests with scores above 80
go run tools/advanced_conclusions_reader.go data.jsonl category=reasoning min_score=80

# Filter by specific model
go run tools/advanced_conclusions_reader.go data.jsonl model_name=llama3.1-gpu
```

## Report Generation (Advanced Reader)

When using `--export`, the advanced reader generates:

1. **HTML Report** (`analysis_report.html`):
   - Interactive dashboard with charts
   - Responsive design for mobile/desktop
   - Color-coded performance indicators
   - Detailed breakdown tables

2. **JSON Report** (`analysis_summary.json`):
   - Machine-readable statistics
   - Structured data for further analysis
   - API-friendly format

## Error Handling

Both tools include robust error handling:

- **JSON parsing errors**: Logged with line numbers, processing continues
- **File access errors**: Immediate termination with clear error messages
- **Memory limits**: Configurable buffer sizes prevent OOM errors
- **Graceful degradation**: Partial results available even with some errors

## Examples

### Basic Analysis
```bash
$ go run read_conclusions.go benchmark_results/conclusions.jsonl
Reading benchmark conclusions from: benchmark_results/conclusions.jsonl
File read successfully in 117.701µs

Processed 5 benchmark conclusions

=== Summary Statistics ===
Average Score: 83.36

By Category:
  basic: 1
  reasoning: 1
  coding: 1
  analysis: 1
  advanced: 1

By Status:
  passed: 5
```

### Advanced Analysis with Export
```bash
$ go run tools/advanced_conclusions_reader.go benchmark_results/conclusions.jsonl --export
Processing benchmark conclusions from: benchmark_results/conclusions.jsonl
Using 4 parallel workers
File processed successfully in 181.06µs
Processed: 5 lines
Reports generated:
  HTML: benchmark_results/analysis_report.html
  JSON: benchmark_results/analysis_summary.json
```

## Performance Tips

1. **For small files (< 10MB)**: Use the simple reader
2. **For large files (> 100MB)**: Use the advanced reader with more workers
3. **For batch processing**: Use the advanced reader with `--export`
4. **Memory-constrained environments**: Reduce `--workers` count
5. **Slow storage**: Use fewer workers to avoid I/O contention

## Integration

Both tools can be easily integrated into CI/CD pipelines:

```bash
# In CI pipeline
go run tools/advanced_conclusions_reader.go latest_benchmarks.jsonl --export
# Upload analysis_report.html as artifact
# Fail build if average_score < threshold
```

## Dependencies

- Go 1.19+ (standard library only)
- No external dependencies
- Cross-platform compatible (Linux, macOS, Windows)

## License

These tools follow the same license as the main project.