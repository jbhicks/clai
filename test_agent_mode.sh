#!/bin/bash
# Quick Agent Mode Testing Script
# Run this to start clai in agent mode for manual testing

export AGENT_MODE=true
export LOG_LEVEL=DEBUG
export OLLAMA_MODEL="llama3.1-gpu:latest"
export OLLAMA_HOST="http://localhost:11434"

echo "=========================================="
echo "Starting CLAI in Agent Mode"
echo "=========================================="
echo ""
echo "Configuration:"
echo "  AGENT_MODE:   $AGENT_MODE"
echo "  LOG_LEVEL:    $LOG_LEVEL"
echo "  MODEL:        $OLLAMA_MODEL"
echo "  HOST:         $OLLAMA_HOST"
echo ""
echo "Try these test queries:"
echo "  1. What is 5 + 3?"
echo "  2. Calculate 15 * 23 + 100"
echo "  3. What is the sum of squares of 3, 4, and 5?"
echo ""
echo "Monitor logs in another terminal:"
echo "  tail -f debug.log | grep AGENT"
echo ""
echo "=========================================="
echo ""

./clai
