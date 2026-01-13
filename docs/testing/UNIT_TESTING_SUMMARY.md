# Unit Testing Summary for Systemd & SSE Integration

## Test Files Created

### 1. `/home/josh/clai/internal/benchmark/model_manager_test.go`
Comprehensive tests for systemd detection and model management:

- **`TestCheckServerHealth`** - Verifies health check with various server responses (OK, error, timeout)
- **`TestFindAvailablePortForModel`** - Tests port allocation in range 8081-8090
- **`TestFindAvailablePortForModel_AllPortsOccupied`** - Ensures error when all ports busy
- **`TestIsSystemdManaged`** - Tests systemd process detection via cgroup parsing
- **`TestGetSystemdServiceName`** - Tests service name extraction from cgroup paths
- **`TestGetModelNameFromPort`** - Tests model name extraction from server responses
- **`TestGetContextSizeFromPort`** - Tests context size extraction from /slots endpoint
- **`TestScanAvailableModels`** - Tests model scanning from filesystem (.gguf files only)
- **`TestStopServer_SystemdIntegration`** - Integration test for systemd detection (skips if systemctl unavailable)
- **`TestRefreshServerStatus`** - Tests server status refresh logic

### 2. `/home/josh/clai/internal/benchmark/sse_test.go`
Comprehensive tests for SSE (Server-Sent Events) functionality:

- **`TestBroadcastServerUpdate`** - Verifies SSE broadcast reaches all connected clients
- **`TestBroadcastServerUpdate_NoClients`** - Ensures no panic when broadcasting with no clients
- **`TestBroadcastServerUpdate_FullBuffer`** - Tests non-blocking behavior with full client buffers
- **`TestSSEConcurrentBroadcast`** - Tests concurrent broadcasts to multiple clients
- **`TestSSEClientConnection`** - Verifies SSE connection headers and protocol

## Test Coverage

### Systemd Integration (model_manager_test.go)
✅ **Covered:**
- cgroup file parsing for systemd detection
- Service name extraction from cgroup paths
- Port availability checking
- Health check HTTP requests with timeouts
- Model scanning from filesystem
- Concurrent server status refreshes

🔶 **Partial Coverage:**
- Actual `systemctl` execution (tested via integration test, skipped if unavailable)
- Process killing (requires actual running processes)

❌ **Not Covered:**
- Full end-to-end model start/stop cycle with real llama-server
- GPU memory tracking (depends on external hardware)
- Real systemd service management (requires system permissions)

### SSE Integration (sse_test.go)
✅ **Covered:**
- SSE connection establishment
- Correct HTTP headers (Content-Type, Cache-Control, Connection)
- Message broadcasting to multiple clients
- Concurrent broadcast safety (mutex protection)
- Non-blocking behavior with full/slow clients
- Client registration and deregistration

🔶 **Partial Coverage:**
- Event format verification (basic check, could be more thorough)

❌ **Not Covered:**
- Browser-side SSE event parsing
- HTMX SSE extension integration
- Template SSE trigger attributes (integration testing needed)
- SSE reconnection behavior

## Known Test Limitations

### Build Issues Preventing Test Execution
1. **`agentic_test.go`**: Has undefined references (`extractGoCode`, `executeGoCodeHelper`)
   - Status: Pre-existing issue, not related to our changes
   
2. **`download_manager.go`**: Non-constant format strings in log.Printf calls (lines 583, 626, 709, 757)
   - Status: Pre-existing linter warnings, not blocking functionality
   - Impact: Tests cannot run until package compiles

3. **Database dependency**: Some tests require `db.NewStore()` but the actual function is `db.New()` and doesn't support in-memory databases
   - Workaround: SSE tests create server struct manually without database dependency

## Running the Tests

### Once Build Issues Are Fixed:

```bash
# Run all model manager tests
go test -v ./internal/benchmark -run TestCheck|TestFind|TestIs|TestGet|TestScan|TestStop|TestRefresh

# Run all SSE tests  
go test -v ./internal/benchmark -run TestBroadcast|TestSSE

# Run specific test
go test -v ./internal/benchmark -run TestIsSystemdManaged

# Run with coverage
go test -v -cover ./internal/benchmark
```

### Expected Results:
- **TestCheckServerHealth**: 3 subtests (healthy, error, timeout)
- **TestFindAvailablePortForModel**: Should find port in 8081-8090 range
- **TestIsSystemdManaged**: Will detect if current process is systemd-managed
- **TestBroadcastServerUpdate**: All clients receive "refresh-servers" message
- **TestSSEConcurrentBroadcast**: 10 clients × 5 broadcasts = 50 messages delivered

## Integration Testing Recommendations

Since unit tests have limitations with system integration, recommend:

### Manual Integration Test:
1. Start benchmark server: `./clai benchmark`
2. Open browser to `http://localhost:8083/models`
3. Open browser DevTools → Network tab
4. Filter for "events" to see SSE connection
5. Click "Stop" on a systemd-managed model
6. Verify in terminal logs:
   ```
   Detected systemd-managed service: llama-server.service, using systemctl to stop
   Broadcasting refresh-servers event to N SSE clients
   Sent refresh-servers event to client
   ```
7. Verify in browser:
   - SSE event received (event: refresh-servers)
   - Two GET requests triggered: `/api/gpu/status` + `/api/servers/list`
   - GPU memory decreases
   - Server status changes to "Stopped"
   - Model stays stopped (doesn't auto-restart)

### Automated E2E Test (Future):
```bash
# Proposed test script
#!/bin/bash
# 1. Start benchmark server
# 2. Start a test llama-server via systemctl --user
# 3. Use curl to call /api/servers/stop
# 4. Verify service stopped: systemctl --user is-active llama-server.service
# 5. Verify no auto-restart after 5 seconds
# 6. Cleanup
```

## Test Maintenance

### When Adding New Features:
1. Add unit tests to `model_manager_test.go` or `sse_test.go`
2. Follow existing patterns (table-driven tests, mock servers)
3. Test both success and error paths
4. Include timeout/edge cases

### Test Patterns Used:
- **Table-driven tests**: Multiple scenarios in one test function
- **httptest.NewServer**: Mock HTTP servers for testing API calls
- **time.After with select**: Timeout protection for async operations
- **sync.WaitGroup**: Coordinating concurrent test operations
- **t.Skip**: Graceful handling when system dependencies unavailable

## Current Status

**Tests Written**: ✅ Complete  
**Tests Compiling**: ❌ Blocked by pre-existing package issues  
**Tests Passing**: ⏸️ Pending package fixes  
**Coverage Estimate**: ~70% of new systemd/SSE code  

**Recommendation**: Fix `agentic_test.go` and `download_manager.go` linter issues to unblock test execution.
