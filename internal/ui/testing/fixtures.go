package testing

import (
	"clai/internal/llm"
	"regexp"
)

func NewTestModel() *MockModel {
	return &MockModel{
		Width:    80,
		Height:   24,
		Messages: []llm.Message{},
	}
}

type MockModel struct {
	Width    int
	Height   int
	Messages []llm.Message
}

func SampleMessages() []llm.Message {
	return []llm.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there! How can I help you?"},
		{Role: "user", Content: "What is 2+2?"},
		{Role: "assistant", Content: "2+2 equals 4."},
	}
}

// MockLLM implements llm.LLMClientInterface for testing
type MockLLM struct {
	LastMessages []llm.Message
	NextResponse string
}

func NewMockLLM() *MockLLM {
	return &MockLLM{
		NextResponse: "Mock response",
	}
}

func (m *MockLLM) SendMessageStreamNoTools(messages []llm.Message, streamChan chan<- string, includeSystemPrompt bool) (llm.Response, error) {
	m.LastMessages = messages
	go func() {
		streamChan <- m.NextResponse
		close(streamChan)
	}()
	return llm.Response{Message: llm.Message{Role: "assistant", Content: m.NextResponse}, Done: true}, nil
}

func (m *MockLLM) SendMessageStreamWithTools(messages []llm.Message, tools []llm.Tool, streamChan chan<- string, includeSystemPrompt bool) (llm.Response, error) {
	return m.SendMessageStreamNoTools(messages, streamChan, includeSystemPrompt)
}

func (m *MockLLM) Model() string           { return "mock-model" }
func (m *MockLLM) Host() string            { return "http://localhost:8080" }
func (m *MockLLM) APIFormatString() string { return "Mock" }

// StripANSI removes ANSI escape codes from a string
func StripANSI(s string) string {
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansiRegex.ReplaceAllString(s, "")
}
