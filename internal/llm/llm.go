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
)

const (
	defaultSystemPrompt = `You are a free agent AI with full code execution capabilities. You can execute bash, python, and javascript code directly on the system.

**Critical rules:**
1. When you need to read files, execute commands, or perform system operations, use code execution by wrapping code in XML tags:
   <code language="bash">cat /path/to/file</code>
   <code language="python">print("Hello")</code>
   <code language="javascript">console.log("Hello")</code>
   Always close code tags with </code> and never leave them unclosed.

2. DO NOT use echo/print/console.log to narrate your thinking. Only use them for actual task output.
   ❌ BAD: <code language="bash">echo "Now I will read the file"</code>
   ✅ GOOD: <code language="bash">cat sample.txt</code>

3. You have filesystem access. Read files directly instead of asking the user.
4. Execute commands as needed to complete tasks.
5. Keep code blocks focused and purposeful.
6. After any tool/code execution, provide the final user-facing answer as plain text (not inside a code block).

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
	host         string
	model        string
	systemPrompt string
	apiFormat    APIFormat
}

func NewClient(host, model, systemPrompt string) *Client {
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}

	c := &Client{
		host:         host,
		model:        model,
		systemPrompt: systemPrompt,
		apiFormat:    FormatUnknown,
	}
	c.detectAPIFormat()
	return c
}

func (c *Client) detectAPIFormat() {
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
		return
	}

	resp, err := http.Post(c.host+"/api/chat", "application/json", bytes.NewBuffer(jsonBody))
	if err == nil {
		defer resp.Body.Close()

		body, readErr := io.ReadAll(resp.Body)
		if readErr == nil {
			var ollamaResp Response
			if err := json.Unmarshal(body, &ollamaResp); err == nil && ollamaResp.Message.Content != "" {
				logger.Debug("[FORMAT-DETECT] Detected Ollama native format")
				c.apiFormat = FormatOllama
				return
			}
		}
	} else {
		logger.Debug("[FORMAT-DETECT] Ollama /api/chat request failed: %v", err)
	}

	openAIReq := OpenAIRequest{
		Model:    c.model,
		Messages: testMessages,
		Stream:   false,
	}
	openAIJSON, err := json.Marshal(openAIReq)
	if err == nil {
		openAIResp, openAIErr := http.Post(c.host+"/v1/chat/completions", "application/json", bytes.NewBuffer(openAIJSON))
		if openAIErr == nil {
			defer openAIResp.Body.Close()
			body, readErr := io.ReadAll(openAIResp.Body)
			if readErr == nil {
				var openAIParsed OpenAIResponse
				if err := json.Unmarshal(body, &openAIParsed); err == nil && len(openAIParsed.Choices) > 0 {
					logger.Debug("[FORMAT-DETECT] Detected OpenAI-compatible format")
					c.apiFormat = FormatOpenAI
					return
				}
			}
		} else {
			logger.Debug("[FORMAT-DETECT] OpenAI /v1/chat/completions request failed: %v", openAIErr)
		}
	}

	logger.Debug("[FORMAT-DETECT] Final API format for model %s: %d (%s)", c.model, c.apiFormat, c.APIFormatString())
}

func (c *Client) getChatEndpoint() string {
	if c.apiFormat == FormatOpenAI {
		return c.host + "/v1/chat/completions"
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("[LLM-STREAM-ERROR] HTTP request failed: %v", err)
		return Response{}, err
	}

	logger.Debug("[LLM-STREAM] HTTP response received, status: %s, starting goroutine", resp.Status)

	go func() {
		defer resp.Body.Close()
		defer close(streamChan)
		logger.Debug("[LLM-STREAM] Goroutine started, apiFormat=%d (2=OpenAI)", c.apiFormat)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 4096), bufio.MaxScanTokenSize)

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
			}
			logger.Info("[LLM-OPENAI-STREAM] Stream reading completed")
		} else {
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

				if llmResp.Done {
					return
				}
			}
		}
	}()

	return Response{}, nil
}

func (c *Client) SendMessageStreamWithTools(messages []Message, tools []Tool, streamChan chan<- string, includeSystemPrompt bool) (Response, error) {
	var allMessages []Message

	// Prepare system prompt with tools for Hermes-style (Qwen) or include tools in request for OpenAI-style
	systemPrompt := c.systemPrompt
	if includeSystemPrompt && c.systemPrompt != "" {
		if len(tools) > 0 && c.apiFormat == FormatOllama {
			toolsJSON, err := json.Marshal(tools)
			if err != nil {
				return Response{}, fmt.Errorf("failed to marshal tools: %w", err)
			}
			systemPrompt = c.systemPrompt + "\n\nAvailable tools:\n" + string(toolsJSON)
		}
		if len(tools) > 0 && strings.Contains(strings.ToLower(c.model), "qwen") {
			systemPrompt = systemPrompt + "\n\nWhen tools are available, do NOT use <code> blocks. If you need to run a tool, respond with a single JSON object only (no extra text): {\"action\":\"bash\",\"command\":\"...\"} or {\"action\":\"python\",\"code\":\"...\"}. After tool execution, respond with the final answer as plain text."
		}
		allMessages = append([]Message{{Role: "system", Content: systemPrompt}}, messages...)
		logger.Debug("[LLM-TOOLS] Final system prompt for model %s: %s", c.model, systemPrompt)
	} else {
		allMessages = messages
	}

	var reqBody interface{}
	if c.apiFormat == FormatOpenAI {
		reqBody = OpenAIRequestWithTools{
			Model:    c.model,
			Messages: allMessages,
			Tools:    tools,
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Debug("[LLM-STREAM-ERROR] HTTP request failed: %v", err)
		return Response{}, err
	}

	logger.Debug("[LLM-STREAM] HTTP response received, status: %s, starting goroutine", resp.Status)

	go func() {
		defer resp.Body.Close()
		defer close(streamChan)
		logger.Debug("[LLM-STREAM] Goroutine started for tools, apiFormat=%d (2=OpenAI)", c.apiFormat)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 4096), bufio.MaxScanTokenSize)

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

func (c *Client) HealthCheck() error {
	resp, err := http.Get(c.host + "/api/tags")
	if err != nil {
		resp, openAIErr := http.Get(c.host + "/v1/models")
		if openAIErr != nil {
			return fmt.Errorf("failed to connect to LLM at %s: %w", c.host, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("llm health check failed with status: %s", resp.Status)
		}
		return nil
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

	resp, err := http.Post(c.host+"/api/show", "application/json", bytes.NewBuffer(jsonBody))
	if err == nil {
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

	resp, err = http.Post(c.host+"/v1/models", "application/json", bytes.NewBuffer(jsonBody))
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

// DefaultSystemPrompt exposes the fallback system prompt.
func DefaultSystemPrompt() string {
	return defaultSystemPrompt
}
