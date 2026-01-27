# Issue 2: Complex Calculation Handling

## Research Findings

### Root Cause: Tool Call Parsing Logic Too Restrictive

**Problem Location**: `/home/josh/clai/internal/llm/agent.go`  
**Lines**: 354-360

**Current Code (BROKEN)**:
```go
if strings.HasPrefix(content, `{"tool_calls"`) {
    // Only detects tool calls at VERY BEGINNING of content
    toolCalls := a.parseToolCalls(content)
    // ... process tool calls
}
```

**Issue**: Models often embed tool calls within explanatory text, not at content start. This causes complete failure of tool execution for complex calculations.

### Test 19: Multi-Step Scientific Reasoning

#### Expected Behavior:
1. Model receives rocket physics query
2. Generates Python tool calls with correct physics equations  
3. Tool calls executed → get numerical results
4. Model synthesizes final answer with values: 3265306, 816, 1632

#### Actual Behavior:
1. Model receives rocket physics query ✓
2. Generates correct tool calls embedded in text ✓
3. **Parser fails to detect tool calls** (not at content start) ❌
4. Tool calls never executed ❌
5. Model never receives calculation results ❌
6. Final answer has explanations but no numbers ❌

#### Evidence from Benchmark:
```
Model generates: "I'll solve this step-by-step... {"tool_calls": [...]}"
Parser checks: strings.HasPrefix(content, `{"tool_calls"`) → false
Result: tool_calls_collected=0, execution skipped
Final response: physics explanations only, missing numerical values
```

### Correct Physics Calculations (Verified)
- Maximum height: `8000²/(2×9.8) = 3,265,306 m` ✓
- Time to max height: `8000/9.8 = 816 s` ✓  
- Total flight time: `2×816 = 1632 s` ✓
- **Model physics reasoning is correct**, calculation logic is correct, **parsing is broken**

### Affected Tests
1. **Test 3** - Simple Calculation (392) - also fails due to same issue
2. **Test 6** - Complex Python Calculation (385) - potentially affected  
3. **Test 18** - Advanced Mathematical Reasoning - likely affected
4. Any test requiring **embedded tool calls in explanatory text**

### Why This Happens

#### Model Behavior:
LLMs are trained to explain their reasoning process before providing tool calls:
1. "Let me solve this step-by-step..."
2. "First, I'll use Python to calculate..."
3. "Here's the calculation: {"tool_calls": [...]}"

#### Parser Limitation:
The `HasPrefix` check only succeeds when tool calls are at the VERY start, which is uncommon for detailed explanations.

### Impact Assessment
- **Severity**: HIGH
- **Scope**: Affects all complex calculation benchmarks
- **False Negative**: Makes capable model appear incompetent
- **Benchmark Integrity**: Results don't reflect actual model abilities

## Implementation Plan

### Phase 1: Fix Tool Call Detection (Critical)
```go
// Replace HasPrefix with Contains
if strings.Contains(content, `"tool_calls"`) {
    // Detects tool calls anywhere in content
    toolCalls := a.parseToolCalls(content)
    // ... process tool calls
}
```

### Phase 2: Improve JSON Extraction
1. Use robust JSON parsing instead of string parsing
2. Handle fragmented/malformed tool calls from streaming
3. Extract tool calls from within explanatory text

### Phase 3: Add Fallback Mechanisms
1. When tool call parsing fails, prompt model for alternative format
2. Implement error recovery for calculation failures
3. Add debugging output for tool call detection issues

### Phase 4: Enhanced Testing
1. Create test cases for embedded tool calls
2. Test with various explanation styles
3. Verify all affected benchmarks pass

## Files to Modify
- `/home/josh/clai/internal/llm/agent.go` (lines 354-360)
- `/home/josh/clai/internal/llm/agent.go` (parseToolCallsFromContent method)

## Risk Assessment
- **Risk Level**: LOW (parser improvement, no logic changes)
- **Testing Required**: HIGH (multiple affected tests)
- **Rollback Plan**: Simple revert of detection logic