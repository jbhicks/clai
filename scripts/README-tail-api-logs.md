# tail-api-logs

A `tail -f`-like script for streaming logs from CLAI model servers via the API SSE endpoint.

## Installation

```bash
# Copy to your PATH
cp scripts/tail-api-logs ~/bin/
chmod +x ~/bin/tail-api-logs

# Or use directly from repo
./scripts/tail-api-logs
```

## Usage

```bash
tail-api-logs [PORT] [OPTIONS]
```

### Basic Usage

```bash
# Tail logs from server on default port 8082
tail-api-logs

# Tail logs from server on port 8083
tail-api-logs 8083

# Show only errors
tail-api-logs 8082 --filter "ERROR\|error"

# Show last 5 minutes only
tail-api-logs 8082 --since 5m

# Don't auto-reconnect if connection drops
tail-api-logs 8082 --no-reconnect
```

## Options

| Option | Description |
|--------|-------------|
| `PORT` | Server port to tail logs from (default: 8082) |
| `-h, --help` | Show help message |
| `-f, --follow` | Follow log stream (default) |
| `-n, --no-follow` | Exit after current buffer |
| `--no-reconnect` | Don't auto-reconnect on disconnect |
| `--show-events` | Show SSE event lines |
| `--raw` | Show raw SSE output without parsing |
| `--filter REGEX` | Filter logs by regex pattern |
| `--host HOST` | API host (default: localhost) |
| `--api-port PORT` | API port (default: 8080) |
| `--since TIME` | Show logs since TIME (e.g., "5m", "1h") |

## Environment Variables

```bash
export CLAI_API_HOST=localhost    # API server hostname
export CLAI_API_PORT=8080         # API server port
```

## Features

- ✅ **Real-time streaming** - Follows logs as they arrive
- ✅ **Auto-reconnection** - Reconnects if connection drops
- ✅ **Colorized output** - Different colors for log levels
- ✅ **Smart filtering** - Filter by regex pattern
- ✅ **SSE parsing** - Handles Server-Sent Events properly
- ✅ **Timestamp highlighting** - Makes timestamps subtle
- ✅ **Log level detection** - Colors ERROR in red, WARN in yellow, etc.
- ✅ **Connection status** - Shows when connected/disconnected

## Examples

### Watch a specific model server
```bash
tail-api-logs 8082
```

### Monitor multiple servers (in different terminals)
```bash
# Terminal 1
tail-api-logs 8082

# Terminal 2  
tail-api-logs 8083
```

### Filter for specific events
```bash
# Only show errors and warnings
tail-api-logs 8082 --filter "ERROR\|WARN\|error\|warning"

# Only show model loading messages
tail-api-logs 8082 --filter "loading\|Loading\|LOADING"

# Only show port/bind messages
tail-api-logs 8082 --filter "port\|bind\|listening"
```

### Debug connection issues
```bash
# Show raw SSE output
tail-api-logs 8082 --raw

# Show SSE events
tail-api-logs 8082 --show-events

# Don't reconnect (useful for one-shot debugging)
tail-api-logs 8082 --no-reconnect
```

### Historical logs
```bash
# Last 5 minutes
tail-api-logs 8082 --since 5m

# Last hour
tail-api-logs 8082 --since 1h

# Since specific time
tail-api-logs 8082 --since "2024-02-08T10:00:00"
```

## Sample Output

```
Tailing logs from: http://localhost:8080/api/servers/logs?port=8082
Press Ctrl+C to stop

Server Info:
  Model: Qwen3-Coder-Next-Base.Q8_0
  Status: running

✓ Connected to log stream

10:30:15 [INFO] llama-server started
10:30:16 [INFO] loading model: Qwen3-Coder-Next-Base.Q8_0
10:30:45 [INFO] model loaded, ready
10:31:02 [INFO] srv  log_server_r: request: GET /health 127.0.0.1 200
10:31:15 [DEBUG] update_slots: all slots are idle
```

## Troubleshooting

### "Connection error: couldn't connect to host"
Make sure the CLAI API server is running:
```bash
curl http://localhost:8080/api/servers
```

### "Server disconnected"
The model server stopped. Check its status:
```bash
curl http://localhost:8080/api/servers/status?port=8082
```

### No logs appearing
The server might not be verbose mode. Check if started with `--verbose` flag.

### Filter not working
Make sure to escape special regex characters:
```bash
# Good
tail-api-logs 8082 --filter "port.*8082"

# Also good
tail-api-logs 8082 --filter "listening"
```

## How It Works

The script connects to the CLAI API SSE endpoint at:
```
http://localhost:8080/api/servers/logs?port=PORT
```

It uses `curl -N` (no-buffer) to stream Server-Sent Events in real-time, parsing the SSE format:
```
event: log
data: [timestamp] message

id: 123
data: next message
```

The script extracts the `data:` content, formats it with colors, and outputs it like `tail -f`.
