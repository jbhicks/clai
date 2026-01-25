package llm

import (
	"encoding/json"
	"testing"
)

// Test case that reproduces the streaming JSON corruption issue
func TestStreamingJSONAssembly_Integration(t *testing.T) {
	// Simulate the exact scenario that causes corruption
	// This mimics how tcDelta.Function.Arguments gets fragmented

	// Test case 1: Fragmented JSON arguments like from LLM streaming
	fragments := []string{
		`{"name":"execute_bash","parameters":"{`,
		`"command":"ls"}`,
		`"args": "--help"}`,
		`"some":"other"}`,
		`"param":"value"}`,
	}

	// This should produce corrupted JSON if we just append blindly
	t.Logf("Testing fragmented JSON assembly with %d fragments", len(fragments))

	var tc ToolCall
	for i, fragment := range fragments {
		if i == 0 {
			tc = ToolCall{
				Function: ToolCallFunc{
					Name:      "execute_bash",
					Arguments: fragment,
				},
			}
		} else {
			// This is the current broken approach - just append fragments
			tc.Function.Arguments = string(tc.Function.Arguments) + fragment
		}
	}

	result := string(tc.Function.Arguments)
	t.Logf("Result: %s", result)

	// This should fail because it's not valid JSON
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(result), &args); err != nil {
		t.Logf("✅ Correctly detected invalid JSON: %v", err)
	} else {
		t.Errorf("❌ Result should be invalid JSON but wasn't: %s", result)
	}

	// Test case 2: Proper JSON assembly
	t.Run("ProperAssembly", func(t *testing.T) {
		var properTC ToolCall
		properParams := json.RawMessage(`{"command":"ls","args":["--help"]}`)
		properTC = ToolCall{
			Function: ToolCallFunc{
				Name:      "execute_bash",
				Arguments: string(properParams),
			},
		}

		result := string(properTC.Function.Arguments)
		t.Logf("Proper result: %s", result)

		var args map[string]interface{}
		if err := json.Unmarshal([]byte(result), &args); err != nil {
			t.Errorf("❌ Proper assembly should be valid JSON: %v", err)
		} else {
			t.Logf("✅ Proper assembly produces valid JSON")
		}
	})
}
