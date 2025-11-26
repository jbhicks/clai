package tools

import (
	"encoding/json"
	"fmt"
)

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type CalculatorParams struct {
	Expression string `json:"expression"`
}

type EchoParams struct {
	Message string `json:"message"`
}

type WebSearchParams struct {
	Query string `json:"query"`
}

var availableTools = []Tool{
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
	},
	{
		Name:        "echo",
		Description: "Echoes the message back to the user.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "The message to echo back",
				},
			},
			"required": []string{"message"},
		},
	},
	{
		Name:        "web_search",
		Description: "Performs a web search for the given query.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search query",
				},
			},
			"required": []string{"query"},
		},
	},
}

func GetAvailableTools() []Tool {
	return availableTools
}

func GetAvailableToolsJSON() (string, error) {
	toolsJSON, err := json.Marshal(availableTools)
	if err != nil {
		return "", fmt.Errorf("error marshalling tools: %w", err)
	}
	return string(toolsJSON), nil
}

func GetToolsByNames(names ...string) []Tool {
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	selected := []Tool{}
	for _, tool := range availableTools {
		if nameSet[tool.Name] {
			selected = append(selected, tool)
		}
	}
	return selected
}

func GetToolDescriptions() string {
	descriptions := ""
	for _, tool := range availableTools {
		descriptions += fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description)
	}
	return descriptions
}
