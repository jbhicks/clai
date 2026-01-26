package llm

import (
	"testing"
)

// Test the splitJSONObjects function directly
func TestDebugSplitObjects(t *testing.T) {
	agent := NewAgent(nil)

	testCases := []struct {
		name            string
		content         string
		expectedObjects int
	}{
		{
			name:            "Complete single tool call",
			content:         `{"id": "call_1", "type": "function", "function": {"name": "execute_python", "arguments": "{\"code\": \"print('hello')\"}"}}`,
			expectedObjects: 1,
		},
		{
			name:            "Complete tool_calls array",
			content:         `{"tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "execute_bash", "arguments": "{\"command\": \"ls\"}"}}]}`,
			expectedObjects: 1,
		},
		{
			name:            "Multiple complete tool calls",
			content:         `{"id": "call_3", "type": "function", "function": {"name": "execute_bash", "arguments": "{\"command\": \"pwd\"}"}}{"id": "call_4", "type": "function", "function": {"name": "execute_javascript", "arguments": "{\"code\": \"console.log('test')\"}"}}`,
			expectedObjects: 2,
		},
		{
			name:            "Text with embedded tool call",
			content:         `Some text content {"id": "call_2", "type": "function", "function": {"name": "execute_python", "arguments": "{\"code\": \"print('test')\"}"}} more text`,
			expectedObjects: 1,
		},
		{
			name:            "Incomplete JSON",
			content:         `{"id": "call_1", "type": "function", "function": {"name": "execute_python", "arguments": "{\"code\": "}}`,
			expectedObjects: 0, // Should not find incomplete objects
		},
		{
			name:            "Mixed complete and incomplete",
			content:         `{"id": "call_3", "type": "function", "function": {"name": "execute_bash", "arguments": "{\"command": "}}{"id": "call_4", "type": "function", "function": {"name": "execute_javascript", "arguments": "{\"code\": \"console.log('test')\"}"}}`,
			expectedObjects: 1, // Only the second complete object
		},
		{
			name:            "No JSON objects",
			content:         `This is just regular text without any JSON objects.`,
			expectedObjects: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("=== Testing: %s ===", tc.name)
			t.Logf("Content: %s", tc.content)

			// Call the internal function directly through a test helper
			objects := agent.extractToolCallsFromContent(tc.content)
			t.Logf("Found %d tool calls (expected %d)", len(objects), tc.expectedObjects)

			if len(objects) != tc.expectedObjects {
				t.Errorf("Expected %d tool calls, got %d for content: %s", tc.expectedObjects, len(objects), tc.content)
			}
			for i, obj := range objects {
				t.Logf("  Call %d: %s (%s)", i, obj.Function.Name, obj.ID)
			}
			t.Log("")
		})
	}
}
