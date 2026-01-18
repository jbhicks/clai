# Bubble Tea Testing Strategy

## Overview

This document defines the comprehensive testing strategy for Bubble Tea UI components in this codebase. All agents and developers must follow these guidelines to ensure consistent, maintainable, and reliable tests across sessions.

## Testing Philosophy

- **Fast by default**: Prefer unit tests over integration tests
- **Mock external dependencies**: Never call real LLM APIs or external services in tests
- **Test behavior, not implementation**: Focus on what the UI does, not how it does it
- **Maintain golden files carefully**: Visual regression tests should complement, not replace, behavioral tests

## Testing Layers

### 1. Component Unit Tests (Primary - Fast & Isolated)

**Purpose**: Test individual UI component logic without running a full Bubble Tea program.

**Characteristics**:
- No program loop (`tea.NewProgram`) required
- Direct calls to `Update()` and `View()` methods
- Tests complete in < 100ms
- Easy to debug and maintain
- No timeouts or async complexity

**When to use**:
- Testing state transitions in `Update()`
- Testing message handling logic
- Testing keyboard input processing
- Testing view rendering with different model states
- Testing edge cases and error conditions

**File location**: `internal/ui/model_test.go`, `internal/ui/chat_test.go`

**Template**:
```go
func TestModelStateTransition(t *testing.T) {
	m := NewModel()
	
	// Send a message
	newModel, cmd := m.Update(userInputMsg{content: "test message"})
	
	// Verify state changed
	if newModel.state != stateThinking {
		t.Errorf("expected state %v, got %v", stateThinking, newModel.state)
	}
	
	// Verify command was returned
	if cmd == nil {
		t.Error("expected command to be returned for LLM call")
	}
}

func TestChatViewRendering(t *testing.T) {
	tests := []struct {
		name     string
		model    Model
		contains string
	}{
		{
			name: "empty chat shows placeholder",
			model: Model{
				messages: []Message{},
				width:    80,
				height:   24,
			},
			contains: "No messages",
		},
		{
			name: "shows user message",
			model: Model{
				messages: []Message{
					{role: "user", content: "hello"},
				},
				width:  80,
				height: 24,
			},
			contains: "hello",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := tt.model.View()
			stripped := stripANSI(view)
			
			if !strings.Contains(stripped, tt.contains) {
				t.Errorf("view should contain %q\nGot:\n%s", tt.contains, stripped)
			}
		})
	}
}
```

### 2. Integration Tests (With Mock I/O)

**Purpose**: Test the full Bubble Tea program with mocked input/output and dependencies.

**Characteristics**:
- Runs full program with `tea.NewProgram()`
- Uses `bytes.Buffer` for stdin/stdout
- Mocks LLM client and tool executor
- Uses `WithoutRenderer()` for speed (no ANSI rendering)
- Requires timeout contexts
- Tests complete in < 5 seconds

**When to use**:
- Testing end-to-end conversation flow
- Testing tool execution integration
- Testing error handling across components
- Testing async message passing
- Verifying program lifecycle (init → run → quit)

**File location**: `internal/ui/integration_test.go`

**Template**:
```go
func TestChatIntegration(t *testing.T) {
	var buf bytes.Buffer
	var in bytes.Buffer
	
	// Create model with mock dependencies
	mockLLM := &testing.MockLLM{
		Response: "This is a test response",
	}
	
	m := NewModel()
	m.llmClient = mockLLM
	
	// Create program with mock I/O
	p := tea.NewProgram(m,
		tea.WithInput(&in),
		tea.WithOutput(&buf),
		tea.WithoutRenderer(), // Fast: no ANSI rendering
	)
	
	// Set timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Send messages in goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)
		p.Send(userInputMsg{content: "test message"})
		time.Sleep(500 * time.Millisecond)
		p.Send(tea.QuitMsg{})
	}()
	
	// Run program
	finalModel, err := p.Run()
	if err != nil {
		t.Fatal(err)
	}
	
	// Verify final state
	m = finalModel.(Model)
	if len(m.messages) < 2 {
		t.Error("expected at least 2 messages (user + assistant)")
	}
	
	// Verify output contains expected content
	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("output should contain user message")
	}
}
```

### 3. Visual Regression Tests (Optional - With teatest)

**Purpose**: Capture and compare full rendered output for visual consistency.

**Characteristics**:
- Uses `github.com/charmbracelet/x/exp/teatest` library
- Stores golden files in `testdata/golden/`
- Pixel-perfect comparison of rendered output
- Can simulate interactive input with `tm.Type()` and `tm.Send()`
- Update snapshots with `go test -update`

**When to use**:
- Verifying layout changes don't break existing UI
- Testing complex multi-pane layouts
- Catching unintended visual regressions
- Documenting expected UI appearance

**File location**: `internal/ui/visual_test.go`

**Template**:
```go
func TestChatUIVisual(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual test in short mode")
	}
	
	m := NewModel()
	m.llmClient = &testing.MockLLM{Response: "Mock response"}
	
	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)
	
	// Simulate user interaction
	tm.Type("hello")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	
	// Wait for response
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Mock response"))
	}, teatest.WithDuration(5*time.Second))
	
	// Quit and capture final output
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	
	out := tm.FinalOutput(t)
	
	// Compare with golden file (testdata/TestChatUIVisual.golden)
	teatest.RequireEqualOutput(t, out)
}
```

## Mock Infrastructure

### Mock LLM Client

**Location**: `internal/ui/testing/mock_llm.go`

**Purpose**: Replace real LLM API calls with predictable responses for testing.

**Implementation**:
```go
package testing

import (
	"context"
	"io"
)

// MockLLM provides a test implementation of the LLM client interface
type MockLLM struct {
	// Response to return for all queries
	Response string
	
	// Error to return (if set)
	Error error
	
	// Track calls for verification
	CallCount int
	LastQuery string
	
	// Optional: Simulate streaming
	StreamChunks []string
}

func (m *MockLLM) Query(ctx context.Context, query string) (string, error) {
	m.CallCount++
	m.LastQuery = query
	
	if m.Error != nil {
		return "", m.Error
	}
	
	return m.Response, nil
}

func (m *MockLLM) StreamQuery(ctx context.Context, query string) (<-chan string, <-chan error) {
	m.CallCount++
	m.LastQuery = query
	
	chunks := make(chan string)
	errors := make(chan error, 1)
	
	go func() {
		defer close(chunks)
		defer close(errors)
		
		if m.Error != nil {
			errors <- m.Error
			return
		}
		
		if len(m.StreamChunks) > 0 {
			for _, chunk := range m.StreamChunks {
				select {
				case <-ctx.Done():
					return
				case chunks <- chunk:
				}
			}
		} else {
			chunks <- m.Response
		}
	}()
	
	return chunks, errors
}
```

### Mock Tool Executor

**Location**: `internal/ui/testing/mock_executor.go`

**Purpose**: Replace real tool execution with controlled test responses.

**Implementation**:
```go
package testing

import "github.com/yourusername/clai/internal/tools"

// MockExecutor provides a test implementation of the tool executor
type MockExecutor struct {
	// Results to return for tool calls
	Results map[string]string
	
	// Error to return (if set)
	Error error
	
	// Track calls
	CallCount int
	LastTool  *tools.ToolCall
}

func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		Results: make(map[string]string),
	}
}

func (m *MockExecutor) Execute(tool *tools.ToolCall) (string, error) {
	m.CallCount++
	m.LastTool = tool
	
	if m.Error != nil {
		return "", m.Error
	}
	
	if result, ok := m.Results[tool.Name]; ok {
		return result, nil
	}
	
	return "mock result", nil
}
```

## Test Utilities

### ANSI Stripping Helper

**Location**: `internal/ui/testing/helpers.go`

**Purpose**: Remove ANSI color codes and formatting for reliable string comparison.

**Implementation**:
```go
package testing

import (
	"strings"
	
	"github.com/charmbracelet/x/ansi"
)

// StripANSI removes all ANSI escape codes from a string
func StripANSI(s string) string {
	stripped := ansi.Strip(s)
	return strings.TrimSpace(stripped)
}

// NormalizeWhitespace collapses multiple spaces/newlines for comparison
func NormalizeWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	
	var normalized []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			normalized = append(normalized, line)
		}
	}
	
	return strings.Join(normalized, "\n")
}
```

### Assertion Helpers

**Implementation**:
```go
// AssertContains checks if haystack contains needle
func AssertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected to contain %q\nGot:\n%s", needle, haystack)
	}
}

// AssertNotContains checks if haystack does NOT contain needle
func AssertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected NOT to contain %q\nGot:\n%s", needle, haystack)
	}
}

// AssertMessageCount verifies number of messages in model
func AssertMessageCount(t *testing.T, m Model, expected int) {
	t.Helper()
	if len(m.messages) != expected {
		t.Errorf("expected %d messages, got %d", expected, len(m.messages))
	}
}
```

### Timeout and Wait Helpers

**Implementation**:
```go
import "time"

// WaitForCondition polls until condition returns true or timeout
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration) {
	t.Helper()
	
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		if condition() {
			return
		}
		
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for condition")
		}
		
		<-ticker.C
	}
}
```

## Directory Structure

```
internal/ui/
├── model.go
├── model_test.go           # Unit tests for model Update() logic
├── chat.go
├── chat_test.go            # Unit tests for chat View() rendering
├── integration_test.go     # Full program integration tests
├── visual_test.go          # Golden file visual regression tests (optional)
├── testdata/
│   ├── golden/            # Golden files for visual tests
│   │   ├── TestChatUIVisual.golden
│   │   ├── TestEmptyState.golden
│   │   └── TestToolExecution.golden
│   └── fixtures/          # Test data and fixtures
│       ├── sample_conversation.json
│       └── sample_tool_response.json
└── testing/               # Shared test utilities
    ├── mock_llm.go       # Mock LLM client implementation
    ├── mock_executor.go  # Mock tool executor implementation
    ├── helpers.go        # ANSI stripping, assertions
    └── fixtures.go       # Common test models and data
```

## Running Tests

### All Tests
```bash
make test
# or
go test ./...
```

### Specific Package
```bash
go test ./internal/ui
```

### Single Test
```bash
go test -run TestModelStateTransition ./internal/ui
```

### With Verbose Output
```bash
go test -v ./internal/ui
```

### Short Mode (Skip Visual Tests)
```bash
go test -short ./...
```

### Update Golden Files
```bash
go test ./internal/ui -update
```

### With Coverage
```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Best Practices

### 1. Always Mock External Dependencies

**❌ Bad**:
```go
func TestChat(t *testing.T) {
	m := NewModel()
	// m.llmClient will call real Ollama API - slow, flaky, requires running service
}
```

**✅ Good**:
```go
func TestChat(t *testing.T) {
	m := NewModel()
	m.llmClient = &testing.MockLLM{Response: "test response"}
	// Fast, reliable, no external dependencies
}
```

### 2. Use Table-Driven Tests

**✅ Good**:
```go
func TestMessageFormatting(t *testing.T) {
	tests := []struct {
		name     string
		message  Message
		expected string
	}{
		{
			name:     "user message",
			message:  Message{role: "user", content: "hello"},
			expected: "You: hello",
		},
		{
			name:     "assistant message",
			message:  Message{role: "assistant", content: "hi"},
			expected: "Assistant: hi",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMessage(tt.message)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
```

### 3. Always Use Timeouts for Integration Tests

**❌ Bad**:
```go
func TestProgram(t *testing.T) {
	p := tea.NewProgram(NewModel())
	p.Run() // Can hang forever if bug exists
}
```

**✅ Good**:
```go
func TestProgram(t *testing.T) {
	p := tea.NewProgram(NewModel())
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	done := make(chan error, 1)
	go func() {
		_, err := p.Run()
		done <- err
	}()
	
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("test timed out")
	}
}
```

### 4. Strip ANSI Codes Before String Comparison

**❌ Bad**:
```go
view := m.View()
if !strings.Contains(view, "hello") {
	// Will fail due to ANSI color codes wrapping "hello"
}
```

**✅ Good**:
```go
view := m.View()
stripped := testing.StripANSI(view)
if !strings.Contains(stripped, "hello") {
	t.Error("view should contain 'hello'")
}
```

### 5. Test State, Not Just Output

**✅ Good**:
```go
func TestUserInput(t *testing.T) {
	m := NewModel()
	m, _ = m.Update(userInputMsg{content: "test"})
	
	// Test state
	if len(m.messages) != 1 {
		t.Error("expected 1 message")
	}
	if m.messages[0].role != "user" {
		t.Error("expected user role")
	}
	if m.state != stateThinking {
		t.Error("expected thinking state")
	}
	
	// Test output
	view := m.View()
	if !strings.Contains(testing.StripANSI(view), "test") {
		t.Error("view should show message")
	}
}
```

### 6. Use `t.Helper()` in Helper Functions

**✅ Good**:
```go
func assertMessageCount(t *testing.T, m Model, expected int) {
	t.Helper() // Makes test failures point to caller, not this function
	if len(m.messages) != expected {
		t.Errorf("expected %d messages, got %d", expected, len(m.messages))
	}
}
```

### 7. Skip Slow Tests in Short Mode

**✅ Good**:
```go
func TestVisualRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual test in short mode")
	}
	// ... visual test code
}
```

## Common Pitfalls

### ❌ Pitfall 1: Testing Without Mocks
```go
// DON'T: This calls real Ollama API
func TestChat(t *testing.T) {
	m := NewModel() // Uses real LLM client
	// Test will be slow, flaky, require Ollama running
}
```

### ❌ Pitfall 2: No Timeout on Program Tests
```go
// DON'T: Can hang forever
func TestProgram(t *testing.T) {
	p := tea.NewProgram(NewModel())
	p.Run() // Hangs if quit message never sent
}
```

### ❌ Pitfall 3: Comparing ANSI-Encoded Strings
```go
// DON'T: Will fail due to color codes
if m.View() == "Expected output" {
	// Fails because View() contains ANSI codes
}
```

### ❌ Pitfall 4: Not Using Table-Driven Tests
```go
// DON'T: Repetitive, hard to maintain
func TestA(t *testing.T) { /* test case 1 */ }
func TestB(t *testing.T) { /* test case 2 */ }
func TestC(t *testing.T) { /* test case 3 */ }

// DO: Use table-driven tests for related cases
```

### ❌ Pitfall 5: Testing Implementation Details
```go
// DON'T: Brittle, coupled to implementation
if m.internalBuffer == "expected" {
	// Breaks if implementation changes
}

// DO: Test behavior and public interface
if testing.StripANSI(m.View()) contains "expected" {
	// Tests what user sees, not how it's stored
}
```

## CI/CD Requirements

### Pre-Merge Requirements

All tests must pass before merging:
```bash
make test
```

### Coverage Goals

- **Minimum coverage**: 70% for `internal/ui` package
- **Target coverage**: 85% for `internal/ui` package
- Run coverage report: `go test -cover ./internal/ui`

### GitHub Actions Integration

```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...
      - name: Check coverage
        run: |
          go tool cover -func=coverage.out
          # Fail if coverage < 70%
```

## Quick Reference

| Test Type | File Pattern | Use When | Speed |
|-----------|-------------|----------|-------|
| Unit | `*_test.go` | Testing logic & state | ⚡ < 100ms |
| Integration | `integration_test.go` | Testing full flow | 🐢 < 5s |
| Visual | `visual_test.go` | Testing layout | 🐌 < 10s |

| Mock | Location | Purpose |
|------|----------|---------|
| MockLLM | `testing/mock_llm.go` | Replace API calls |
| MockExecutor | `testing/mock_executor.go` | Replace tool execution |

| Helper | Purpose |
|--------|---------|
| `StripANSI()` | Remove color codes |
| `AssertContains()` | Check string presence |
| `WaitForCondition()` | Poll until true |

---

**Remember**: Fast unit tests are your primary defense. Integration and visual tests catch edge cases.
