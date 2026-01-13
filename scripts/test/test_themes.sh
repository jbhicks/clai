#!/bin/bash

echo "=== Testing Color Detection ==="
echo "TERM=$TERM"
echo "COLORTERM=$COLORTERM" 
echo "TERM_PROGRAM=$TERM_PROGRAM"
echo "tput colors=$(tput colors)"

echo ""
echo "=== Testing Theme Selection ==="
echo "Current environment should use 256-color themes"

echo ""
echo "=== Testing Forced True Color ==="
CLAI_FORCE_TRUE_COLOR=1 echo "With CLAI_FORCE_TRUE_COLOR=1, should use true color themes"

echo ""
echo "=== Available 256-color themes ==="
echo "1. Dracula 256"
echo "2. Tokyo Night 256"

echo ""
echo "=== To test in the actual app ==="
echo "1. Run 'make dev' in a terminal"
echo "2. Use Ctrl+D to switch between themes"
echo "3. Check debug.log for color detection messages"
echo "4. If colors still look wrong, try: CLAI_FORCE_TRUE_COLOR=1 make dev"