#!/bin/bash
# Clean test of oh-my-opencode without interfering OpenCode sessions

echo "🧹 Preparing clean oh-my-opencode test environment..."
echo ""

# Stop any running OpenCode processes
echo "1. Stopping OpenCode processes..."
pkill -f "/usr/bin/opencode" 2>/dev/null || pkill -f "opencode serve" 2>/dev/null || echo "No OpenCode processes found"
sleep 2

# Check llama server
echo "2. Checking llama server..."
if curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "✅ Llama server is running"
else
    echo "❌ Llama server is NOT running"
    echo "   Starting server..."
    /home/josh/llama.cpp-rocm-wmma/build/bin/llama-server \
        -m /mnt/media/models/Qwen3-Coder-30B-A3B-Instruct-UD-Q8_K_XL.gguf \
        --host 0.0.0.0 \
        --port 8081 \
        -c 131072 \
        -ngl 999 \
        -fa on \
        -b 2048 \
        -ub 512 > /tmp/llama-server.log 2>&1 &
    SERVER_PID=$!
    echo "   Server started with PID: $SERVER_PID"
    sleep 5
    if curl -s http://localhost:8081/health > /dev/null 2>&1; then
        echo "✅ Server started successfully"
    else
        echo "❌ Server failed to start"
        exit 1
    fi
fi

echo ""
echo "3. Testing oh-my-opencode..."

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
echo "4. Testing basic oh-my-opencode run..."

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
