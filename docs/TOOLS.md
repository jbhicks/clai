# Tool System Documentation

This document explains how the tool system works in `clai` and how to add new tools.

## Architecture Overview

The tool system consists of three main components:

1. **Tool Definitions** (`internal/tools/tools.go`) - Defines available tools and their JSON schemas
2. **Tool Execution** (`internal/tools/executor.go`) - Implements the actual tool logic
3. **LLM Integration** (`internal/llm/llm.go`) - Handles tool protocol conversion for different API formats

## How It Works

### 1. Tool Flow

```
User Query → LLM (with tool schemas) → LLM decides to call tool → 
Tool Executor runs tool → Result returned to LLM → LLM responds to user
```

### 2. Tool Definition Format

Tools are defined using JSON Schema for parameters. This follows the same pattern as the TypeScript MCP SDK (manual schema definition).

```go
Tool{
    Name:        "tool_name",
    Description: "What the tool does",
    Parameters: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param1": map[string]interface{}{
                "type":        "string",
                "description": "What this parameter does",
            },
            "param2": map[string]interface{}{
                "type":        "number",
                "description": "Another parameter",
            },
        },
        "required": []string{"param1"},
    },
}
```

### 3. Parameter Structs

Each tool has a corresponding Go struct for type-safe parameter unmarshaling:

```go
type ToolNameParams struct {
    Param1 string `json:"param1"`
    Param2 int    `json:"param2"`
}
```

## Adding a New Tool

Follow these steps to add a new tool:

### Step 1: Define Parameter Struct

In `internal/tools/tools.go`, add a struct for your tool's parameters:

```go
type MyNewToolParams struct {
    Input string `json:"input"`
    Count int    `json:"count"`
}
```

### Step 2: Add Tool Definition

In `internal/tools/tools.go`, add your tool to the `availableTools` slice:

```go
{
    Name:        "my_new_tool",
    Description: "Clear description of what the tool does",
    Parameters: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "input": map[string]interface{}{
                "type":        "string",
                "description": "What this parameter does",
            },
            "count": map[string]interface{}{
                "type":        "number",
                "description": "How many times to do something",
            },
        },
        "required": []string{"input"},
    },
}
```

### Step 3: Add Executor Case

In `internal/tools/executor.go`, add a case to the `ExecuteTool` switch statement:

```go
case "my_new_tool":
    var p MyNewToolParams
    if err := json.Unmarshal(params, &p); err != nil {
        return "", fmt.Errorf("error unmarshalling my_new_tool params: %w", err)
    }
    return executeMyNewTool(p)
```

### Step 4: Implement Tool Logic

In `internal/tools/executor.go`, implement the actual tool function:

```go
func executeMyNewTool(params MyNewToolParams) (string, error) {
    // Implement your tool logic here
    result := fmt.Sprintf("Processed '%s' %d times", params.Input, params.Count)
    return result, nil
}
```

### Step 5: Test Your Tool

Test the tool manually by running the application and asking the LLM to use it:

```bash
make dev  # Start the app in dev mode
```

Then in the chat interface:
```
> Can you use my_new_tool with input "hello" and count 3?
```

## JSON Schema Reference

The `Parameters` field uses standard JSON Schema. Common types:

### String Parameter
```go
"param_name": map[string]interface{}{
    "type":        "string",
    "description": "Description for the LLM",
}
```

### Number Parameter
```go
"param_name": map[string]interface{}{
    "type":        "number",
    "description": "Numeric value",
}
```

### Boolean Parameter
```go
"param_name": map[string]interface{}{
    "type":        "boolean",
    "description": "True or false value",
}
```

### Array Parameter
```go
"param_name": map[string]interface{}{
    "type":        "array",
    "description": "List of items",
    "items": map[string]interface{}{
        "type": "string",
    },
}
```

### Enum Parameter
```go
"param_name": map[string]interface{}{
    "type":        "string",
    "description": "One of the allowed values",
    "enum":        []string{"option1", "option2", "option3"},
}
```

### Required Parameters
Add parameter names to the `required` array:
```go
"required": []string{"param1", "param2"},
```

## API Format Compatibility

The tool system supports two API formats:

1. **Ollama Native Format** - Tools are passed directly in the `tools` field
2. **OpenAI-Compatible Format** - Tools are wrapped in `{type: "function", function: {...}}` structure

The conversion happens automatically in `internal/llm/llm.go` via the `convertToOpenAITools()` function. No changes are needed when adding new tools.

## Example: The Calculator Tool

Here's the complete implementation of the calculator tool as a reference:

**Parameter Struct:**
```go
type CalculatorParams struct {
    Expression string `json:"expression"`
}
```

**Tool Definition:**
```go
{
    Name:        "calculator",
    Description: "A simple calculator that evaluates a mathematical expression.",
    Parameters: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "expression": map[string]interface{}{
                "type":        "string",
                "description": "Mathematical expression to evaluate (e.g., '2 + 2', '10 * 5')",
            },
        },
        "required": []string{"expression"},
    },
}
```

**Executor Case:**
```go
case "calculator":
    var p CalculatorParams
    if err := json.Unmarshal(params, &p); err != nil {
        return "", fmt.Errorf("error unmarshalling calculator params: %w", err)
    }
    return executeCalculator(p)
```

**Implementation:**
```go
func executeCalculator(params CalculatorParams) (string, error) {
    expression, err := govaluate.NewEvaluableExpression(params.Expression)
    if err != nil {
        return "", fmt.Errorf("error creating evaluable expression: %w", err)
    }

    result, err := expression.Evaluate(nil)
    if err != nil {
        return "", fmt.Errorf("error evaluating expression: %w", err)
    }

    return fmt.Sprintf("%v", result), nil
}
```

## Best Practices

1. **Clear Descriptions** - The LLM uses descriptions to understand when to use each tool. Be specific and include examples.

2. **Error Handling** - Always return meaningful error messages that can help debug issues.

3. **Parameter Validation** - Validate parameters in your executor function, not just in the JSON schema.

4. **Type Safety** - Use Go structs for parameters to leverage compile-time type checking.

5. **Minimal Dependencies** - Prefer standard library when possible. Document any external dependencies required.

6. **Return Format** - Return strings from executor functions. Format complex results as JSON strings if needed.

7. **Testing** - Test both success and failure cases manually through the chat interface.

## Debugging Tools

Enable debug logging to see tool calls:

```bash
make dev  # Automatically sets DEBUG=true
```

Check `debug.log` for detailed logs:
```bash
tail -f debug.log
```

Look for log entries like:
- `[LLM-REQ]` - Request sent to LLM (includes tool schemas)
- `[LLM-RESP]` - Response from LLM (includes tool calls)
- `[TOOL-EXECUTION]` - Tool execution details

## Future Improvements

Potential enhancements to the tool system:

1. **Schema Generation** - Generate JSON schemas from Go struct tags
2. **Tool Validation** - Validate tool definitions at startup
3. **Tool Tests** - Add unit tests for individual tool implementations
4. **Dynamic Tool Loading** - Load tools from plugins or external files
5. **Tool Permissions** - Add confirmation prompts for dangerous operations
