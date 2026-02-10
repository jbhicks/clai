package mcpbridge

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// HermesIntegration generates system prompts in Hermes 3 native format.
//
// Hermes 3 Format:
//
//	<<|im_start|>>system
//	You are a function calling AI model...
//
//	<tools>
//	{JSON tool definitions}
//	</tools>
//
//	For each function call return a JSON object...
//	<oes>
//
// This implementation provides the 3-core-tool progressive disclosure system:
// 1. python: Execute code with tool orchestration
// 2. search_available_modules: Discover MCP servers
// 3. inspect_module: Get detailed tool documentation
//
// Example:
//
//	hermes := NewHermesIntegration(vfs)
//	prompt := hermes.GenerateSystemPrompt([]string{"google-drive", "salesforce"})
//	// Returns Hermes 3 formatted system prompt with ~1.5K tokens instead of 50K
type HermesIntegration struct {
	vfs *VirtualFS
}

// NewHermesIntegration creates a new Hermes integration
func NewHermesIntegration(vfs *VirtualFS) *HermesIntegration {
	return &HermesIntegration{vfs: vfs}
}

// GenerateSystemPrompt creates a Hermes 3 native format system prompt
//
// The prompt includes:
// - Instructions for progressive disclosure workflow
// - 3 core tool definitions (python, search_modules, inspect_module)
// - Response format instructions
// - Python environment context
func (hi *HermesIntegration) GenerateSystemPrompt() string {
	var prompt strings.Builder

	// Hermes 3 native format header
	prompt.WriteString("<<|im_start|>>system\n")
	prompt.WriteString("You are a function calling AI model with Python code execution capabilities.\n\n")

	// Progressive disclosure instructions
	prompt.WriteString("## Available Resources\n")
	prompt.WriteString("You have access to a virtual Python environment with MCP tools organized as modules.\n")
	prompt.WriteString("Instead of loading all tools at once, follow this workflow:\n\n")
	prompt.WriteString("1. DISCOVER: Call search_available_modules() to see what servers exist\n")
	prompt.WriteString("2. INSPECT: Call inspect_module() to understand specific tool functions\n")
	prompt.WriteString("3. CODE: Write Python code using the python() tool to orchestrate tool calls\n\n")

	// Core tool definitions in Hermes format
	prompt.WriteString("## Available Tools\n<tools>\n")

	for _, tool := range CoreTools {
		toolJSON, _ := json.Marshal(tool)
		prompt.WriteString(string(toolJSON))
		prompt.WriteString("\n")
	}

	prompt.WriteString("</tools>\n\n")

	// Response format instruction (Hermes 3 expects this exact format)
	prompt.WriteString("## Response Format\n")
	prompt.WriteString("For each function call, return a JSON object with this schema:\n")
	prompt.WriteString(`{"name": "function_name", "arguments": {...}}` + "\n\n")

	// Python environment context
	prompt.WriteString("## Python Environment\n")
	prompt.WriteString("The filesystem at /servers/ contains tool modules:\n")

	// List available servers
	servers := hi.vfs.ListServers()
	if len(servers) > 0 {
		prompt.WriteString("\nAvailable servers:\n")
		for _, server := range servers {
			alias := moduleAlias(server)
			prompt.WriteString(fmt.Sprintf("- %s (import as: %s)\n", server, alias))
		}
	}

	prompt.WriteString("\nKey features:\n")
	prompt.WriteString("- Import modules: import servers.google_drive as gd\n")
	prompt.WriteString("- Call tools: doc = gd.get_document(\"abc123\")\n")
	prompt.WriteString("- Process data: filtered = [d for d in docs if d['size'] > 1000]\n")
	prompt.WriteString("- Use control flow: for, if, while loops\n")
	prompt.WriteString("- Variables persist between python() calls\n\n")

	prompt.WriteString("<oes>\n")

	return prompt.String()
}

// GenerateMinimalPrompt creates the smallest possible system prompt
// Use this when token count is critical
func (hi *HermesIntegration) GenerateMinimalPrompt() string {
	var prompt strings.Builder

	prompt.WriteString("<<|im_start|>>system\n")
	prompt.WriteString("You are a function calling AI model with code execution.\n\n")
	prompt.WriteString("Workflow: 1) search_available_modules(), 2) inspect_module(), 3) python()\n\n")

	prompt.WriteString("<tools>\n")
	for _, tool := range CoreTools {
		toolJSON, _ := json.Marshal(tool)
		prompt.WriteString(string(toolJSON))
		prompt.WriteString("\n")
	}
	prompt.WriteString("</tools>\n\n")

	prompt.WriteString(`Response: {"name": "function_name", "arguments": {...}}` + "\n")
	prompt.WriteString("<oes>\n")

	return prompt.String()
}

// FormatToolResponse formats a tool result for Hermes 3
//
// Hermes 3 expects tool results in a specific format that includes
// the tool_call_id and the result content
func (hi *HermesIntegration) FormatToolResponse(toolCallID string, result interface{}, isError bool) string {
	var response strings.Builder

	response.WriteString("<|im_start|>>tool\n")
	response.WriteString(fmt.Sprintf("<tool_call_id>%s</tool_call_id>\n", toolCallID))

	if isError {
		response.WriteString(fmt.Sprintf("<error>%v</error>\n", result))
	} else {
		// Convert result to JSON string
		resultJSON, _ := json.Marshal(result)
		response.WriteString(fmt.Sprintf("<result>%s</result>\n", string(resultJSON)))
	}

	response.WriteString("<|im_end|>>\n")

	return response.String()
}

// FormatUserMessage formats a user message for Hermes 3
func (hi *HermesIntegration) FormatUserMessage(content string) string {
	return fmt.Sprintf("<<|im_start|>>user\n%s\n<|im_end|>>\n", content)
}

// FormatAssistantMessage formats an assistant message for Hermes 3
func (hi *HermesIntegration) FormatAssistantMessage(content string) string {
	return fmt.Sprintf("<<|im_start|>>assistant\n%s\n<|im_end|>>\n", content)
}

// IsHermesStyleToolCall checks if a string is a valid Hermes 3 tool call
func IsHermesStyleToolCall(s string) bool {
	// Try to parse as JSON
	var call ToolCall
	if err := json.Unmarshal([]byte(s), &call); err != nil {
		return false
	}

	// Must have name and arguments
	return call.Name != "" && call.Arguments != nil
}

// ParseHermesToolCall parses a Hermes 3 tool call from JSON string
func ParseHermesToolCall(s string) (*ToolCall, error) {
	var call ToolCall
	if err := json.Unmarshal([]byte(s), &call); err != nil {
		return nil, fmt.Errorf("invalid tool call format: %w", err)
	}

	if call.Name == "" {
		return nil, fmt.Errorf("tool call missing 'name' field")
	}

	return &call, nil
}

// ExtractToolCallsFromResponse extracts all tool calls from a Hermes 3 response
// The response may contain multiple tool calls mixed with regular text
func ExtractToolCallsFromResponse(response string) ([]ToolCall, string) {
	var toolCalls []ToolCall
	var remainingText strings.Builder

	// Pattern to find JSON tool calls
	// Look for {"name": "...", "arguments": {...}} patterns
	jsonPattern := regexp.MustCompile(`\{[^{}]*"name"[^{}]*"arguments"[^{}]*\}`)

	// Find all potential JSON objects
	indices := jsonPattern.FindAllStringIndex(response, -1)

	lastEnd := 0
	for _, idx := range indices {
		start, end := idx[0], idx[1]

		// Add text before this JSON
		if start > lastEnd {
			remainingText.WriteString(response[lastEnd:start])
		}

		// Try to parse as tool call
		jsonStr := response[start:end]
		if call, err := ParseHermesToolCall(jsonStr); err == nil {
			toolCalls = append(toolCalls, *call)
		} else {
			// Not a valid tool call, keep as text
			remainingText.WriteString(jsonStr)
		}

		lastEnd = end
	}

	// Add remaining text
	if lastEnd < len(response) {
		remainingText.WriteString(response[lastEnd:])
	}

	return toolCalls, strings.TrimSpace(remainingText.String())
}

// CountTokens provides a rough estimate of token count for a string
// This helps compare our progressive approach vs traditional approach
func CountTokens(s string) int {
	// Rough estimate: ~4 chars per token for English text
	return len(s) / 4
}

// CalculateTokenSavings shows the benefit of progressive disclosure
func (hi *HermesIntegration) CalculateTokenSavings(traditionalToolCount int) map[string]interface{} {
	// Traditional approach: all tools in system prompt
	traditionalTokens := traditionalToolCount * 500 // ~500 tokens per tool

	// Our approach: 3 core tools
	ourPrompt := hi.GenerateSystemPrompt()
	ourTokens := CountTokens(ourPrompt)

	// With progressive discovery
	discoveryTokens := 150 // ~150 tokens for search + inspect
	totalWithDiscovery := ourTokens + discoveryTokens

	return map[string]interface{}{
		"traditional_approach": traditionalTokens,
		"mcp_code_bridge":      ourTokens,
		"with_discovery":       totalWithDiscovery,
		"savings_percent":      float64(traditionalTokens-totalWithDiscovery) / float64(traditionalTokens) * 100,
		"savings_tokens":       traditionalTokens - totalWithDiscovery,
	}
}
