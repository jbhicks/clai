# Model Compatibility Matrix

This document describes tool calling support and compatibility for different LLM models used with clai.

## Tool Call Formats

clai supports two tool call formats:

1. **Structured Format** (OpenAI-compatible): JSON-based tool calls using the standard OpenAI function calling format
2. **Text-based Format**: XML-like tags with function parameters (e.g., `<tool_call>\n<function=calculator`)

## Supported Models

| Model Family | Tool Call Format | Auto-Detection | Notes |
|-------------|------------------|----------------|-------|
| Qwen3-Coder | Text-based | ✅ Yes | Uses XML-like tags; requires text parsing |
| Qwen2.5-Coder | Text-based | ✅ Yes | Uses XML-like tags; requires text parsing |
| Llama 3.x | Structured | ✅ Yes | Full OpenAI format support |
| Mistral | Structured | ✅ Yes | Full OpenAI format support |
| Other models | Auto | ⚠️ Fallback | Attempts structured, falls back to text parsing |

## Configuration

### Environment Variable

Set the `TOOL_CALL_FORMAT` environment variable to override auto-detection:

```bash
export TOOL_CALL_FORMAT=structured  # Force OpenAI structured format
export TOOL_CALL_FORMAT=text        # Force text-based parsing
export TOOL_CALL_FORMAT=auto        # Auto-detect based on model (default)
```

### Programmatic Configuration

```go
client := llm.NewClient("http://localhost:11434", "qwen3-coder:30b")
client.SetToolCallFormat("text")
```

## Format Detection Logic

clai automatically detects the appropriate format based on the model name:

```go
func detectToolCallFormat(modelName string) ToolCallFormat {
    lowerModel := strings.ToLower(modelName)
    
    // Known text-based models
    if strings.Contains(lowerModel, "qwen") {
        return TextToolCallFormat
    }
    
    // Known structured models
    if strings.Contains(lowerModel, "llama") || strings.Contains(lowerModel, "mistral") {
        return StructuredToolCallFormat
    }
    
    // Default: auto-detect
    return AutoToolCallFormat
}
```

## Text-Based Tool Call Format

### Example Output from Qwen Models

```
<tool_call>
<function=calculator
<expression=2+2
</tool_call>
```

### Parsing Behavior

1. Extracts function name from `<function=X` tag
2. Extracts parameters as key-value pairs (e.g., `<expression=2+2`)
3. Converts to standard JSON format: `{"expression":"2+2"}`
4. Strips all XML-like tags from UI output

## Structured Tool Call Format

### Example Output from Llama/Mistral Models

Models using this format emit tool calls via Ollama's native tool calling API:

```json
{
  "name": "calculator",
  "parameters": {
    "expression": "2+2"
  }
}
```

No special parsing is required; tool calls are handled directly by Ollama.

## System Prompts

### For Structured Format (Llama/Mistral)

```
When using tools, you MUST respond with valid JSON in the OpenAI function calling format.
Do NOT use XML-like tags (e.g., <tool_call>, <function=...>).
```

### For Text Format (Qwen)

No additional prompting is required. The parser handles the model's natural output format.

## Troubleshooting

### Issue: Model not detected correctly

**Solution**: Override detection using `TOOL_CALL_FORMAT=text` or `TOOL_CALL_FORMAT=structured`

### Issue: XML tags appearing in chat output

**Symptoms**: Messages contain `<tool_call>`, `<function=...>`, etc.

**Solution**: Ensure text-based format is enabled. clai should automatically strip these tags from UI output.

### Issue: Tool calls not executing

**Symptoms**: Model mentions tools but they don't execute

**Possible causes**:
1. Model using unexpected format - try `TOOL_CALL_FORMAT=text`
2. Tool schema mismatch - verify tool definitions match expected parameters
3. Check `debug.log` for parsing errors

### Issue: Model refuses to use tools

**Symptoms**: Model describes what it would do but doesn't call tools

**Solution**: 
1. Ensure model supports tool calling (e.g., use `-instruct` or chat-optimized variants)
2. Try adding explicit instruction: "Use the available tools to complete this task"

## Adding Support for New Models

To add support for a new model:

1. Test the model's tool calling output format
2. Add detection logic to `detectToolCallFormat()` in `internal/llm/llm.go`
3. If the model uses a non-standard format, implement a custom parser
4. Add test cases to `internal/llm/toolparse_test.go`
5. Update this compatibility matrix

## Technical Details

### Text Parser Implementation

Location: `internal/llm/llm.go:192` (`parseTextBasedToolCalls()`)

The parser:
- Extracts function name from `<function=X>` pattern
- Collects all `<key=value>` pairs as parameters
- Converts to JSON: `{"key": "value", ...}`
- Handles multi-line values and special characters

### Tag Stripping

Location: `internal/llm/llm.go:241` (`stripToolCallTags()`, `stripToolCallTagsUI()`)

Two variants:
- `stripToolCallTags()`: Removes tags entirely
- `stripToolCallTagsUI()`: Replaces tags with human-readable descriptions

### Format Selection Flow

```
Model initialized
     ↓
detectToolCallFormat(modelName)
     ↓
  ┌──────────────┐
  │ Model name   │
  │ contains     │
  │ "qwen"?      │
  └──────────────┘
    ↓ Yes    ↓ No
  Text    Structured/Auto
    ↓           ↓
Parse text   Use Ollama API
  output       directly
```

## Future Enhancements

Planned improvements:

1. **Retry with explicit instructions**: If structured format fails, retry with text format
2. **Metrics tracking**: Log success/failure rates per model and format
3. **Custom format plugins**: Allow users to define custom parsers for new models
4. **Format negotiation**: Automatically switch formats based on success rate
