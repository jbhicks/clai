package llm

import (
	"clai/internal/tools"
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentParseThought(t *testing.T) {
	agent := NewAgent(nil)

	response := "I need to calculate 5 + 3 using JavaScript.\n\n<code language=\"javascript\">\n5 + 3\n</code>"

	parsed, err := agent.parseResponse(response)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Thought != "I need to calculate 5 + 3 using JavaScript." {
		t.Errorf("Expected thought, got: %s", parsed.Thought)
	}

	if parsed.Code != "5 + 3" {
		t.Errorf("Expected code '5 + 3', got: %s", parsed.Code)
	}

	if parsed.Final != "" {
		t.Errorf("Expected no final answer, got: %s", parsed.Final)
	}
}

func TestAgentParseFinal(t *testing.T) {
	agent := NewAgent(nil)

	response := "The result is 8."

	parsed, err := agent.parseResponse(response)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Final != "The result is 8." {
		t.Errorf("Expected final answer, got: %s", parsed.Final)
	}

	if parsed.Code != "" {
		t.Errorf("Expected no code, got: %s", parsed.Code)
	}
}

func TestAgentParseCodeBlock(t *testing.T) {
	agent := NewAgent(nil)

	response := "Need to execute JavaScript\n\n<code language=\"javascript\">\nvar x = 10;\nvar y = 20;\nx + y\n</code>"

	parsed, err := agent.parseResponse(response)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Code == "" {
		t.Errorf("Expected code to be parsed, got empty string")
	}

	if !strings.Contains(parsed.Code, "var x = 10") {
		t.Errorf("Expected code to contain 'var x = 10', got: %s", parsed.Code)
	}
}

func TestAgentParseFinalAnswerOnly(t *testing.T) {
	agent := NewAgent(nil)

	response := "The answer is 42."

	parsed, err := agent.parseResponse(response)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Final != "The answer is 42." {
		t.Errorf("Expected final answer, got: %s", parsed.Final)
	}
}

// Tool calling tests
func TestAgentParseToolCallFromStream(t *testing.T) {
	// Test parsing tool calls from streaming chunks
	streamChan := make(chan string, 10)

	// Simulate streaming response with tool call
	toolCall := tools.ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: tools.ToolCallFunc{
			Name:      "execute_bash",
			Arguments: "{\"command\":\"echo hello\"}",
		},
	}

	toolCallJSON, _ := json.Marshal(toolCall)
	streamChan <- "Let me run a command for you."
	streamChan <- string(toolCallJSON)
	streamChan <- " That should show the output."
	close(streamChan)

	// Test the parsing logic
	var fullContent strings.Builder
	var toolCalls []tools.ToolCall

	for chunk := range streamChan {
		// Try to parse as tool call first
		var parsedToolCall tools.ToolCall
		if err := json.Unmarshal([]byte(chunk), &parsedToolCall); err == nil && parsedToolCall.ID != "" {
			toolCalls = append(toolCalls, parsedToolCall)
		} else {
			// Not a tool call, treat as content
			fullContent.WriteString(chunk)
		}
	}

	content := fullContent.String()
	if !strings.Contains(content, "Let me run a command") {
		t.Errorf("Expected content to contain assistant message, got: %s", content)
	}

	if len(toolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got: %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "execute_bash" {
		t.Errorf("Expected tool call name 'execute_bash', got: %s", toolCalls[0].Function.Name)
	}
}

func TestAgentToolExecution(t *testing.T) {
	// Test tool execution logic
	toolCall := tools.ToolCall{
		ID:   "call_test",
		Type: "function",
		Function: tools.ToolCallFunc{
			Name:      "execute_bash",
			Arguments: "{\"command\":\"echo 'test output'\"}",
		},
	}

	result, err := tools.ExecuteTool(toolCall)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if !strings.Contains(result, "test output") {
		t.Errorf("Expected tool result to contain 'test output', got: %s", result)
	}
}

func TestAgentToolCallJSONParsing(t *testing.T) {
	tests := []struct {
		name        string
		jsonChunk   string
		expectParse bool
		toolName    string
	}{
		{
			name:        "valid tool call",
			jsonChunk:   `{"id":"call_123","type":"function","function":{"name":"execute_bash","arguments":"{\"command\":\"ls\"}"}}`,
			expectParse: true,
			toolName:    "execute_bash",
		},
		{
			name:        "regular content",
			jsonChunk:   "This is regular assistant response text.",
			expectParse: false,
			toolName:    "",
		},
		{
			name:        "malformed JSON",
			jsonChunk:   `{"id":"call_123","type":"function","function":{"name":`,
			expectParse: false,
			toolName:    "",
		},
		{
			name:        "tool call without ID",
			jsonChunk:   `{"type":"function","function":{"name":"execute_bash","arguments":"{}"}}`,
			expectParse: false,
			toolName:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var toolCall tools.ToolCall
			err := json.Unmarshal([]byte(tt.jsonChunk), &toolCall)
			parsed := err == nil && toolCall.ID != ""

			if parsed != tt.expectParse {
				t.Errorf("Expected parse=%v, got parse=%v", tt.expectParse, parsed)
			}

			if parsed && toolCall.Function.Name != tt.toolName {
				t.Errorf("Expected tool name '%s', got '%s'", tt.toolName, toolCall.Function.Name)
			}
		})
	}
}

func TestAgentParseFragmentedToolCalls(t *testing.T) {
	content := `{"id":"call_1","type":"function","function":{"name":"execute_bash","arguments":"{"}}` +
		`{"id":"","type":"","function":{"name":"","arguments":"command"}}` +
		`{"id":"","type":"","function":{"name":"","arguments":"\":"}}` +
		`{"id":"","type":"","function":{"name":"","arguments":"\"cat"}}` +
		`{"id":"","type":"","function":{"name":"","arguments":" internal"}}` +
		`{"id":"","type":"","function":{"name":"","arguments":"/"}}` +
		`{"id":"","type":"","function":{"name":"","arguments":"llm"}}` +
		`{"id":"","type":"","function":{"name":"","arguments":"/sample"}}` +
		`{"id":"","type":"","function":{"name":"","arguments":".txt"}}` +
		`{"id":"","type":"","function":{"name":"","arguments":"\""}}` +
		`{"id":"","type":"","function":{"name":"","arguments":"}"}}`

	toolCalls := parseFragmentedToolCalls(content)
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "execute_bash" {
		t.Fatalf("expected tool call execute_bash, got %s", toolCalls[0].Function.Name)
	}

	if !strings.Contains(toolCalls[0].Function.Arguments, "internal/llm/sample.txt") {
		t.Fatalf("expected arguments to contain sample path, got %s", toolCalls[0].Function.Arguments)
	}
}

func TestAgentAddMessageWithToolCallID(t *testing.T) {
	agent := NewAgent(nil)

	// Test adding message without tool_call_id
	agent.AddMessage("user", "test message")
	if len(agent.messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(agent.messages))
	}
	if agent.messages[0].ToolCallID != "" {
		t.Errorf("Expected empty tool_call_id, got: %s", agent.messages[0].ToolCallID)
	}

	// Test adding message with tool_call_id
	agent.AddMessage("tool", "tool result", "call_123")
	if len(agent.messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(agent.messages))
	}
	if agent.messages[1].ToolCallID != "call_123" {
		t.Errorf("Expected tool_call_id 'call_123', got: %s", agent.messages[1].ToolCallID)
	}
}

func TestAgentBackwardCompatibilityXML(t *testing.T) {
	agent := NewAgent(nil)

	// Test that XML parsing still works when no tool calls are present
	response := "I need to run a command.\n\n<code language=\"bash\">\necho 'hello world'\n</code>"

	parsed, err := agent.parseResponse(response)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Code != "echo 'hello world'" {
		t.Errorf("Expected code 'echo 'hello world'', got: %s", parsed.Code)
	}

	if parsed.Language != "bash" {
		t.Errorf("Expected language 'bash', got: %s", parsed.Language)
	}
}

// Mock LLM client for integration testing
type MockLLMClient struct {
	callCount int
	mode      string // "tool_calling" or "xml_fallback"
}

func (m *MockLLMClient) SendMessageStreamNoTools(messages []Message, streamChan chan<- string, includeSystemPrompt bool) (Response, error) {
	return Response{}, nil
}

func (m *MockLLMClient) SendMessageStreamWithTools(messages []Message, tools []Tool, streamChan chan<- string, includeSystemPrompt bool) (Response, error) {
	m.callCount++

	switch m.mode {
	case "tool_calling":
		if m.callCount == 1 {
			// First call - return tool call
			toolCallJSON := `{"id":"call_123","type":"function","function":{"name":"execute_bash","arguments":"{\"command\":\"echo 'hello from tool'\"}"}}`
			streamChan <- toolCallJSON
		} else {
			// Second call - return final answer after tool execution
			streamChan <- "Based on the tool result, the command output is: hello from tool"
		}
	case "xml_fallback":
		// Return XML content instead of tool call
		streamChan <- "I need to run a command.\n\n<code language=\"bash\">\necho 'hello world'\n</code>"
	}

	close(streamChan)
	return Response{}, nil
}

func (m *MockLLMClient) Model() string           { return "mock-qwen" }
func (m *MockLLMClient) Host() string            { return "mock:8080" }
func (m *MockLLMClient) APIFormat() APIFormat    { return FormatOllama }
func (m *MockLLMClient) APIFormatString() string { return "Ollama" }
func (m *MockLLMClient) HealthCheck() error      { return nil }

func TestAgentToolCallingIntegration(t *testing.T) {
	// Test full agent loop with tool calls using mocked responses

	mockClient := &MockLLMClient{mode: "tool_calling"}

	agent := NewAgent(mockClient)

	result, err := agent.Run("Run a command that says hello")
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Verify the result contains the tool output
	if !strings.Contains(result, "hello from tool") {
		t.Errorf("Expected result to contain tool output 'hello from tool', got: %s", result)
	}

	// Check that tool message has correct tool_call_id
	toolMessageFound := false
	for _, msg := range agent.messages {
		if msg.Role == "tool" && msg.ToolCallID == "call_123" {
			toolMessageFound = true
			break
		}
	}
	if !toolMessageFound {
		t.Error("Expected to find tool message with tool_call_id 'call_123'")
	}
}

func TestAgentFallbackToXMLWhenNoTools(t *testing.T) {
	// Test that agent falls back to XML parsing when no tool calls are present

	mockClient := &MockLLMClient{mode: "xml_fallback"}

	agent := NewAgent(mockClient)

	result, err := agent.Run("Run a command")
	if err != nil {
		t.Fatalf("Agent run failed: %v", err)
	}

	// Verify XML parsing still works
	if result == "" {
		t.Errorf("Expected some result from XML fallback, got empty string")
	}

	// Check that it contains the expected output from XML execution
	if !strings.Contains(result, "hello world") {
		t.Errorf("Expected result to contain 'hello world' from XML execution, got: %s", result)
	}
}

func TestParseToolCallsForBenchmark(t *testing.T) {
	agent := NewAgent(&MockLLMClient{})

	content := `{"id":"call_123","type":"function","function":{"name":"execute_bash","arguments":"{\"command\":\"cat internal/llm/sample.txt\"}"}}`
	toolCalls := agent.ParseToolCallsForBenchmark(content)

	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "execute_bash" {
		t.Errorf("Expected tool name 'execute_bash', got: %s", toolCalls[0].Function.Name)
	}

	if !strings.Contains(toolCalls[0].Function.Arguments, "internal/llm/sample.txt") {
		t.Errorf("Expected tool arguments to contain sample.txt path, got: %s", toolCalls[0].Function.Arguments)
	}
}
