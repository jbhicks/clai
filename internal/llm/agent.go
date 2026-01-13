package llm

import (
	"clai/internal/logger"
	"clai/internal/tools"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type AgentResponse struct {
	Thought  string
	Code     string
	Language string
	Final    string
}

type ThinkResult struct {
	Content   string
	ToolCalls []tools.ToolCall
}

type Agent struct {
	client         LLMClientInterface
	messages       []Message
	jsExecutor     *JSExecutor
	statusCallback func(iteration int, thought string, executingCode bool, language string, code string)
	taskHistory    []string
	loopDetector   *LoopDetector
	maxMessages    int
}

type LoopDetector struct {
	codeHistory        []string
	observationHistory []string
	maxRepeats         int
}

func NewAgent(client LLMClientInterface) *Agent {
	return &Agent{
		client:      client,
		messages:    []Message{},
		jsExecutor:  NewJSExecutor(),
		taskHistory: []string{},
		maxMessages: 20, // Keep last 20 messages to prevent context overflow
		loopDetector: &LoopDetector{
			codeHistory:        []string{},
			observationHistory: []string{},
			maxRepeats:         3,
		},
	}
}

func (a *Agent) AddMessage(role, content string, toolCallID ...string) {
	message := Message{
		Role:    role,
		Content: content,
	}
	if len(toolCallID) > 0 {
		message.ToolCallID = toolCallID[0]
	}
	a.messages = append(a.messages, message)
	a.trimMessages()
}

func (a *Agent) trimMessages() {
	if len(a.messages) <= a.maxMessages {
		return
	}

	// Keep system message and last N messages
	var trimmed []Message

	// Always keep the first message if it's a system message
	if len(a.messages) > 0 && a.messages[0].Role == "system" {
		trimmed = append(trimmed, a.messages[0])
		// Keep last maxMessages-1 messages (excluding system)
		start := len(a.messages) - (a.maxMessages - 1)
		if start < 1 {
			start = 1
		}
		trimmed = append(trimmed, a.messages[start:]...)
	} else {
		// Keep last maxMessages messages
		start := len(a.messages) - a.maxMessages
		if start < 0 {
			start = 0
		}
		trimmed = append(trimmed, a.messages[start:]...)
	}

	a.messages = trimmed
	logger.Debug("[AGENT-TRIM] Trimmed messages from %d to %d", len(a.messages)+a.maxMessages-len(a.messages), len(a.messages))
}

func (a *Agent) SetStatusCallback(cb func(iteration int, thought string, executingCode bool, language string, code string)) {
	a.statusCallback = cb
}

func (a *Agent) parseResponse(response string) (*AgentResponse, error) {
	result := &AgentResponse{}

	// Try multiple code block formats in priority order:
	// 1. Full XML: <code language="python">
	// 2. Simplified XML: <code python>
	// 3. Malformed simplified: <code python (missing >)
	// 4. Markdown: ```python

	// Pattern 1: Full XML format
	fullXMLRe := regexp.MustCompile(`(?s)<code\s+language="([^"]+)">\s*(.+?)\s*</code>`)
	if matches := fullXMLRe.FindStringSubmatch(response); len(matches) > 2 {
		result.Language = strings.TrimSpace(matches[1])
		result.Code = strings.TrimSpace(matches[2])
		if result.Language == "" {
			result.Language = "bash"
		}

		// Extract thought as everything before the code block
		parts := strings.Split(response, "<code")
		if len(parts) > 0 {
			result.Thought = strings.TrimSpace(parts[0])
		}

		logger.Debug("[AGENT-PARSE] Matched full XML format: %s", result.Language)
		return result, nil
	}

	// Pattern 2: Simplified XML format - <code python>
	simplifiedXMLRe := regexp.MustCompile(`(?s)<code\s+(bash|python|javascript|js|sh|py)>\s*(.+?)\s*</code>`)
	if matches := simplifiedXMLRe.FindStringSubmatch(response); len(matches) > 2 {
		result.Language = strings.TrimSpace(matches[1])
		result.Code = strings.TrimSpace(matches[2])

		// Normalize language names
		if result.Language == "js" {
			result.Language = "javascript"
		} else if result.Language == "py" {
			result.Language = "python"
		} else if result.Language == "sh" {
			result.Language = "bash"
		}

		// Extract thought as everything before the code block
		parts := strings.Split(response, "<code")
		if len(parts) > 0 {
			result.Thought = strings.TrimSpace(parts[0])
		}

		logger.Debug("[AGENT-PARSE] Matched simplified XML format: %s", result.Language)
		return result, nil
	}

	// Pattern 3: Malformed simplified XML - <code python (missing > on opening tag)
	malformedXMLRe := regexp.MustCompile(`(?s)<code\s+(bash|python|javascript|js|sh|py)\s+(.+?)(?:</code|$)`)
	if matches := malformedXMLRe.FindStringSubmatch(response); len(matches) > 2 {
		result.Language = strings.TrimSpace(matches[1])
		result.Code = strings.TrimSpace(matches[2])

		// Normalize language names
		if result.Language == "js" {
			result.Language = "javascript"
		} else if result.Language == "py" {
			result.Language = "python"
		} else if result.Language == "sh" {
			result.Language = "bash"
		}

		// Extract thought as everything before the code block
		parts := strings.Split(response, "<code")
		if len(parts) > 0 {
			result.Thought = strings.TrimSpace(parts[0])
		}

		logger.Debug("[AGENT-PARSE] Matched malformed simplified XML format: %s", result.Language)
		return result, nil
	}

	// Pattern 4: Markdown code blocks - ```python
	markdownRe := regexp.MustCompile("(?s)```(bash|python|javascript|js|sh|py)\\s*\\n(.+?)```")
	if matches := markdownRe.FindStringSubmatch(response); len(matches) > 2 {
		result.Language = strings.TrimSpace(matches[1])
		result.Code = strings.TrimSpace(matches[2])

		// Normalize language names
		if result.Language == "js" {
			result.Language = "javascript"
		} else if result.Language == "py" {
			result.Language = "python"
		} else if result.Language == "sh" {
			result.Language = "bash"
		}

		// Extract thought as everything before the code block
		parts := strings.Split(response, "```")
		if len(parts) > 0 {
			result.Thought = strings.TrimSpace(parts[0])
		}

		logger.Debug("[AGENT-PARSE] Matched markdown format: %s", result.Language)
		return result, nil
	}

	// No code block found, treat entire response as final answer
	result.Final = strings.TrimSpace(response)
	result.Thought = strings.TrimSpace(response)

	logger.Debug("[AGENT-PARSE] Thought: %q, Code: %d chars (%s), Final: %q", result.Thought, len(result.Code), result.Language, result.Final)
	return result, nil
}
func (a *Agent) Think() (ThinkResult, error) {
	logger.Debug("[AGENT-THINK] Think method called")
	streamChan := make(chan string, 100)

	_, err := a.client.SendMessageStreamWithTools(a.messages, tools.GetAvailableTools(), streamChan, true)
	if err != nil {
		return ThinkResult{}, fmt.Errorf("LLM request failed: %w", err)
	}

	var fullContent strings.Builder
	var toolCalls []tools.ToolCall

	for chunk := range streamChan {
		logger.Debug("[AGENT-THINK] Received chunk: %q", chunk)
		// For now, collect all chunks as content - tool calls will be parsed from final content
		fullContent.WriteString(chunk)
	}

	response := fullContent.String()
	logger.Debug("[AGENT-THINK] Full response content: %s", response)

	// Parse tool calls from the complete response content (Hermes-style)
	toolCalls = a.parseToolCallsFromContent(response)

	logger.Debug("[AGENT-THINK] Tool calls found: %d", len(toolCalls))

	return ThinkResult{
		Content:   response,
		ToolCalls: toolCalls,
	}, nil
}

// parseToolCallsFromContent extracts tool calls from response content (Hermes-style)
func (a *Agent) parseToolCallsFromContent(content string) []tools.ToolCall {
	logger.Debug("[AGENT-PARSE-TOOL] Method called with content length: %d", len(content))
	var toolCalls []tools.ToolCall
	logger.Debug("[AGENT-PARSE-TOOL] Parsing content for tool calls: %s", content)

	// First try to parse <tool_call> + JSON format (model's current format)
	// Handle both single JSON objects and concatenated multiple objects
	toolCallRe := regexp.MustCompile(`<tool_call>\s*(\{.+\}(?:\{.+?\})*)`)
	toolCallMatches := toolCallRe.FindAllStringSubmatch(content, -1)
	logger.Debug("[AGENT-PARSE-TOOL] Found %d <tool_call> + JSON matches", len(toolCallMatches))

	for _, match := range toolCallMatches {
		jsonStr := match[1]
		logger.Debug("[AGENT-PARSE-TOOL] Trying to parse <tool_call> JSON: %s", jsonStr)

		// Try to parse as single JSON object first
		var toolCall tools.ToolCall
		if err := json.Unmarshal([]byte(jsonStr), &toolCall); err == nil && toolCall.Type == "function" && toolCall.Function.Name != "" {
			toolCalls = append(toolCalls, toolCall)
			logger.Debug("[AGENT-PARSE-TOOL] Successfully parsed <tool_call> JSON: %s", toolCall.Function.Name)
		} else {
			// Try to split concatenated JSON objects
			jsonObjects := splitJSONObjects(jsonStr)
			for _, objStr := range jsonObjects {
				if err := json.Unmarshal([]byte(objStr), &toolCall); err == nil && toolCall.Type == "function" && toolCall.Function.Name != "" {
					toolCalls = append(toolCalls, toolCall)
					logger.Debug("[AGENT-PARSE-TOOL] Successfully parsed split JSON: %s", toolCall.Function.Name)
				}
			}
		}
	}

	// Also try XML-style tool calls (Hermes format)
	xmlRe := regexp.MustCompile(`<tool_call><function=(\w+)\{(.+)\}`)
	xmlMatches := xmlRe.FindAllStringSubmatch(content, -1)
	logger.Debug("[AGENT-PARSE-TOOL] Found %d XML tool call matches", len(xmlMatches))

	for _, match := range xmlMatches {
		logger.Debug("[AGENT-PARSE-TOOL] XML match: %v", match)
		if len(match) >= 3 {
			functionName := match[1]
			argsStr := match[2]
			logger.Debug("[AGENT-PARSE-TOOL] Parsing XML tool call: %s with args: %s", functionName, argsStr)

			// Parse the arguments as JSON
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
				logger.Debug("[AGENT-PARSE-TOOL] Failed to parse JSON args: %v", err)
				continue
			}

			// Create tool call based on function name
			var toolCall tools.ToolCall
			toolCall.Type = "function"
			toolCall.ID = "xml-parsed-" + functionName
			toolCall.Function.Name = functionName

			// Convert args to JSON string
			argsJSON, _ := json.Marshal(args)
			toolCall.Function.Arguments = string(argsJSON)

			toolCalls = append(toolCalls, toolCall)
			logger.Debug("[AGENT-PARSE-TOOL] Successfully parsed XML tool call: %s", functionName)
		}
	}

	// Also try JSON format as fallback
	re := regexp.MustCompile(`\{[^{}]*"function"[^{}]*\}`)
	jsonMatches := re.FindAllString(content, -1)
	logger.Debug("[AGENT-PARSE-TOOL] Found %d JSON tool call matches", len(jsonMatches))

	for _, match := range jsonMatches {
		logger.Debug("[AGENT-PARSE-TOOL] Trying to parse JSON: %s", match)
		var toolCall tools.ToolCall
		if err := json.Unmarshal([]byte(match), &toolCall); err == nil && toolCall.Type == "function" && toolCall.Function.Name != "" {
			toolCalls = append(toolCalls, toolCall)
			logger.Debug("[AGENT-PARSE-TOOL] Successfully parsed JSON tool call: %s", toolCall.Function.Name)
		} else {
			logger.Debug("[AGENT-PARSE-TOOL] Failed to parse JSON as tool call: %v", err)
		}
	}

	return toolCalls
}

// splitJSONObjects attempts to split concatenated JSON objects
func splitJSONObjects(jsonStr string) []string {
	var objects []string
	braceCount := 0
	start := 0

	for i, char := range jsonStr {
		switch char {
		case '{':
			if braceCount == 0 {
				start = i
			}
			braceCount++
		case '}':
			braceCount--
			if braceCount == 0 {
				// Found a complete JSON object
				objects = append(objects, jsonStr[start:i+1])
			}
		}
	}

	return objects
}

func (ld *LoopDetector) detectLoop(code, observation string) (bool, string) {
	if code != "" {
		ld.codeHistory = append(ld.codeHistory, code)
		if len(ld.codeHistory) >= ld.maxRepeats {
			recent := ld.codeHistory[len(ld.codeHistory)-ld.maxRepeats:]
			allSame := true
			for i := 1; i < len(recent); i++ {
				if recent[i] != recent[0] {
					allSame = false
					break
				}
			}
			if allSame {
				return true, fmt.Sprintf("Loop detected: same code executed %d times consecutively", ld.maxRepeats)
			}
		}
	}

	if observation != "" {
		ld.observationHistory = append(ld.observationHistory, observation)
		if len(ld.observationHistory) >= ld.maxRepeats {
			recent := ld.observationHistory[len(ld.observationHistory)-ld.maxRepeats:]
			allSame := true
			for i := 1; i < len(recent); i++ {
				if recent[i] != recent[0] {
					allSame = false
					break
				}
			}
			if allSame {
				return true, fmt.Sprintf("Loop detected: same observation received %d times consecutively", ld.maxRepeats)
			}
		}

		permanentErrors := []string{
			"permission denied",
			"no such file or directory",
			"command not found",
			"cannot access",
		}
		obsLower := strings.ToLower(observation)
		for _, errPattern := range permanentErrors {
			if strings.Contains(obsLower, errPattern) {
				count := 0
				for _, obs := range ld.observationHistory {
					if strings.Contains(strings.ToLower(obs), errPattern) {
						count++
					}
				}
				if count >= ld.maxRepeats {
					return true, fmt.Sprintf("Loop detected: permanent error '%s' encountered %d times", errPattern, count)
				}
			}
		}
	}

	return false, ""
}

func (a *Agent) Run(query string) (string, error) {
	a.AddMessage("user", query)

	logger.Debug("[AGENT-RUN] Starting agent loop with query: %s", query)

	iteration := 0
	for {
		iteration++
		logger.Debug("[AGENT-ITER] Iteration %d", iteration)

		thinkResult, err := a.Think()
		if err != nil {
			return "", err
		}

		// Handle tool calls first
		if len(thinkResult.ToolCalls) > 0 {
			logger.Debug("[AGENT-TOOL-CALLS] Processing %d tool calls", len(thinkResult.ToolCalls))

			// Add assistant message with tool calls
			a.AddMessage("assistant", thinkResult.Content)

			// Execute each tool call and add tool results
			for _, toolCall := range thinkResult.ToolCalls {
				logger.Debug("[AGENT-TOOL] Executing tool: %s", toolCall.Function.Name)

				toolResult, err := tools.ExecuteTool(toolCall)
				if err != nil {
					logger.Debug("[AGENT-TOOL-ERROR] Tool execution failed: %v", err)
					a.AddMessage("tool", fmt.Sprintf("Tool execution error: %v", err), toolCall.ID)
				} else {
					logger.Debug("[AGENT-TOOL-SUCCESS] Tool result: %s", toolResult)
					a.AddMessage("tool", toolResult, toolCall.ID)
				}
			}

			// Continue the loop to get next response after tool execution
			continue
		}

		// No tool calls, fall back to XML code block parsing
		parsed, err := a.parseResponse(thinkResult.Content)
		if err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		// If no code block found, treat as final answer
		if parsed.Code == "" {
			logger.Debug("[AGENT-NO-CODE] No code block found, treating as final answer")
			parsed.Final = thinkResult.Content
		}

		if parsed.Thought != "" {
			a.taskHistory = append(a.taskHistory, parsed.Thought)
		}

		if a.statusCallback != nil {
			a.statusCallback(iteration, parsed.Thought, parsed.Code != "", parsed.Language, parsed.Code)
		}

		if parsed.Final != "" {
			logger.Debug("[AGENT-COMPLETE] Final answer reached: %s", parsed.Final)
			if a.statusCallback != nil {
				a.statusCallback(0, "", false, "", "")
			}
			return parsed.Final, nil
		}

		var observation strings.Builder

		if parsed.Code != "" {
			logger.Debug("[AGENT-CODE] Executing %s code", parsed.Language)
			output, err := tools.ExecuteCode(parsed.Language, parsed.Code)
			if err != nil {
				observation.WriteString(fmt.Sprintf("Observation (Code execution error): %v\n", err))
			} else {
				observation.WriteString(fmt.Sprintf("Observation (Code output): %s\n", output))
			}
		}

		if observation.Len() > 0 {
			obsStr := observation.String()

			isLoop, loopReason := a.loopDetector.detectLoop(parsed.Code, obsStr)
			if isLoop {
				logger.Debug("[AGENT-LOOP] %s", loopReason)
				if a.statusCallback != nil {
					a.statusCallback(0, "", false, "", "")
				}
				return fmt.Sprintf("Task stopped: %s\n\nLast observation:\n%s", loopReason, obsStr), nil
			}

			a.AddMessage("assistant", thinkResult.Content)
			a.AddMessage("user", obsStr)
			logger.Debug("[AGENT-OBSERVATION] Added observation: %s", obsStr)
		} else {
			logger.Debug("[AGENT-WARNING] No delegation or code found, but no final answer either - returning full response")
			return thinkResult.Content, nil
		}
	}
}
