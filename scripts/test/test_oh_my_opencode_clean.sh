#!/bin/bash
# Clean test of oh-my-opencode without interfering OpenCode sessions

echo "🧹 Preparing clean oh-my-opencode test environment..."
echo ""

# Check llama server (assumed to be managed by systemd)
echo "1. Checking llama server..."
if curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "✅ Llama server is running via systemd"
else
    echo "❌ Llama server is NOT running"
    echo "   Start it with: sudo systemctl start llama-server"
    echo "   Or check status: sudo systemctl status llama-server"
    exit 1
fi

echo ""
echo "2. Testing oh-my-opencode..."

# Set up environment
export PATH="$HOME/.nvm/versions/node/v24.12.0/bin:$HOME/.bun/bin:$PATH"
source ~/.bash_profile

# Test doctor command
echo "   Running doctor check..."
if bun run oh-my-opencode doctor > /tmp/doctor.log 2>&1; then
    echo "✅ Doctor check passed"
    echo "   Results: $(grep "passed\|failed\|warnings\|skipped" /tmp/doctor.log | tail -1)"
else
    echo "❌ Doctor check failed"
    echo "   Check: cat /tmp/doctor.log"
fi

echo ""
echo "3. Testing oh-my-opencode run..."

# Test basic run command
if timeout 30 bun run oh-my-opencode run "ultrawork: explain what oh-my-opencode does in 3 sentences" > /tmp/omo_run.log 2>&1; then
    if grep -q "oh-my-opencode\|agent\|orchestrat" /tmp/omo_run.log; then
        echo "✅ oh-my-opencode run successful"
        echo "   Output contains expected content"
    else
        echo "⚠️  Run completed but output unclear"
        echo "   Check: cat /tmp/omo_run.log"
    fi
else
    echo "❌ oh-my-opencode run failed or timed out"
    echo "   Check: cat /tmp/omo_run.log"
fi

echo ""
echo "📋 Test Results Summary:"
echo "========================"
echo ""
echo "If successful, oh-my-opencode can be used for CLAI Ralph MVP!"
echo "If failed, we'll proceed with native CLAI orchestrator implementation."
echo ""
echo "Log files:"
echo "  Doctor: /tmp/doctor.log"
echo "  Run: /tmp/omo_run.log"
echo "  Server: /tmp/llama-server.log"
