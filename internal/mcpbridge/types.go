// Package mcpbridge provides Anthropic's MCP Code Mode adapted for Hermes 3's native function calling format.
// It combines the efficiency of code-based tool orchestration with Hermes 3's JSON tool calling capabilities.
//
// Architecture Overview:
//
// 1. Virtual Filesystem Layer (virtual_fs.go):
//   - Creates the illusion of a Python environment with importable tool modules
//   - MCP server tools become Python modules with function signatures
//   - Dynamically generates Python code from MCP tool definitions
//
// 2. Code Translator Layer (translator.go):
//   - Parses Python code written by the LLM
//   - Extracts tool calls (imports, function calls)
//   - Converts to Hermes 3 JSON format: {"name": "tool_name", "arguments": {...}}
//   - Handles dependency resolution between tool calls
//
// 3. Hermes Integration Layer (hermes.go):
//   - Generates system prompts in Hermes 3 native format
//   - Implements 3-core-tool progressive disclosure:
//   - python: Execute code with tool orchestration
//   - search_available_modules: Discover MCP servers
//   - inspect_module: Get detailed tool documentation
//
// 4. Progressive Discovery (discovery.go):
//   - Enables efficient tool discovery without loading all upfront
//   - 97% token reduction vs traditional approach (1.5K vs 50K tokens)
//
// 5. Result Management (result_manager.go):
//   - Keeps large results out of LLM context
//   - Stores in execution environment, allows filtering before LLM sees them
//
// 6. State Persistence (state.go):
//   - Maintains Python namespace across multiple turns
//   - Variables persist like in a real Python session
//
// Usage:
//
//	// Create MCP bridge
//	bridge := mcpbridge.New(
//	    mcpbridge.WithWorkspace("/path/to/workspace"),
//	    mcpbridge.WithMCPServers(servers...),
//	)
//
//	// Generate Hermes 3 system prompt
//	prompt := bridge.GenerateSystemPrompt()
//
//	// Handle tool calls from Hermes 3
//	result, err := bridge.ExecuteToolCall(ctx, toolCall)
//
// For more details, see docs/development/MCP_CODE_BRIDGE_DESIGN.md
package mcpbridge

import (
	"context"
	"encoding/json"
)

// ToolCall represents a function call in Hermes 3 native format
// This is the format Hermes 3 emits: {"name": "tool_name", "arguments": {...}}
type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolDefinition defines a tool with its JSON schema parameters
// This matches the format used in Hermes 3 <tools> XML block
type ToolDefinition struct {
	Type     string       `json:"type"`     // Always "function"
	Function ToolFunction `json:"function"` // Tool details
}

// ToolFunction contains the actual function definition
type ToolFunction struct {
	Name        string                 `json:"name"`        // Function name
	Description string                 `json:"description"` // What the tool does
	Parameters  map[string]interface{} `json:"parameters"`  // JSON schema for parameters
}

// ParsedToolCall represents a tool call extracted from Python code
// This is the intermediate format between Python and Hermes JSON
type ParsedToolCall struct {
	Server       string                 // MCP server name (e.g., "google-drive")
	Tool         string                 // Tool name (e.g., "get_document")
	Arguments    map[string]interface{} // Parsed arguments from Python call
	ReturnVar    string                 // Variable name for result (e.g., "doc")
	Dependencies []string               // Variables this call depends on
}

// ModuleSummary provides high-level info about an MCP server module
// Used for progressive discovery (cheap - 10-50 tokens)
type ModuleSummary struct {
	Name        string   `json:"name"`        // Server name
	Description string   `json:"description"` // What the server does
	ToolCount   int      `json:"tool_count"`  // Number of available tools
	Categories  []string `json:"categories"`  // Tool categories
}

// VirtualModule represents a Python module generated from an MCP server
type VirtualModule struct {
	Name       string           // Python module name (e.g., "google_drive")
	ServerName string           // MCP server name (e.g., "google-drive")
	Tools      []ToolDefinition // Tools from this server
	PythonCode string           // Generated Python module content
}

// ExecutionResult contains the outcome of executing Python code
type ExecutionResult struct {
	Results   map[string]interface{} // Variable assignments from code
	NewState  map[string]interface{} // Complete updated Python namespace
	ToolCalls []ParsedToolCall       // Tool calls that were executed
	Error     error                  // Any execution error
}

// MCPClientInterface defines the interface for MCP server clients
// Implementations connect via stdio or SSE
type MCPClientInterface interface {
	// ListTools returns all tools available on this server
	ListTools(ctx context.Context) ([]ToolDefinition, error)

	// CallTool executes a specific tool with given arguments
	CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (interface{}, error)

	// GetServerInfo returns metadata about the server
	GetServerInfo(ctx context.Context) (ServerInfo, error)
}

// ServerInfo contains metadata about an MCP server
type ServerInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Categories  []string `json:"categories"`
}

// StoredResult represents a large result stored by reference
type StoredResult struct {
	ID       string                 // Reference ID (e.g., "__result_123")
	Data     interface{}            // Actual result data
	Size     int                    // Estimated token size
	Metadata map[string]interface{} // Additional context
}

// Config contains configuration for the MCP bridge
type Config struct {
	WorkspaceDir       string   // Directory for persistent state
	EnablePersistence  bool     // Save state to disk
	ResultThreshold    int      // Token threshold for result storage
	MaxTurns           int      // Maximum turns per session
	EnableTokenization bool     // Tokenize PII before LLM context
	AllowedMCPServers  []string // Whitelist of MCP servers
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		WorkspaceDir:       "./workspace",
		EnablePersistence:  true,
		ResultThreshold:    1000,
		MaxTurns:           50,
		EnableTokenization: false,
		AllowedMCPServers:  []string{}, // Empty = all allowed
	}
}

// CoreTools contains the 3 core tools for progressive disclosure
var CoreTools = []ToolDefinition{
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
						"type":        "string",
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
						"type":        "string",
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
						"type":        "string",
						"description": "Name of module to inspect (e.g., 'google-drive')",
					},
					"detail_level": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"overview", "signatures", "full"},
						"default":     "signatures",
						"description": "Level of detail",
					},
				},
				"required": []string{"module_name"},
			},
		},
	},
}

// estimateTokenSize provides a rough estimate of token count for data
// This is used to determine if result should be stored by reference
func estimateTokenSize(data interface{}) int {
	// Simple heuristic: ~4 chars per token for JSON
	jsonBytes, _ := json.Marshal(data)
	return len(jsonBytes) / 4
}
