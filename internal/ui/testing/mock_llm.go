package testing

import (
	"clai/internal/llm"
)

type MockLLM struct {
	Response     string
	Error        error
	CallCount    int
	LastMessages []llm.Message
	StreamChunks []string
}

func NewMockLLM() *MockLLM {
	return &MockLLM{
		Response:     "Mock response",
		StreamChunks: []string{"Mock ", "response"},
	}
}

func (m *MockLLM) SendMessageStreamNoTools(messages []llm.Message, streamChan chan<- string, includeSystemPrompt bool) (llm.Response, error) {
	m.CallCount++
	m.LastMessages = messages

	go func() {
		defer close(streamChan)

		if m.Error != nil {
			return
		}

		if len(m.StreamChunks) > 0 {
			for _, chunk := range m.StreamChunks {
				streamChan <- chunk
			}
		} else {
			streamChan <- m.Response
		}
	}()

	if m.Error != nil {
		return llm.Response{}, m.Error
	}

	return llm.Response{
		Message: llm.Message{
			Role:    "assistant",
			Content: m.Response,
		},
	}, nil
}

func (m *MockLLM) Model() string {
	return "mock-model"
}

func (m *MockLLM) Host() string {
	return "http://localhost:11434"
}

func (m *MockLLM) APIFormatString() string {
	return "Mock"
}
