#!/bin/bash

# notify_downloads_complete.sh
# Monitors model downloads and notifies when complete

MODELS_DIR="${MODELS_PATH:-/home/josh/models}"
LOG_FILE="$MODELS_DIR/download_all.log"
CLAI_BIN="/home/josh/clai/clai"

# Check interval in seconds
CHECK_INTERVAL=30

# Expected models from --recommended
EXPECTED_MODELS=(
    "Hermes-3-Llama-3.1-8B.Q4_K_M.gguf"
    "Meta-Llama-3.1-8B-Instruct.Q4_K_M.gguf"
    "Mistral-Nemo-Instruct-2407.Q4_K_M.gguf"
)

echo "🔍 Monitoring model downloads..."
echo "Checking every ${CHECK_INTERVAL}s for completion"
echo "Expected models: ${EXPECTED_MODELS[@]}"
echo ""

count_downloaded() {
    local count=0
    for model in "${EXPECTED_MODELS[@]}"; do
        if [ -f "$MODELS_DIR/$model" ]; then
            ((count++))
        fi
    done
    echo $count
}

get_file_sizes() {
    for model in "${EXPECTED_MODELS[@]}"; do
        if [ -f "$MODELS_DIR/$model" ]; then
            size=$(du -h "$MODELS_DIR/$model" | cut -f1)
            echo "  ✅ $model ($size)"
        elif [ -f "$MODELS_DIR/$model.tmp" ]; then
            size=$(du -h "$MODELS_DIR/$model.tmp" | cut -f1)
            echo "  ⏳ $model (downloading... $size so far)"
        else
            echo "  ⏳ $model (queued)"
        fi
    done
}

# Show initial status
echo "Current status:"
get_file_sizes
echo ""

# Monitor until all downloads complete
while true; do
    downloaded=$(count_downloaded)
    total=${#EXPECTED_MODELS[@]}
    
    if [ "$downloaded" -eq "$total" ]; then
        # All downloads complete!
        echo ""
        echo "🎉 ============================================"
        echo "🎉  ALL DOWNLOADS COMPLETE! ($total/$total)"
        echo "🎉 ============================================"
        echo ""
        echo "Downloaded models:"
        get_file_sizes
        echo ""
        echo "📊 Next steps:"
        echo "   1. Test Hermes 3 (best for tool calling):"
        echo "      cd /home/josh/clai && ./clai models test Hermes-3-Llama-3.1-8B.Q4_K_M.gguf"
        echo ""
        echo "   2. Or test all downloaded models:"
        echo "      cd /home/josh/clai && ./clai models test --all"
        echo ""
        
        # Try to send desktop notification if notify-send is available
        if command -v notify-send &> /dev/null; then
            notify-send "clai Model Downloads Complete" "All 3 recommended models downloaded. Ready for testing!" --urgency=normal
        fi
        
        # Try to play a beep if available
        if command -v paplay &> /dev/null && [ -f /usr/share/sounds/freedesktop/stereo/complete.oga ]; then
            paplay /usr/share/sounds/freedesktop/stereo/complete.oga
        elif command -v beep &> /dev/null; then
            beep -f 800 -l 200 -n -f 1000 -l 300
        else
            # Terminal bell as fallback
            echo -e '\a'
        fi
        
        break
    else
        # Still downloading
        timestamp=$(date '+%Y-%m-%d %H:%M:%S')
        echo "[$timestamp] Progress: $downloaded/$total models complete"
        
        # Show which ones are done
        get_file_sizes
        echo ""
        
        sleep $CHECK_INTERVAL
    fi
done

echo "Monitor script complete. Exiting."
