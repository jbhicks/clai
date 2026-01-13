#!/bin/bash
# Ralph-OpenCode: Fancy Autonomous Development Loop
# Usage: ./ralph-omo.sh [options]
#
# Options:
#   --max-iterations N   Maximum iterations (default: 50)
#   --model MODEL        Model to use (default: opencode/claude-opus-4-5)
#   --verbose            Enable verbose logging
#   --config FILE        Config file path
#   --help               Show help

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MAX_ITERATIONS=50
MODEL="opencode/claude-opus-4-5"
VERBOSE=false
CONFIG_FILE=""

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --max-iterations)
      MAX_ITERATIONS="$2"
      shift 2
      ;;
    --model)
      MODEL="$2"
      shift 2
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    --config)
      CONFIG_FILE="$2"
      shift 2
      ;;
    --help)
      echo "Ralph-OpenCode: Autonomous Development Loop"
      echo ""
      echo "Usage: $0 [options]"
      echo ""
      echo "Options:"
      echo "  --max-iterations N   Maximum iterations (default: 50)"
      echo "  --model MODEL        Model to use (default: opencode/claude-opus-4-5)"
      echo "  --verbose            Enable verbose logging"
      echo "  --config FILE        Config file path"
      echo "  --help               Show this help"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Build command
CMD="bun run $SCRIPT_DIR/src/index.ts"
CMD="$CMD --maxIterations $MAX_ITERATIONS"
CMD="$CMD --model $MODEL"

if [ "$VERBOSE" = true ]; then
  CMD="$CMD --verbose"
fi

if [ -n "$CONFIG_FILE" ]; then
  CMD="$CMD --config $CONFIG_FILE"
fi

# Run Ralph loop
echo "🏔️  Starting Ralph-OpenCode loop..."
echo "   Max iterations: $MAX_ITERATIONS"
echo "   Model: $MODEL"
echo ""

exec $CMD
