# Debug Server for clai

## Overview
The clai application now includes a Unix socket-based debug server that allows external tools to inspect and interact with the running application. This is particularly useful for debugging UI issues and understanding the application state.

## Architecture

### Components
1. **Debug Server** (`internal/ui/debug_server.go`): Unix socket server that listens on `/tmp/clai.sock`
2. **Debug Handler** (`internal/ui/model.go:handleDebugCommand`): Handles debug commands within the Bubble Tea event loop
3. **Debug Client** (`internal/debug/client.go`): Go library for sending commands to the debug server
4. **Debug CLI** (`cmd/clai/debug.go`): Subcommands accessible via `clai debug`

### How It Works
1. Server starts when the app launches (in `main.go`)
2. Accepts connections on `/tmp/clai.sock`
3. Receives JSON commands, forwards them to Bubble Tea as messages
4. Bubble Tea processes the message and sends JSON response back
5. Connection is closed after response is sent

## Available Commands

All commands are accessed via `clai debug COMMAND`.

### `ping`
Tests if the server is responsive.

```bash
clai debug ping
```

Output:
```
pong
```

### `inspect`
Captures the current UI state including viewport content, dimensions, and state information.

```bash
clai debug inspect
```

Returns:
- Terminal dimensions
- Chat pane dimensions  
- Viewport state (offset, height, total lines)
- Message count
- Active pane
- **Raw viewport content** (what the user actually sees, including ANSI codes)

### `get_history`
Retrieves all messages in the current conversation.

```bash
clai debug get_history
```

### `switch_pane`
Switches between chat and log panes.

```bash
clai debug switch_pane
```

## Usage for Debugging UI Issues

When debugging layout or rendering issues:

1. **Start the app** with `make dev`
2. **Run inspect** to see actual rendered output:
   ```bash
   clai debug inspect
   ```
3. **Analyze the output**:
   - Check if dimensions match expectations
   - Look for overflow (lines extending beyond expected width)
   - Verify ANSI escape codes are correctly rendered
   - Check viewport offset for scroll position

Example output:
```
================================================================================
DEBUG INSPECT OUTPUT
================================================================================
Terminal Size: 167x84
Chat Pane Size: 98x81
Viewport Height: 77
Viewport Offset: 42
Total Lines: 119
Message Count: 21
================================================================================
VIEWPORT CONTENT (what user sees):
================================================================================
[actual rendered content with ANSI codes]
================================================================================
```

## Implementation Details

### Thread Safety
- The server runs in its own goroutine
- Each connection is handled in a separate goroutine
- Commands are sent to Bubble Tea through its message system (thread-safe)
- Responses are sent synchronously before closing the connection

### Why This Approach?
- **No polling**: Uses Bubble Tea's message system
- **Accurate state**: Captures actual rendered output from viewport
- **Non-intrusive**: Doesn't interfere with normal app operation
- **Flexible**: Easy to add new commands

## Adding New Commands

To add a new command:

1. **Add case in `handleDebugCommand`** (`internal/ui/model.go`):
```go
case "my_command":
    resp = DebugResponse{
        Success: true,
        Data: map[string]interface{}{
            "result": someValue,
        },
    }
```

2. **Add handler in `cmd/clai/debug.go`** to expose the command via CLI

3. **Update usage text** in `runDebugCommand` function if needed

## Limitations

- Only one connection processed at a time (per design - sequential commands)
- Large viewport content may take time to transfer
- Socket must be cleaned up manually if app crashes (`rm /tmp/clai.sock`)
