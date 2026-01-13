#!/bin/bash
# Monitor download progress in real-time

echo "=== CLAI Download Monitor ==="
echo "Monitoring downloads every 2 seconds. Press Ctrl+C to stop."
echo ""

while true; do
    clear
    echo "=== CLAI Download Monitor - $(date '+%H:%M:%S') ==="
    echo ""
    
    # Get download status from API
    response=$(curl -s http://localhost:8080/api/models/downloads)
    
    if echo "$response" | grep -q "No active downloads"; then
        echo "❌ No active downloads"
        echo ""
        echo "To check if downloads exist in memory:"
        echo "  tail -20 /home/josh/clai/benchmark.log | grep -i download"
        echo ""
        echo "To check if SSE is working:"
        echo "  tail -20 /home/josh/clai/benchmark.log | grep -i sse"
    else
        # Extract download info using grep and sed
        echo "$response" | grep -oP '(?<=<span style="color: #e2e8f0; font-size: 13px; font-weight: 500;">)[^<]+' | while read filename; do
            echo "📦 File: $filename"
        done
        echo ""
        
        # Extract progress percentages
        echo "$response" | grep -oP '\d+\.\d+%' | while read percent; do
            echo "   Progress: $percent"
        done
        echo ""
        
        # Extract speeds
        echo "$response" | grep -oP '\d+\.\d+ MB/s' | while read speed; do
            echo "   Speed: $speed"
        done
        echo ""
        
        # Extract download sizes
        echo "$response" | grep -oP '\d+\.\d+ (GB|MB) / \d+\.\d+ (GB|MB)' | while read size; do
            echo "   Size: $size"
        done
        echo ""
        
        # Show recent log activity
        echo "--- Recent Log Activity ---"
        tail -5 /home/josh/clai/benchmark.log | grep -i "download\|retry\|SSE" | tail -3
    fi
    
    echo ""
    echo "--- Server Info ---"
    echo "Server: $(curl -s http://localhost:8080/health 2>&1)"
    echo "SSE Connections: $(tail -20 /home/josh/clai/benchmark.log | grep -c 'SSE')"
    
    sleep 2
done
