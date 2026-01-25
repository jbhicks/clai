package llm

import (
	"bufio"
	"bytes"
	"clai/internal/logger"
	"clai/internal/tools"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultSystemPrompt = `You are a free agent AI with full code execution capabilities. You can execute bash, python, and javascript code directly on the system.

**Critical rules:**
1. When you need to read files, execute commands, or perform system operations, use code execution by wrapping code in XML tags:
   <code language="bash">cat /path/to/file</code>
   <code language="python">print("Hello")</code>
   <code language="javascript">console.log("Hello")</code>

2. DO NOT use echo/print/console.log to narrate your thinking. Only use them for actual task output.
   ❌ BAD: <code language="bash">echo "Now I will read the file"</code>
   ✅ GOOD: <code language="bash">cat sample.txt</code>

3. You have filesystem access. Read files directly instead of asking the user.
4. Execute commands as needed to complete tasks.
5. Keep code blocks focused and purposeful.

**Efficiency Guidelines:**
6. Avoid redundant operations - don't make multiple identical tool calls
   - Before reading a file, check if you've already read it in this conversation
   - Don't repeat the same command with identical parameters
   - Consolidate multiple operations when possible (e.g., combine file reads)
7. Plan your approach: Think through what tools you need before making calls
   - Make one comprehensive call instead of multiple small calls
   - Use a single tool call to accomplish multiple related tasks when feasible

**Default behavior when uncertain:**
- If asked about project status, plans, or "what's next", read TODO.md, README.md, or AGENTS.md
- If asked about code structure or how something works, read the relevant source files
- If asked about configuration or setup, read config files (.yaml, .json, .toml, Makefile, package.json, etc.)
- When you don't know something about the project, explore the filesystem first (ls, find, grep, cat) rather than guessing
- Be proactive: if a question implies file knowledge, read those files before answering

Answer questions clearly and execute code when needed to provide accurate information.`
)

type APIFormat int

const (
	FormatUnknown APIFormat = iota
	FormatOllama
	FormatOpenAI
)

type LLMClientInterface interface {
	SendMessageStreamNoTools(messages []Message, streamChan chan<- string, includeSystemPrompt bool) (Response, error)
	SendMessageStreamWithTools(messages []Message, tools []Tool, streamChan chan<- string, includeSystemPrompt bool) (Response, error)
	Model() string
	Host() string
	APIFormatString() string
}

type Client struct {
	host           string
	model          string
	systemPrompt   string
	apiFormat      APIFormat
	circuitBreaker *CircuitBreaker
	backoff        *ExponentialBackoff
}

func NewClient(host, model, systemPrompt string) *Client {
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}

	// For Qwen models, append tool schemas to system prompt with specific optimizations
	if strings.Contains(strings.ToLower(model), "qwen") {
		// Add Qwen-specific efficiency instructions
		systemPrompt = systemPrompt + "\n\n**Qwen-Specific Instructions:**\n- CRITICAL: Make one tool call at a time when possible\n- Avoid making multiple identical tool calls in the same response\n- If you need to read the same file multiple times, read it once and reuse the information\n- Consolidate operations into single comprehensive tool calls\n- Review your planned tool calls to eliminate duplicates before sending\n"

		tools := tools.GetAvailableTools()
		if len(tools) > 0 {
			toolsJSON, err := json.Marshal(tools)
			if err == nil {
				systemPrompt = systemPrompt + "\n\nYou have access to tools. When you need to use a tool, respond with a JSON object in this exact format:\n\n{\"tool_calls\": [{\"id\": \"call_id\", \"type\": \"function\", \"function\": {\"name\": \"tool_name\", \"arguments\": \"{\\\"param\\\": \\\"value\\\"}\" }}]}\n\nAvailable tools:\n" + string(toolsJSON)
			}
		}
	}

	c := &Client{
		host:           host,
		model:          model,
		systemPrompt:   systemPrompt,
		apiFormat:      FormatUnknown,
		circuitBreaker: NewCircuitBreaker(fmt.Sprintf("llm-%s-%s", host, model), 3, 30*time.Second),
		backoff:        NewExponentialBackoff(100*time.Millisecond, 5*time.Second, 3),
	}
	logger.Debug("[LLM] About to detect API format")
	c.detectAPIFormat()
	logger.Debug("[LLM] API format detection completed: %d", c.apiFormat)
	return c
}

func (c *Client) detectAPIFormat() {
	logger.Debug("[FORMAT-DETECT] Starting API format detection for host %s", c.host)

	err := c.backoff.RetryWithBackoff(func() error {
		testMessages := []Message{{Role: "user", Content: "hi"}}
		reqBody := Request{
			Model:    c.model,
			Messages: testMessages,
			Stream:   false,
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			logger.Debug("[FORMAT-DETECT] Failed to marshal test request: %v", err)
			c.apiFormat = FormatOllama
			return nil // Don't retry formatting errors
		}

		logger.Debug("[FORMAT-DETECT] Sending test request to %s", c.host+"/api/chat")
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(c.host+"/api/chat", "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			logger.Debug("[FORMAT-DETECT] Failed to send test request: %v", err)
			return err // Retry on network errors
		}
		defer resp.Body.Close()

		logger.Debug("[FORMAT-DETECT] Received response with status %s", resp.Status)
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Debug("[FORMAT-DETECT] Failed to read response: %v", err)
			c.apiFormat = FormatOllama
			return nil // Don't retry read errors
		}

		logger.Debug("[FORMAT-DETECT] Response body length: %d", len(body))
		var ollamaResp Response
		if err := json.Unmarshal(body, &ollamaResp); err == nil && ollamaResp.Message.Content != "" {
			logger.Debug("[FORMAT-DETECT] Detected Ollama native format")
			c.apiFormat = FormatOllama
			return nil // Success, don't retry
		}

		var openAIResp OpenAIStreamChunk
		if err := json.Unmarshal(body, &openAIResp); err == nil && len(openAIResp.Choices) > 0 {
			logger.Debug("[FORMAT-DETECT] Detected OpenAI-compatible format")
			c.apiFormat = FormatOpenAI
			return nil // Success, don't retry
		}

		// Default to Ollama if format detection fails
		logger.Debug("[FORMAT-DETECT] Could not detect format, defaulting to Ollama")
		c.apiFormat = FormatOllama
		return nil // Success, don't retry
	})

	// If all retries failed, default to Ollama
	if err != nil {
		logger.Debug("[FORMAT-DETECT] All retries failed, defaulting to Ollama: %v", err)
		c.apiFormat = FormatOllama
	}

	logger.Debug("[FORMAT-DETECT] Final API format for model %s: %d (%s)", c.model, c.apiFormat, c.APIFormatString())
}

func (c *Client) getChatEndpoint() string {
	if c.apiFormat == FormatOpenAI {
		return c.host + "/chat/completions"
	}
	return c.host + "/api/chat"
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type Tool = tools.Tool
type ToolFunction = tools.ToolFunction
type ToolCall = tools.ToolCall
type ToolCallFunc = tools.ToolCallFunc

type Request struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type Response struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

type OpenAIDelta struct {
	Content   string     `json:"content,omitempty"`
	Role      string     `json:"role,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type OpenAIChoice struct {
	Delta        OpenAIDelta   `json:"delta"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
	Index        int           `json:"index"`
}

type OpenAIMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []tools.ToolCall `json:"tool_calls,omitempty"`
}

type OpenAIStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
}

type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type RequestWithTools struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
	Stream   bool      `json:"stream"`
}

type OpenAIRequestWithTools struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
	Stream   bool      `json:"stream"`
}

func (c *Client) SendMessageStreamNoTools(messages []Message, streamChan chan<- string, includeSystemPrompt bool) (Response, error) {
	// Check circuit breaker before making requests
	if state, failures, _ := c.circuitBreaker.Stats(); state == "open" {
		return Response{}, fmt.Errorf("LLM circuit breaker is open due to %d consecutive failures", failures)
	}

	var allMessages []Message
	if includeSystemPrompt && c.systemPrompt != "" {
		allMessages = append([]Message{{Role: "system", Content: c.systemPrompt}}, messages...)
	} else {
		allMessages = messages
	}

	var reqBody interface{}
	if c.apiFormat == FormatOpenAI {
		reqBody = OpenAIRequest{
			Model:    c.model,
			Messages: allMessages,
			Stream:   true,
		}
	} else {
		reqBody = Request{
			Model:    c.model,
			Messages: allMessages,
			Stream:   true,
		}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, err
	}

	prettyReq, _ := json.MarshalIndent(reqBody, "", "  ")
	logger.Debug("[LLM-REQ-NO-TOOLS] %s", string(prettyReq))

	req, err := http.NewRequest("POST", c.getChatEndpoint(), bytes.NewBuffer(jsonBody))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{
		Timeout: 4 * time.Minute, // Prevent 524 timeouts
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("[LLM-STREAM-ERROR] HTTP request failed: %v", err)
		c.circuitBreaker.recordResult(err)
		return Response{}, err
	}

	logger.Debug("[LLM-STREAM] HTTP response received, status: %s, starting goroutine", resp.Status)

	go func() {
		defer resp.Body.Close()
		defer close(streamChan)
		logger.Debug("[LLM-STREAM] Goroutine started, apiFormat=%d (2=OpenAI)", c.apiFormat)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 32*1024), 1024*1024) // 32KB initial, 1MB max for better performance

		// Check for successful response before starting stream reading
		if resp.StatusCode != http.StatusOK {
			logger.Error("[LLM-STREAM] Unexpected status code: %d", resp.StatusCode)
			streamChan <- fmt.Sprintf("Error: HTTP %d", resp.StatusCode)
			return
		}

		if c.apiFormat == FormatOpenAI {
			logger.Info("[LLM-OPENAI-STREAM] Starting OpenAI stream reading (no tools)")

			for scanner.Scan() {
				line := scanner.Text()
				line = strings.TrimSpace(line)

				if line == "" {
					continue
				}

				if !strings.HasPrefix(line, "data: ") {
					continue
				}

				data := strings.TrimPrefix(line, "data: ")

				if data == "[DONE]" {
					logger.Info("[LLM-OPENAI-STREAM] Received [DONE] marker")
					c.circuitBreaker.recordResult(nil) // Record success
					return
				}

				var chunk OpenAIStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					logger.Info("[LLM-OPENAI-ERROR] Failed to parse chunk: %v", err)
					continue
				}

				if len(chunk.Choices) > 0 {
					choice := chunk.Choices[0]
					if choice.Delta.Content != "" {
						streamChan <- choice.Delta.Content
					}

					if choice.FinishReason != "" && choice.FinishReason != "null" {
						logger.Info("[LLM-OPENAI-STREAM] Stream completed with reason: %s", choice.FinishReason)
						return
					}
				}
			}
			if err := scanner.Err(); err != nil {
				logger.Info("[LLM-OPENAI-ERROR] Scanner error: %v", err)
			} else {
				logger.Info("[LLM-OPENAI-STREAM] Scanner finished without error (may indicate empty response or connection closed)")
			}
		} else {
			// Ollama/Hermes-style streaming
			for scanner.Scan() {
				raw := scanner.Bytes()
				var llmResp Response
				if err := json.Unmarshal(raw, &llmResp); err != nil {
					logger.Debug("[LLM-RAW-ERROR] %v", err)
					return
				}

				prettyResp, _ := json.MarshalIndent(llmResp, "", "  ")
				logger.Debug("[LLM-RESP-STREAM] %s", string(prettyResp))

				if llmResp.Message.Content != "" {
					streamChan <- llmResp.Message.Content
				}

				// Handle tool calls in Ollama response (Hermes-style)
				if len(llmResp.Message.ToolCalls) > 0 {
					for _, toolCall := range llmResp.Message.ToolCalls {
						toolCallJSON, _ := json.Marshal(toolCall)
						streamChan <- string(toolCallJSON)
					}
				}

				if llmResp.Done {
					c.circuitBreaker.recordResult(nil) // Record success
					return
				}
			}
		}
	}()

	return Response{}, nil
}

func (c *Client) SendMessageStreamWithTools(messages []Message, tools []Tool, streamChan chan<- string, includeSystemPrompt bool) (Response, error) {
	// Check circuit breaker before making requests
	if state, failures, _ := c.circuitBreaker.Stats(); state == "open" {
		return Response{}, fmt.Errorf("LLM circuit breaker is open due to %d consecutive failures", failures)
	}

	var allMessages []Message

	// Prepare system prompt with tools for Hermes-style (Qwen) or include tools in request for OpenAI-style
	systemPrompt := c.systemPrompt
	if includeSystemPrompt && c.systemPrompt != "" {
		if c.apiFormat == FormatOpenAI && len(tools) > 0 {
			// For OpenAI-compatible models with tools, modify system prompt to prioritize tool usage
			systemPrompt = strings.Replace(c.systemPrompt,
				`**Critical rules:**
1. When you need to read files, execute commands, or perform system operations, use code execution by wrapping code in XML tags:
   <code language="bash">cat /path/to/file</code>
   <code language="python">print("Hello")</code>
   <code language="javascript">console.log("Hello")</code>`,
				`**Critical rules:**
1. When you need to read files, execute commands, or perform system operations, use the available tools by making function calls.
2. Do not use XML code tags - use the provided tools instead.`,
				1)
		} else if len(tools) > 0 && c.apiFormat == FormatOllama && !strings.Contains(strings.ToLower(c.model), "qwen") {
			toolsJSON, err := json.Marshal(tools)
			if err != nil {
				return Response{}, fmt.Errorf("failed to marshal tools: %w", err)
			}
			systemPrompt = c.systemPrompt + "\n\nAvailable tools:\n" + string(toolsJSON)
		}
		allMessages = append([]Message{{Role: "system", Content: systemPrompt}}, messages...)
		logger.Debug("[LLM-TOOLS] Final system prompt for model %s: %s", c.model, systemPrompt)
	} else {
		allMessages = messages
	}

	var reqBody interface{}
	if c.apiFormat == FormatOpenAI {
		var requestTools []Tool
		// Include tools for OpenAI-compatible models, but skip Qwen (they get tools via system prompt)
		if !strings.Contains(strings.ToLower(c.model), "qwen") {
			requestTools = tools
		}
		reqBody = OpenAIRequestWithTools{
			Model:    c.model,
			Messages: allMessages,
			Tools:    requestTools,
			Stream:   true,
		}
	} else {
		// Ollama format: tools already injected into system prompt
		reqBody = Request{
			Model:    c.model,
			Messages: allMessages,
			Stream:   true,
		}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, err
	}

	prettyReq, _ := json.MarshalIndent(reqBody, "", "  ")
	logger.Info("[LLM-REQ-WITH-TOOLS-STREAM] Request body: %s", string(prettyReq))

	req, err := http.NewRequest("POST", c.getChatEndpoint(), bytes.NewBuffer(jsonBody))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{
		Timeout: 4 * time.Minute, // Prevent 524 timeouts
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("[LLM-STREAM-ERROR] HTTP request failed: %v", err)
		c.circuitBreaker.recordResult(err)
		return Response{}, err
	}

	logger.Debug("[LLM-STREAM] HTTP response received, status: %s, starting goroutine", resp.Status)

	go func() {
		defer resp.Body.Close()
		defer close(streamChan)
		logger.Debug("[LLM-STREAM] Goroutine started for tools, apiFormat=%d (2=OpenAI)", c.apiFormat)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 32*1024), 1024*1024) // 32KB initial, 1MB max for better performance

		// Check for successful response before starting stream reading
		if resp.StatusCode != http.StatusOK {
			logger.Error("[LLM-STREAM] Unexpected status code: %d", resp.StatusCode)
			streamChan <- fmt.Sprintf("Error: HTTP %d", resp.StatusCode)
			return
		}

		if c.apiFormat == FormatOpenAI {
			logger.Info("[LLM-OPENAI-STREAM] Starting OpenAI SSE response reading (with tools)")

			for scanner.Scan() {
				line := scanner.Text()
				logger.Info("[LLM-OPENAI-STREAM] Read line: %q", line)

				if !strings.HasPrefix(line, "data: ") {
					continue
				}

				data := strings.TrimPrefix(line, "data: ")

				if data == "[DONE]" {
					logger.Info("[LLM-OPENAI-STREAM] Stream complete")
					c.circuitBreaker.recordResult(nil) // Record success
					return
				}

				var chunk OpenAIStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					logger.Debug("[LLM-OPENAI-STREAM-ERROR] Failed to parse chunk: %v, data: %q", err, data)
					continue
				}

				if len(chunk.Choices) > 0 {
					delta := chunk.Choices[0].Delta

					if delta.Content != "" {
						logger.Info("[LLM-OPENAI-STREAM] Sending content chunk: %q", delta.Content)
						streamChan <- delta.Content
					}

					if len(delta.ToolCalls) > 0 {
						for _, toolCall := range delta.ToolCalls {
							toolCallJSON, _ := json.Marshal(toolCall)
							logger.Info("[LLM-OPENAI-STREAM] Sending tool call chunk: %q", string(toolCallJSON))
							streamChan <- string(toolCallJSON)
						}
					}
				}
			}

			if err := scanner.Err(); err != nil {
				logger.Error("[LLM-OPENAI-STREAM] Scanner error: %v", err)
			} else {
				logger.Info("[LLM-OPENAI-STREAM] Scanner finished without error (may indicate empty response or connection closed)")
			}
		} else {
			// Ollama/Hermes-style streaming
			for scanner.Scan() {
				raw := scanner.Bytes()
				var llmResp Response
				if err := json.Unmarshal(raw, &llmResp); err != nil {
					logger.Debug("[LLM-RAW-ERROR] %v", err)
					return
				}

				prettyResp, _ := json.MarshalIndent(llmResp, "", "  ")
				logger.Debug("[LLM-RESP-STREAM] %s", string(prettyResp))

				if llmResp.Message.Content != "" {
					streamChan <- llmResp.Message.Content
				}

				// Handle tool calls in Ollama response (Hermes-style)
				if len(llmResp.Message.ToolCalls) > 0 {
					for _, toolCall := range llmResp.Message.ToolCalls {
						toolCallJSON, _ := json.Marshal(toolCall)
						streamChan <- string(toolCallJSON)
					}
				}

				if llmResp.Done {
					return
				}
			}
		}
	}()

	return Response{}, nil
}

func (c *Client) Model() string {
	return c.model
}

func (c *Client) Host() string {
	return c.host
}

func (c *Client) APIFormat() APIFormat {
	return c.apiFormat
}

func (c *Client) APIFormatString() string {
	switch c.apiFormat {
	case FormatOllama:
		return "Ollama"
	case FormatOpenAI:
		return "OpenAI"
	default:
		return "Unknown"
	}
}

// CircuitBreakerStatus returns the current status of the circuit breaker
func (c *Client) CircuitBreakerStatus() (state string, failures int, lastFailTime time.Time) {
	return c.circuitBreaker.Stats()
}

func (c *Client) HealthCheck() error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(c.host + "/api/tags")
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama at %s: %w", c.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama health check failed with status: %s", resp.Status)
	}
	return nil
}

type ModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

type ShowModelResponse struct {
	License    string                 `json:"license"`
	Modelfile  string                 `json:"modelfile"`
	Parameters string                 `json:"parameters"`
	Template   string                 `json:"template"`
	Details    ModelDetails           `json:"details"`
	ModelInfo  map[string]interface{} `json:"model_info"`
}

type ShowModelRequest struct {
	Name    string `json:"name"`
	Verbose bool   `json:"verbose,omitempty"`
}

func (c *Client) GetModelInfo() (*ShowModelResponse, error) {
	reqBody := ShowModelRequest{
		Name:    c.model,
		Verbose: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(c.host+"/api/show", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model info request failed with status: %s", resp.Status)
	}

	var modelResp ShowModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelResp); err != nil {
		return nil, fmt.Errorf("error decoding model info response: %w", err)
	}

	return &modelResp, nil
}
