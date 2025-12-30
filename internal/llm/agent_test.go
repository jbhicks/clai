package llm

import (
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
