# Issue 1: HTTP 500 Protocol Errors

## Research Findings

### Root Cause
**File**: `/home/josh/clai/internal/benchmark/server.go`  
**Lines**: 2126-2129

The `sendPromptToModel` function constructs messages incorrectly using `[]map[string]string` instead of the proper `[]llm.Message` struct:

```go
// CURRENT (INCORRECT)
"messages": []map[string]string{
    {"role": "system", "content": systemPrompt},
    {"role": "user", "content": prompt},
},

// SHOULD BE (CORRECT)
"messages": []llm.Message{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: prompt},
},
```

### Correct Message Structure
**File**: `/home/josh/clai/internal/llm/llm.go`  
**Lines**: 183-187

```go
type Message struct {
    Role       string     `json:"role"`
    Content    string     `json:"content,omitempty"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID string     `json:"tool_call_id,omitempty"`
}
```

### Why This Causes HTTP 500
The LLM server validates incoming message structure and expects `Message` struct fields. When receiving `[]map[string]string`, it rejects system/user messages but may accept assistant-generated messages (which follow different path), explaining the specific error message "All non-assistant messages must contain 'content'".

## Implementation Plan

### Phase 1: Fix Message Construction
1. Update `sendPromptToModel` function to use proper `[]llm.Message`
2. Import `llm` package in benchmark server
3. Update message construction to use struct fields (`Role`, `Content`)

### Phase 2: Testing
1. Run affected tests (13, 15) to verify fix
2. Run full benchmark suite to ensure no regression
3. Test with different model types (OpenAI, Ollama)

### Phase 3: Validation
1. Add unit test for message construction
2. Add integration test for benchmark server
3. Update documentation if needed

## Files to Modify
- `/home/josh/clai/internal/benchmark/server.go` (lines 2126-2129)
- Potential import additions at top of file

## Risk Assessment
- **Risk Level**: LOW (structural fix, no logic changes)
- **Testing Required**: Medium (verify all benchmark tests still pass)
- **Rollback Plan**: Simple revert of message construction