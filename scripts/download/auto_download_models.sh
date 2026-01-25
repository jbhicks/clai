#!/bin/bash
# Automated model download script - downloads all recommended models in background

MODELS_DIR="${MODELS_PATH:-/home/josh/models}"
LOG_FILE="$MODELS_DIR/download_all.log"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Downloading Recommended Models for Tool Calling${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo ""

mkdir -p "$MODELS_DIR"

# Array of models to download
declare -A MODELS
MODELS[hermes3]="https://huggingface.co/NousResearch/Hermes-3-Llama-3.1-8B-GGUF/resolve/main/Hermes-3-Llama-3.1-8B.Q4_K_M.gguf"
MODELS[llama31-8b]="https://huggingface.co/bartowski/Meta-Llama-3.1-8B-Instruct-GGUF/resolve/main/Meta-Llama-3.1-8B-Instruct-Q4_K_M.gguf"
MODELS[mistral-nemo]="https://huggingface.co/bartowski/Mistral-Nemo-Instruct-2407-GGUF/resolve/main/Mistral-Nemo-Instruct-2407-Q4_K_M.gguf"
MODELS[qwen25-14b]="https://huggingface.co/bartowski/Qwen2.5-14B-Instruct-GGUF/resolve/main/Qwen2.5-14B-Instruct-Q4_K_M.gguf"
MODELS[gpt-oss-120b]="https://huggingface.co/bartowski/gpt-oss-120b-GGUF/resolve/main/gpt-oss-120b-Q4_K_M.gguf"
MODELS[llama31-70b]="https://huggingface.co/bartowski/Meta-Llama-3.1-70B-Instruct-GGUF/resolve/main/Meta-Llama-3.1-70B-Instruct-Q4_K_M.gguf"

# Parse command line arguments
if [ "$1" == "--all" ]; then
    DOWNLOAD_LIST=("hermes3" "llama31-8b" "mistral-nemo" "qwen25-14b" "gpt-oss-120b" "llama31-70b")
elif [ "$1" == "--small" ]; then
    echo -e "${YELLOW}Downloading small models only (< 10GB)${NC}"
    DOWNLOAD_LIST=("hermes3" "llama31-8b" "mistral-nemo" "qwen25-14b")
elif [ "$1" == "--recommended" ]; then
    echo -e "${YELLOW}Downloading top 3 recommended models${NC}"
    DOWNLOAD_LIST=("hermes3" "llama31-8b" "mistral-nemo")
elif [ -n "$1" ]; then
    # Download specific model
    if [[ -v "MODELS[$1]" ]]; then
        DOWNLOAD_LIST=("$1")
    else
        echo -e "${RED}Unknown model: $1${NC}"
        echo ""
        echo "Available models:"
        for key in "${!MODELS[@]}"; do
            echo "  - $key"
        done
        exit 1
    fi
else
    echo "Usage:"
    echo "  $0 --recommended      # Download Hermes 3, Llama 3.1 8B, Mistral Nemo"
    echo "  $0 --small            # Download all models < 10GB"
    echo "  $0 --all              # Download all 6 models (~134GB)"
    echo "  $0 <model-name>       # Download specific model"
    echo ""
    echo "Available models:"
    echo "  hermes3        - 4.9GB - Best for tool calling"
    echo "  llama31-8b     - 4.9GB - Meta official"
    echo "  mistral-nemo   - 7.1GB - 128k context"
    echo "  qwen25-14b     - 8.5GB - General purpose"
    echo "  gpt-oss-120b   - 68GB  - OpenAI reasoning (fast on Strix!)"
    echo "  llama31-70b    - 40GB  - Most capable"
    exit 0
fi

echo -e "${GREEN}Starting downloads...${NC}"
echo "" | tee -a "$LOG_FILE"

for model in "${DOWNLOAD_LIST[@]}"; do
    url="${MODELS[$model]}"
    filename=$(basename "$url")
    output_path="$MODELS_DIR/$filename"
    
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Model:${NC} $model"
    echo -e "${YELLOW}File:${NC} $filename"
    
    if [ -f "$output_path" ]; then
        echo -e "${GREEN}✓ Already exists${NC}"
        echo ""
        continue
    fi
    
    echo -e "${YELLOW}Downloading...${NC}"
    echo "Started: $(date)" | tee -a "$LOG_FILE"
    
    # Download with progress bar and resume support
    wget -c -O "$output_path.tmp" "$url" 2>&1 | tee -a "$LOG_FILE"
    
    if [ ${PIPESTATUS[0]} -eq 0 ]; then
        mv "$output_path.tmp" "$output_path"
        echo -e "${GREEN}✓ Download complete${NC}"
        echo "Completed: $(date)" | tee -a "$LOG_FILE"
    else
        echo -e "${RED}✗ Download failed${NC}"
        rm -f "$output_path.tmp"
        echo "Failed: $(date)" | tee -a "$LOG_FILE"
    fi
    echo ""
done

echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}Downloads complete!${NC}"
echo ""
echo "Downloaded models:"
ls -lh "$MODELS_DIR"/*.gguf 2>/dev/null | awk '{print "  " $9 " (" $5 ")"}'
echo ""
echo "Next: ./test_models.sh --all"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
