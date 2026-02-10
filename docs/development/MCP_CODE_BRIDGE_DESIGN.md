# MCP Code Bridge: Hermes 3 Native CodeAct Implementation

## Overview

This document describes the implementation of Anthropic's MCP Code Mode adapted for Hermes 3's native function calling format. The goal is to combine the efficiency of code-based tool orchestration with Hermes 3's JSON tool calling capabilities.

**Status:** Design Phase  
**Priority:** High  
**Estimated Implementation Time:** 4-6 weeks

---

## Problem Statement

### Current Tool Calling Limitations

1. **Token Overload**: All 100+ tool definitions load into system prompt upfront (~50K tokens)
2. **Sequential Execution**: Agent calls one tool at a time, waits for result, calls next
3. **Context Bloat**: Large tool results flow through LLM context window repeatedly
4. **No Composition**: Cannot easily combine tools (loops, conditionals, data flow)

### What MCP Code Mode Solves

1. **Progressive Loading**: Agent discovers tools on-demand via filesystem metaphor
2. **Code Orchestration**: Write Python to call multiple tools in one turn
3. **Context Efficiency**: Large results stay in execution environment
4. **Natural Composition**: Python control flow (loops, conditionals)

### What Hermes 3 Brings

- Native JSON function calling: `{"name": "tool_name", "arguments": {...}}`
- Trained on function calling patterns
- Uses `<tools>` XML wrapper in system prompt (container only)
- Pydantic-style JSON schema for tool definitions

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     User Query                                   │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│              Hermes 3 System Prompt (3 core tools)                │
│  ┌─────────────┐  ┌─────────────────┐  ┌─────────────────────┐ │
│  │   python    │  │ search_modules  │  │   inspect_module    │ │
│  └─────────────┘  └─────────────────┘  └─────────────────────┘ │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│              LLM Response (JSON function call)                   │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                 MCP Code Bridge Layer                            │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  if tool == "python":                                      │  │
│  │    1. Parse Python AST                                     │  │
│  │    2. Extract imports → identify MCP servers             │  │
│  │    3. Extract function calls → map to tool calls         │  │
│  │    4. Execute via MCP clients                            │  │
│  │    5. Return results as Python namespace                 │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  if tool == "search_modules":                            │  │
│  │    → Return list of available MCP servers               │  │
│  └──────────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  if tool == "inspect_module":                            │  │
│  │    → Return Python function signatures                   │  │
│  └──────────────────────────────────────────────────────────┘  │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                    MCP Servers                                   │
│    google-drive    salesforce    slack    notion    ...          │
└─────────────────────────────────────────────────────────────────┘
```

---

## Core Components

### 1. Virtual Filesystem Layer (`virtual_fs.go`)

**Purpose**: Create the illusion of a Python environment with importable tool modules

**Key Insight**: The agent thinks it's writing Python code importing modules, but these modules are dynamically generated representations of MCP server tools.

**Implementation**:

```go
// MCP server tools become Python modules
type VirtualModule struct {
    Name        string              // e.g., "google_drive"
    ServerName  string              // e.g., "google-drive"
    Tools       []ToolDefinition    // Tools exposed by this server
    PythonCode  string              // Generated Python module content
}

// Generate Python module from MCP tools
func (vfs *VirtualFS) GenerateModule(serverName string) (*VirtualModule, error) {
    // 1. List tools from MCP server
    tools, err := vfs.mcpClient.ListTools(serverName)
    if err != nil {
        return nil, err
    }
    
    // 2. Generate Python module with function signatures
    module := &VirtualModule{
        Name:       sanitizeModuleName(serverName),
        ServerName: serverName,
        Tools:      tools,
    }
    
    // 3. Create Python code with docstrings
    var code strings.Builder
    code.WriteString(fmt.Sprintf(`"""%s Tools Module

Auto-generated module for MCP server: %s

Available functions:
`, serverName, serverName))
    
    for _, tool := range tools {
        sig := generatePythonSignature(tool)
        code.WriteString(fmt.Sprintf("- %s\n", sig))
    }
    code.WriteString(`"""` + "\n\n")
    
    // 4. Generate each function with proper typing
    for _, tool := range tools {
        funcCode := generatePythonFunction(tool)
        code.WriteString(funcCode + "\n\n")
    }
    
    module.PythonCode = code.String()
    return module, nil
}
```

**Example Generated Python Module**:

```python
"""Google Drive Tools Module

Auto-generated module for MCP server: google-drive

Available functions:
- get_document(document_id: str, fields: Optional[str] = None) -> dict
- list_files(folder_id: str = "root", page_size: int = 100) -> list
- search_documents(query: str) -> list
"""

def get_document(document_id: str, fields: Optional[str] = None) -> dict:
    """
    Retrieves a document from Google Drive
    
    Args:
        document_id: The ID of the document to retrieve
        fields: Specific fields to return (optional)
    
    Returns:
        Document object with title, body content, metadata, permissions
    """
    pass  # Implementation injected by MCP bridge

def list_files(folder_id: str = "root", page_size: int = 100) -> list:
    """
    Lists files in a Google Drive folder
    
    Args:
        folder_id: ID of the folder (default: root)
        page_size: Number of files to return per page
    
    Returns:
        List of file objects with id, name, mimeType, modifiedTime
    """
    pass

def search_documents(query: str) -> list:
    """
    Searches for documents in Google Drive
    
    Args:
        query: Search query string
    
    Returns:
        List of matching document objects
    """
    pass
```

**Why This Works**:
- Hermes 3 knows Python (trained on Python code)
- Python signatures are self-documenting
- Type hints help the model understand expected inputs/outputs
- Docstrings provide detailed tool descriptions without loading all at once

---

### 2. Code Translator Layer (`translator.go`)

**Purpose**: Parse Python code written by the LLM and convert it to Hermes 3 JSON tool calls

**Key Insight**: We're not executing Python (security risk), we're parsing it to extract what tools the agent wants to call.

**Implementation**:

```go
// Parse Python AST to extract tool calls
type CodeTranslator struct {
    vfs *VirtualFS
}

type ParsedToolCall struct {
    Server       string                 // e.g., "google-drive"
    Tool         string                 // e.g., "get_document"
    Arguments    map[string]interface{} // Parsed from Python call
    ReturnVar    string                 // Variable name for result (e.g., "doc")
    Dependencies []string               // Variables this call depends on
}

func (ct *CodeTranslator) ParsePythonCode(code string, state map[string]interface{}) ([]ParsedToolCall, error) {
    // Use Python parser (github.com/go-python/gpython or regex-based for MVP)
    // Walk AST looking for:
    // - Import statements (to map aliases to servers)
    // - Function calls (to extract tool invocations)
    // - Variable assignments (to track data flow)
    
    // Step 1: Parse imports
    // `import servers.google_drive as gd` → alias "gd" = "google-drive" server
    imports := parseImports(code)
    
    // Step 2: Find function calls
    // `doc = gd.get_document("abc123")` → 
    //   ParsedToolCall{
    //     Server: "google-drive",
    //     Tool: "get_document",
    //     Arguments: {"document_id": "abc123"},
    //     ReturnVar: "doc",
    //   }
    calls := parseFunctionCalls(code, imports)
    
    // Step 3: Analyze data flow
    // If call uses `doc` from previous call, it's a dependency
    for i, call := range calls {
        call.Dependencies = findDependencies(call, state, calls[:i])
    }
    
    return calls, nil
}
```

**Example Parse Flow**:

```python
# Agent writes:
import servers.google_drive as gd
import servers.salesforce as sf

doc = gd.get_document("abc123")
leads = sf.query("SELECT Id FROM Lead WHERE Name LIKE '%Q4%'")
sf.update_record(
    object_type="Lead",
    record_id=leads[0]['Id'],
    data={"Notes": doc['content']}
)
```

```go
// Parsed result:
[]ParsedToolCall{
    {
        Server: "google-drive",
        Tool: "get_document", 
        Arguments: {"document_id": "abc123"},
        ReturnVar: "doc",
        Dependencies: [],
    },
    {
        Server: "salesforce",
        Tool: "query",
        Arguments: {"soql_query": "SELECT Id FROM Lead WHERE Name LIKE '%Q4%'"},
        ReturnVar: "leads",
        Dependencies: [],
    },
    {
        Server: "salesforce",
        Tool: "update_record",
        Arguments: {
            "object_type": "Lead",
            "record_id": "{{leads[0]['Id']}}",  // Template for dependency
            "data": {"Notes": "{{doc['content']}}"},
        },
        ReturnVar: "",
        Dependencies: ["leads", "doc"],
    },
}
```

**Dependency Resolution**:

```go
func (ct *CodeTranslator) executeWithDependencies(
    ctx context.Context,
    calls []ParsedToolCall,
    state map[string]interface{},
) (map[string]interface{}, error) {
    results := make(map[string]interface{})
    
    for _, call := range calls {
        // Resolve dependencies from state or previous results
        args := resolveArguments(call.Arguments, state, results)
        
        // Execute via MCP
        result, err := ct.vfs.mcpClient.CallTool(ctx, call.Server, call.Tool, args)
        if err != nil {
            return nil, fmt.Errorf("tool %s.%s failed: %w", call.Server, call.Tool, err)
        }
        
        // Store result
        if call.ReturnVar != "" {
            results[call.ReturnVar] = result
        }
    }
    
    return results, nil
}
```

---

### 3. Hermes 3 Integration Layer (`hermes.go`)

**Purpose**: Generate system prompts in Hermes 3's native format with the 3-core-tool progressive disclosure system

**Hermes 3 Format**:

```
<<|im_start|>>system
You are a function calling AI model with code execution capabilities.

<tools>
[JSON tool definitions here]
</tools>

For each function call return a JSON object with the following schema:
{'title': 'FunctionCall', 'type': 'object', 'properties': {'arguments': {'title': 'Arguments', 'type': 'object'}, 'name': {'title': 'Name', 'type': 'string'}}, 'required': ['name', 'arguments']}
<oes>
```

**Implementation**:

```go
// Three core tools for progressive disclosure
type CoreTools struct {
    Python           ToolDefinition  // Execute Python code
    SearchModules    ToolDefinition  // Discover available servers
    InspectModule    ToolDefinition  // Get detailed tool docs
}

func (hi *HermesIntegration) GenerateSystemPrompt(servers []string) string {
    var prompt strings.Builder
    
    // Hermes 3 native format header
    prompt.WriteString("<<|im_start|>>system\n")
    prompt.WriteString("You are a function calling AI model with Python code execution capabilities.\n\n")
    
    // Instructions for progressive disclosure workflow
    prompt.WriteString("## Available Resources\n")
    prompt.WriteString("You have access to a virtual Python environment with MCP tools organized as modules.\n")
    prompt.WriteString("Follow this workflow:\n\n")
    prompt.WriteString("1. DISCOVER: Call search_available_modules() to see what servers exist\n")
    prompt.WriteString("2. INSPECT: Call inspect_module() to understand specific tool functions\n")
    prompt.WriteString("3. CODE: Write Python code using the python() tool to orchestrate tool calls\n\n")
    
    // Core tool definitions in Hermes format
    prompt.WriteString("<tools>\n")
    for _, tool := range hi.coreTools {
        toolJSON, _ := json.Marshal(tool)
        prompt.WriteString(string(toolJSON) + "\n")
    }
    prompt.WriteString("</tools>\n\n")
    
    // Response format instruction (Hermes 3 expects this)
    prompt.WriteString("For each function call, return a JSON object with this schema:\n")
    prompt.WriteString(`{"name": "function_name", "arguments": {...}}` + "\n\n")
    
    // Python environment context
    prompt.WriteString("## Python Environment\n")
    prompt.WriteString("- The filesystem at /servers/ contains tool modules\n")
    prompt.WriteString("- Import modules: `import servers.google_drive as gd`\n")
    prompt.WriteString("- Call tools: `doc = gd.get_document(\"abc123\")`\n")
    prompt.WriteString("- Process data: `filtered = [d for d in docs if d['size'] > 1000]`\n")
    prompt.WriteString("- Variables persist between python() calls\n\n")
    
    prompt.WriteString("<|im_end|>\n")
    
    return prompt.String()
}
```

**Core Tool Definitions**:

```go
var coreTools = []Tool{
    {
        Type: "function",
        Function: ToolFunction{
            Name: "python",
            Description: `Execute Python code with access to MCP tools via imports.

Write Python code to:
1. Import tool modules: import servers.google_drive as gd
2. Call tools: doc = gd.get_document("abc123")
3. Process data: filtered = [d for d in docs if d['size'] > 1000]
4. Use control flow: for, if, while
5. Variables persist between calls

The code will be parsed and tool calls executed via MCP. Results returned as Python variables.`,
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "code": map[string]interface{}{
                        "type": "string",
                        "description": "Python code to execute. Use imports to access tools.",
                    },
                },
                "required": []string{"code"},
            },
        },
    },
    {
        Type: "function",
        Function: ToolFunction{
            Name: "search_available_modules",
            Description: `List available MCP server modules.

Use this to discover what tools are available without loading them all.
Returns a list of module names like ["google-drive", "salesforce", "slack"].

Call this first before using any tools to see what's available.`,
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "keyword": map[string]interface{}{
                        "type": "string",
                        "description": "Optional filter keyword to search for specific modules",
                    },
                },
            },
        },
    },
    {
        Type: "function",
        Function: ToolFunction{
            Name: "inspect_module",
            Description: `Get detailed documentation for a specific tool module.

Returns Python-style function signatures and docstrings.
Use this to understand how to call specific tools before writing code.

Detail levels:
- "overview": Just function names
- "signatures": Function signatures with types
- "full": Complete docstrings with examples`,
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "module_name": map[string]interface{}{
                        "type": "string",
                        "description": "Name of module to inspect (e.g., 'google-drive')",
                    },
                    "detail_level": map[string]interface{}{
                        "type": "string",
                        "enum": []string{"overview", "signatures", "full"},
                        "default": "signatures",
                        "description": "Level of detail",
                    },
                },
                "required": []string{"module_name"},
            },
        },
    },
}
```

---

### 4. Progressive Discovery Layer (`discovery.go`)

**Purpose**: Enable efficient tool discovery without loading all definitions upfront

**Token Savings**:

| Approach | Tokens in System Prompt | Per-Turn Discovery |
|----------|------------------------|-------------------|
| Traditional | 50,000 (100 tools × 500 tokens) | 0 |
| Progressive | 1,500 (3 core tools) | 100-500 |
| **Savings** | **97%** | - |

**Implementation**:

```go
type DiscoveryEngine struct {
    vfs *VirtualFS
}

type ModuleSummary struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    ToolCount   int    `json:"tool_count"`
    Categories  []string `json:"categories"`
}

// High-level discovery (cheap - 10-50 tokens)
func (de *DiscoveryEngine) SearchModules(keyword string) []ModuleSummary {
    var results []ModuleSummary
    
    for name, server := range de.vfs.mcpServers {
        if keyword == "" || strings.Contains(name, keyword) {
            results = append(results, ModuleSummary{
                Name:        name,
                Description: server.Description,
                ToolCount:   len(server.Tools),
                Categories:  server.Categories,
            })
        }
    }
    
    return results
}

// Detailed inspection (more expensive - 100-500 tokens)
func (de *DiscoveryEngine) InspectModule(name string, detailLevel string) string {
    server := de.vfs.mcpServers[name]
    module, _ := de.vfs.GenerateModule(name)
    
    switch detailLevel {
    case "overview":
        // Just names: "Available functions: get_document, list_files, search_documents"
        return getToolNames(server)
        
    case "signatures":
        // Python-style signatures with types
        // def get_document(document_id: str, fields: Optional[str] = None) -> dict
        return getToolSignatures(server)
        
    case "full":
        // Complete Python module with docstrings
        return module.PythonCode
    }
}
```

**Why Detail Levels Matter**:

```python
# Agent workflow with progressive disclosure:

# Turn 1: Search (10 tokens returned)
Assistant: I'll help you work with your meeting notes. Let me first see what tools are available.
Call: search_available_modules(keyword="document")
Result: [
    {"name": "google-drive", "description": "Google Drive file operations", "tool_count": 8},
    {"name": "notion", "description": "Notion workspace integration", "tool_count": 12},
    {"name": "dropbox", "description": "Dropbox file management", "tool_count": 6}
]

# Turn 2: Inspect (100 tokens returned)
Assistant: Google Drive looks relevant. Let me see what functions it provides.
Call: inspect_module(module_name="google-drive", detail_level="signatures")
Result: """
Available functions:
- get_document(document_id: str, fields: Optional[str] = None) -> dict
- list_files(folder_id: str = "root", page_size: int = 100) -> list
- search_documents(query: str) -> list
- upload_file(name: str, content: bytes, folder_id: str = "root") -> dict
"""

# Turn 3: Code (orchestrates multiple tools)
Assistant: Perfect! Now I'll write Python code to find and process your meeting notes.
Call: python(code="""
import servers.google_drive as gd

# Search for meeting notes
docs = gd.search_documents("meeting notes Q4")
if not docs:
    raise Exception("No meeting notes found")

# Get the most recent one
doc = gd.get_document(docs[0]['id'])

# Extract and process content
lines = doc['content'].split('\n')
action_items = [line for line in lines if line.startswith('- [ ]')]

print(f"Found {len(action_items)} action items in {doc['name']}")
""")
```

**Total tokens for discovery**: ~150 tokens vs 50,000 in traditional approach

---

### 5. Result Management Layer (`result_manager.go`)

**Purpose**: Keep large results out of LLM context by storing them in the execution environment

**The Problem**:

```
Without Result Management:
┌─────────────┐
│ Google Drive │───(10,000 rows)───>┌──────────┐
│   Query     │                     │   LLM    │───( overwhelmed )
└─────────────┘                     │ Context  │
                                    └──────────┘

With Result Management:
┌─────────────┐
│ Google Drive │───(10,000 rows)───>┌─────────────┐
│   Query     │                     │ Execution   │
└─────────────┘                     │ Environment │
                                    └──────┬──────┘
                                           │
            ┌──────────┐                   │ filter
            │   LLM    │<──( 5 rows )──────┘
            │ Context  │
            └──────────┘
```

**Implementation**:

```go
type ResultManager struct {
    workspace string                    // Directory for persisted results
    refs      map[string]interface{}    // In-memory result cache
    counter   int                       // For generating unique IDs
}

type StoredResult struct {
    ID       string
    Data     interface{}
    Size     int       // Token size estimate
    Metadata map[string]interface{}
}

// Store large result and return reference
func (rm *ResultManager) StoreResult(data interface{}) *StoredResult {
    refID := fmt.Sprintf("__result_%d", rm.counter)
    rm.counter++
    
    // Estimate token size
    size := estimateTokenSize(data)
    
    stored := &StoredResult{
        ID:   refID,
        Data: data,
        Size: size,
    }
    
    rm.refs[refID] = stored
    
    // Persist to workspace if large
    if size > 1000 {  // Threshold for persistence
        rm.saveToDisk(refID, data)
    }
    
    return stored
}

// Generate Python representation for LLM
func (rm *ResultManager) GetPythonRepresentation(refID string) string {
    stored := rm.refs[refID]
    
    // For large results, return reference object with preview
    if stored.Size > 1000 {
        preview := generatePreview(stored.Data, 5)  // First 5 items
        return fmt.Sprintf(`ResultRef("%s", count=%d, preview=%s)`,
            refID, getItemCount(stored.Data), preview)
    }
    
    // Small results can be returned inline
    return toPythonRepr(stored.Data)
}

// Execute filtering in Go (not in LLM)
func (rm *ResultManager) FilterResult(refID string, filterFunc func(interface{}) bool) interface{} {
    stored := rm.refs[refID]
    
    // Apply filter in execution environment
    filtered := filterData(stored.Data, filterFunc)
    
    // Store filtered result
    return rm.StoreResult(filtered)
}
```

**Example Usage**:

```python
# Agent writes code that handles large data:

# Step 1: Query returns 10,000 rows
all_orders = salesforce.query("SELECT * FROM Orders")
# Bridge stores result: __result_1 = [10,000 orders]
# Returns to LLM: all_orders = ResultRef("__result_1", count=10000, preview=[...])

# Step 2: Filter in execution environment (no LLM tokens!)
pending = [o for o in all_orders if o['status'] == 'pending'][:50]
# Bridge executes filter in Go
# Returns: pending = ResultRef("__result_2", count=50, preview=[...])

# Step 3: LLM sees only 50 items
print(f"Processing {len(pending)} pending orders...")
for order in pending:
    process(order)
```

**Privacy Bonus**: Data can be tokenized (e.g., `[EMAIL_1]`) before entering LLM context, with real values only in execution environment

---

### 6. State Persistence Layer (`state.go`)

**Purpose**: Maintain Python namespace across multiple turns like a real Python session

**The Problem**:

```
Traditional Agent (stateless):
┌────────┐      ┌────────┐      ┌────────┐
│ Turn 1 │  ──>  │ Turn 2 │  ──>  │ Turn 3 │
│ x = 5  │       │ x = ???│       │ ???    │  (must re-fetch)
└────────┘      └────────┘      └────────┘

Code Mode Agent (stateful):
┌────────┐      ┌────────┐      ┌────────┐
│ Turn 1 │  ──>  │ Turn 2 │  ──>  │ Turn 3 │
│ x = 5  │       │ x = 5  │       │ x = 5  │  (persists)
│        │       │ y = x*2│       │ z = x+y│
└────────┘      └────────┘      └────────┘
```

**Implementation**:

```go
type PersistentState struct {
    Variables   map[string]interface{}  // Python namespace
    Files       map[string]string       // Files in workspace/
    Imports     []string                // Currently imported modules
    TurnCount   int
    WorkspaceDir string
}

// Serialize state to Python code that recreates it
func (ps *PersistentState) Serialize() string {
    var code strings.Builder
    
    // Re-import modules
    for _, imp := range ps.Imports {
        code.WriteString(fmt.Sprintf("import %s\n", imp))
    }
    if len(ps.Imports) > 0 {
        code.WriteString("\n")
    }
    
    // Restore variables with comments
    code.WriteString("# === Session State ===\n")
    for name, value := range ps.Variables {
        repr := toPythonRepr(value)
        code.WriteString(fmt.Sprintf("%s = %s\n", name, repr))
    }
    code.WriteString("# === End State ===\n\n")
    
    return code.String()
}

// Update state from execution results
func (ps *PersistentState) Update(results map[string]interface{}, imports []string) {
    // Add new imports
    for _, imp := range imports {
        if !contains(ps.Imports, imp) {
            ps.Imports = append(ps.Imports, imp)
        }
    }
    
    // Update variables
    for name, value := range results {
        ps.Variables[name] = value
    }
    
    ps.TurnCount++
    
    // Persist to disk
    ps.Save()
}

// Save to workspace/session_state.json
func (ps *PersistentState) Save() error {
    data, _ := json.Marshal(ps)
    return os.WriteFile(
        filepath.Join(ps.WorkspaceDir, "session_state.json"),
        data,
        0644,
    )
}
```

**Example Persistence**:

```python
# Turn 1: Initial setup
import servers.google_drive as gd
doc = gd.get_document("abc123")
processed = analyze(doc)

# State after Turn 1:
# Variables: {doc: {...}, processed: {...}}
# Imports: ["servers.google_drive"]

# Turn 2: Continue from previous state
# Context prepended automatically:
"""
import servers.google_drive as gd

# === Session State ===
doc = {...}
processed = {...}
# === End State ===

# Write code to continue:
"""

# Agent continues seamlessly:
print(f"Previously processed {len(processed)} items")
more_data = fetch_additional()
combined = processed + more_data

# State after Turn 2:
# Variables: {doc: {...}, processed: {...}, more_data: {...}, combined: {...}}
```

**Benefits**:
- Natural programming workflow (variables persist)
- No need to re-fetch expensive data
- Can resume long-running tasks after interruption
- Agent can build complex state over time

---

## Implementation Roadmap

### Phase 1: Foundation (Week 1-2)

**Goals**: Create the virtual filesystem and basic translation layer

**Tasks**:
1. Create `internal/mcpbridge/` package structure
2. Implement `VirtualFS` with module generation
3. Create Python signature generator from JSON schemas
4. Build basic Python AST parser (or use regex for MVP)
5. Implement `CodeTranslator.ParsePythonCode()`

**Deliverables**:
- Virtual filesystem generates Python modules
- Can parse simple Python code to extract tool calls
- Unit tests for parsing edge cases

### Phase 2: Hermes Integration (Week 3-4)

**Goals**: Connect to Hermes 3 native format with 3-core-tool system

**Tasks**:
1. Implement `HermesIntegration.GenerateSystemPrompt()`
2. Define the 3 core tools (python, search_modules, inspect_module)
3. Create progressive disclosure workflow
4. Build execution loop handling tool responses

**Deliverables**:
- System prompts generate in Hermes 3 format
- Agent can search → inspect → code workflow
- Integration tests with mocked MCP servers

### Phase 3: MCP Integration (Week 5-6)

**Goals**: Connect to real MCP servers and handle execution

**Tasks**:
1. Implement MCP client connection (stdio/sse)
2. Build `executeWithDependencies()` for ordered tool calls
3. Handle error cases and retries
4. Add result filtering and state persistence

**Deliverables**:
- Can connect to real MCP servers (Google Drive, Salesforce, etc.)
- Multi-tool orchestration works end-to-end
- State persists across turns

### Phase 4: Optimization (Week 7-8)

**Goals**: Production-ready with performance optimizations

**Tasks**:
1. Result pagination for large datasets
2. Privacy-preserving data tokenization
3. Caching for repeated tool calls
4. Metrics and monitoring
5. Documentation and examples

**Deliverables**:
- Handles 10K+ row datasets efficiently
- PII tokenization working
- Performance benchmarks vs traditional approach
- User documentation

---

## File Structure

```
internal/mcpbridge/
├── README.md                    # Package documentation
├── virtual_fs.go               # Virtual filesystem with tool modules
├── virtual_fs_test.go          # Tests for module generation
├── translator.go               # Python → Hermes JSON conversion
├── translator_test.go          # Tests for code parsing
├── hermes.go                   # Hermes 3 native format integration
├── hermes_test.go              # Tests for prompt generation
├── discovery.go                # Progressive tool disclosure
├── discovery_test.go           # Tests for search/inspect
├── result_manager.go          # Context-efficient result handling
├── result_manager_test.go      # Tests for result storage/filtering
├── state.go                    # Persistent Python namespace
├── state_test.go               # Tests for state persistence
├── executor.go                 # Main execution loop
├── executor_test.go            # Integration tests
├── types.go                    # Shared type definitions
└── mcp/                        # MCP client integration
    ├── client.go               # MCP stdio/sse connection
    ├── client_test.go          # MCP client tests
    ├── server_manager.go       # Multiple server management
    └── tool_invoker.go         # Tool call execution
```

---

## Testing Strategy

### Unit Tests

```go
// virtual_fs_test.go
func TestGenerateModule(t *testing.T) {
    vfs := NewVirtualFS()
    
    // Mock MCP server with tools
    server := &MockMCPClient{
        Tools: []ToolDefinition{
            {
                Name: "get_document",
                Parameters: map[string]interface{}{
                    "properties": map[string]interface{}{
                        "document_id": map[string]interface{}{"type": "string"},
                    },
                },
            },
        },
    }
    vfs.RegisterServer("google-drive", server)
    
    // Generate module
    module, err := vfs.GenerateModule("google-drive")
    assert.NoError(t, err)
    
    // Verify Python code contains function
    assert.Contains(t, module.PythonCode, "def get_document(")
    assert.Contains(t, module.PythonCode, "document_id: str")
}

// translator_test.go
func TestParsePythonCode(t *testing.T) {
    translator := NewCodeTranslator()
    
    code := `
import servers.google_drive as gd
doc = gd.get_document("abc123")
`
    
    calls, err := translator.ParsePythonCode(code, nil)
    assert.NoError(t, err)
    assert.Len(t, calls, 1)
    assert.Equal(t, "google-drive", calls[0].Server)
    assert.Equal(t, "get_document", calls[0].Tool)
    assert.Equal(t, "abc123", calls[0].Arguments["document_id"])
}
```

### Integration Tests

```go
// executor_test.go
func TestEndToEndCodeExecution(t *testing.T) {
    // Setup
    agent := NewCodeModeAgent()
    agent.RegisterMCPServer(&MockGoogleDriveServer{})
    agent.RegisterMCPServer(&MockSalesforceServer{})
    
    // Execute multi-tool workflow
    result, err := agent.Run(`
        Get my latest meeting notes from Google Drive and add them to 
        the Q4 lead in Salesforce
    `)
    
    assert.NoError(t, err)
    assert.Contains(t, result, "Updated lead")
    
    // Verify both servers were called
    assert.Equal(t, 1, agent.ExecutionCount["google-drive"])
    assert.Equal(t, 2, agent.ExecutionCount["salesforce"])  // query + update
}
```

### Benchmarks

```go
// Benchmark token usage
func BenchmarkProgressiveDiscovery(b *testing.B) {
    agent := NewCodeModeAgent()
    agent.Load100Tools()
    
    b.Run("Traditional", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            agent.SendAllToolsInPrompt()
        }
    })
    
    b.Run("Progressive", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            agent.SearchThenInspect("google-drive")
        }
    })
}
```

---

## Migration Path

### Backward Compatibility

The MCP Code Bridge will coexist with the existing tool system:

```go
// internal/llm/agent.go
func (a *Agent) DetermineApproach(model string) ToolApproach {
    if tools.IsHermesStyleModel(model) && a.Config.EnableMCPBridge {
        return MCPCodeBridgeApproach
    }
    return TraditionalToolApproach
}
```

### Configuration

```go
// Config option in clai configuration
type Config struct {
    // ... existing fields
    
    // MCP Code Bridge settings
    EnableMCPBridge     bool     `json:"enable_mcp_bridge"`
    MCPCodeBridgeServers []string `json:"mcp_bridge_servers"`
}
```

---

## Success Metrics

1. **Token Efficiency**: 90%+ reduction in system prompt tokens vs traditional approach
2. **Performance**: Multi-tool orchestration in single turn (vs multiple turns)
3. **Context Efficiency**: Large results (>1000 tokens) never enter LLM context
4. **Adoption**: Successfully handles 100+ MCP server tools
5. **Reliability**: <1% parsing error rate (vs 2.4% in unstructured code)

---

## Open Questions

1. **Python Parser**: Use full AST parser (heavy) or regex-based (lightweight but fragile)?
2. **MCP Server Discovery**: Auto-discover from filesystem or explicit configuration?
3. **Error Handling**: Retry failed tool calls automatically or ask agent?
4. **Security**: Sandboxing for any code execution (even parsing)?
5. **State Sharing**: Share state across different MCP servers or isolate?

---

## References

- [CodeAct Paper](https://arxiv.org/abs/2402.01030) - Apple ML Research
- [Anthropic MCP Code Mode](https://www.anthropic.com/engineering/code-execution-with-mcp)
- [Hugging Face Structured CodeAgents](https://huggingface.co/blog/structured-codeagent)
- [Hermes 3 Technical Report](https://arxiv.org/pdf/2408.11857)
- [MCP Specification](https://modelcontextprotocol.io/)

---

## Next Steps

1. ✅ **Document Complete** (This file)
2. 🔄 **Review & Feedback** (Team review of design)
3. ⏳ **Phase 1 Implementation** (Start with `virtual_fs.go`)
4. ⏳ **Phase 2 Integration** (Connect to existing agent)
5. ⏳ **Testing & Validation** (Benchmarks, real MCP servers)

**Ready to start Phase 1 implementation?** The first file to create is `internal/mcpbridge/virtual_fs.go`.
