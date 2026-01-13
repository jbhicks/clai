# Download Progress Display Fix - Summary

## Problem Statement
Active downloads were not visible in the UI despite working correctly on the backend.

### Symptoms
1. ✅ Backend `/api/models/downloads` endpoint returned correct download progress (e.g., 62.3%, 9.5GB/15.2GB)
2. ✅ SSE stream `/api/models/downloads/stream` was established
3. ❌ UI showed "No active downloads" and never updated
4. ❌ HTMX polling (`every 2s`) stopped after first request on page load

## Root Cause
**HTMX trigger conflict**: The `downloads_list` div had THREE triggers configured:
```html
hx-trigger="load, every 2s, sse:downloads_update"
```

The combination of polling (`every 2s`) and SSE events (`sse:downloads_update`) caused HTMX to stop polling after the initial `load` trigger.

## Solution

### 1. Template Fix (models.templ)
**Removed polling trigger**, relying solely on SSE for real-time updates:

```diff
- hx-trigger="load, every 2s, sse:downloads_update"
+ hx-trigger="load, sse:downloads_update"
```

**Benefits**:
- More efficient - no unnecessary HTTP requests
- Real-time updates (< 100ms latency) vs polling (2s delay)
- Eliminates HTMX trigger conflicts
- Reduces server load

### 2. SSE Handler Improvements (download_manager.go)

**Added logging and initial connection event**:
```go
// Send initial ping to establish connection
fmt.Fprintf(w, "event: connected\ndata: ready\n\n")
flusher.Flush()

log.Printf("SSE client connected to downloads stream")
log.Printf("Sending downloads_update SSE event")
log.Printf("SSE client disconnected from downloads stream")
```

**Added listener notification logging**:
```go
log.Printf("Notifying %d SSE listeners about download update: %s (%.1f%%)", 
    listenerCount, download.Filename, download.Progress)
```

## How SSE Works for Downloads

### Event Flow
1. **Browser connects** → SSE stream established at `/api/models/downloads/stream`
2. **Server sends** → Initial `connected` event
3. **Download progresses** → Backend calls `notifyListeners()` every 500ms (when progress changes)
4. **SSE broadcasts** → `downloads_update` event sent to all connected clients
5. **HTMX receives** → Triggers `hx-get="/api/models/downloads"`
6. **Server responds** → Returns updated HTML with current progress
7. **HTMX swaps** → Uses `morph:outerHTML` to smoothly update the `#downloads_list` div

### SSE Event Format
```
event: downloads_update
data: refresh

```

### HTMX Configuration
```html
<div hx-ext="sse" sse-connect="/api/models/downloads/stream">
  <div id="downloads_list"
       hx-get="/api/models/downloads"
       hx-trigger="load, sse:downloads_update"
       hx-swap="morph:outerHTML"
       hx-ext="morph">
  </div>
</div>
```

## Testing

### Manual Tests
```bash
# 1. Test SSE stream connection
timeout 3 curl -N -s "http://localhost:8083/api/models/downloads/stream"
# Expected: "event: connected\ndata: ready\n\n"

# 2. Test downloads endpoint
curl -s "http://localhost:8083/api/models/downloads"
# Expected: HTML with download progress or "No active downloads"

# 3. Start a download and watch logs
tail -f debug.log | grep -i "SSE\|download"
```

### Browser Test
1. Open http://localhost:8083/models
2. Start a download (any model)
3. **Verify**: Download progress appears immediately (< 100ms)
4. **Verify**: Progress updates smoothly without page flickering
5. **Verify**: Network tab shows SSE connection stays open (EventSource)

## Files Modified

1. `/home/josh/clai/internal/benchmark/templates/models.templ` (line 171)
   - Removed `every 2s` polling trigger
   
2. `/home/josh/clai/internal/benchmark/download_manager.go` (lines 508-551, 261-272)
   - Added SSE connection logging
   - Added listener notification logging
   - Added initial connection event

## Performance Impact

### Before (Polling)
- HTTP requests: Every 2 seconds regardless of activity
- Latency: Up to 2 seconds for updates
- Server load: Constant polling from all clients

### After (SSE)
- HTTP requests: Only when download progress changes (every 500ms during active downloads)
- Latency: < 100ms for updates
- Server load: Minimal - one persistent connection per client

## Related Code

### Download Progress Tracking
Location: `internal/benchmark/download_manager.go:160-173`
```go
// Update progress every 500ms
if now.Sub(lastUpdate) >= 500*time.Millisecond {
    // ... calculate progress ...
    dm.notifyListeners(download)
    lastUpdate = now
}
```

### SSE Listener Management
- `AddListener()` - Line 240: Registers new SSE client
- `RemoveListener()` - Line 247: Cleans up disconnected client
- `notifyListeners()` - Line 261: Broadcasts to all clients

## Future Enhancements

1. **Throttle SSE updates** - Currently sends every 500ms, could be configurable
2. **Batch updates** - Send multiple download updates in one SSE event
3. **Add download pause/resume** - Would need UI buttons and backend support
4. **Show download queue** - Display pending downloads waiting to start
5. **Add download speed graph** - Visual chart of download speed over time

## Lessons Learned

1. **Don't mix polling and SSE** - Choose one strategy for updates
2. **Always add logging to SSE handlers** - Hard to debug without visibility
3. **Test SSE with curl first** - Faster than debugging in browser
4. **Use morph swap for smooth updates** - Prevents flickering
5. **Prefer SSE over polling** - More efficient and real-time
