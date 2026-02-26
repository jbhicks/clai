package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// IsHermesStyleModel checks if the model name indicates support for Hermes-style tool calling
// This includes Qwen3, Hermes, and other models trained for native function calling
func IsHermesStyleModel(modelName string) bool {
	modelLower := strings.ToLower(modelName)
	return strings.Contains(modelLower, "qwen3") ||
		strings.Contains(modelLower, "hermes") ||
		strings.Contains(modelLower, "llama-3.1") ||
		strings.Contains(modelLower, "llama-3.2") ||
		strings.Contains(modelLower, "llama3.1") ||
		strings.Contains(modelLower, "llama3.2")
}

// IsStepFlashModel checks if the model is Step-3.5-Flash which uses <tool_call> format
func IsStepFlashModel(modelName string) bool {
	modelLower := strings.ToLower(modelName)
	return strings.Contains(modelLower, "step") && strings.Contains(modelLower, "flash")
}

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
				Name:        "execute_code",
				Description: `Execute Python, Bash, or JavaScript code to accomplish any task. Use it to read/write files, run shell commands, process data, and compose multiple operations.`,
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"language": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"python", "bash", "javascript"},
							"description": "Language: python, bash, or javascript",
						},
						"code": map[string]interface{}{
							"type":        "string",
							"description": "Code to execute",
						},
						"purpose": map[string]interface{}{
							"type":        "string",
							"description": "What this code does",
						},
					},
					"required": []string{"language", "code"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "execute_bash",
				Description: "Execute shell commands for system operations and file manipulation.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Shell command to execute",
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
				Description: "Execute Python code for scripting, data processing, and computations.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{
							"type":        "string",
							"description": "Python code to execute",
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
				Description: "Execute JavaScript/Node.js code for scripting and operations.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{
							"type":        "string",
							"description": "JavaScript code to execute",
						},
					},
					"required": []string{"code"},
				},
			},
		},
	}
}

// GetCodeActTools returns a minimal set of tools optimized for CodeAct-style operation
// This uses a single general-purpose execute_code tool instead of specific language tools
func GetCodeActTools() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name: "execute_code",
				Description: `Execute code to perform any task. Write Python, Bash, or JavaScript code to accomplish your goal.

This is a general-purpose tool - use it to:
- Read/write files and manipulate the filesystem
- Execute shell commands via subprocess
- Make HTTP requests and process data
- Use available libraries and packages
- Compose multiple operations together in one execution
- Write loops, conditionals, and complex logic

The code runs with access to the current working directory and standard libraries.

Always print results so they appear in the output. For Python, use print(). For Bash, output goes to stdout. For JavaScript, use console.log().`,
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"language": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"python", "bash", "javascript"},
							"description": "Programming language: python, bash, or javascript",
						},
						"code": map[string]interface{}{
							"type":        "string",
							"description": "Complete, runnable code to execute",
						},
						"purpose": map[string]interface{}{
							"type":        "string",
							"description": "What this code accomplishes (brief description)",
						},
					},
					"required": []string{"language", "code"},
				},
			},
		},
	}
}

// ExecuteTool executes a tool call and returns the result
func ExecuteTool(toolCall ToolCall) (string, error) {
	switch toolCall.Function.Name {
	case "execute_code":
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("failed to parse tool arguments as JSON: %w", err)
		}
		language, ok := args["language"].(string)
		if !ok {
			return "", fmt.Errorf("language parameter is required and must be a string")
		}
		code, ok := args["code"].(string)
		if !ok {
			return "", fmt.Errorf("code parameter is required and must be a string")
		}
		return ExecuteCode(language, code)

	case "execute_bash":
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("failed to parse tool arguments as JSON: %w", err)
		}
		command, ok := args["command"].(string)
		if !ok {
			return "", fmt.Errorf("command parameter is required and must be a string")
		}
		return ExecuteCode("bash", command)

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
