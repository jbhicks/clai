package llm

import (
	"clai/internal/logger"
	"clai/internal/tools"
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

func (a *Agent) AddMessage(role, content string) {
	a.messages = append(a.messages, Message{
		Role:    role,
		Content: content,
	})
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

	logger.Debug("[AGENT-PARSE] Thought: %q, Code: %d chars (%s), Final: %q",
		result.Thought, len(result.Code), result.Language, result.Final)

	return result, nil
}

func (a *Agent) Think() (string, error) {
	streamChan := make(chan string, 100)

	_, err := a.client.SendMessageStreamNoTools(a.messages, streamChan, false)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}

	var fullResponse strings.Builder
	for chunk := range streamChan {
		fullResponse.WriteString(chunk)
	}

	response := fullResponse.String()
	logger.Debug("[AGENT-THINK] Full response:\n%s", response)

	return response, nil
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

		response, err := a.Think()
		if err != nil {
			return "", err
		}

		parsed, err := a.parseResponse(response)
		if err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		// If no code block found, treat as final answer
		if parsed.Code == "" {
			logger.Debug("[AGENT-NO-CODE] No code block found, treating as final answer")
			parsed.Final = response
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

			a.AddMessage("assistant", response)
			a.AddMessage("user", obsStr)
			logger.Debug("[AGENT-OBSERVATION] Added observation: %s", obsStr)
		} else {
			logger.Debug("[AGENT-WARNING] No delegation or code found, but no final answer either - returning full response")
			return response, nil
		}
	}
}
