package tools

import (
	"encoding/json"
	"fmt"
	"github.com/Knetic/govaluate"
)

func ExecuteTool(name string, params json.RawMessage) (string, error) {
	switch name {
	case "calculator":
		var p CalculatorParams
		if err := json.Unmarshal(params, &p); err != nil {
			return "", fmt.Errorf("error unmarshalling calculator params: %w", err)
		}
		return executeCalculator(p)
	case "echo":
		var p EchoParams
		if err := json.Unmarshal(params, &p); err != nil {
			return "", fmt.Errorf("error unmarshalling echo params: %w", err)
		}
		return executeEcho(p)
	case "web_search":
		var p WebSearchParams
		if err := json.Unmarshal(params, &p); err != nil {
			return "", fmt.Errorf("error unmarshalling web search params: %w", err)
		}
		return executeWebSearch(p)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

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

func executeEcho(params EchoParams) (string, error) {
	return params.Message, nil
}

func executeWebSearch(params WebSearchParams) (string, error) {
	// Placeholder for web search functionality.
	// In a real application, this would integrate with a web search API.
	return fmt.Sprintf("Search results for '%s': No real search performed, this is a placeholder.", params.Query), nil
}
