#!/bin/bash

# Quick status check for model loading
# Usage: ./check_load_status.sh

PID=3593421
LOG_FILE="/home/josh/clai/model_load_monitor.log"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  MODEL LOADING STATUS CHECK"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check if process is running
if ! ps -p $PID > /dev/null 2>&1; then
    echo "❌ Process $PID is not running"
    echo ""
    if [ -f "$LOG_FILE" ]; then
        echo "Last 5 log entries:"
        tail -5 "$LOG_FILE"
    fi
    exit 1
fi

# Get current stats
STATS=$(ps -p $PID -o pid,pcpu,pmem,rss,etime --no-headers 2>/dev/null)
CPU=$(echo $STATS | awk '{print $2}')
MEM_PCT=$(echo $STATS | awk '{print $3}')
RSS_KB=$(echo $STATS | awk '{print $4}')
ELAPSED=$(echo $STATS | awk '{print $5}')
RSS_GB=$(awk "BEGIN {printf \"%.1f\", $RSS_KB/1024/1024}")

# Check health
HEALTH=$(curl -s --max-time 2 http://localhost:8082/health 2>/dev/null)
if echo "$HEALTH" | grep -q '"status":"ok"'; then
    STATUS="✅ READY"
    COLOR="\033[0;32m"
elif echo "$HEALTH" | grep -q '"error"'; then
    STATUS="⏳ LOADING"
    COLOR="\033[0;33m"
else
    STATUS="❓ UNKNOWN"
    COLOR="\033[0;37m"
fi
NC="\033[0m"

# Display current status
echo -e "Status:        ${COLOR}${STATUS}${NC}"
echo "Elapsed Time:  $ELAPSED"
echo "CPU Usage:     ${CPU}%"
echo "Memory (RAM):  ${RSS_GB} GB (${MEM_PCT}%)"
echo ""

# Show monitoring progress
if [ -f "$LOG_FILE" ]; then
    TOTAL_CHECKS=$(grep -c "^\[" "$LOG_FILE")
    echo "Monitor Checks: $TOTAL_CHECKS"
    echo ""
    echo "Last 3 check results:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    tail -3 "$LOG_FILE" | grep "^\["
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
fi

echo ""

# Check UI display
echo "GPU Status UI:"
UI_STATUS=$(curl -s http://localhost:8081/api/gpu/status 2>/dev/null | grep -A 1 "Port 8082" | grep "CPU:" | sed 's/<[^>]*>//g' | xargs)
if [ -n "$UI_STATUS" ]; then
    echo "  $UI_STATUS"
else
    echo "  (UI not available)"
fi

echo ""
echo "Full log: $LOG_FILE"
echo ""
