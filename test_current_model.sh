#!/bin/bash
# Quick test of current model to establish baseline

echo "Testing current model (Qwen3-Coder-30B)..."
echo "This will take approximately 5-10 minutes"
echo ""

cd /home/josh/clai

# Run the benchmark
./test_models.sh Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf

echo ""
echo "Baseline test complete!"
echo ""
echo "Next steps:"
echo "  1. Review results in ./model_test_results/"
echo "  2. Download Hermes 3: ./download_models.sh"
echo "  3. Test Hermes 3: ./test_models.sh Hermes-3-Llama-3.1-8B.Q4_K_M.gguf"
