package llm

import (
	"bufio"
	"bytes"
	"clai/internal/logger"
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

2. DO NOT use echo/print/console.log to narrate your thinking. Only use them for actual task output.
   ❌ BAD: <code language="bash">echo "Now I will read the file"</code>
   ✅ GOOD: <code language="bash">cat sample.txt</code>

3. You have filesystem access. Read files directly instead of asking the user.
4. Execute commands as needed to complete tasks.
5. Keep code blocks focused and purposeful.

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
	if err != nil {
		logger.Debug("[FORMAT-DETECT] Failed to send test request: %v", err)
		c.apiFormat = FormatOllama
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Debug("[FORMAT-DETECT] Failed to read response: %v", err)
		c.apiFormat = FormatOllama
		return
	}

	var ollamaResp Response
	if err := json.Unmarshal(body, &ollamaResp); err == nil && ollamaResp.Message.Content != "" {
		logger.Debug("[FORMAT-DETECT] Detected Ollama native format")
		c.apiFormat = FormatOllama
		return
	}

	var openAIResp OpenAIStreamChunk
	if err := json.Unmarshal(body, &openAIResp); err == nil && len(openAIResp.Choices) > 0 {
		logger.Debug("[FORMAT-DETECT] Detected OpenAI-compatible format")
		c.apiFormat = FormatOpenAI
		return
	}

	logger.Debug("[FORMAT-DETECT] Unknown format, defaulting to Ollama native")
	c.apiFormat = FormatOllama
}


type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

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
	Content string `json:"content"`
	Role    string `json:"role,omitempty"`
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

type OpenAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
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

	req, err := http.NewRequest("POST", c.host+"/api/chat", bytes.NewBuffer(jsonBody))
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
