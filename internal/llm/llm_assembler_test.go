package llm

import (
	"clai/internal/tools"
	"testing"
)

func TestStreamingToolCallAssembler_ProcessDelta(t *testing.T) {
	assembler := NewStreamingToolCallAssembler()
	defer assembler.Reset()

	tests := []struct {
		name     string
		deltas   []OpenAIDelta
		expected []tools.ToolCall
	}{
		{
			name: "Single complete tool call in one delta",
			deltas: []OpenAIDelta{
				{
					ToolCalls: []tools.ToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: tools.ToolCallFunc{
								Name:      "execute_bash",
								Arguments: `{"command": "ls"}`,
							},
						},
					},
				},
			},
			expected: []tools.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: tools.ToolCallFunc{
						Name:      "execute_bash",
						Arguments: `{"command": "ls"}`,
					},
				},
			},
		},
		{
			name: "Fragmented tool call across multiple deltas",
			deltas: []OpenAIDelta{
				{
					ToolCalls: []tools.ToolCall{
						{
							ID:   "call_456",
							Type: "function",
							Function: tools.ToolCallFunc{
								Name:      "",
								Arguments: "",
							},
						},
					},
				},
				{
					ToolCalls: []tools.ToolCall{
						{
							ID:   "call_456",
							Type: "function",
							Function: tools.ToolCallFunc{
								Name:      "execute_python",
								Arguments: "",
							},
						},
					},
				},
				{
					ToolCalls: []tools.ToolCall{
						{
							ID:   "call_456",
							Type: "function",
							Function: tools.ToolCallFunc{
								Name:      "execute_python",
								Arguments: `{"code": "print('hello')"}`,
							},
						},
					},
				},
			},
			expected: []tools.ToolCall{
				{
					ID:   "call_456",
					Type: "function",
					Function: tools.ToolCallFunc{
						Name:      "execute_python",
						Arguments: `{"code": "print('hello')"}`,
					},
				},
			},
		},
		{
			name: "Multiple tool calls in sequence",
			deltas: []OpenAIDelta{
				{
					ToolCalls: []tools.ToolCall{
						{
							ID:   "call_789",
							Type: "function",
							Function: tools.ToolCallFunc{
								Name:      "execute_bash",
								Arguments: `{"command": "pwd"}`,
							},
						},
					},
				},
				{
					ToolCalls: []tools.ToolCall{
						{
							ID:   "call_790",
							Type: "function",
							Function: tools.ToolCallFunc{
								Name:      "execute_javascript",
								Arguments: `{"code": "console.log('test')"}`,
							},
						},
					},
				},
			},
			expected: []tools.ToolCall{
				{
					ID:   "call_789",
					Type: "function",
					Function: tools.ToolCallFunc{
						Name:      "execute_bash",
						Arguments: `{"command": "pwd"}`,
					},
				},
				{
					ID:   "call_790",
					Type: "function",
					Function: tools.ToolCallFunc{
						Name:      "execute_javascript",
						Arguments: `{"code": "console.log('test')"}`,
					},
				},
			},
		},
		{
			name:     "No tool calls",
			deltas:   []OpenAIDelta{{}},
			expected: []tools.ToolCall{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assembler.Reset()
			var completedCalls []tools.ToolCall

			for _, delta := range tt.deltas {
				calls := assembler.ProcessDelta(delta)
				completedCalls = append(completedCalls, calls...)
			}

			if len(completedCalls) != len(tt.expected) {
				t.Errorf("Expected %d completed calls, got %d", len(tt.expected), len(completedCalls))
				return
			}

			for i, expected := range tt.expected {
				if completedCalls[i].ID != expected.ID {
					t.Errorf("Expected ID %s, got %s", expected.ID, completedCalls[i].ID)
				}
				if completedCalls[i].Function.Name != expected.Function.Name {
					t.Errorf("Expected name %s, got %s", expected.Function.Name, completedCalls[i].Function.Name)
				}
				if completedCalls[i].Function.Arguments != expected.Function.Arguments {
					t.Errorf("Expected arguments %s, got %s", expected.Function.Arguments, completedCalls[i].Function.Arguments)
				}
			}
		})
	}
}

func TestStreamingToolCallAssembler_Validation(t *testing.T) {
	assembler := NewStreamingToolCallAssembler()
	defer assembler.Reset()

	tests := []struct {
		name       string
		toolCall   tools.ToolCall
		shouldPass bool
	}{
		{
			name: "Valid bash tool call",
			toolCall: tools.ToolCall{
				ID:   "call_valid",
				Type: "function",
				Function: tools.ToolCallFunc{
					Name:      "execute_bash",
					Arguments: `{"command": "ls -la"}`,
				},
			},
			shouldPass: true,
		},
		{
			name: "Valid python tool call",
			toolCall: tools.ToolCall{
				ID:   "call_python",
				Type: "function",
				Function: tools.ToolCallFunc{
					Name:      "execute_python",
					Arguments: `{"code": "print('hello')"}`,
				},
			},
			shouldPass: true,
		},
		{
			name: "Missing command parameter",
			toolCall: tools.ToolCall{
				ID:   "call_missing_param",
				Type: "function",
				Function: tools.ToolCallFunc{
					Name:      "execute_bash",
					Arguments: `{"wrong": "parameter"}`,
				},
			},
			shouldPass: false,
		},
		{
			name: "Unknown function name",
			toolCall: tools.ToolCall{
				ID:   "call_unknown",
				Type: "function",
				Function: tools.ToolCallFunc{
					Name:      "unknown_function",
					Arguments: `{"param": "value"}`,
				},
			},
			shouldPass: false,
		},
		{
			name: "Invalid JSON arguments",
			toolCall: tools.ToolCall{
				ID:   "call_invalid_json",
				Type: "function",
				Function: tools.ToolCallFunc{
					Name:      "execute_bash",
					Arguments: `{invalid json}`,
				},
			},
			shouldPass: false,
		},
		{
			name: "Wrong type",
			toolCall: tools.ToolCall{
				ID:   "call_wrong_type",
				Type: "not_function",
				Function: tools.ToolCallFunc{
					Name:      "execute_bash",
					Arguments: `{"command": "ls"}`,
				},
			},
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assembler.validateToolCall(&tt.toolCall)
			if result != tt.shouldPass {
				t.Errorf("Expected validation result %v, got %v", tt.shouldPass, result)
			}
		})
	}
}

func TestStreamingToolCallAssembler_Reset(t *testing.T) {
	assembler := NewStreamingToolCallAssembler()

	// Add some active calls
	delta := OpenAIDelta{
		ToolCalls: []tools.ToolCall{
			{
				ID:   "call_active",
				Type: "function",
				Function: tools.ToolCallFunc{
					Name: "execute_bash",
				},
			},
		},
	}

	assembler.ProcessDelta(delta)

	// Verify we have active calls
	if assembler.GetActiveCount() != 1 {
		t.Errorf("Expected 1 active call, got %d", assembler.GetActiveCount())
	}

	// Reset and verify no active calls
	assembler.Reset()
	if assembler.GetActiveCount() != 0 {
		t.Errorf("Expected 0 active calls after reset, got %d", assembler.GetActiveCount())
	}
}

func TestStreamingToolCallAssembler_GetActiveCount(t *testing.T) {
	assembler := NewStreamingToolCallAssembler()
	defer assembler.Reset()

	// Initially no active calls
	if assembler.GetActiveCount() != 0 {
		t.Errorf("Expected 0 active calls initially, got %d", assembler.GetActiveCount())
	}

	// Add incomplete call
	delta := OpenAIDelta{
		ToolCalls: []tools.ToolCall{
			{
				ID:   "call_incomplete",
				Type: "function",
				Function: tools.ToolCallFunc{
					Name: "execute_bash", // No arguments yet
				},
			},
		},
	}

	assembler.ProcessDelta(delta)

	// Should have 1 active call
	if assembler.GetActiveCount() != 1 {
		t.Errorf("Expected 1 active call, got %d", assembler.GetActiveCount())
	}

	// Complete the call
	deltaComplete := OpenAIDelta{
		ToolCalls: []tools.ToolCall{
			{
				ID:   "call_incomplete",
				Type: "function",
				Function: tools.ToolCallFunc{
					Name:      "execute_bash",
					Arguments: `{"command": "ls"}`,
				},
			},
		},
	}

	assembler.ProcessDelta(deltaComplete)

	// Should have 0 active calls again
	if assembler.GetActiveCount() != 0 {
		t.Errorf("Expected 0 active calls after completion, got %d", assembler.GetActiveCount())
	}
}
