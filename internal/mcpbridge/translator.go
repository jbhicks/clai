package mcpbridge

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CodeTranslator parses Python code written by the LLM and converts it to Hermes 3 JSON tool calls.
//
// It analyzes Python AST (or uses regex-based parsing for MVP) to extract:
// - Import statements (maps aliases to MCP servers)
// - Function calls (extracts tool invocations)
// - Variable assignments (tracks data flow and dependencies)
//
// Example:
//
//	translator := NewCodeTranslator(vfs)
//
//	code := `
//	import servers.google_drive as gd
//	doc = gd.get_document("abc123")
//	leads = sf.query("SELECT Id FROM Lead")
//	`
//
//	calls, err := translator.ParsePythonCode(code, nil)
//	// calls[0]: {Server: "google-drive", Tool: "get_document", Arguments: {...}, ReturnVar: "doc"}
//	// calls[1]: {Server: "salesforce", Tool: "query", Arguments: {...}, ReturnVar: "leads"}
//
// After parsing, the tool calls can be executed via MCP and results returned to the agent.
type CodeTranslator struct {
	vfs *VirtualFS
}

// NewCodeTranslator creates a new code translator
func NewCodeTranslator(vfs *VirtualFS) *CodeTranslator {
	return &CodeTranslator{vfs: vfs}
}

// ParseResult contains extracted tool calls and their mapping to Python variables
type ParseResult struct {
	ToolCalls []ParsedToolCall       // Tool calls extracted from code
	Imports   map[string]string      // alias -> server name
	Variables map[string]interface{} // Current Python namespace
}

// ParsePythonCode analyzes Python code and extracts tool calls
//
// For MVP, this uses regex-based parsing. Future versions should use a proper Python AST parser
// like github.com/go-python/gpython for more robust handling.
func (ct *CodeTranslator) ParsePythonCode(code string, state map[string]interface{}) (*ParseResult, error) {
	result := &ParseResult{
		ToolCalls: make([]ParsedToolCall, 0),
		Imports:   make(map[string]string),
		Variables: make(map[string]interface{}),
	}

	// Copy existing state
	for k, v := range state {
		result.Variables[k] = v
	}

	// Step 1: Parse import statements
	// Pattern: import servers.google_drive as gd
	// Pattern: import servers.google_drive
	// Pattern: from servers.google_drive import get_document
	ct.parseImports(code, result)

	// Step 2: Find function calls
	// Pattern: doc = gd.get_document("abc123")
	// Pattern: leads = sf.query("SELECT ...")
	ct.parseFunctionCalls(code, result)

	// Step 3: Analyze data flow for dependencies
	ct.analyzeDependencies(result, state)

	return result, nil
}

// parseImports extracts import statements from Python code
func (ct *CodeTranslator) parseImports(code string, result *ParseResult) {
	// Pattern 1: import servers.google_drive as gd
	// Pattern 2: import servers.google_drive
	importPattern := regexp.MustCompile(`import\s+servers\.([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:as\s+([a-zA-Z_][a-zA-Z0-9_]*))?`)
	matches := importPattern.FindAllStringSubmatch(code, -1)

	for _, match := range matches {
		moduleName := match[1] // e.g., "google_drive"
		alias := match[2]      // e.g., "gd" or empty

		// Find server by module name
		serverName := ct.findServerByModule(moduleName)
		if serverName == "" {
			continue
		}

		// Use alias if provided, otherwise use module name
		if alias == "" {
			alias = moduleName
		}

		result.Imports[alias] = serverName
	}

	// Pattern 3: from servers.google_drive import get_document
	fromImportPattern := regexp.MustCompile(`from\s+servers\.([a-zA-Z_][a-zA-Z0-9_]*)\s+import\s+([a-zA-Z_][a-zA-Z0-9_,\s]+)`)
	fromMatches := fromImportPattern.FindAllStringSubmatch(code, -1)

	for _, match := range fromMatches {
		moduleName := match[1]
		tools := match[2] // Could be "tool1, tool2, tool3"

		serverName := ct.findServerByModule(moduleName)
		if serverName == "" {
			continue
		}

		// Split tools and create import for each
		toolList := strings.Split(tools, ",")
		for _, tool := range toolList {
			tool = strings.TrimSpace(tool)
			// Create pseudo-alias for direct tool import
			result.Imports[tool] = serverName
		}
	}
}

// findServerByModule finds the server name that matches a Python module name
func (ct *CodeTranslator) findServerByModule(moduleName string) string {
	// Try exact match first
	if _, ok := ct.vfs.GetServer(moduleName); ok {
		return moduleName
	}

	// Try with hyphen instead of underscore
	hyphenated := strings.ReplaceAll(moduleName, "_", "-")
	if _, ok := ct.vfs.GetServer(hyphenated); ok {
		return hyphenated
	}

	// Search all servers for matching module name
	for _, serverName := range ct.vfs.ListServers() {
		if sanitizeModuleName(serverName) == moduleName {
			return serverName
		}
	}

	return ""
}

// parseFunctionCalls extracts function calls from Python code
func (ct *CodeTranslator) parseFunctionCalls(code string, result *ParseResult) {
	// Pattern: alias.tool_name(args) or variable = alias.tool_name(args)
	// Captures: variable (optional), alias, tool_name, arguments
	callPattern := regexp.MustCompile(`(?:([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*)?([a-zA-Z_][a-zA-Z0-9_]*)\.([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)`)

	matches := callPattern.FindAllStringSubmatch(code, -1)

	for _, match := range matches {
		returnVar := match[1] // e.g., "doc" or empty
		alias := match[2]     // e.g., "gd"
		toolName := match[3]  // e.g., "get_document"
		argsStr := match[4]   // e.g., '"abc123", fields="title,content"'

		// Look up server from alias
		serverName, ok := result.Imports[alias]
		if !ok {
			// Try direct function call (from X import Y style)
			if s, ok := result.Imports[toolName]; ok {
				serverName = s
			} else {
				continue // Unknown alias
			}
		}

		// Parse arguments
		args := ct.parseArguments(argsStr)

		call := ParsedToolCall{
			Server:    serverName,
			Tool:      toolName,
			Arguments: args,
			ReturnVar: returnVar,
		}

		result.ToolCalls = append(result.ToolCalls, call)

		// Track variable if it has a return value
		if returnVar != "" {
			result.Variables[returnVar] = placeholderValue(toolName)
		}
	}
}

// parseArguments parses Python function arguments into a map
func (ct *CodeTranslator) parseArguments(argsStr string) map[string]interface{} {
	args := make(map[string]interface{})

	if strings.TrimSpace(argsStr) == "" {
		return args
	}

	// Split by comma, but respect nested structures
	argList := splitArguments(argsStr)

	for i, arg := range argList {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}

		// Check for keyword argument: key=value
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			args[key] = ct.parseValue(value)
		} else {
			// Positional argument - use index as key
			args[fmt.Sprintf("_arg_%d", i)] = ct.parseValue(arg)
		}
	}

	return args
}

// splitArguments splits comma-separated arguments while respecting nested structures
func splitArguments(argsStr string) []string {
	var args []string
	var current strings.Builder
	depth := 0
	inString := false
	stringChar := rune(0)

	for _, r := range argsStr {
		switch {
		case inString:
			current.WriteRune(r)
			if r == stringChar {
				// Check for escaped char
				if current.Len() > 1 {
					str := current.String()
					if str[len(str)-2] != '\\' {
						inString = false
					}
				}
			}
		case r == '"' || r == '\'':
			inString = true
			stringChar = r
			current.WriteRune(r)
		case r == '(' || r == '[' || r == '{':
			depth++
			current.WriteRune(r)
		case r == ')' || r == ']' || r == '}':
			depth--
			current.WriteRune(r)
		case r == ',' && depth == 0:
			args = append(args, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// parseValue parses a Python value into Go interface{}
func (ct *CodeTranslator) parseValue(value string) interface{} {
	value = strings.TrimSpace(value)

	// String literal
	if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
		(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
		return value[1 : len(value)-1]
	}

	// Boolean
	if value == "True" {
		return true
	}
	if value == "False" {
		return false
	}

	// None
	if value == "None" {
		return nil
	}

	// Integer
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}

	// Float
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal
	}

	// List [a, b, c]
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		items := splitArguments(value[1 : len(value)-1])
		result := make([]interface{}, len(items))
		for i, item := range items {
			result[i] = ct.parseValue(item)
		}
		return result
	}

	// Dict {k: v}
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		// Simple dict parsing
		result := make(map[string]interface{})
		content := value[1 : len(value)-1]
		pairs := splitArguments(content)
		for _, pair := range pairs {
			if strings.Contains(pair, ":") {
				parts := strings.SplitN(pair, ":", 2)
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				// Remove quotes from key if present
				key = strings.Trim(key, `"'`)
				result[key] = ct.parseValue(val)
			}
		}
		return result
	}

	// Variable reference - keep as string for now
	return value
}

// analyzeDependencies determines which tool calls depend on which variables
func (ct *CodeTranslator) analyzeDependencies(result *ParseResult, state map[string]interface{}) {
	// Build set of all available variables (from state + previous calls)
	availableVars := make(map[string]bool)
	for k := range state {
		availableVars[k] = true
	}

	for i, call := range result.ToolCalls {
		// Check if any argument references an available variable
		deps := make([]string, 0)

		for _, val := range call.Arguments {
			if strVal, ok := val.(string); ok {
				// Check if this is a variable reference or variable access
				if availableVars[strVal] || isVariableAccess(strVal) {
					// Extract variable name from expressions like "doc['content']"
					varName := extractVariableName(strVal)
					if varName != "" && availableVars[varName] {
						deps = append(deps, varName)
					}
				}
			}
		}

		result.ToolCalls[i].Dependencies = deps

		// Mark return variable as available for subsequent calls
		if call.ReturnVar != "" {
			availableVars[call.ReturnVar] = true
		}
	}
}

// isVariableAccess checks if a string appears to access a variable
func isVariableAccess(s string) bool {
	// Pattern: varname, varname[key], varname.method(), etc.
	return regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(?:\[|\.|$)`).MatchString(s)
}

// extractVariableName extracts the base variable name from an expression
func extractVariableName(expr string) string {
	// doc['content'] -> doc
	// leads[0]['Id'] -> leads
	// result.method() -> result

	parts := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)`).FindStringSubmatch(expr)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// placeholderValue creates a placeholder value based on tool name
func placeholderValue(toolName string) interface{} {
	// Return different types based on tool name patterns
	if strings.Contains(toolName, "list") || strings.Contains(toolName, "search") {
		return []interface{}{}
	}
	if strings.Contains(toolName, "get") || strings.Contains(toolName, "fetch") {
		return map[string]interface{}{}
	}
	return ""
}

// ExecuteWithDependencies executes tool calls in order, resolving dependencies
func (ct *CodeTranslator) ExecuteWithDependencies(
	ctx context.Context,
	calls []ParsedToolCall,
	state map[string]interface{},
) (*ExecutionResult, error) {
	results := &ExecutionResult{
		Results:   make(map[string]interface{}),
		ToolCalls: calls,
		NewState:  make(map[string]interface{}),
	}

	// Copy existing state
	for k, v := range state {
		results.NewState[k] = v
	}

	// Execute calls in order (assuming they're already ordered correctly)
	for _, call := range calls {
		// Resolve arguments that reference variables
		resolvedArgs := ct.resolveArguments(call.Arguments, results.NewState)

		// Get MCP client for this server
		client, ok := ct.vfs.GetServer(call.Server)
		if !ok {
			return nil, fmt.Errorf("server not found: %s", call.Server)
		}

		// Execute tool via MCP
		result, err := client.CallTool(ctx, call.Tool, resolvedArgs)
		if err != nil {
			results.Error = fmt.Errorf("tool %s.%s failed: %w", call.Server, call.Tool, err)
			return results, results.Error
		}

		// Store result
		if call.ReturnVar != "" {
			results.Results[call.ReturnVar] = result
			results.NewState[call.ReturnVar] = result
		}
	}

	return results, nil
}

// resolveArguments replaces variable references with actual values
func (ct *CodeTranslator) resolveArguments(args map[string]interface{}, state map[string]interface{}) map[string]interface{} {
	resolved := make(map[string]interface{})

	for key, val := range args {
		switch v := val.(type) {
		case string:
			// Check if it's a variable reference
			if stateVal, ok := state[v]; ok {
				resolved[key] = stateVal
			} else if isVariableAccess(v) {
				// Try to evaluate variable access
				evaluated := ct.evaluateVariableAccess(v, state)
				if evaluated != nil {
					resolved[key] = evaluated
				} else {
					resolved[key] = v // Keep original if can't evaluate
				}
			} else {
				resolved[key] = v
			}
		default:
			resolved[key] = v
		}
	}

	return resolved
}

// evaluateVariableAccess evaluates Python-style variable access
// e.g., "doc['content']", "leads[0]['Id']", "result.name"
func (ct *CodeTranslator) evaluateVariableAccess(expr string, state map[string]interface{}) interface{} {
	// This is a simplified evaluator - full implementation would need proper parsing

	parts := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)(.*)$`).FindStringSubmatch(expr)
	if len(parts) < 2 {
		return nil
	}

	varName := parts[1]
	access := parts[2]

	// Get base variable
	val, ok := state[varName]
	if !ok {
		return nil
	}

	// Apply access patterns
	remaining := access
	for remaining != "" {
		remaining = strings.TrimSpace(remaining)

		// Array/dict index: [key]
		if strings.HasPrefix(remaining, "[") {
			endIdx := strings.Index(remaining, "]")
			if endIdx == -1 {
				break
			}

			key := remaining[1:endIdx]
			remaining = remaining[endIdx+1:]

			// Evaluate key
			keyVal := ct.parseValue(key)

			switch v := val.(type) {
			case map[string]interface{}:
				if strKey, ok := keyVal.(string); ok {
					val = v[strKey]
				}
			case []interface{}:
				if intKey, ok := keyVal.(int); ok && intKey < len(v) {
					val = v[intKey]
				}
			default:
				return nil
			}
		} else if strings.HasPrefix(remaining, ".") {
			// Attribute access: .attr
			parts := regexp.MustCompile(`^\.([a-zA-Z_][a-zA-Z0-9_]*)`).FindStringSubmatch(remaining)
			if len(parts) < 2 {
				break
			}

			attr := parts[1]
			remaining = remaining[len(parts[0]):]

			switch v := val.(type) {
			case map[string]interface{}:
				val = v[attr]
			default:
				return nil
			}
		} else {
			break
		}
	}

	return val
}
