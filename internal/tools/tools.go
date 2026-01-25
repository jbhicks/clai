package tools

import (
	"clai/internal/logger"
	"encoding/json"
	"fmt"
)

// Tools package now only contains code execution functionality.
// All function calling / tool definitions have been removed in favor of agent mode.

// Tool represents a tool definition for function calling
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction represents a function within a tool
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall represents a tool call from the model
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

// ToolCallFunc represents the function call details
type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// GetAvailableTools returns the list of tools available for function calling
func GetAvailableTools() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "execute_bash",
				Description: "Execute bash/shell commands. Use this for system operations, file manipulation, and general command-line tasks.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "The bash command to execute",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "execute_python",
				Description: "Execute Python code. Use this for Python scripting, data processing, and computations.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{
							"type":        "string",
							"description": "The Python code to execute",
						},
					},
					"required": []string{"code"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "execute_javascript",
				Description: "Execute JavaScript/Node.js code. Use this for JavaScript operations and Node.js scripting.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{
							"type":        "string",
							"description": "The JavaScript code to execute",
						},
					},
					"required": []string{"code"},
				},
			},
		},
	}
}

// ExecuteTool executes a tool call and returns the result
func ExecuteTool(toolCall ToolCall) (string, error) {
	switch toolCall.Function.Name {
	case "execute_bash":
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("failed to parse tool arguments as JSON: %w", err)
		}
		command, ok := args["command"].(string)
		if !ok {
			return "", fmt.Errorf("command parameter is required and must be a string")
		}
		logger.Debug("[TOOLS-EXEC] Executing bash command: %s", command)
		result, err := ExecuteCode("bash", command)
		logger.Debug("[TOOLS-EXEC] Command result: %q, err: %v", result, err)
		return result, err

	case "execute_python":
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("failed to parse tool arguments as JSON: %w", err)
		}
		code, ok := args["code"].(string)
		if !ok {
			return "", fmt.Errorf("code parameter is required and must be a string")
		}
		return ExecuteCode("python", code)

	case "execute_javascript":
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("failed to parse tool arguments as JSON: %w", err)
		}
		code, ok := args["code"].(string)
		if !ok {
			return "", fmt.Errorf("code parameter is required and must be a string")
		}
		return ExecuteCode("javascript", code)

	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}
}
