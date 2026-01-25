#!/bin/bash
# Model Testing Script for AMD Strix Halo + ROCm llama.cpp

set -e

MODELS_DIR="${MODELS_PATH:-/home/josh/models}"
LLAMA_SERVER="/home/josh/llama.cpp-rocm-wmma/build/bin/llama-server"
SERVER_PORT=8081
TEST_RESULTS_DIR="./model_test_results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Create results directory
mkdir -p "$TEST_RESULTS_DIR"

echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}    LLM Model Benchmark Suite - AMD Strix Halo + ROCm${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo ""

# Function to wait for server to be ready
wait_for_server() {
    echo -e "${YELLOW}Waiting for llama-server to be ready...${NC}"
    for i in {1..30}; do
        if curl -s http://localhost:$SERVER_PORT/health > /dev/null 2>&1; then
            echo -e "${GREEN}Server is ready!${NC}"
            return 0
        fi
        sleep 1
        echo -n "."
    done
    echo -e "${RED}Server failed to start${NC}"
    return 1
}

# Function to stop existing server
stop_server() {
    echo -e "${YELLOW}Stopping existing llama-server instances...${NC}"
    pkill -f "llama-server.*$SERVER_PORT" || true
    sleep 2
}

# Function to start server with a specific model
start_server() {
    local model_path=$1
    local model_name=$(basename "$model_path" .gguf)
    
    echo -e "${BLUE}Starting llama-server with model: ${model_name}${NC}"
    
    $LLAMA_SERVER \
        -m "$model_path" \
        --host 0.0.0.0 \
        --port $SERVER_PORT \
        -c 131072 \
        -ngl 999 \
        -fa on \
        -b 2048 \
        -ub 512 \
        > "$TEST_RESULTS_DIR/${model_name}_server.log" 2>&1 &
    
    local server_pid=$!
    echo -e "${GREEN}Server started with PID: $server_pid${NC}"
    
    if ! wait_for_server; then
        echo -e "${RED}Failed to start server for $model_name${NC}"
        return 1
    fi
    
    return 0
}

# Function to run benchmark tests
run_benchmark() {
    local model_name=$1
    local output_file="$TEST_RESULTS_DIR/${model_name}_results_${TIMESTAMP}.txt"
    
    echo -e "${BLUE}Running benchmark tests for: ${model_name}${NC}"
    echo -e "${YELLOW}Output will be saved to: ${output_file}${NC}"
    
    # Run the Go test
    go test -v -run TestModelBenchmark_CurrentModel ./internal/llm \
        > "$output_file" 2>&1
    
    local exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
        echo -e "${GREEN}✓ Benchmark completed successfully${NC}"
    else
        echo -e "${YELLOW}⚠ Benchmark completed with warnings (see results)${NC}"
    fi
    
    # Extract and display summary
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    grep -A 50 "MODEL BENCHMARK SUMMARY" "$output_file" || true
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    
    return $exit_code
}

# Main testing logic
if [ "$1" == "--all" ]; then
    echo -e "${YELLOW}Testing ALL available models...${NC}"
    echo ""
    
    # Find all GGUF models
    models=($(find "$MODELS_DIR" -maxdepth 1 -name "*.gguf" -type f))
    
    if [ ${#models[@]} -eq 0 ]; then
        echo -e "${RED}No GGUF models found in $MODELS_DIR${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Found ${#models[@]} models to test${NC}"
    echo ""
    
    # Test each model
    for model in "${models[@]}"; do
        model_name=$(basename "$model" .gguf)
        echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${BLUE}║  Testing: ${model_name}${NC}"
        echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
        
        stop_server
        
        if start_server "$model"; then
            run_benchmark "$model_name"
            echo ""
        else
            echo -e "${RED}Skipping $model_name due to server startup failure${NC}"
            echo ""
        fi
    done
    
    stop_server
    
elif [ -n "$1" ]; then
    # Test specific model
    model_file="$MODELS_DIR/$1"
    
    if [ ! -f "$model_file" ]; then
        echo -e "${RED}Model not found: $model_file${NC}"
        echo -e "${YELLOW}Available models:${NC}"
        ls -1 "$MODELS_DIR"/*.gguf 2>/dev/null || echo "No models found"
        exit 1
    fi
    
    model_name=$(basename "$model_file" .gguf)
    
    echo -e "${BLUE}Testing single model: ${model_name}${NC}"
    echo ""
    
    stop_server
    
    if start_server "$model_file"; then
        run_benchmark "$model_name"
    fi
    
    stop_server
    
else
    # Interactive mode - list models and ask
    echo -e "${YELLOW}Available models:${NC}"
    echo ""
    
    models=($(find "$MODELS_DIR" -maxdepth 1 -name "*.gguf" -type f))
    
    if [ ${#models[@]} -eq 0 ]; then
        echo -e "${RED}No GGUF models found in $MODELS_DIR${NC}"
        exit 1
    fi
    
    for i in "${!models[@]}"; do
        model_name=$(basename "${models[$i]}" .gguf)
        size=$(du -h "${models[$i]}" | cut -f1)
        echo -e "  ${GREEN}[$i]${NC} $model_name ${YELLOW}($size)${NC}"
    done
    
    echo ""
    echo -e "${YELLOW}Usage:${NC}"
    echo -e "  ./test_models.sh ${GREEN}--all${NC}           # Test all models"
    echo -e "  ./test_models.sh ${GREEN}<model.gguf>${NC}    # Test specific model"
    echo ""
    echo -e "${YELLOW}Example:${NC}"
    echo -e "  ./test_models.sh Hermes-3-Llama-3.1-8B.Q4_K_M.gguf"
    echo ""
fi

echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}Results saved in: ${TEST_RESULTS_DIR}/${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════════${NC}"
