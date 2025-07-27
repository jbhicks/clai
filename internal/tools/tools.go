package tools

import (
	"encoding/json"
	"fmt"
)

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
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
		Parameters:  CalculatorParams{},
	},
	{
		Name:        "echo",
		Description: "Echoes the message back to the user.",
		Parameters:  EchoParams{},
	},
	{
		Name:        "web_search",
		Description: "Performs a web search for the given query.",
		Parameters:  WebSearchParams{},
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
