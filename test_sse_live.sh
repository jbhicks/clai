#!/bin/bash
# Quick SSE Live Test
# Opens a browser connection to test SSE in real-time

echo "🧪 SSE Live Connection Test"
echo "=============================="
echo ""

# Check if server is running
if ! ps aux | grep -q "[c]lai benchmark"; then
    echo "❌ Server is not running. Start with: make dev"
    exit 1
fi

echo "✓ Server is running on http://localhost:8080"
echo ""

# Test SSE connection with verbose output
echo "📡 Testing SSE connection..."
echo "   (Will timeout after 10 seconds - that's expected!)"
echo "   Press Ctrl+C to stop early"
echo ""

# Connect to SSE endpoint and show any data that comes through
timeout 10 curl -N -v http://localhost:8080/api/servers/events 2>&1 | grep -E "(Connected|event:|data:|SSE)" || echo "Connection timed out (expected)"

echo ""
echo ""
echo "=============================="
echo "Next Steps:"
echo "=============================="
echo ""
echo "1. Open browser to: http://localhost:8080/testing"
echo "2. Open DevTools: Press F12"
echo "3. Go to Network tab, filter by 'events'"
echo "4. You should see an EventSource connection"
echo "5. Go to Models tab, click 'Run Benchmarks'"
echo "6. Return to Testing tab and watch for:"
echo "   • Toast notifications: '📊 Test completed...'"
echo "   • Table updates: Pass/fail counts incrementing"
echo "   • Network events: 'benchmark-update' messages"
echo ""
echo "If you see issues, check logs:"
echo "   • Server stderr: Where 'make dev' is running"
echo "   • Browser console: F12 → Console tab"
echo ""
