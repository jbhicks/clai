package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

// Test case that reproduces the streaming JSON corruption issue
func TestStreamingJSONAssembly(t *testing.T) {
	// Test case 1: Fragmented JSON arguments like from LLM streaming
	fragments := []string{
		`{"name":"execute_bash","parameters":"{`,
		`"command":"ls"}`,
		`"args":["--help"]}`,
		`"some":"other"}`,
		`"param":"value"}`,
	}

	t.Logf("Testing fragmented JSON assembly with %d fragments", len(fragments))

	// This simulates the current broken approach
	var corruptedResult strings.Builder
	for _, fragment := range fragments {
		corruptedResult.WriteString(fragment)
	}

	t.Logf("Corrupted result: %s", corruptedResult.String())

	// Verify it's invalid JSON
	var invalidJSON interface{}
	if err := json.Unmarshal([]byte(corruptedResult.String()), &invalidJSON); err == nil {
		t.Errorf("❌ Expected invalid JSON but got valid: %v", invalidJSON)
	} else {
		t.Logf("✅ Correctly detected invalid JSON: %v", err)
	}

	// Test case 2: Proper JSON assembly
	properJSON := `{"name":"execute_bash","parameters":{"command":"ls","args":["--help"]}}`
	t.Logf("Proper JSON: %s", properJSON)

	var validJSON interface{}
	if err := json.Unmarshal([]byte(properJSON), &validJSON); err != nil {
		t.Errorf("❌ Expected valid JSON but got error: %v", err)
	} else {
		t.Logf("✅ Correctly parsed valid JSON: %v", validJSON)
	}
}
