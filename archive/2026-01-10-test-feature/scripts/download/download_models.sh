#!/bin/bash
# Download recommended models for tool calling and compare them

MODELS_DIR="/home/josh/models"
DOWNLOAD_LOG="$MODELS_DIR/download.log"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Recommended Models for Tool Calling - Download Script${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo ""

# Recommended models for tool calling
# Format: "URL|filename|description|size"
RECOMMENDED_MODELS=(
    "https://huggingface.co/bartowski/Meta-Llama-3.1-8B-Instruct-GGUF/resolve/main/Meta-Llama-3.1-8B-Instruct-Q4_K_M.gguf|Meta-Llama-3.1-8B-Instruct-Q4_K_M.gguf|Meta Llama 3.1 8B - Official tool calling support|4.9GB"
    "https://huggingface.co/NousResearch/Hermes-3-Llama-3.1-8B-GGUF/resolve/main/Hermes-3-Llama-3.1-8B.Q4_K_M.gguf|Hermes-3-Llama-3.1-8B.Q4_K_M.gguf|Hermes 3 8B - Specialized for tool calling|4.9GB"
    "https://huggingface.co/bartowski/Mistral-Nemo-Instruct-2407-GGUF/resolve/main/Mistral-Nemo-Instruct-2407-Q4_K_M.gguf|Mistral-Nemo-Instruct-2407-Q4_K_M.gguf|Mistral Nemo 12B - 128k context|7.1GB"
    "https://huggingface.co/bartowski/Qwen2.5-14B-Instruct-GGUF/resolve/main/Qwen2.5-14B-Instruct-Q4_K_M.gguf|Qwen2.5-14B-Instruct-Q4_K_M.gguf|Qwen 2.5 14B - General purpose with tools|8.5GB"
    "https://huggingface.co/bartowski/gpt-oss-120b-GGUF/resolve/main/gpt-oss-120b-Q4_K_M.gguf|gpt-oss-120b-Q4_K_M.gguf|GPT-OSS-120B - OpenAI reasoning + tools (tested on Strix Halo)|68GB"
    "https://huggingface.co/bartowski/Meta-Llama-3.1-70B-Instruct-GGUF/resolve/main/Meta-Llama-3.1-70B-Instruct-Q4_K_M.gguf|Meta-Llama-3.1-70B-Instruct-Q4_K_M.gguf|Llama 3.1 70B - Most capable (large)|40GB"
)

download_model() {
    local url=$1
    local filename=$2
    local description=$3
    local size=$4
    local output_path="$MODELS_DIR/$filename"
    
    echo -e "${BLUE}Model:${NC} $description"
    echo -e "${YELLOW}Size:${NC} $size"
    echo -e "${YELLOW}File:${NC} $filename"
    echo ""
    
    if [ -f "$output_path" ]; then
        echo -e "${GREEN}✓ Already downloaded${NC}"
        echo ""
        return 0
    fi
    
    echo -e "${YELLOW}Downloading...${NC}"
    
    # Use wget with resume support
    if wget -c -O "$output_path.tmp" "$url" 2>&1 | tee -a "$DOWNLOAD_LOG"; then
        mv "$output_path.tmp" "$output_path"
        echo -e "${GREEN}✓ Download completed${NC}"
        echo ""
        return 0
    else
        echo -e "${RED}✗ Download failed${NC}"
        rm -f "$output_path.tmp"
        echo ""
        return 1
    fi
}

# Main menu
echo -e "${YELLOW}What would you like to do?${NC}"
echo ""
echo "  1) Download recommended models for testing"
echo "  2) Download specific model (choose from list)"
echo "  3) Show currently downloaded models"
echo "  4) Download all recommended models"
echo ""
read -p "Enter choice [1-4]: " choice
echo ""

case $choice in
    1)
        echo -e "${GREEN}Recommended models for tool calling:${NC}"
        echo ""
        for i in "${!RECOMMENDED_MODELS[@]}"; do
            IFS='|' read -r url filename description size <<< "${RECOMMENDED_MODELS[$i]}"
            echo -e "  ${GREEN}[$i]${NC} $description ${YELLOW}($size)${NC}"
        done
        echo ""
        read -p "Enter model numbers to download (space-separated, e.g., '0 1 2'): " selections
        echo ""
        
        for idx in $selections; do
            if [ "$idx" -ge 0 ] && [ "$idx" -lt "${#RECOMMENDED_MODELS[@]}" ]; then
                IFS='|' read -r url filename description size <<< "${RECOMMENDED_MODELS[$idx]}"
                echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
                download_model "$url" "$filename" "$description" "$size"
            fi
        done
        ;;
        
    2)
        echo -e "${GREEN}Available models:${NC}"
        echo ""
        for i in "${!RECOMMENDED_MODELS[@]}"; do
            IFS='|' read -r url filename description size <<< "${RECOMMENDED_MODELS[$i]}"
            echo -e "  ${GREEN}[$i]${NC} $description ${YELLOW}($size)${NC}"
            echo -e "      $filename"
            echo ""
        done
        
        read -p "Enter model number: " idx
        echo ""
        
        if [ "$idx" -ge 0 ] && [ "$idx" -lt "${#RECOMMENDED_MODELS[@]}" ]; then
            IFS='|' read -r url filename description size <<< "${RECOMMENDED_MODELS[$idx]}"
            download_model "$url" "$filename" "$description" "$size"
        else
            echo -e "${RED}Invalid selection${NC}"
        fi
        ;;
        
    3)
        echo -e "${GREEN}Currently downloaded models:${NC}"
        echo ""
        if ls -1 "$MODELS_DIR"/*.gguf > /dev/null 2>&1; then
            for model in "$MODELS_DIR"/*.gguf; do
                size=$(du -h "$model" | cut -f1)
                name=$(basename "$model")
                echo -e "  ${GREEN}•${NC} $name ${YELLOW}($size)${NC}"
            done
        else
            echo -e "${YELLOW}No models found${NC}"
        fi
        echo ""
        ;;
        
    4)
        echo -e "${YELLOW}Downloading ALL recommended models...${NC}"
        echo -e "${RED}Warning: This will download approximately 134GB of data${NC}"
        echo ""
        read -p "Continue? [y/N]: " confirm
        
        if [[ "$confirm" =~ ^[Yy]$ ]]; then
            for i in "${!RECOMMENDED_MODELS[@]}"; do
                IFS='|' read -r url filename description size <<< "${RECOMMENDED_MODELS[$i]}"
                echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
                echo -e "${BLUE}[$((i+1))/${#RECOMMENDED_MODELS[@]}]${NC}"
                download_model "$url" "$filename" "$description" "$size"
            done
        else
            echo -e "${YELLOW}Cancelled${NC}"
        fi
        ;;
        
    *)
        echo -e "${RED}Invalid choice${NC}"
        exit 1
        ;;
esac

echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}Done!${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo -e "  1. Run: ${GREEN}./test_models.sh --all${NC}      # Test all downloaded models"
echo -e "  2. Or:  ${GREEN}./test_models.sh <model.gguf>${NC} # Test a specific model"
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
