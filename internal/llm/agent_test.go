package llm

import (
	"strings"
	"testing"
)

func TestAgentParseThought(t *testing.T) {
	agent := NewAgent(nil)

	response := "Thought: I need to calculate 5 + 3 using JavaScript.\n\nCode:\n```javascript\n5 + 3\n```\n\nFinal Answer: The result is 8."

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

	if parsed.Final != "The result is 8." {
		t.Errorf("Expected final answer, got: %s", parsed.Final)
	}
}

func TestAgentParseDelegation(t *testing.T) {
	agent := NewAgent(nil)

	response := "Thought: I need to delegate this task to multiple sub-agents.\n\nDelegation: [{\"subtask\": \"Calculate 5 + 3\", \"role\": \"math\"}, {\"subtask\": \"Calculate 10 * 2\", \"role\": \"math\"}]"

	parsed, err := agent.parseResponse(response)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(parsed.Delegation) != 2 {
		t.Errorf("Expected 2 subtasks, got: %d", len(parsed.Delegation))
	}

	if len(parsed.Delegation) > 0 && parsed.Delegation[0].Description != "Calculate 5 + 3" {
		t.Errorf("Expected first subtask description, got: %s", parsed.Delegation[0].Description)
	}
}

func TestAgentParseCodeBlock(t *testing.T) {
	agent := NewAgent(nil)

	response := "Thought: Need to execute JavaScript\n\nCode:\n```javascript\nvar x = 10;\nvar y = 20;\nx + y\n```\n\nFinal Answer: Result is 30"

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

	response := "Thought: The answer is obvious.\n\nFinal Answer: 42"

	parsed, err := agent.parseResponse(response)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Final != "42" {
		t.Errorf("Expected final answer '42', got: %s", parsed.Final)
	}
}
