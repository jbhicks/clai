package mcpbridge

import (
	"context"
	"fmt"
	"strings"
)

// DiscoveryEngine enables efficient tool discovery without loading all definitions upfront.
//
// Instead of sending 100 tool definitions (~50K tokens) in the system prompt,
// the agent discovers tools on-demand via three functions:
//
// 1. SearchModules: List available MCP servers (cheap - ~10 tokens)
// 2. InspectModule: Get detailed tool documentation (medium - ~100-500 tokens)
// 3. Full documentation only when needed (expensive - ~500+ tokens)
//
// Token Savings:
// - Traditional approach: 50,000 tokens (100 tools × 500 tokens each)
// - Progressive approach: 1,500 tokens (3 core tools) + 100-500 tokens (discovery)
// - Total savings: ~97% reduction in system prompt tokens
//
// Example workflow:
//
//	engine := NewDiscoveryEngine(vfs)
//
//	// Step 1: Search (10 tokens)
//	modules := engine.SearchModules("document")
//	// Returns: [{"name": "google-drive", "tool_count": 8}, {"name": "notion", "tool_count": 12}]
//
//	// Step 2: Inspect (100 tokens)
//	docs := engine.InspectModule("google-drive", "signatures")
//	// Returns: Python signatures for all 8 tools
//
//	// Step 3: Code (write Python using discovered tools)
//	// Total discovery cost: ~110 tokens vs 4,000 tokens for full google-drive module

type DiscoveryEngine struct {
	vfs *VirtualFS
}

// NewDiscoveryEngine creates a new discovery engine
func NewDiscoveryEngine(vfs *VirtualFS) *DiscoveryEngine {
	return &DiscoveryEngine{vfs: vfs}
}

// SearchModules returns a list of available MCP server modules
//
// This is the first step in progressive disclosure. It's cheap (~10 tokens)
// and gives the agent an overview of what's available.
//
// If keyword is provided, only servers matching the keyword are returned.
func (de *DiscoveryEngine) SearchModules(keyword string) []ModuleSummary {
	var results []ModuleSummary

	servers := de.vfs.ListServers()

	for _, serverName := range servers {
		// Check if keyword matches (if provided)
		if keyword != "" && !matchesKeyword(serverName, keyword) {
			continue
		}

		// Get server info from MCP client
		client, ok := de.vfs.GetServer(serverName)
		if !ok {
			continue
		}

		info, err := client.GetServerInfo(context.Background())
		if err != nil {
			// Fall back to basic info
			info = ServerInfo{
				Name:        serverName,
				Description: fmt.Sprintf("%s MCP server", serverName),
			}
		}

		// Get tool count
		tools, err := client.ListTools(context.Background())
		toolCount := 0
		if err == nil {
			toolCount = len(tools)
		}

		summary := ModuleSummary{
			Name:        serverName,
			Description: info.Description,
			ToolCount:   toolCount,
			Categories:  info.Categories,
		}

		results = append(results, summary)
	}

	return results
}

// matchesKeyword checks if a server name matches a keyword
func matchesKeyword(serverName, keyword string) bool {
	lowerName := strings.ToLower(serverName)
	lowerKeyword := strings.ToLower(keyword)

	// Check if keyword is in name
	if strings.Contains(lowerName, lowerKeyword) {
		return true
	}

	// Check individual words
	words := strings.Split(lowerName, "-")
	for _, word := range words {
		if strings.Contains(word, lowerKeyword) {
			return true
		}
	}

	return false
}

// InspectModule returns detailed documentation for a specific module
//
// Detail levels control how much information is returned:
// - "overview": Just function names (~10-20 tokens)
// - "signatures": Function signatures with types (~50-100 tokens per tool)
// - "full": Complete docstrings with examples (~200-500 tokens per tool)
//
// The agent can request different detail levels based on what it needs:
// - Use "overview" for quick scanning
// - Use "signatures" for understanding interfaces
// - Use "full" only when implementing complex workflows
func (de *DiscoveryEngine) InspectModule(serverName string, detailLevel string) (string, error) {
	// Validate detail level
	validLevels := map[string]bool{
		"overview":   true,
		"signatures": true,
		"full":       true,
	}

	if !validLevels[detailLevel] {
		detailLevel = "signatures" // Default
	}

	// Get the MCP client
	client, ok := de.vfs.GetServer(serverName)
	if !ok {
		return "", fmt.Errorf("server not found: %s", serverName)
	}

	// Fetch tools from server
	tools, err := client.ListTools(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to list tools: %w", err)
	}

	// Generate documentation based on detail level
	switch detailLevel {
	case "overview":
		doc, _ := de.generateOverview(serverName, tools)
		return doc, nil
	case "signatures":
		doc, _ := de.generateSignatures(serverName, tools)
		return doc, nil
	case "full":
		doc, _ := de.generateFullDocs(serverName, tools)
		return doc, nil
	default:
		doc, _ := de.generateSignatures(serverName, tools)
		return doc, nil
	}
}

// generateOverview creates a simple list of tool names
func (de *DiscoveryEngine) generateOverview(serverName string, tools []ToolDefinition) (string, string) {
	var docs strings.Builder

	docs.WriteString(fmt.Sprintf("## %s Tools (Overview)\n\n", serverName))
	docs.WriteString(fmt.Sprintf("Available functions (%d total):\n", len(tools)))

	for _, tool := range tools {
		docs.WriteString(fmt.Sprintf("- %s\n", tool.Function.Name))
	}

	return docs.String(), ""
}

// generateSignatures creates Python-style function signatures
func (de *DiscoveryEngine) generateSignatures(serverName string, tools []ToolDefinition) (string, string) {
	var docs strings.Builder

	docs.WriteString(fmt.Sprintf("## %s Tools (Signatures)\n\n", serverName))
	docs.WriteString(fmt.Sprintf("Python module: import servers.%s as %s\n\n",
		sanitizeModuleName(serverName), moduleAlias(serverName)))

	for _, tool := range tools {
		// Generate signature
		sig := generateFunctionSignature(tool.Function)

		docs.WriteString(fmt.Sprintf("```python\n%s\n```\n", sig))
		docs.WriteString(fmt.Sprintf("%s\n\n", tool.Function.Description))
	}

	return docs.String(), ""
}

// generateFullDocs creates complete Python module documentation
func (de *DiscoveryEngine) generateFullDocs(serverName string, tools []ToolDefinition) (string, string) {
	// Use the VirtualFS to generate full module
	module, err := de.vfs.GenerateModule(serverName)
	if err != nil {
		return fmt.Sprintf("Error generating module: %v", err), ""
	}

	return module.PythonCode, ""
}

// CalculateDiscoveryCost estimates the token cost for different discovery strategies
//
// This helps compare:
// - Loading all tools upfront (expensive)
// - Progressive discovery (cheap)
// - Lazy loading with caching (optimal)
func (de *DiscoveryEngine) CalculateDiscoveryCost(strategy string, toolsPerServer int, serverCount int) map[string]interface{} {
	switch strategy {
	case "traditional":
		// Load all tools in system prompt
		totalTools := toolsPerServer * serverCount
		tokens := totalTools * 500 // ~500 tokens per tool
		return map[string]interface{}{
			"strategy":       "traditional",
			"tokens":         tokens,
			"description":    "All tools in system prompt",
			"servers_loaded": serverCount,
			"tools_loaded":   totalTools,
		}

	case "progressive":
		// 3 core tools + search + inspect
		coreTokens := 1500   // 3 core tools
		searchTokens := 10   // Search results
		inspectTokens := 100 // Signatures for ~2-3 tools
		total := coreTokens + searchTokens + inspectTokens

		return map[string]interface{}{
			"strategy":               "progressive",
			"tokens":                 total,
			"description":            "Discover tools as needed",
			"core_tools":             coreTokens,
			"discovery":              searchTokens + inspectTokens,
			"savings_vs_traditional": float64(toolsPerServer*serverCount*500-total) / float64(toolsPerServer*serverCount*500) * 100,
		}

	case "cached":
		// After first discovery, cache results
		// Subsequent queries only pay for what they use
		return map[string]interface{}{
			"strategy":    "cached",
			"tokens":      1500, // Just core tools
			"description": "Core tools only, discovered tools cached",
		}

	default:
		return map[string]interface{}{
			"strategy": "unknown",
			"tokens":   0,
		}
	}
}

// GetRecommendedStrategy suggests the best discovery strategy based on context
//
// Factors:
// - Number of available servers
// - User query specificity
// - Available context window
// - Latency requirements
func (de *DiscoveryEngine) GetRecommendedStrategy(
	serverCount int,
	userQuery string,
	contextWindow int,
) string {
	// If few servers (<5), traditional might be fine
	if serverCount < 5 {
		return "traditional"
	}

	// If query is specific (mentions specific tools), use progressive
	for _, server := range de.vfs.ListServers() {
		if strings.Contains(strings.ToLower(userQuery), strings.ToLower(server)) {
			return "progressive"
		}
	}

	// If context window is limited, definitely use progressive
	if contextWindow < 8000 {
		return "progressive"
	}

	// Default to progressive for scalability
	return "progressive"
}

// SearchResult represents the result of a module search
// This is returned by the search_available_modules tool
func FormatSearchResults(results []ModuleSummary) string {
	if len(results) == 0 {
		return "No modules found matching your criteria."
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d available modules:\n\n", len(results)))

	for _, result := range results {
		output.WriteString(fmt.Sprintf("**%s**\n", result.Name))
		output.WriteString(fmt.Sprintf("  Description: %s\n", result.Description))
		output.WriteString(fmt.Sprintf("  Tools available: %d\n", result.ToolCount))

		if len(result.Categories) > 0 {
			output.WriteString(fmt.Sprintf("  Categories: %s\n", strings.Join(result.Categories, ", ")))
		}

		output.WriteString("\n")
	}

	output.WriteString("To inspect a specific module, use: inspect_module(module_name=\"<name>\")")

	return output.String()
}

// FormatInspectResult formats inspection results for the LLM
func FormatInspectResult(serverName string, detailLevel string, content string) string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("## Module: %s (detail: %s)\n\n", serverName, detailLevel))
	output.WriteString(content)
	output.WriteString("\n\n")
	output.WriteString("You can now write Python code to use these tools. Example:\n")
	output.WriteString(fmt.Sprintf("```python\nimport servers.%s as %s\n",
		sanitizeModuleName(serverName), moduleAlias(serverName)))
	output.WriteString(fmt.Sprintf("result = %s.tool_name(...)\n```", moduleAlias(serverName)))

	return output.String()
}
