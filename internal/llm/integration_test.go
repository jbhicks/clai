package llm

import (
	"clai/internal/tools"
	"testing"
)

func TestToolCallAssemblyIntegration(t *testing.T) {
	// This test simulates the complete OpenAI streaming workflow with fragmented tool calls

	assembler := NewStreamingToolCallAssembler()
	defer assembler.Reset()

	// Simulate fragmented OpenAI streaming deltas for a complete tool call
	deltas := []OpenAIDelta{
		// First chunk: ID and type only
		{
			ToolCalls: []tools.ToolCall{
				{
					ID:   "call_test_123",
					Type: "function",
					Function: tools.ToolCallFunc{
						Name:      "",
						Arguments: "",
					},
				},
			},
		},
		// Second chunk: function name
		{
			ToolCalls: []tools.ToolCall{
				{
					ID:   "call_test_123",
					Type: "function",
					Function: tools.ToolCallFunc{
						Name:      "execute_bash",
						Arguments: "",
					},
				},
			},
		},
		// Third chunk: partial arguments
		{
			ToolCalls: []tools.ToolCall{
				{
					ID:   "call_test_123",
					Type: "function",
					Function: tools.ToolCallFunc{
						Name:      "execute_bash",
						Arguments: `{"comman`,
					},
				},
			},
		},
		// Final chunk: complete arguments
		{
			ToolCalls: []tools.ToolCall{
				{
					ID:   "call_test_123",
					Type: "function",
					Function: tools.ToolCallFunc{
						Name:      "execute_bash",
						Arguments: `d": "ls -la"}`,
					},
				},
			},
		},
	}

	var completedCalls []tools.ToolCall

	// Process all deltas
	for i, delta := range deltas {
		calls := assembler.ProcessDelta(delta)
		completedCalls = append(completedCalls, calls...)

		// Only the final delta should produce a completed call
		expectedCount := 0
		if i == len(deltas)-1 {
			expectedCount = 1
		}

		if len(calls) != expectedCount {
			t.Errorf("Delta %d: Expected %d completed calls, got %d", i, expectedCount, len(calls))
		}
	}

	// Verify the final completed call
	if len(completedCalls) != 1 {
		t.Fatalf("Expected 1 completed call total, got %d", len(completedCalls))
	}

	call := completedCalls[0]
	if call.ID != "call_test_123" {
		t.Errorf("Expected ID 'call_test_123', got '%s'", call.ID)
	}
	if call.Type != "function" {
		t.Errorf("Expected type 'function', got '%s'", call.Type)
	}
	if call.Function.Name != "execute_bash" {
		t.Errorf("Expected function name 'execute_bash', got '%s'", call.Function.Name)
	}
	if call.Function.Arguments != `{"command": "ls -la"}` {
		t.Errorf("Expected arguments '{\"command\": \"ls -la\"}', got '%s'", call.Function.Arguments)
	}
}

func TestAgentToolCallDetection(t *testing.T) {
	agent := NewAgent(nil)

	// Test the improved detectCompleteToolCalls function
	chunk := `{"id": "call_1", "type": "function", "function": {"name": "execute_bash", "arguments": "{\"command\": \"ls\"}"}}`

	toolCalls, complete := agent.detectCompleteToolCalls(chunk)
	if !complete {
		t.Error("Expected complete tool call detection")
	}
	if len(toolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Function.Name != "execute_bash" {
		t.Errorf("Expected function name 'execute_bash', got '%s'", toolCalls[0].Function.Name)
	}
}

func TestFallbackMechanisms(t *testing.T) {
	agent := NewAgent(nil)

	tests := []struct {
		name     string
		content  string
		expected int // Number of tool calls expected
	}{
		{
			name:     "OpenAI tool_calls format",
			content:  `{"tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "execute_bash", "arguments": "{\"command\": \"ls\"}"}}]}`,
			expected: 1,
		},
		{
			name:     "Individual tool call",
			content:  `{"id": "call_2", "type": "function", "function": {"name": "execute_python", "arguments": "{\"code\": \"print('hello')\"}"}}`,
			expected: 1,
		},
		{
			name:     "Multiple tool calls",
			content:  `{"id": "call_3", "type": "function", "function": {"name": "execute_bash", "arguments": "{\"command\": \"pwd\"}"}}{"id": "call_4", "type": "function", "function": {"name": "execute_javascript", "arguments": "{\"code\": \"console.log('test')\"}"}}`,
			expected: 2,
		},
		{
			name:     "No tool calls",
			content:  `This is just regular text without any tool calls.`,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolCalls := agent.extractToolCallsFromContent(tt.content)
			if len(toolCalls) != tt.expected {
				t.Errorf("Expected %d tool calls, got %d", tt.expected, len(toolCalls))
			}
		})
	}
}
