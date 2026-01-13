# SSE Real-Time Benchmark Updates - Verification Guide

## What Should Happen

When a benchmark runs, the Testing page should update **live after each test completes** (not just at the end):

1. **Toast notifications** appear: "📊 Test completed - updating results..."
2. **Results table** updates showing incremental pass/fail counts
3. **No page refresh** required
4. **Updates every ~few seconds** as tests complete

## How to Verify

### 1. Check Server Logs for SSE Activity

```bash
# Watch logs in real-time
tail -f /home/josh/clai/tmp/air.log

# Or check recent logs
tail -n 100 /home/josh/clai/tmp/air.log | grep -E "(SSE|benchmark-update)"
```

**Expected log output during benchmark:**
```
SSE client connected, total clients: 1
Broadcasting benchmark-update event to 1 SSE clients
Sent benchmark-update event to client
  ✅ Simple Math: Validation passed (1234ms)
Broadcasting benchmark-update event to 1 SSE clients
Sent benchmark-update event to client
  ✅ String Manipulation: Validation passed (987ms)
...
```

### 2. Test SSE Connection Manually

```bash
# Connect to SSE endpoint (should stay open)
curl -N http://localhost:8080/api/servers/events

# This will hang - that's expected! SSE keeps the connection open.
# Press Ctrl+C to stop
```

**Expected:** Connection stays open indefinitely (until you cancel)

### 3. Verify in Browser

1. **Open Testing page:** http://localhost:8080/testing
2. **Open Browser DevTools:** Press F12
3. **Go to Network tab**
4. **Look for:** `events` connection with type `eventsource`
5. **Start a benchmark** from Models page
6. **Watch the Network tab:**
   - Should see `benchmark-update` events coming through
   - Should see `sse:benchmark-update` triggers firing

### 4. Verify Toast Notifications

1. **Open Testing page**
2. **Start benchmark** from Models → "Run Benchmarks" button
3. **Watch top-right corner**
4. **Should see:** Green toast slides in: "📊 Test completed - updating results..."
5. **Toast should:** 
   - Auto-dismiss after 2 seconds
   - Appear multiple times (once per test)

### 5. Verify Table Updates

1. **Open Testing page** while benchmark is running
2. **Watch the results table**
3. **Should see:**
   - "Passed" count incrementing (8, 9, 10...)
   - "Failed" count incrementing (0, 1, 2...)
   - "Success Rate" updating live
   - **NO full page reload**

## Troubleshooting

### Problem: No SSE logs appearing

**Check:**
```bash
# Is server running?
ps aux | grep "clai benchmark"

# Is it listening on port 8080?
curl -s http://localhost:8080/api/benchmark/results | head -n 5

# Try restarting
pkill -f "tmp/clai" && cd /home/josh/clai && make dev
```

### Problem: SSE connection fails in browser

**Check browser console for errors:**
- F12 → Console tab
- Look for: `EventSource failed` or CORS errors

**Verify endpoint works:**
```bash
curl -N http://localhost:8080/api/servers/events
```

### Problem: Events broadcast but table doesn't update

**Check HTMX trigger syntax:**
```html
<!-- CORRECT -->
hx-trigger="load, every 5s, sse:benchmark-update"

<!-- WRONG -->
hx-trigger="sse:benchmark-update from:body"  ❌
```

**Check SSE extension is loaded:**
- View page source
- Look for: `<script src="https://unpkg.com/htmx.org@2.0.4/dist/ext/sse.js"></script>`

### Problem: Toast doesn't appear

**Check JavaScript console** (F12 → Console):
```javascript
// Manually test event listener
document.body.dispatchEvent(new CustomEvent('htmx:sseBeforeMessage', {
    detail: { type: 'benchmark-update' }
}));
```

**Should:** Toast appears in top-right corner

## Manual Test Procedure

### Complete End-to-End Test

```bash
# 1. Start dev server (if not already running)
cd /home/josh/clai && make dev

# 2. In another terminal, watch logs
tail -f /home/josh/clai/tmp/air.log

# 3. Open browser to http://localhost:8080/testing

# 4. Open DevTools (F12) → Network tab → Filter: "events"

# 5. Go to Models tab, click "Run Benchmarks"

# 6. Immediately switch back to Testing tab (or it auto-switches!)

# 7. Watch for:
#    - Browser: Toast notifications every few seconds
#    - Browser: Table updating incrementally
#    - Network: SSE events appearing
#    - Logs: "Broadcasting benchmark-update" messages

# 8. After benchmark completes:
#    - Final results should match total tests run
#    - All toasts should have auto-dismissed
```

## Expected Timeline

For a 12-test benchmark:

```
T+0s:    Benchmark starts
T+2s:    Test 1 completes → broadcast → toast → table updates (Passed: 1)
T+4s:    Test 2 completes → broadcast → toast → table updates (Passed: 2)
T+6s:    Test 3 completes → broadcast → toast → table updates (Passed: 3)
...
T+24s:   Test 12 completes → broadcast → toast → table updates (Passed: 12)
T+24s:   Benchmark ends
```

## Debug Commands

```bash
# Check if SSE clients are connected
lsof -i :8080 | grep ESTABLISHED

# Count SSE connections
lsof -i :8080 | grep ESTABLISHED | wc -l

# Test broadcast manually (requires running benchmark)
# (No way to manually trigger from CLI, must run actual benchmark)

# Check database for real-time updates
sqlite3 /home/josh/clai/clai.db "SELECT PassedTests, FailedTests FROM agentic_benchmark_runs ORDER BY ID DESC LIMIT 1;"

# Monitor DB updates in real-time
watch -n 1 'sqlite3 /home/josh/clai/clai.db "SELECT PassedTests, FailedTests FROM agentic_benchmark_runs ORDER BY ID DESC LIMIT 1;"'
```

## Success Criteria

✅ **Working correctly if:**
- SSE client connects when page loads (check logs)
- Broadcasts happen after each test (check logs)
- Toast appears multiple times during benchmark (check browser)
- Table updates incrementally, not just at end (check browser)
- No JavaScript errors in console (check DevTools)

❌ **NOT working if:**
- Table only updates at very end (after all tests complete)
- No toasts appear during benchmark
- SSE logs show "0 clients connected"
- Browser console shows EventSource errors

## Architecture Flow

```
Test completes
    ↓
SaveAgenticBenchmarkResult() → database
    ↓
UpdateAgenticBenchmarkRun() → database
    ↓
broadcastBenchmarkUpdate() → sends "benchmark-update" to SSE clients
    ↓
Browser receives SSE event
    ↓
HTMX detects "sse:benchmark-update" trigger
    ↓
HTMX fetches /api/benchmark/results
    ↓
Table updates via morphDOM (idiomorph)
    ↓
JavaScript event listener triggers toast
    ↓
Toast appears for 2 seconds, then fades
```

## Files Involved

- **Backend SSE:** `/home/josh/clai/internal/benchmark/server.go`
  - `handleServerEvents()` (line 136-177) - SSE connection handler
  - `broadcastBenchmarkUpdate()` (line 199-221) - Broadcast function
  - Calls at lines 943, 960, 977, 1003 - After each test

- **Frontend Template:** `/home/josh/clai/internal/benchmark/templates/testing.templ`
  - Line 74: SSE connection
  - Line 34: HTMX trigger with `sse:benchmark-update`
  - Line 106-125: Toast JavaScript

- **Generated Template:** `/home/josh/clai/internal/benchmark/templates/testing_templ.go`
  - Auto-generated from testing.templ
  - Regenerate with: `cd internal/benchmark/templates && templ generate`

## Next Steps if Issues Found

1. **Check logs first** - Most issues show up in server logs
2. **Verify SSE connection** - Use curl or browser Network tab
3. **Test toast manually** - Use JavaScript console
4. **Verify HTMX syntax** - Check trigger attribute
5. **Regenerate templates** - `templ generate` in templates dir
6. **Restart server** - `pkill -f "tmp/clai" && make dev`
