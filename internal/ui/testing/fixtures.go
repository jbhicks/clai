package testing

import (
	"clai/internal/llm"
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
