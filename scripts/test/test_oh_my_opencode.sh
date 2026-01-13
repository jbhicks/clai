#!/bin/bash
# Comprehensive test script for oh-my-opencode setup

echo "🔍 Testing oh-my-opencode Setup"
echo "=================================="
echo ""

# Check if llama server is running
echo "1. Checking Llama Server Status:"
if curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "✅ Llama server is running on port 8081"
else
    echo "❌ Llama server is NOT running on port 8081"
    echo "   Start it with: /tmp/start_llama_service.sh"
fi
echo ""

# Set up PATH for oh-my-opencode
export PATH="$HOME/.nvm/versions/node/v24.12.0/bin:$PATH"

# Check oh-my-opencode installation
echo "2. Checking oh-my-opencode Installation:"
if command -v oh-my-opencode > /dev/null 2>&1; then
    VERSION=$(oh-my-opencode --version)
    echo "✅ oh-my-opencode is installed (version: $VERSION)"
else
    echo "❌ oh-my-opencode is NOT installed"
fi
echo ""

# Check OpenCode plugin configuration
echo "3. Checking OpenCode Plugin Configuration:"
if grep -q "oh-my-opencode" ~/.config/opencode/opencode.json 2>/dev/null; then
    echo "✅ oh-my-opencode is configured as plugin in opencode.json"
else
    echo "❌ oh-my-opencode is NOT configured as plugin"
fi
echo ""

# Check oh-my-opencode configuration
echo "4. Checking oh-my-opencode Configuration:"
if [ -f ~/.config/opencode/oh-my-opencode.json ]; then
    echo "✅ oh-my-opencode config exists"
    echo "   Configured agents:"
    jq -r '.agents // {} | keys[]' ~/.config/opencode/oh-my-opencode.json 2>/dev/null || echo "   (unable to parse)"
else
    echo "❌ oh-my-opencode config does NOT exist"
fi
echo ""

# Test oh-my-opencode directly
echo "5. Testing oh-my-opencode Direct Execution:"
if command -v oh-my-opencode > /dev/null 2>&1; then
    echo "   Running basic test..."
    if timeout 10 oh-my-opencode run "echo hello" > /tmp/omo_test.log 2>&1; then
        if grep -q "hello" /tmp/omo_test.log; then
            echo "✅ oh-my-opencode executed successfully"
        else
            echo "⚠️  oh-my-opencode ran but output unclear"
        fi
    else
        echo "❌ oh-my-opencode failed to execute"
        echo "   Check: cat /tmp/omo_test.log"
    fi
else
    echo "   Skipping - not installed"
fi
echo ""

# Check current OpenCode status
echo "6. Current OpenCode Status:"
if pgrep -f "opencode" > /dev/null; then
    echo "✅ OpenCode process is running"
else
    echo "ℹ️  No OpenCode process currently running"
fi
echo ""

echo "📋 Summary & Next Steps:"
echo "=========================="
echo ""
echo "If all checks are ✅, try:"
echo "  oh-my-opencode run \"ultrawork: help me refactor this code\""
echo ""
echo "If issues persist, the setup may need adjustment for your specific OpenCode version/configuration."
