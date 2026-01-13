#!/bin/bash
# Monitor download progress for GPT-OSS 120B model
# Usage: ./monitor_download.sh

MODEL_DIR="$HOME/models"
LOG_FILE="$HOME/clai/benchmark.log"
MODEL_PREFIX="openai_gpt-oss-120b-Q8_0"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

clear

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  GPT-OSS 120B Download Monitor${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check if files exist
echo -e "${YELLOW}File Status:${NC}"
ls -lh "$MODEL_DIR/${MODEL_PREFIX}"*.gguf 2>/dev/null | while read -r line; do
    if echo "$line" | grep -q "00001-of-00002"; then
        size=$(echo "$line" | awk '{print $5}')
        echo -e "  Part 1: ${GREEN}$size${NC}"
    elif echo "$line" | grep -q "00002-of-00002"; then
        size=$(echo "$line" | awk '{print $5}')
        echo -e "  Part 2: ${GREEN}$size${NC}"
    elif echo "$line" | grep -qv "00001\|00002"; then
        size=$(echo "$line" | awk '{print $5}')
        echo -e "  ${GREEN}✓ FINAL: $size${NC}"
    fi
done

# Check if final concatenated file exists
if [ -f "$MODEL_DIR/${MODEL_PREFIX}.gguf" ]; then
    echo ""
    echo -e "${GREEN}✓✓✓ DOWNLOAD COMPLETE! ✓✓✓${NC}"
    echo -e "${GREEN}Final file ready at: $MODEL_DIR/${MODEL_PREFIX}.gguf${NC}"
    exit 0
fi

echo ""
echo -e "${YELLOW}Recent Download Activity (last 10 entries):${NC}"
grep -i "download" "$LOG_FILE" | tail -10 | while read -r line; do
    if echo "$line" | grep -qi "error\|fail"; then
        echo -e "  ${RED}$line${NC}"
    elif echo "$line" | grep -qi "complete\|success\|concatenat"; then
        echo -e "  ${GREEN}$line${NC}"
    else
        echo -e "  $line"
    fi
done

echo ""
echo -e "${YELLOW}Concatenation Status:${NC}"
if grep -q "Successfully concatenated.*${MODEL_PREFIX}" "$LOG_FILE" 2>/dev/null; then
    echo -e "  ${GREEN}✓ Concatenation completed!${NC}"
    grep "Successfully concatenated.*${MODEL_PREFIX}" "$LOG_FILE" | tail -1
else
    echo -e "  Waiting for both parts to complete..."
fi

echo ""
echo -e "${BLUE}----------------------------------------${NC}"
echo -e "Run this script again to check progress"
echo -e "Or run: ${YELLOW}watch -n 10 ./monitor_download.sh${NC}"
echo -e "         (auto-refresh every 10 seconds)"
echo -e "${BLUE}----------------------------------------${NC}"
