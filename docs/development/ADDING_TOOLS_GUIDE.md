# How to Add New Tools to MCP Code Bridge

This guide explains the three ways to add tools to the MCP Code Bridge system.

---

## Overview

The MCP Code Bridge provides a virtual filesystem where tools appear as Python modules. There are three ways to add tools:

1. **Internal Tool Adapter** - Wrap existing clai tools
2. **External MCP Server** - Connect to external MCP-compliant servers  
3. **Native Go Tools** - Write tools directly in Go

---

## Method 1: Internal Tool Adapter (Easiest)

Use this to expose existing clai tools through the MCP Code Bridge.

### Example: Adding a Calculator Tool

```go
// Step 1: Define your tool
func CreateCalculatorTool() ToolDefinition {
    return ToolDefinition{
        Type: "function",
        Function: ToolFunction{
            Name:        "calculate",
            Description: "Evaluate mathematical expressions",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "expression": map[string]interface{}{
                        "type":        "string",
                        "description": "Math expression to evaluate (e.g., '2 + 2', '15 * 3')",
                    },
                },
                "required": []string{"expression"},
            },
        },
    }
}

// Step 2: Create executor function
func executeCalculator(toolCall ToolCall) (string, error) {
    expression, ok := toolCall.Arguments["expression"].(string)
    if !ok {
        return "", fmt.Errorf("expression parameter required")
    }
    
    // Evaluate expression using your favorite math library
    result, err := evaluateMath(expression)
    if err != nil {
        return "", err
    }
    
    return fmt.Sprintf("%v", result), nil
}

// Step 3: Create adapter with your tool
tools := []ToolDefinition{CreateCalculatorTool()}
adapter := mcpbridge.NewInternalToolAdapter(
    "math-tools",
    "Mathematical computation tools",
    tools,
    executeCalculator,
)

// Step 4: Register with VirtualFS
vfs := mcpbridge.NewVirtualFS()
vfs.RegisterServer("math-tools", adapter)
```

### Agent Usage

```python
# Agent discovers your tool
import servers.math_tools as math

result = math.calculate(expression="15 * 23 + 47")
# Returns: 392
```

---

## Method 2: External MCP Server (Recommended for Complex Tools)

Connect to external MCP-compliant servers (e.g., Google Drive, Slack, Notion).

### Example: Connecting Google Drive

```go
// Create external MCP client
client := mcpbridge.NewExternalMCPClient(
    "google-drive",                           // Server name
    "npx",                                    // Command to run
    []string{"-y", "@modelcontextprotocol/server-google-drive"},  // Args
    nil,                                      // Environment variables
    "",                                       // URL (empty = stdio)
)

// Connect to server
ctx := context.Background()
if err := client.Connect(ctx); err != nil {
    log.Fatal(err)
}
defer client.Disconnect()

// Register with VirtualFS
vfs := mcpbridge.NewVirtualFS()
vfs.RegisterServer("google-drive", client)
```

### Agent Usage

```python
# Agent discovers and uses Google Drive
import servers.google_drive as gd

# Search for documents
docs = gd.search_documents("meeting notes Q4")

# Get specific document
doc = gd.get_document(docs[0]['id'])
print(doc['content'])
```

### Pre-built MCP Servers Available

| Server | Installation | Purpose |
|--------|--------------|---------|
| google-drive | `npx -y @modelcontextprotocol/server-google-drive` | File operations |
| slack | `npx -y @modelcontextprotocol/server-slack` | Messaging |
| notion | `npx -y @modelcontextprotocol/server-notion` | Docs & databases |
| github | `npx -y @modelcontextprotocol/server-github` | Git operations |
| postgres | `npx -y @modelcontextprotocol/server-postgres` | Database |
| sqlite | `npx -y @modelcontextprotocol/server-sqlite` | Local database |

---

## Method 3: Native Go Tools (Most Control)

Write tools directly in Go for maximum performance and control.

### Example: File System Tool

```go
// Define the tool
type FileSystemTool struct{}

func (t *FileSystemTool) ListTools(ctx context.Context) ([]ToolDefinition, error) {
    return []ToolDefinition{
        {
            Type: "function",
            Function: ToolFunction{
                Name:        "read_file",
                Description: "Read contents of a file",
                Parameters: map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "path": map[string]interface{}{
                            "type":        "string",
                            "description": "Path to file",
                        },
                    },
                    "required": []string{"path"},
                },
            },
        },
        {
            Type: "function",
            Function: ToolFunction{
                Name:        "list_directory",
                Description: "List files in a directory",
                Parameters: map[string]interface{}{
                    "type": "object",
                    "properties": map[string]interface{}{
                        "path": map[string]interface{}{
                            "type":        "string",
                            "description": "Directory path",
                        },
                    },
                },
            },
        },
    }, nil
}

func (t *FileSystemTool) CallTool(
    ctx context.Context,
    toolName string,
    arguments map[string]interface{},
) (interface{}, error) {
    switch toolName {
    case "read_file":
        path := arguments["path"].(string)
        content, err := os.ReadFile(path)
        if err != nil {
            return nil, err
        }
        return map[string]string{"content": string(content)}, nil
        
    case "list_directory":
        path, _ := arguments["path"].(string)
        if path == "" {
            path = "."
        }
        entries, err := os.ReadDir(path)
        if err != nil {
            return nil, err
        }
        
        files := make([]map[string]interface{}, len(entries))
        for i, entry := range entries {
            files[i] = map[string]interface{}{
                "name":  entry.Name(),
                "is_dir": entry.IsDir(),
            }
        }
        return files, nil
        
    default:
        return nil, fmt.Errorf("unknown tool: %s", toolName)
    }
}

func (t *FileSystemTool) GetServerInfo(ctx context.Context) (ServerInfo, error) {
    return ServerInfo{
        Name:        "filesystem",
        Description: "Local file system operations",
    }, nil
}

// Register
vfs := mcpbridge.NewVirtualFS()
vfs.RegisterServer("filesystem", &FileSystemTool{})
```

---

## Complete Integration Example

Here's how to set up the full system with multiple tool sources:

```go
package main

import (
    "clai/internal/mcpbridge"
    "context"
)

func main() {
    ctx := context.Background()
    
    // Create virtual filesystem
    vfs := mcpbridge.NewVirtualFS()
    
    // 1. Add internal tools (existing clai tools)
    internalAdapter := mcpbridge.CreateDefaultToolAdapter()
    vfs.RegisterServer("clai-tools", internalAdapter)
    
    // 2. Add external MCP servers
    googleDrive := mcpbridge.NewExternalMCPClient(
        "google-drive",
        "npx",
        []string{"-y", "@modelcontextprotocol/server-google-drive"},
        nil,
        "",
    )
    if err := googleDrive.Connect(ctx); err == nil {
        vfs.RegisterServer("google-drive", googleDrive)
    }
    
    slack := mcpbridge.NewExternalMCPClient(
        "slack",
        "npx",
        []string{"-y", "@modelcontextprotocol/server-slack"},
        nil,
        "",
    )
    if err := slack.Connect(ctx); err == nil {
        vfs.RegisterServer("slack", slack)
    }
    
    // 3. Add native Go tools
    vfs.RegisterServer("filesystem", &FileSystemTool{})
    
    // Create executor
    config := mcpbridge.DefaultExecutorConfig()
    executor := mcpbridge.NewExecutor(vfs, config)
    
    // Configure LLM client (you provide this)
    llm := &MyLLMClient{}
    executor.WithLLM(llm)
    
    // Run
    result, err := executor.Run(ctx, "Summarize my meeting notes from Google Drive and post to Slack")
    if err != nil {
        panic(err)
    }
    
    println(result)
}
```

---

## Tool Discovery Flow

When an agent starts, it follows this discovery process:

```
1. System prompt (1.5K tokens) provides 3 core tools
   └─> python, search_available_modules, inspect_module

2. Agent calls: search_available_modules()
   └─> Returns: ["clai-tools", "google-drive", "slack", "filesystem"]

3. Agent calls: inspect_module("google-drive", detail="signatures")
   └─> Returns: Python function signatures

4. Agent writes Python code:
   └─> import servers.google_drive as gd
   └─> doc = gd.get_document("abc123")
```

**Total discovery cost**: ~150 tokens vs 50K tokens in traditional approach.

---

## Tool Schema Best Practices

### Good Tool Description

```go
Description: `Read a file from the local filesystem.

Use this tool when you need to:
- View file contents
- Read configuration files
- Parse data files

The path should be relative to the current working directory.
Large files may be truncated.`,
```

### Parameter Schema

```go
Parameters: map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{
        "path": map[string]interface{}{
            "type":        "string",
            "description": "Path to file (relative or absolute)",
        },
        "limit": map[string]interface{}{
            "type":        "integer",
            "description": "Maximum lines to read (for large files)",
            "default":     1000,
        },
    },
    "required": []string{"path"},
},
```

### Type Hints Matter

The MCP Code Bridge generates Python type hints from your schema:

```python
def read_file(path: str, limit: int = 1000) -> dict:
    """Read a file from the local filesystem."""
    pass
```

Use precise types (Literal for enums, Optional for nullable, etc.).

---

## Testing New Tools

### Unit Test Example

```go
func TestMyNewTool(t *testing.T) {
    // Create adapter
    tools := []ToolDefinition{CreateMyTool()}
    adapter := mcpbridge.NewInternalToolAdapter(
        "my-tools", "My tools", tools, executeMyTool,
    )
    
    // Test ListTools
    ctx := context.Background()
    listed, err := adapter.ListTools(ctx)
    if err != nil {
        t.Fatal(err)
    }
    
    if len(listed) != 1 {
        t.Errorf("Expected 1 tool, got %d", len(listed))
    }
    
    // Test CallTool
    result, err := adapter.CallTool(ctx, "my_tool", map[string]interface{}{
        "param": "value",
    })
    
    if err != nil {
        t.Errorf("Tool execution failed: %v", err)
    }
    
    // Verify result
    resultMap, ok := result.(map[string]interface{})
    if !ok {
        t.Error("Expected map result")
    }
    
    if resultMap["status"] != "success" {
        t.Error("Expected success status")
    }
}
```

### Integration Test

```go
func TestToolIntegration(t *testing.T) {
    // Full workflow
    vfs := mcpbridge.NewVirtualFS()
    vfs.RegisterServer("my-tools", CreateMyAdapter())
    
    executor := mcpbridge.NewExecutor(vfs, mcpbridge.DefaultExecutorConfig())
    executor.WithLLM(&MockLLM{
        Responses: []string{
            // Turn 1: Search modules
            `{"name": "search_available_modules", "arguments": {}}`,
            // Turn 2: Inspect module  
            `{"name": "inspect_module", "arguments": {"module_name": "my-tools"}}`,
            // Turn 3: Use tool
            `{"name": "python", "arguments": {"code": "import servers.my_tools as tools\nresult = tools.my_tool(param='test')"}}`,
            // Turn 4: Done
            "Task completed successfully",
        },
    })
    
    result, err := executor.Run(context.Background(), "Test my tool")
    if err != nil {
        t.Fatal(err)
    }
    
    if !strings.Contains(result, "completed") {
        t.Error("Expected successful completion")
    }
}
```

---

## Common Patterns

### Pattern 1: Tool Composition

```python
# Agent composes multiple tools in one Python code block
import servers.google_drive as gd
import servers.slack as slack

# Get document
doc = gd.get_document("abc123")

# Extract summary (in Python)
summary = doc['content'][:500] + "..."

# Post to Slack
slack.send_message(
    channel="general",
    text=f"Meeting summary: {summary}"
)
```

### Pattern 2: Batch Operations

```python
import servers.google_drive as gd

# Get all files
files = gd.list_files(folder_id="Q4_Meetings")

# Process in Python (no LLM tokens!)
for file in files:
    if file['name'].endswith('.txt'):
        doc = gd.get_document(file['id'])
        process_document(doc)
```

### Pattern 3: Error Handling

```python
import servers.salesforce as sf

try:
    leads = sf.query("SELECT Id, Name FROM Lead WHERE Status = 'New'")
    for lead in leads[:10]:  # Limit to first 10
        sf.update_record(
            object_type="Lead",
            record_id=lead['Id'],
            data={"Status": "Contacted"}
        )
    print(f"Updated {len(leads)} leads")
except Exception as e:
    print(f"Error: {e}")
```

---

## Troubleshooting

### Tool Not Found

```
Error: server not found: my-server
```

**Solution**: Check that you registered the server with `vfs.RegisterServer()`

### Tool Execution Failed

```
Error: tool my_tool failed: missing required parameter
```

**Solution**: Ensure your tool executor validates all required parameters

### Connection Refused (External MCP)

```
Error: failed to start MCP server: exec: "npx": executable file not found
```

**Solution**: Install Node.js and npm/npx, or use full path to executable

---

## Next Steps

1. **Start Simple**: Use Method 1 (Internal Adapter) for existing clai tools
2. **Add External**: Connect to 1-2 external MCP servers (Method 2)
3. **Build Native**: Create custom Go tools for domain-specific needs (Method 3)
4. **Test Thoroughly**: Use the test patterns shown above
5. **Profile**: Check token usage with `executor.GetStats()`

---

## Reference

- [MCP Specification](https://modelcontextprotocol.io/)
- [MCP Server Registry](https://github.com/modelcontextprotocol/servers)
- [MCP Python SDK](https://github.com/modelcontextprotocol/python-sdk)
- [Hermes 3 Technical Report](https://arxiv.org/pdf/2408.11857)
