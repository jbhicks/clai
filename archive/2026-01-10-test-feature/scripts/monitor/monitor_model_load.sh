#!/bin/bash

# Monitor gpt-oss-120b model loading progress
# Usage: ./monitor_model_load.sh [port] [interval_seconds]

PORT=${1:-8082}
CHECK_INTERVAL=${2:-5}
MODEL_NAME="gpt-oss-120b"

echo "========================================="
echo "Monitoring ${MODEL_NAME} model loading"
echo "Port: ${PORT}"
echo "Check interval: ${CHECK_INTERVAL}s"
echo "========================================="
echo ""

# Function to format bytes to human readable
format_bytes() {
    local bytes=$1
    if [ $bytes -lt 1024 ]; then
        echo "${bytes}B"
    elif [ $bytes -lt 1048576 ]; then
        echo "$(awk "BEGIN {printf \"%.1f\", $bytes/1024}")KB"
    elif [ $bytes -lt 1073741824 ]; then
        echo "$(awk "BEGIN {printf \"%.1f\", $bytes/1048576}")MB"
    else
        echo "$(awk "BEGIN {printf \"%.2f\", $bytes/1073741824}")GB"
    fi
}

# Function to format seconds to human readable time
format_time() {
    local seconds=$1
    local hours=$((seconds / 3600))
    local mins=$(((seconds % 3600) / 60))
    local secs=$((seconds % 60))
    printf "%02d:%02d:%02d" $hours $mins $secs
}

START_TIME=$(date +%s)

while true; do
    CURRENT_TIME=$(date +%s)
    ELAPSED=$((CURRENT_TIME - START_TIME))
    
    echo "=== $(date '+%Y-%m-%d %H:%M:%S') | Elapsed: $(format_time $ELAPSED) ==="
    
    # Check if process is running
    PID=$(ps aux | grep "${MODEL_NAME}" | grep llama-server | grep -v grep | awk '{print $2}')
    
    if [ -z "$PID" ]; then
        echo "❌ Model server process not found!"
        echo ""
        echo "Checking if it completed successfully..."
        
        # Check if server responds
        STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${PORT}/health 2>/dev/null)
        if [ "$STATUS" = "200" ]; then
            echo "✅ Model loaded successfully! Server is responding."
            exit 0
        else
            echo "❌ Server not responding. Process may have crashed."
            echo ""
            echo "Last 20 lines of server log:"
            tail -20 /tmp/llama-server-${PORT}.log 2>/dev/null || echo "Log file not found"
            exit 1
        fi
    fi
    
    echo "📊 Process ID: ${PID}"
    
    # Get process stats
    PS_OUTPUT=$(ps -p $PID -o %cpu,%mem,rss,vsz,etime 2>/dev/null | tail -1)
    if [ -n "$PS_OUTPUT" ]; then
        CPU=$(echo $PS_OUTPUT | awk '{print $1}')
        MEM=$(echo $PS_OUTPUT | awk '{print $2}')
        RSS=$(echo $PS_OUTPUT | awk '{print $3}')
        VSZ=$(echo $PS_OUTPUT | awk '{print $4}')
        RUNTIME=$(echo $PS_OUTPUT | awk '{print $5}')
        
        RSS_BYTES=$((RSS * 1024))
        VSZ_BYTES=$((VSZ * 1024))
        
        echo "💻 CPU: ${CPU}%"
        echo "🧠 Memory: ${MEM}%"
        echo "📈 RSS (Resident): $(format_bytes $RSS_BYTES)"
        echo "📊 VSZ (Virtual): $(format_bytes $VSZ_BYTES)"
        echo "⏱️  Process Runtime: ${RUNTIME}"
    fi
    
    # Check VRAM usage using rocm-smi
    if command -v rocm-smi &> /dev/null; then
        echo ""
        echo "🎮 GPU Memory (VRAM):"
        VRAM_OUTPUT=$(rocm-smi --showmeminfo vram 2>/dev/null | grep -A 1 "GPU\[0\]")
        if [ -n "$VRAM_OUTPUT" ]; then
            echo "$VRAM_OUTPUT"
        else
            echo "  Unable to get VRAM info"
        fi
    fi
    
    # Check server health endpoint
    echo ""
    echo "🔍 Server Health Check:"
    HEALTH=$(curl -s http://localhost:${PORT}/health 2>&1)
    if [ $? -eq 0 ]; then
        if echo "$HEALTH" | grep -q "Loading model"; then
            echo "⏳ Status: Still loading model..."
        elif echo "$HEALTH" | grep -q "error"; then
            ERROR_MSG=$(echo "$HEALTH" | grep -oP '"message":"[^"]*"' | cut -d'"' -f4)
            echo "⚠️  Status: ${ERROR_MSG}"
        else
            echo "✅ Status: Model loaded! Server is ready."
            echo ""
            echo "🎉 Loading completed successfully!"
            echo "⏱️  Total time: $(format_time $ELAPSED)"
            
            # Get final model info
            MODEL_INFO=$(curl -s http://localhost:${PORT}/v1/models 2>/dev/null)
            if [ -n "$MODEL_INFO" ]; then
                echo ""
                echo "📋 Final Model Info:"
                echo "$MODEL_INFO" | python3 -m json.tool 2>/dev/null || echo "$MODEL_INFO"
            fi
            
            exit 0
        fi
    else
        echo "⚠️  Cannot connect to server (this is normal during early startup)"
    fi
    
    # Try to get model metadata from API
    METADATA=$(curl -s http://localhost:${PORT}/v1/models 2>/dev/null)
    if [ $? -eq 0 ] && echo "$METADATA" | grep -q "data"; then
        echo ""
        echo "📋 Model Metadata:"
        CTX=$(echo "$METADATA" | grep -oP '"context_window":\d+' | cut -d':' -f2)
        if [ -n "$CTX" ]; then
            echo "  Context Window: ${CTX} tokens"
        fi
    fi
    
    # Show last few lines of server log for errors
    LOG_FILE="/tmp/llama-server-${PORT}.log"
    if [ -f "$LOG_FILE" ]; then
        ERRORS=$(tail -50 "$LOG_FILE" | grep -i "error\|fail\|warn" | tail -3)
        if [ -n "$ERRORS" ]; then
            echo ""
            echo "⚠️  Recent warnings/errors from log:"
            echo "$ERRORS"
        fi
    fi
    
    echo ""
    echo "---"
    echo ""
    
    sleep $CHECK_INTERVAL
done
