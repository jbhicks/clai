#!/bin/bash
# SSE Verification Test Script
# Tests if Server-Sent Events are working for real-time benchmark updates

echo "🔍 SSE Real-Time Updates Verification"
echo "======================================"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test 1: Check if server is running
echo "1️⃣  Checking if benchmark server is running..."
if ps aux | grep -q "[c]lai benchmark"; then
    echo -e "${GREEN}✓${NC} Server is running"
else
    echo -e "${RED}✗${NC} Server is NOT running"
    echo "   Start with: make dev"
    exit 1
fi
echo ""

# Test 2: Check if API is responding
echo "2️⃣  Testing API endpoint..."
if curl -s -f http://localhost:8080/api/benchmark/results > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} API is responding"
else
    echo -e "${RED}✗${NC} API is not responding"
    exit 1
fi
echo ""

# Test 3: Test SSE connection
echo "3️⃣  Testing SSE connection (5 second timeout)..."
SSE_TEST=$(timeout 5 curl -N -s http://localhost:8080/api/servers/events 2>&1)
if [ $? -eq 124 ]; then
    echo -e "${GREEN}✓${NC} SSE connection works (stayed open as expected)"
else
    echo -e "${RED}✗${NC} SSE connection failed or closed unexpectedly"
    echo "   Output: $SSE_TEST"
fi
echo ""

# Test 4: Check recent logs for SSE activity
echo "4️⃣  Checking server logs for SSE activity..."
if [ -f /home/josh/clai/tmp/air.log ]; then
    SSE_LOGS=$(tail -n 100 /home/josh/clai/tmp/air.log | grep -c "SSE")
    if [ $SSE_LOGS -gt 0 ]; then
        echo -e "${GREEN}✓${NC} Found $SSE_LOGS SSE-related log entries"
        echo "   Recent SSE logs:"
        tail -n 100 /home/josh/clai/tmp/air.log | grep "SSE" | tail -n 5 | sed 's/^/   /'
    else
        echo -e "${YELLOW}⚠${NC}  No SSE logs found (may not have had connections yet)"
    fi
else
    echo -e "${YELLOW}⚠${NC}  Log file not found: /home/josh/clai/tmp/air.log"
fi
echo ""

# Test 5: Check for broadcast calls in logs
echo "5️⃣  Checking for benchmark update broadcasts..."
if [ -f /home/josh/clai/tmp/air.log ]; then
    BROADCAST_LOGS=$(tail -n 200 /home/josh/clai/tmp/air.log | grep -c "benchmark-update")
    if [ $BROADCAST_LOGS -gt 0 ]; then
        echo -e "${GREEN}✓${NC} Found $BROADCAST_LOGS benchmark-update events"
        echo "   Recent broadcasts:"
        tail -n 200 /home/josh/clai/tmp/air.log | grep "benchmark-update" | tail -n 3 | sed 's/^/   /'
    else
        echo -e "${YELLOW}⚠${NC}  No broadcast events found (run a benchmark to test)"
    fi
else
    echo -e "${YELLOW}⚠${NC}  Log file not found"
fi
echo ""

# Test 6: Check database for recent benchmark activity
echo "6️⃣  Checking for recent benchmark runs..."
if [ -f /home/josh/clai/clai.db ]; then
    RECENT_RUNS=$(sqlite3 /home/josh/clai/clai.db "SELECT COUNT(*) FROM agentic_benchmark_runs WHERE datetime(CreatedAt) > datetime('now', '-1 hour');" 2>/dev/null)
    if [ ! -z "$RECENT_RUNS" ] && [ $RECENT_RUNS -gt 0 ]; then
        echo -e "${GREEN}✓${NC} Found $RECENT_RUNS benchmark run(s) in last hour"
        echo "   Latest run:"
        sqlite3 /home/josh/clai/clai.db "SELECT ModelName, PassedTests || '/' || TotalTests || ' passed', SuccessRate || '%' FROM agentic_benchmark_runs ORDER BY ID DESC LIMIT 1;" 2>/dev/null | sed 's/^/   /'
    else
        echo -e "${YELLOW}⚠${NC}  No recent benchmark runs (start one to test real-time updates)"
    fi
else
    echo -e "${YELLOW}⚠${NC}  Database not found"
fi
echo ""

# Summary
echo "======================================"
echo "📋 Summary"
echo "======================================"
echo ""
echo "SSE is configured to:"
echo "  • Broadcast after each test completes"
echo "  • Send 'benchmark-update' events to all connected clients"
echo "  • Trigger HTMX to refresh the results table"
echo "  • Display toast notifications in the browser"
echo ""
echo "To test end-to-end:"
echo "  1. Open http://localhost:8080/testing in browser"
echo "  2. Open DevTools (F12) → Network tab"
echo "  3. Go to Models tab → Click 'Run Benchmarks'"
echo "  4. Watch Testing tab for:"
echo "     - Toast notifications appearing"
echo "     - Results table updating live"
echo "     - Network tab showing 'benchmark-update' events"
echo ""
echo "For detailed verification steps, see:"
echo "  ${YELLOW}SSE_VERIFICATION.md${NC}"
echo ""
