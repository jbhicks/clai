# Agent Mode Testing Guide

## Quick Start

```sh
# Enable agent mode
export AGENT_MODE=true

# Optional: Enable debug logging to see agent iterations
export LOG_LEVEL=DEBUG

# Run the app
make dev
```

## Test Scenarios

### 1. Simple Math Calculation
**Query:** `What is 5 + 3?`

**Expected Behavior:**
- Agent should use JavaScript code execution
- Should return `8` as the final answer
- Check `debug.log` for:
  - `[AGENT-RUN] Starting agent loop`
  - `[AGENT-CODE] Executing JavaScript code`
  - `[JS-EXEC] Running code: 5 + 3`
  - `[AGENT-COMPLETE] Final answer reached`

### 2. Multi-Step Calculation
**Query:** `Calculate 15 * 23 + 100 and log the intermediate steps`

**Expected Behavior:**
- Agent should use `log()` calls in JavaScript
- Should show intermediate values in observation
- Final answer should be `445`
- Check `debug.log` for:
  - `[JS-LOG]` entries showing intermediate steps
  - `[AGENT-OBSERVATION]` with code output

### 3. Complex Reasoning
**Query:** `What is the sum of squares of 3, 4, and 5?`

**Expected Behavior:**
- Agent should calculate 3² + 4² + 5² = 9 + 16 + 25 = 50
- May use multiple iterations to refine approach
- Check for thought process in logs

### 4. JSON Manipulation
**Query:** `Create a JSON object with name "test" and value 42, then stringify it`

**Expected Behavior:**
- Agent should use `JSON.stringify()`
- Should return properly formatted JSON string
- Demonstrates JavaScript object support

### 5. Delegation Test (Advanced)
**Query:** `Calculate both 5 + 3 and 10 * 2, then sum the results`

**Expected Behavior:**
- Agent *may* delegate to sub-agents (implementation decides)
- Should handle parallel subtasks if delegated
- Final answer should be `28` (8 + 20)
- Check `debug.log` for:
  - `[AGENT-DELEGATION] Delegating N subtasks` (if delegated)
  - `[AGENT-DELEGATE] Starting subtask` entries

## Debugging Agent Behavior

### Check Agent Iterations
```sh
tail -f debug.log | grep "AGENT"
```

### Check JavaScript Execution
```sh
tail -f debug.log | grep "JS-"
```

### Full Debug Output
```sh
export LOG_LEVEL=DEBUG
make dev
# In another terminal:
tail -f debug.log
```

## Common Issues

### Agent Loops Forever
- Check that LLM is following the structured format
- Look for `[AGENT-WARNING] No delegation or code found`
- Agent may need stronger system prompt adherence from model

### JavaScript Errors
- Check `[JS-EXEC-ERROR]` in logs
- Verify code syntax in agent response
- Goja supports ES5.1+ with partial modern features (no modules, no fetch)

### Agent Timeout
- Agent has max 20 iterations
- If task is too complex, agent returns "Task incomplete"
- Consider breaking query into smaller parts

### Sub-Agent Recursion
- Sub-agents have max 5 iterations each
- Prevents infinite delegation chains
- Check `[AGENT-DELEGATE]` logs for depth

## Expected Log Flow (Simple Math)

```
[INFO] Agent mode enabled
[AGENT-RUN] Starting agent loop with query: What is 5 + 3?
[AGENT-ITER] Iteration 1/20
[AGENT-THINK] Full response: Thought: I need to calculate...
[AGENT-PARSE] Thought: "...", Code: 15 chars, Final: ""
[AGENT-CODE] Executing JavaScript code
[JS-EXEC] Running code: 5 + 3
[JS-EXEC-OUTPUT] 8
[AGENT-OBSERVATION] Added observation: Observation (Code output): 8
[AGENT-ITER] Iteration 2/20
[AGENT-PARSE] Thought: "...", Final: "8"
[AGENT-COMPLETE] Final answer reached: 8
[AGENT-CMD] Agent completed: 8
[AGENT-RESPONSE] Agent completed: 8
```

## Performance Notes

- Simple math: ~2-3 iterations typical
- Complex tasks: May use 5-10 iterations
- Delegation adds latency (parallel but LLM-bound)
- JavaScript execution is fast (<1ms typically)
