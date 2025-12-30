# Systemd Integration & GPU Status SSE Fix Summary

## Issues Fixed

### 1. Models Auto-Restarting After Stop
**Problem**: When clicking "Stop" in the web UI, models would be killed but immediately restart.

**Root Cause**: Models were managed by systemd user services (`llama-server.service`, `llama-embed.service`) with auto-restart enabled. When clai killed the process with `process.Kill()`, systemd would immediately restart it.

**Solution**: Added systemd detection to `StopServer()` function:
- Checks if process is managed by systemd by reading `/proc/{pid}/cgroup`
- Extracts service name from cgroup path
- Uses `systemctl --user stop {service}` instead of `kill` for systemd-managed processes
- Falls back to `kill` if systemctl fails

**Files Modified**:
- `/home/josh/clai/internal/benchmark/model_manager.go`
  - Added `isSystemdManaged(pid int) bool` (line 343)
  - Added `getSystemdServiceName(pid int) string` (line 354)
  - Modified `StopServer()` to detect and stop systemd services (line 492-509)
  - Added `checkServerHealth(url string) bool` (line 245)
  - Added `findAvailablePortForModel() (int, error)` (line 256)

### 2. GPU Memory Not Updating After Stop
**Problem**: When stopping a model, the GPU memory usage didn't update until the next 3-second polling interval.

**Root Cause**: SSE connection element was a sibling to the GPU status div, but GPU status was listening for events `from:body`. HTMX SSE events are dispatched on the connection element itself and need parent-child relationship to propagate to listeners.

**Solution**: Restructured template to wrap content in SSE connection divs:
- Made SSE connection div a parent of both GPU status and servers list
- Removed `from:body` modifier from SSE triggers
- Separated server updates SSE from downloads SSE (different endpoints)

**Files Modified**:
- `/home/josh/clai/internal/benchmark/templates/models.templ`
  - Wrapped nav, GPU status, and servers list in SSE connection div (line 6)
  - Changed `hx-trigger` from `sse:refresh-servers from:body` to `sse:refresh-servers` (lines 17, 39)
  - Separated downloads into its own SSE wrapper (line 49)

- `/home/josh/clai/internal/benchmark/server.go`
  - Added logging to `broadcastServerUpdate()` for debugging (line 186-202)

### 3. Misleading Section Title
**Problem**: "Running Model Servers" title was inaccurate since the section shows both running and stopped models.

**Solution**: Changed title to "Model Servers" and updated description.

**Files Modified**:
- `/home/josh/clai/internal/benchmark/templates/models.templ`
  - Changed title from "Running Model Servers" to "Model Servers" (line 28)
  - Updated description from "Start and stop" to "Manage" (line 30)

## How It Works Now

### Stopping a Model:

1. **User clicks "Stop"** → POST to `/api/servers/stop`
2. **Handler detects systemd**:
   ```go
   if mm.isSystemdManaged(server.PID) {
       serviceName := mm.getSystemdServiceName(server.PID)
       cmd := exec.Command("systemctl", "--user", "stop", serviceName)
       cmd.Run()
   }
   ```
3. **Service is stopped** (systemd doesn't restart it)
4. **Handler returns** updated servers list HTML immediately
5. **Goroutine broadcasts** SSE `refresh-servers` event (~500ms later)
6. **Both divs update**:
   - `#gpu_status` receives `sse:refresh-servers` event
   - `#servers_list` receives `sse:refresh-servers` event
7. **GPU memory updates** within ~1 second

### SSE Event Flow:

```
Server Action (Start/Stop)
    ↓
HandleStartServer/HandleStopServer
    ↓
broadcastServerUpdate() → sends "refresh-servers" SSE event
    ↓
SSE connection div (parent) receives event
    ↓
Child divs with hx-trigger="sse:refresh-servers" trigger GET requests
    ↓
/api/gpu/status + /api/servers/list respond with fresh HTML
    ↓
HTMX morphs DOM with new content
```

## Testing

### Verify Systemd Detection:
```bash
# Check if process is managed by systemd
cat /proc/{PID}/cgroup
# Should show: .../llama-server.service or .../llama-embed.service

# Stop via web UI and check logs
# Should see: "Detected systemd-managed service: llama-server.service, using systemctl to stop"
```

### Verify SSE Updates:
```bash
# Open browser console, watch Network tab for SSE events
# Stop a model - should see:
# 1. Immediate server list update (from POST response)
# 2. SSE event "refresh-servers" after ~500ms
# 3. Two GET requests: /api/gpu/status + /api/servers/list
# 4. GPU memory decreases within 1 second
```

## Architecture Decisions

### Why Parent-Child SSE Structure?
HTMX SSE extension dispatches events on the element with `sse-connect`. Child elements can listen with `hx-trigger="sse:eventname"`. Using `from:body` requires events to bubble, which SSE events don't do by default.

### Why Separate SSE Connections?
Server updates (`/api/servers/events`) and download updates (`/api/models/downloads/stream`) are different event streams. Each needs its own SSE connection. We wrap each section in its own SSE div to keep concerns separated.

### Why Keep Polling + SSE?
- **SSE**: Provides immediate updates when user actions trigger changes
- **Polling (every 3s)**: Safety net for dropped SSE connections and external changes (other processes, thermal events, etc.)

This hybrid approach is more resilient than SSE-only or polling-only.

## Files Changed

1. `/home/josh/clai/internal/benchmark/model_manager.go` - Systemd detection
2. `/home/josh/clai/internal/benchmark/templates/models.templ` - SSE structure + title
3. `/home/josh/clai/internal/benchmark/server.go` - SSE broadcast logging

## Related Documentation

- HTMX SSE: [/bigskysoftware/htmx/v2.0.4 SSE extension](https://htmx.org/extensions/sse/)
- Systemd cgroups: `/proc/{pid}/cgroup` format
- AGENTS.md: Added "Preventing Flickering with Morph Swaps" section (HTMX best practices)
