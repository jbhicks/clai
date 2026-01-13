#!/bin/bash

echo "=== Testing Color Rendering Quality ==="
echo

echo "Current 256-color palette being used:"
echo "Background (235): $(tput setab 235)    $(tput sgr0)"
echo "Background (233): $(tput setab 233)    $(tput sgr0)"
echo "Red (197):        $(tput setaf 197)████$(tput sgr0)"
echo "Green (48):       $(tput setaf 48)████$(tput sgr0)"
echo "Blue (99):        $(tput setaf 99)████$(tput sgr0)"
echo "Yellow (220):     $(tput setaf 220)████$(tput sgr0)"
echo "Magenta (205):    $(tput setaf 205)████$(tput sgr0)"
echo "Cyan (51):        $(tput setaf 51)████$(tput sgr0)"
echo "White (255):      $(tput setaf 255)████$(tput sgr0)"
echo

echo "=== Border Test ==="
echo "Testing rounded border rendering:"
echo "$(tput setaf 99)┌─────────────────┐$(tput sgr0)"
echo "$(tput setaf 99)│ Tool Message   │$(tput sgr0)"
echo "$(tput setaf 99)│ with borders   │$(tput sgr0)"
echo "$(tput setaf 99)└─────────────────┘$(tput sgr0)"
echo

echo "=== Common Issues ==="
echo "1. If borders look broken: Terminal might not handle Unicode well"
echo "2. If colors look washed out: Terminal color profile mismatch"
echo "3. If alignment is off: Width calculation issues"
echo

echo "=== Terminal Info ==="
echo "TERM: $TERM"
echo "COLORTERM: $COLORTERM"
echo "tput colors: $(tput colors)"
echo

echo "=== Suggestions ==="
echo "1. Try: CLAI_FORCE_TRUE_COLOR=1 make dev"
echo "2. Check terminal settings for 'True Color' support"
echo "3. In WezTerm: ensure 'term = \"wezterm\"' in remote config"