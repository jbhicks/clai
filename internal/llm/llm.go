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
	defaultSystemPrompt = `You are a helpful AI assistant that can use tools to answer questions.
When a user asks a question, you can use the available tools to help you answer.
To use a tool, respond with a JSON object in the following format:
{
  "tool_calls": [
	{
	  "name": "tool_name",
	  "parameters": {
		"param1": "value1",
		"param2": "value2"
	  }
	}
  ]
}
If you don't need to use a tool, just respond with a normal message.`
)

type APIFormat int

const (
	FormatUnknown APIFormat = iota
	FormatOllama
	FormatOpenAI
)

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
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
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
	Delta        OpenAIDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
	Index        int         `json:"index"`
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
func (c *Client) SendMessageWithTools(messages []Message, toolList []tools.Tool) (Response, error) {
	allMessages := append([]Message{{Role: "system", Content: c.systemPrompt}}, messages...)

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
	limited := io.LimitReader(resp.Body, maxResponseSize)

	if c.apiFormat == FormatOpenAI {
		var openAIResp OpenAIStreamChunk
		if err := json.NewDecoder(limited).Decode(&openAIResp); err != nil {
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
			llmResp.Message.Content = openAIResp.Choices[0].Delta.Content
		}

		prettyResp, _ := json.MarshalIndent(llmResp, "", "  ")
		log.Printf("[LLM-RESP] %s", string(prettyResp))

		return llmResp, nil
	}

	var llmResp Response
	if err := json.NewDecoder(limited).Decode(&llmResp); err != nil {
		return Response{}, fmt.Errorf("error decoding LLM response (possibly too large or malformed): %w", err)
	}

	prettyResp, _ := json.MarshalIndent(llmResp, "", "  ")
	log.Printf("[LLM-RESP] %s", string(prettyResp))

	return llmResp, nil
}

// ClassifyIntent asks the LLM if the query requires a tool call, and which tool.
func (c *Client) ClassifyIntent(query string) (string, error) {
	// Build a system prompt listing available tools
	availableTools := []string{"calculator", "echo", "web_search"}
	prompt := "Does this query require a tool call? If yes, which tool? Respond with the tool name or 'none'. Available tools: " +
		fmt.Sprintf("%v", availableTools)

	messages := []Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: query},
	}

	// Send to LLM without any tools
	request := Request{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
	}
	jsonBody, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(c.host+"/api/chat", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var llmResp Response
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return "", err
	}

	// Parse the tool name from the response
	toolName := llmResp.Message.Content
	return toolName, nil
}

func (c *Client) SendMessageStream(messages []Message, streamChan chan<- string, toolCallChan chan<- []ToolCall) (Response, error) {
	allMessages := append([]Message{{Role: "system", Content: c.systemPrompt}}, messages...)

	var reqBody interface{}
	if c.apiFormat == FormatOpenAI {
		reqBody = OpenAIRequest{
			Model:    c.model,
			Messages: allMessages,
			Tools:    convertToOpenAITools(tools.GetAvailableTools()),
			Stream:   true,
		}
	} else {
		reqBody = Request{
			Model:    c.model,
			Messages: allMessages,
			Tools:    tools.GetAvailableTools(),
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
