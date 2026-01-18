# API and Concurrency Guidelines

This document outlines the rules for network communication, model server configuration, and concurrency management in CLAI.

## Concurrency and Mutexes

### Never Hold Locks During I/O
**CRITICAL**: Holding mutex locks while performing I/O operations (HTTP requests, file operations, external commands) causes deadlocks and severe performance issues.

**✅ CORRECT Pattern - Gather Data First, Lock Briefly:**
1. Do all I/O operations first (without locks).
2. Collect results in local variables.
3. Acquire lock briefly to update shared state.
4. Release lock.

```go
func (mm *ModelManager) Refresh() {
    // 1. Gather data
    resp, _ := http.Get(url)
    
    // 2. Lock briefly
    mm.mu.Lock()
    defer mm.mu.Unlock()
    mm.status = resp.Status
}
```

## HTTP and SSE API Interactions

### Never Fetch SSE Endpoints in the Foreground
SSE endpoints (like `/api/servers/events`) are designed for real-time streaming. Attempting to curl/fetch them in the foreground will BLOCK THE ENTIRE THREAD INDEFINITELY.
- Use browser inspection or background redirection: `curl -s <url> > /tmp/output.txt &`

### Handle Socket Disconnections Gracefully
Large models can cause socket timeouts. Always handle potential socket errors in fetch operations and consider retry logic.
- Add timeout handling: `http.Client{Timeout: 30 * time.Second}`
- Use `verbose: true` for debugging socket issues.

## Model Server Configuration

### llama.cpp vs Ollama
- **llama.cpp**: The primary server for LLM operations.
- **Ollama**: DO NOT attempt to install or run Ollama; use the existing llama.cpp server.
- The server supports OpenAI-compatible API formats.

### Use bufio.Scanner
Use `bufio.Scanner` for efficient line-by-line file reading to minimize memory overhead.
