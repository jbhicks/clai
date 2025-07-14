package llm

import (
"bufio"
"bytes"
"clai/internal/tools"
"encoding/json"
"fmt"
"net/http"
"log"
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

type Client struct {
	host         string
	model        string
	systemPrompt string
}

func NewClient(host, model, systemPrompt string) *Client {
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}
	return &Client{
		host:         host,
		model:        model,
		systemPrompt: systemPrompt,
	}
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

func (c *Client) SendMessage(messages []Message) (Response, error) {
	allMessages := append([]Message{{Role: "system", Content: c.systemPrompt}}, messages...)

	reqBody := Request{
		Model:    c.model,
		Messages: allMessages,
		Tools:    tools.GetAvailableTools(),
		Stream:   false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, err
	}

	resp, err := http.Post(c.host+"/api/chat", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	var llmResp Response
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return Response{}, err
	}

	return llmResp, nil
}

func (c *Client) SendMessageStream(messages []Message, streamChan chan<- string) (Response, error) {
	allMessages := append([]Message{{Role: "system", Content: c.systemPrompt}}, messages...)

	reqBody := Request{
		Model:    c.model,
		Messages: allMessages,
		Tools:    tools.GetAvailableTools(),
		Stream:   true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, err
	}

	// Pretty print the outgoing request JSON
	prettyReq, _ := json.MarshalIndent(reqBody, "", "  ")
	log.Printf("[LLM-REQ] %s", string(prettyReq))

	resp, err := http.Post(c.host+"/api/chat", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return Response{}, err
	}

	go func() {
		defer resp.Body.Close()
		defer close(streamChan)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			raw := scanner.Bytes()
			// Log the raw JSON response for debugging
		   // log.Printf("[LLM-RAW] %s", string(raw)) // Disabled to prevent log flooding
			var llmResp Response
			if err := json.Unmarshal(raw, &llmResp); err != nil {
				// handle error, maybe send to a different channel
				log.Printf("[LLM-RAW-ERROR] %v", err)
				return
			}
			streamChan <- llmResp.Message.Content
			if llmResp.Done {
				return
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
