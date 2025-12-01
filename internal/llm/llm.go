package llm

import (
	"bufio"
	"bytes"
	"clai/internal/tools"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

2. DO NOT use echo/print/console.log to narrate your thinking. Only use them for actual task output.
   ❌ BAD: <code language="bash">echo "Now I will read the file"</code>
   ✅ GOOD: <code language="bash">cat sample.txt</code>

3. You have filesystem access. Read files directly instead of asking the user.
4. Execute commands as needed to complete tasks.
5. Keep code blocks focused and purposeful.

Answer questions clearly and execute code when needed to provide accurate information.`
)

type APIFormat int

const (
	FormatUnknown APIFormat = iota
	FormatOllama
	FormatOpenAI
)

type LLMClientInterface interface {
	SendMessageStream(messages []Message, streamChan chan<- string, toolCallChan chan<- []ToolCall) (Response, error)
	SendMessageStreamNoTools(messages []Message, streamChan chan<- string, toolCallChan chan<- []ToolCall) (Response, error)
	SendMessageStreamWithTools(messages []Message, streamChan chan<- string, toolCallChan chan<- []ToolCall, selectedTools []tools.Tool) (Response, error)
	SelectToolsForQuery(query string) ([]tools.Tool, error)
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
		log.Printf("[FORMAT-DETECT] Failed to marshal test request: %v", err)
		c.apiFormat = FormatOllama
		return
	}

	resp, err := http.Post(c.host+"/api/chat", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("[FORMAT-DETECT] Failed to send test request: %v", err)
		c.apiFormat = FormatOllama
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[FORMAT-DETECT] Failed to read response: %v", err)
		c.apiFormat = FormatOllama
		return
	}

	var ollamaResp Response
	if err := json.Unmarshal(body, &ollamaResp); err == nil && ollamaResp.Message.Content != "" {
		log.Printf("[FORMAT-DETECT] Detected Ollama native format")
		c.apiFormat = FormatOllama
		return
	}

	var openAIResp OpenAIStreamChunk
	if err := json.Unmarshal(body, &openAIResp); err == nil && len(openAIResp.Choices) > 0 {
		log.Printf("[FORMAT-DETECT] Detected OpenAI-compatible format")
		c.apiFormat = FormatOpenAI
		return
	}

	log.Printf("[FORMAT-DETECT] Unknown format, defaulting to Ollama native")
	c.apiFormat = FormatOllama
}

type ToolCall struct {
	Name       string          `json:"name"`
	Parameters json.RawMessage `json:"parameters"`
}

type Message struct {
	Role          string     `json:"role"`
	Content       string     `json:"content"`
	ToolCalls     []ToolCall `json:"tool_calls,omitempty"`
	SelectedTools []string   `json:"selected_tools,omitempty"`
}

type Request struct {
	Model    string       `json:"model"`
	Messages []Message    `json:"messages"`
	Tools    []tools.Tool `json:"tools,omitempty"`
	Stream   bool         `json:"stream"`
}

type Response struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

type OpenAIToolCallDelta struct {
	Index    int                          `json:"index"`
	ID       string                       `json:"id,omitempty"`
	Type     string                       `json:"type,omitempty"`
	Function *OpenAIToolFunctionCallDelta `json:"function,omitempty"`
}

type OpenAIToolFunctionCallDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type OpenAIDelta struct {
	Content   string                `json:"content"`
	Role      string                `json:"role,omitempty"`
	ToolCalls []OpenAIToolCallDelta `json:"tool_calls,omitempty"`
}

type OpenAIChoice struct {
	Delta        OpenAIDelta   `json:"delta"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
	Index        int           `json:"index"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIStreamChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
}

type OpenAIToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type OpenAITool struct {
	Type     string             `json:"type"`
	Function OpenAIToolFunction `json:"function"`
}

type OpenAIRequest struct {
	Model    string       `json:"model"`
	Messages []Message    `json:"messages"`
	Tools    []OpenAITool `json:"tools,omitempty"`
	Stream   bool         `json:"stream"`
}

func convertToOpenAITools(ollamaTools []tools.Tool) []OpenAITool {
	openAITools := make([]OpenAITool, len(ollamaTools))
	for i, t := range ollamaTools {
		openAITools[i] = OpenAITool{
			Type: "function",
			Function: OpenAIToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}
	return openAITools
}

func (c *Client) SendMessage(messages []Message) (Response, error) {
	return c.SendMessageWithTools(messages, tools.GetAvailableTools())
}

// SendMessageWithTools allows specifying which tools to include in the request.
func (c *Client) sendMessageWithToolsRaw(messages []Message, toolList []tools.Tool, includeSystemPrompt bool) (Response, error) {
	var allMessages []Message
	if includeSystemPrompt && c.systemPrompt != "" {
		allMessages = append([]Message{{Role: "system", Content: c.systemPrompt}}, messages...)
	} else {
		allMessages = messages
	}

	var jsonBody []byte
	var err error

	if c.apiFormat == FormatOpenAI {
		reqBody := OpenAIRequest{
			Model:    c.model,
			Messages: allMessages,
			Tools:    convertToOpenAITools(toolList),
			Stream:   false,
		}
		jsonBody, err = json.Marshal(reqBody)
	} else {
		reqBody := Request{
			Model:    c.model,
			Messages: allMessages,
			Tools:    toolList,
			Stream:   false,
		}
		jsonBody, err = json.Marshal(reqBody)
	}

	if err != nil {
		return Response{}, err
	}

	resp, err := http.Post(c.host+"/api/chat", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	log.Printf("Ollama response status: %s", resp.Status)
	for k, v := range resp.Header {
		log.Printf("Header: %s: %v", k, v)
	}

	const maxResponseSize = 1 << 20
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return Response{}, fmt.Errorf("error reading response body: %w", err)
	}
	log.Printf("[LLM-RAW-RESP] %s", string(bodyBytes))

	if c.apiFormat == FormatOpenAI {
		var openAIResp OpenAIStreamChunk
		if err := json.Unmarshal(bodyBytes, &openAIResp); err != nil {
			return Response{}, fmt.Errorf("error decoding OpenAI response: %w", err)
		}

		llmResp := Response{
			Message: Message{
				Role:    "assistant",
				Content: "",
			},
			Done: true,
		}

		if len(openAIResp.Choices) > 0 {
			if openAIResp.Choices[0].Message.Content != "" {
				llmResp.Message.Content = openAIResp.Choices[0].Message.Content
			} else {
				llmResp.Message.Content = openAIResp.Choices[0].Delta.Content
			}
		}

		prettyResp, _ := json.MarshalIndent(llmResp, "", "  ")
		log.Printf("[LLM-RESP] %s", string(prettyResp))

		return llmResp, nil
	}

	var llmResp Response
	if err := json.Unmarshal(bodyBytes, &llmResp); err != nil {
		return Response{}, fmt.Errorf("error decoding LLM response (possibly too large or malformed): %w", err)
	}

	prettyResp, _ := json.MarshalIndent(llmResp, "", "  ")
	log.Printf("[LLM-RESP] %s", string(prettyResp))

	return llmResp, nil
}

func (c *Client) SendMessageWithTools(messages []Message, toolList []tools.Tool) (Response, error) {
	return c.sendMessageWithToolsRaw(messages, toolList, true)
}

func (c *Client) SelectToolsForQuery(query string) ([]tools.Tool, error) {
	prompt := fmt.Sprintf(`You are a tool selection assistant. Analyze the user's query and determine which tools are needed.

User query: "%s"

Available tools:
%s

Instructions:
- Tools should ONLY be selected if the query explicitly requires external capabilities
- For simple conversations, greetings, or questions you can answer directly, respond with: none
- For mathematical calculations, select: calculator
- For web searches or current information, select: web_search
- Respond with ONLY the tool name(s) needed, comma-separated
- Examples: "calculator" or "web_search,calculator" or "none"
- Do NOT include any explanations, reasoning, or additional text

Your response:`,
		query,
		tools.GetToolDescriptions())

	log.Printf("[TOOL-SELECT] Sending prompt to LLM: %q", prompt)

	messages := []Message{
		{Role: "user", Content: prompt},
	}

	resp, err := c.sendMessageWithToolsRaw(messages, []tools.Tool{}, false)
	if err != nil {
		log.Printf("[TOOL-SELECT] Error selecting tools, falling back to all tools: %v", err)
		return tools.GetAvailableTools(), nil
	}

	response := strings.TrimSpace(resp.Message.Content)
	log.Printf("[TOOL-SELECT] LLM raw response: %q (len=%d)", response, len(response))
	log.Printf("[TOOL-SELECT] Response struct: %+v", resp)

	if response == "" || strings.ToLower(response) == "none" {
		log.Printf("[TOOL-SELECT] No tools selected, returning empty list")
		return []tools.Tool{}, nil
	}

	toolNames := []string{}
	for _, name := range strings.Split(response, ",") {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			toolNames = append(toolNames, trimmed)
		}
	}

	selected := tools.GetToolsByNames(toolNames...)
	if len(selected) == 0 {
		log.Printf("[TOOL-SELECT] No valid tools matched, falling back to all tools")
		return tools.GetAvailableTools(), nil
	}

	log.Printf("[TOOL-SELECT] Selected %d tools: %v", len(selected), toolNames)
	return selected, nil
}

func (c *Client) SendMessageStream(messages []Message, streamChan chan<- string, toolCallChan chan<- []ToolCall) (Response, error) {
	return c.SendMessageStreamWithTools(messages, streamChan, toolCallChan, tools.GetAvailableTools())
}

func (c *Client) SendMessageStreamNoTools(messages []Message, streamChan chan<- string, toolCallChan chan<- []ToolCall) (Response, error) {
	allMessages := append([]Message{{Role: "system", Content: c.systemPrompt}}, messages...)

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
	log.Printf("[LLM-REQ-NO-TOOLS] %s", string(prettyReq))

	req, err := http.NewRequest("POST", c.host+"/api/chat", bytes.NewBuffer(jsonBody))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[LLM-STREAM-ERROR] HTTP request failed: %v", err)
		return Response{}, err
	}

	log.Printf("[LLM-STREAM] HTTP response received, status: %s, starting goroutine", resp.Status)

	go func() {
		defer resp.Body.Close()
		defer close(streamChan)
		defer close(toolCallChan)
		log.Printf("[LLM-STREAM] Goroutine started, apiFormat=%d (2=OpenAI)", c.apiFormat)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 4096), bufio.MaxScanTokenSize)

		if c.apiFormat == FormatOpenAI {
			log.Printf("[LLM-OPENAI-STREAM] Starting OpenAI stream reading (no tools)")

			for scanner.Scan() {
				line := scanner.Text()
				log.Printf("[LLM-OPENAI-STREAM] Raw line: %q", line)
				line = strings.TrimSpace(line)

				if line == "" {
					continue
				}

				if !strings.HasPrefix(line, "data: ") {
					log.Printf("[LLM-OPENAI-STREAM] Line doesn't have 'data: ' prefix, skipping")
					continue
				}

				data := strings.TrimPrefix(line, "data: ")
				log.Printf("[LLM-OPENAI-STREAM] Data after prefix removal: %q", data)

				if data == "[DONE]" {
					log.Printf("[LLM-OPENAI-STREAM] Received [DONE] marker")
					return
				}

				var chunk OpenAIStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					log.Printf("[LLM-OPENAI-ERROR] Failed to parse chunk: %v, data: %q", err, data)
					continue
				}

				log.Printf("[LLM-OPENAI-STREAM] Parsed chunk with %d choices", len(chunk.Choices))
				if len(chunk.Choices) > 0 {
					choice := chunk.Choices[0]
					if choice.Delta.Content != "" {
						log.Printf("[LLM-OPENAI-STREAM] Sending content to channel: %q", choice.Delta.Content)
						streamChan <- choice.Delta.Content
					}

					if choice.FinishReason != "" && choice.FinishReason != "null" {
						log.Printf("[LLM-OPENAI-STREAM] Received finish reason: %s", choice.FinishReason)
						return
					}
				}
			}
			if err := scanner.Err(); err != nil {
				log.Printf("[LLM-OPENAI-ERROR] Scanner error: %v", err)
			}
			log.Printf("[LLM-OPENAI-STREAM] Stream reading completed")
		} else {
			for scanner.Scan() {
				raw := scanner.Bytes()
				var llmResp Response
				if err := json.Unmarshal(raw, &llmResp); err != nil {
					log.Printf("[LLM-RAW-ERROR] %v", err)
					return
				}

				prettyResp, _ := json.MarshalIndent(llmResp, "", "  ")
				log.Printf("[LLM-RESP-STREAM] %s", string(prettyResp))

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

func (c *Client) SendMessageStreamWithTools(messages []Message, streamChan chan<- string, toolCallChan chan<- []ToolCall, selectedTools []tools.Tool) (Response, error) {
	allMessages := append([]Message{{Role: "system", Content: c.systemPrompt}}, messages...)

	var reqBody interface{}
	if c.apiFormat == FormatOpenAI {
		reqBody = OpenAIRequest{
			Model:    c.model,
			Messages: allMessages,
			Tools:    convertToOpenAITools(selectedTools),
			Stream:   true,
		}
	} else {
		reqBody = Request{
			Model:    c.model,
			Messages: allMessages,
			Tools:    selectedTools,
			Stream:   true,
		}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, err
	}

	// Pretty print the outgoing request JSON
	prettyReq, _ := json.MarshalIndent(reqBody, "", "  ")
	log.Printf("[LLM-REQ] %s", string(prettyReq))

	req, err := http.NewRequest("POST", c.host+"/api/chat", bytes.NewBuffer(jsonBody))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[LLM-STREAM-ERROR] HTTP request failed: %v", err)
		return Response{}, err
	}

	log.Printf("[LLM-STREAM] HTTP response received, status: %s, starting goroutine", resp.Status)

	go func() {
		defer resp.Body.Close()
		defer close(streamChan)
		defer close(toolCallChan)
		log.Printf("[LLM-STREAM] Goroutine started, apiFormat=%d (2=OpenAI)", c.apiFormat)

		var accumulatedToolCalls []ToolCall
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 4096), bufio.MaxScanTokenSize)

		if c.apiFormat == FormatOpenAI {
			log.Printf("[LLM-OPENAI-STREAM] Starting OpenAI stream reading")
			toolCallsMap := make(map[int]*ToolCall)

			for scanner.Scan() {
				line := scanner.Text()
				log.Printf("[LLM-OPENAI-STREAM] Raw line: %q", line)
				line = strings.TrimSpace(line)

				if line == "" {
					continue
				}

				if !strings.HasPrefix(line, "data: ") {
					log.Printf("[LLM-OPENAI-STREAM] Line doesn't have 'data: ' prefix, skipping")
					continue
				}

				data := strings.TrimPrefix(line, "data: ")
				log.Printf("[LLM-OPENAI-STREAM] Data after prefix removal: %q", data)

				if data == "[DONE]" {
					log.Printf("[LLM-OPENAI-STREAM] Received [DONE] marker")
					return
				}

				var chunk OpenAIStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					log.Printf("[LLM-OPENAI-ERROR] Failed to parse chunk: %v, data: %q", err, data)
					continue
				}

				log.Printf("[LLM-OPENAI-STREAM] Parsed chunk with %d choices", len(chunk.Choices))
				if len(chunk.Choices) > 0 {
					choice := chunk.Choices[0]
					if choice.Delta.Content != "" {
						log.Printf("[LLM-OPENAI-STREAM] Sending content to channel: %q", choice.Delta.Content)
						streamChan <- choice.Delta.Content
					}

					for _, tcDelta := range choice.Delta.ToolCalls {
						if toolCallsMap[tcDelta.Index] == nil {
							toolCallsMap[tcDelta.Index] = &ToolCall{
								Name:       "",
								Parameters: json.RawMessage{},
							}
						}
						tc := toolCallsMap[tcDelta.Index]
						if tcDelta.Function != nil {
							if tcDelta.Function.Name != "" {
								tc.Name = tcDelta.Function.Name
							}
							if tcDelta.Function.Arguments != "" {
								tc.Parameters = append(tc.Parameters, []byte(tcDelta.Function.Arguments)...)
							}
						}
						log.Printf("[LLM-OPENAI-STREAM] Accumulated tool call %d: name=%s, args=%s", tcDelta.Index, tc.Name, string(tc.Parameters))
					}

					if choice.FinishReason != "" && choice.FinishReason != "null" {
						log.Printf("[LLM-OPENAI-STREAM] Received finish reason: %s", choice.FinishReason)
						if choice.FinishReason == "tool_calls" && len(toolCallsMap) > 0 {
							for _, tc := range toolCallsMap {
								accumulatedToolCalls = append(accumulatedToolCalls, *tc)
							}
							log.Printf("[LLM-TOOL-CALLS] %d tool calls detected", len(accumulatedToolCalls))
							toolCallChan <- accumulatedToolCalls
						}
						return
					}
				}
			}
			if err := scanner.Err(); err != nil {
				log.Printf("[LLM-OPENAI-ERROR] Scanner error: %v", err)
			}
			log.Printf("[LLM-OPENAI-STREAM] Stream reading completed")
		} else {
			for scanner.Scan() {
				raw := scanner.Bytes()
				var llmResp Response
				if err := json.Unmarshal(raw, &llmResp); err != nil {
					log.Printf("[LLM-RAW-ERROR] %v", err)
					return
				}

				prettyResp, _ := json.MarshalIndent(llmResp, "", "  ")
				log.Printf("[LLM-RESP-STREAM] %s", string(prettyResp))

				if len(llmResp.Message.ToolCalls) > 0 {
					accumulatedToolCalls = append(accumulatedToolCalls, llmResp.Message.ToolCalls...)
				}

				if llmResp.Message.Content != "" {
					streamChan <- llmResp.Message.Content
				}

				if llmResp.Done {
					if len(accumulatedToolCalls) > 0 {
						log.Printf("[LLM-TOOL-CALLS] %d tool calls detected", len(accumulatedToolCalls))
						toolCallChan <- accumulatedToolCalls
					}
					return
				}
			}
		}
	}()

	// This is not ideal, but we need to return a response.
	// A better approach would be to have a single streaming function that returns a channel.
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

	resp, err := http.Post(c.host+"/api/show", "application/json", bytes.NewBuffer(jsonBody))
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
