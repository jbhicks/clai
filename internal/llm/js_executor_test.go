package llm

import (
	"strings"
	"testing"
)

func TestJSExecutorBasicMath(t *testing.T) {
	executor := NewJSExecutor()

	code := `5 + 3`
	output, err := executor.Execute(code)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if output != "8" {
		t.Errorf("Expected '8', got: %s", output)
	}
}

func TestJSExecutorWithLog(t *testing.T) {
	executor := NewJSExecutor()

	code := `
		log("Hello");
		log("World");
		42
	`
	output, err := executor.Execute(code)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(output, "Hello") {
		t.Errorf("Expected output to contain 'Hello', got: %s", output)
	}
	if !strings.Contains(output, "World") {
		t.Errorf("Expected output to contain 'World', got: %s", output)
	}
	if !strings.Contains(output, "42") {
		t.Errorf("Expected output to contain '42', got: %s", output)
	}
}

func TestJSExecutorConsoleLog(t *testing.T) {
	executor := NewJSExecutor()

	code := `
		console.log("Testing console");
		100
	`
	output, err := executor.Execute(code)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(output, "Testing console") {
		t.Errorf("Expected output to contain 'Testing console', got: %s", output)
	}
}

func TestJSExecutorComplexCalculation(t *testing.T) {
	executor := NewJSExecutor()

	code := `
		var a = 15;
		var b = 23;
		var result = a * b + 100;
		log("Calculation: " + a + " * " + b + " + 100");
		result
	`
	output, err := executor.Execute(code)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(output, "445") {
		t.Errorf("Expected result to contain '445' (15*23+100), got: %s", output)
	}
}

func TestJSExecutorError(t *testing.T) {
	executor := NewJSExecutor()

	code := `
		undefined.foo()
	`
	_, err := executor.Execute(code)

	if err == nil {
		t.Errorf("Expected error for invalid JavaScript, got nil")
	}
}

func TestJSExecutorJSON(t *testing.T) {
	executor := NewJSExecutor()

	code := `
		var data = {"name": "test", "value": 42};
		JSON.stringify(data);
	`
	output, err := executor.Execute(code)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !strings.Contains(output, `"name":"test"`) && !strings.Contains(output, `"name": "test"`) {
		t.Errorf("Expected JSON output, got: %s", output)
	}
}
