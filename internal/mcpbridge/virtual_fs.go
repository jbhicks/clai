package mcpbridge

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// VirtualFS creates the illusion of a Python environment with importable tool modules.
// MCP server tools become Python modules that can be imported and called.
//
// Example:
//
//	vfs := NewVirtualFS()
//	vfs.RegisterServer("google-drive", mcpClient)
//
//	// Generate Python module for google-drive
//	module, err := vfs.GenerateModule("google-drive")
//	// module.PythonCode contains:
//	//   def get_document(document_id: str, fields: Optional[str] = None) -> dict
//	//   def list_files(folder_id: str = "root", page_size: int = 100) -> list
//	//   ... etc
type VirtualFS struct {
	servers   map[string]MCPClientInterface // server name -> MCP client
	modules   map[string]*VirtualModule     // server name -> generated module
	workspace string                        // Path for persistent storage
	mu        sync.RWMutex                  // Protects maps
}

// NewVirtualFS creates a new virtual filesystem
func NewVirtualFS() *VirtualFS {
	return &VirtualFS{
		servers:   make(map[string]MCPClientInterface),
		modules:   make(map[string]*VirtualModule),
		workspace: "./workspace",
	}
}

// WithWorkspace sets the workspace directory for persistent storage
func (vfs *VirtualFS) WithWorkspace(path string) *VirtualFS {
	vfs.workspace = path
	return vfs
}

// RegisterServer adds an MCP server to the virtual filesystem
func (vfs *VirtualFS) RegisterServer(name string, client MCPClientInterface) {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	vfs.servers[name] = client
	// Clear any cached module for this server
	delete(vfs.modules, name)
}

// UnregisterServer removes an MCP server
func (vfs *VirtualFS) UnregisterServer(name string) {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	delete(vfs.servers, name)
	delete(vfs.modules, name)
}

// ListServers returns all registered server names
func (vfs *VirtualFS) ListServers() []string {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	servers := make([]string, 0, len(vfs.servers))
	for name := range vfs.servers {
		servers = append(servers, name)
	}
	return servers
}

// GetServer returns the MCP client for a server
func (vfs *VirtualFS) GetServer(name string) (MCPClientInterface, bool) {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	client, ok := vfs.servers[name]
	return client, ok
}

// GenerateModule creates a VirtualModule for an MCP server
// This generates Python code with function signatures for all tools
func (vfs *VirtualFS) GenerateModule(serverName string) (*VirtualModule, error) {
	vfs.mu.RLock()

	// Check cache first
	if module, ok := vfs.modules[serverName]; ok {
		vfs.mu.RUnlock()
		return module, nil
	}

	// Get the MCP client
	client, ok := vfs.servers[serverName]
	vfs.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	// Fetch tools from MCP server
	tools, err := client.ListTools(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools for %s: %w", serverName, err)
	}

	// Generate Python module
	module := &VirtualModule{
		Name:       sanitizeModuleName(serverName),
		ServerName: serverName,
		Tools:      tools,
	}

	module.PythonCode = vfs.generatePythonModule(serverName, tools)

	// Cache the module
	vfs.mu.Lock()
	vfs.modules[serverName] = module
	vfs.mu.Unlock()

	return module, nil
}

// generatePythonModule creates Python code from tool definitions
func (vfs *VirtualFS) generatePythonModule(serverName string, tools []ToolDefinition) string {
	var code strings.Builder

	// Module docstring header
	code.WriteString(fmt.Sprintf(`"""%s Tools Module

Auto-generated module for MCP server: %s

This module provides access to %d tools via Python function calls.
Import this module and call functions to execute tools.

Example:
    import servers.%s as %s
    
    # Call a tool
    result = %s.some_tool(arg1="value")

Available functions:
`, serverName, serverName, len(tools), sanitizeModuleName(serverName),
		moduleAlias(serverName), moduleAlias(serverName)))

	// List all available functions
	for _, tool := range tools {
		sig := generateFunctionSignature(tool.Function)
		code.WriteString(fmt.Sprintf("- %s\n", sig))
	}

	code.WriteString(`"""` + "\n\n")

	// Import typing for type hints
	code.WriteString("from typing import Optional, Dict, List, Any\n\n")

	// Generate each function
	for _, tool := range tools {
		funcCode := generatePythonFunction(tool.Function)
		code.WriteString(funcCode)
		code.WriteString("\n\n")
	}

	return code.String()
}

// GetModulePath returns the virtual filesystem path for a module
func (vfs *VirtualFS) GetModulePath(serverName string) string {
	return filepath.Join("servers", sanitizeModuleName(serverName))
}

// GetWorkspacePath returns the path to the workspace directory
func (vfs *VirtualFS) GetWorkspacePath() string {
	return vfs.workspace
}

// sanitizeModuleName converts server name to valid Python module name
// e.g., "google-drive" -> "google_drive"
func sanitizeModuleName(name string) string {
	// Replace hyphens with underscores
	name = strings.ReplaceAll(name, "-", "_")

	// Remove any invalid characters
	valid := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	name = valid.ReplaceAllString(name, "_")

	// Ensure it starts with a letter
	if len(name) > 0 && !isLetter(name[0]) {
		name = "_" + name
	}

	return name
}

// moduleAlias creates a short alias for import
// e.g., "google-drive" -> "gd"
func moduleAlias(name string) string {
	parts := strings.Split(name, "-")
	if len(parts) == 1 {
		return name
	}

	// Create alias from initials
	var alias strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			alias.WriteByte(part[0])
		}
	}

	return alias.String()
}

// isLetter checks if byte is a letter
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// generateFunctionSignature creates a Python signature from a tool function
func generateFunctionSignature(fn ToolFunction) string {
	var sig strings.Builder
	sig.WriteString(fn.Name)
	sig.WriteString("(")

	params, ok := fn.Parameters["properties"].(map[string]interface{})
	if !ok {
		sig.WriteString(")")
		return sig.String()
	}

	required, _ := fn.Parameters["required"].([]string)
	reqSet := make(map[string]bool)
	for _, r := range required {
		reqSet[r] = true
	}

	first := true
	for paramName, paramDef := range params {
		if !first {
			sig.WriteString(", ")
		}
		first = false

		paramMap, ok := paramDef.(map[string]interface{})
		if !ok {
			sig.WriteString(paramName)
			continue
		}

		paramType := paramMap["type"].(string)
		pyType := jsonTypeToPythonType(paramType, paramMap)

		sig.WriteString(paramName)
		sig.WriteString(": ")
		sig.WriteString(pyType)

		// Add default if not required
		if !reqSet[paramName] {
			defaultVal := getDefaultValue(paramType, paramMap)
			sig.WriteString(" = ")
			sig.WriteString(defaultVal)
		}
	}

	sig.WriteString(") -> Any")
	return sig.String()
}

// generatePythonFunction creates a complete Python function with docstring
func generatePythonFunction(fn ToolFunction) string {
	var code strings.Builder

	// Function signature
	sig := generateFunctionSignature(fn)
	code.WriteString("def ")
	code.WriteString(sig)
	code.WriteString(":\n")

	// Docstring
	code.WriteString(`    """`)
	code.WriteString(fn.Description)
	code.WriteString("\n\n")

	// Args documentation
	params, ok := fn.Parameters["properties"].(map[string]interface{})
	if ok && len(params) > 0 {
		code.WriteString("    Args:\n")
		for paramName, paramDef := range params {
			paramMap, _ := paramDef.(map[string]interface{})
			desc := ""
			if d, ok := paramMap["description"].(string); ok {
				desc = d
			}

			paramType := paramMap["type"].(string)
			pyType := jsonTypeToPythonType(paramType, paramMap)

			code.WriteString(fmt.Sprintf("        %s (%s): %s\n", paramName, pyType, desc))
		}
		code.WriteString("\n")
	}

	// Returns documentation
	code.WriteString("    Returns:\n")
	code.WriteString("        Tool execution result\n")
	code.WriteString(`    """` + "\n")

	// Pass statement (actual execution handled by bridge)
	code.WriteString("    pass  # Tool execution handled by MCP bridge\n")

	return code.String()
}

// jsonTypeToPythonType converts JSON schema type to Python type annotation
func jsonTypeToPythonType(jsonType string, paramDef map[string]interface{}) string {
	switch jsonType {
	case "string":
		if enum, ok := paramDef["enum"].([]interface{}); ok && len(enum) > 0 {
			// Create Literal type for enums
			values := make([]string, len(enum))
			for i, v := range enum {
				values[i] = fmt.Sprintf("\"%v\"", v)
			}
			return fmt.Sprintf("Literal[%s]", strings.Join(values, ", "))
		}
		return "str"
	case "integer":
		return "int"
	case "number":
		return "float"
	case "boolean":
		return "bool"
	case "array":
		if items, ok := paramDef["items"].(map[string]interface{}); ok {
			itemType := "Any"
			if t, ok := items["type"].(string); ok {
				itemType = jsonTypeToPythonType(t, items)
			}
			return fmt.Sprintf("List[%s]", itemType)
		}
		return "List[Any]"
	case "object":
		return "Dict[str, Any]"
	default:
		return "Any"
	}
}

// getDefaultValue returns a sensible default value for a type
func getDefaultValue(jsonType string, paramDef map[string]interface{}) string {
	switch jsonType {
	case "string":
		if def, ok := paramDef["default"].(string); ok {
			return fmt.Sprintf("\"%s\"", def)
		}
		return "\"\""
	case "integer", "number":
		if def, ok := paramDef["default"].(float64); ok {
			return fmt.Sprintf("%v", def)
		}
		return "0"
	case "boolean":
		if def, ok := paramDef["default"].(bool); ok {
			return fmt.Sprintf("%v", def)
		}
		return "False"
	case "array":
		return "[]"
	case "object":
		return "{}"
	default:
		return "None"
	}
}
