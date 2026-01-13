# Background Refresh Performance Fix

## Problem

Initial page load was taking **5-6 seconds**, making the UI feel sluggish and unresponsive.

### Root Cause Analysis

The `/api/servers/list` endpoint (called on every page load) was doing synchronous I/O operations:

```go
func (s *Server) HandleListModels(w http.ResponseWriter, r *http.Request) {
    // ...
    s.modelManager.RefreshServerStatus()  // ❌ SLOW: 10 HTTP health checks (5 seconds worst case)
    s.modelManager.UpdateVRAMUsage()      // ❌ SLOW: rocm-smi command execution (100-500ms)
    // ...
}
```

**`RefreshServerStatus()`** performance:
- Loops through ports 8081-8090 (10 ports)
- Makes HTTP request to `http://localhost:PORT/v1/models` for each port
- Each request has 500ms timeout
- **Worst case:** 10 ports × 500ms = **5 seconds** (when no servers running)
- **Best case:** 10 ports × ~10ms = **100ms** (when all servers respond immediately)

**`UpdateVRAMUsage()`** performance:
- Executes external command: `rocm-smi --showpids`
- Parses output to get per-process VRAM usage
- **Typical time:** 100-500ms

**Total initial load time: 5-6 seconds worst case!**

This happened on:
- Initial page load (`/` → `/models` → `hx-get="/api/servers/list"`)
- Every manual refresh
- After Start/Stop/Delete actions (because handlers call `HandleListModels`)

## Solution: Background Refresh Pattern

Instead of refreshing on every request, use a **background goroutine** that refreshes cached data every 3 seconds.

### Implementation

#### 1. Added Background Refresh Fields to ModelManager

```go
type ModelManager struct {
    mu              sync.RWMutex
    servers         map[string]*ModelServer
    modelsDir       string
    llamaServerBin  string
    downloadManager *DownloadManager
    stopRefresh     chan struct{}  // ✅ Signal to stop background refresh
}
```

#### 2. Started Background Worker in Constructor

```go
func NewModelManager() *ModelManager {
    mm := &ModelManager{
        // ... initialization ...
        stopRefresh: make(chan struct{}),
    }
    
    // ✅ Start background refresh goroutine
    go mm.backgroundRefresh()
    
    return mm
}
```

#### 3. Background Refresh Goroutine

```go
func (mm *ModelManager) backgroundRefresh() {
    ticker := time.NewTicker(3 * time.Second)
    defer ticker.Stop()
    
    // Do an initial refresh immediately (on startup)
    mm.RefreshServerStatus()
    mm.UpdateVRAMUsage()
    
    for {
        select {
        case <-ticker.C:
            // Refresh every 3 seconds
            mm.RefreshServerStatus()
            mm.UpdateVRAMUsage()
        case <-mm.stopRefresh:
            // Graceful shutdown
            return
        }
    }
}
```

**Key Points:**
- Runs in background, doesn't block HTTP requests
- Refreshes every 3 seconds (adjustable)
- Does initial refresh on startup (so first request has data)
- Gracefully stops when `mm.Stop()` is called

#### 4. Removed Synchronous Refresh from Handler

**Before (SLOW):**
```go
func (s *Server) HandleListModels(w http.ResponseWriter, r *http.Request) {
    models, _ := s.modelManager.ScanAvailableModels()
    
    s.modelManager.RefreshServerStatus()  // ❌ 5 seconds!
    s.modelManager.UpdateVRAMUsage()      // ❌ 500ms!
    
    models = s.modelManager.GetServerStatus()
    // ... render HTML ...
}
```

**After (FAST):**
```go
func (s *Server) HandleListModels(w http.ResponseWriter, r *http.Request) {
    models, _ := s.modelManager.ScanAvailableModels()
    
    // ✅ Get cached status (refreshed by background goroutine)
    models = s.modelManager.GetServerStatus()
    
    // ... render HTML ...
}
```

### Performance Results

**Before:**
```bash
$ time curl -s http://localhost:8080/api/servers/list > /dev/null
real    0m5.234s   # ❌ 5+ seconds
```

**After:**
```bash
$ time curl -s http://localhost:8080/api/servers/list > /dev/null
real    0m0.004s   # ✅ 4 milliseconds
```

**Improvement: 1000x faster (5000ms → 4ms)**

### Files Modified

1. **`/home/josh/clai/internal/benchmark/model_manager.go`**
   - Line 46: Added `stopRefresh chan struct{}` field
   - Lines 48-64: Modified `NewModelManager()` to start background refresh
   - Lines 66-84: Added `backgroundRefresh()` method
   - Lines 86-89: Added `Stop()` method for graceful shutdown
   - Lines 636-643: Removed synchronous refresh calls from `HandleListModels()`

### Trade-offs

**Pros:**
- **Instant page loads** - No waiting for I/O operations
- **Better UX** - UI feels responsive and snappy
- **Consistent refresh rate** - Status updates every 3s regardless of user activity
- **Reduced server load** - Fewer redundant refresh calls (was refreshing on every request)

**Cons:**
- **Slight staleness** - Data can be up to 3 seconds old
  - **Mitigation:** 3 seconds is acceptable for status polling (users won't notice)
  - **Note:** HTMX already polls every 3s with `hx-trigger="every 3s"`, so this matches existing behavior
- **Resource usage** - Background goroutine runs continuously
  - **Mitigation:** Single goroutine with 3s sleep, minimal CPU/memory impact

### Why This Pattern Works

1. **Server status doesn't change frequently**
   - Servers start/stop based on user actions (not random)
   - Port health checks don't need to be real-time
   - 3-second staleness is imperceptible to users

2. **Matches existing polling behavior**
   - HTMX template already has `hx-trigger="load, every 3s"`
   - Background refresh interval (3s) aligns with UI polling (3s)
   - No additional latency from user's perspective

3. **Reduces redundant work**
   - Before: Every page load → 10 HTTP health checks
   - After: One background task → 10 HTTP health checks every 3s
   - If 10 users load page simultaneously, only 1 refresh happens (not 10)

### Alternative Approaches Considered

#### Option 1: Reduce Timeout (Not Chosen)
```go
client := &http.Client{Timeout: 50 * time.Millisecond} // Down from 500ms
```
**Problem:** Would cause false negatives for slow-responding servers

#### Option 2: Parallel Health Checks (Not Chosen)
```go
var wg sync.WaitGroup
for port := 8081; port <= 8090; port++ {
    wg.Add(1)
    go func(p int) {
        defer wg.Done()
        checkServerHealth(p)
    }(port)
}
wg.Wait()
```
**Problem:** Still blocks the HTTP request for 500ms (slowest port's timeout)

#### Option 3: Background Refresh (CHOSEN)
**Why:** Completely decouples I/O from HTTP request handling

### Future Improvements

1. **Event-based refresh triggers**
   - After Start/Stop/Delete actions, trigger immediate refresh
   - Don't wait 3 seconds for background worker
   ```go
   func (mm *ModelManager) TriggerRefresh() {
       go func() {
           mm.RefreshServerStatus()
           mm.UpdateVRAMUsage()
       }()
   }
   ```

2. **Adaptive polling**
   - Slow down refresh when no servers are running (e.g., 10s interval)
   - Speed up when activity is detected (e.g., 1s interval after Start/Stop)

3. **Server-Sent Events (SSE) for real-time updates**
   - Already implemented in templates (`hx-ext="sse"`)
   - Backend broadcasts status changes immediately
   - Could eliminate polling entirely

### Testing

**Manual Test:**
1. Open `http://localhost:8080/models` in browser
2. Page should load instantly (< 100ms)
3. Server status should update every 3 seconds (watch Running/Stopped status)

**Performance Test:**
```bash
# Test initial page load
time curl -s http://localhost:8080/ -o /dev/null

# Test models page
time curl -s http://localhost:8080/models -o /dev/null

# Test API endpoint
time curl -s http://localhost:8080/api/servers/list -o /dev/null
```

Expected results: All < 10ms

**Load Test:**
```bash
# Simulate 10 concurrent users
for i in {1..10}; do
    curl -s http://localhost:8080/api/servers/list > /dev/null &
done
wait
```

Expected: All requests complete in < 50ms (no blocking)

## Conclusion

By moving expensive I/O operations (HTTP health checks, rocm-smi command) to a background goroutine, we achieved a **1000x performance improvement** in page load times (5000ms → 4ms). The UI now feels instant and responsive, while still keeping server status up-to-date with 3-second polling.

This pattern follows best practices for high-performance web applications:
- **Never block HTTP requests on slow I/O**
- **Cache frequently-accessed data**
- **Use background workers for periodic updates**
- **Match refresh rate to user expectations** (3s is acceptable for status polling)
