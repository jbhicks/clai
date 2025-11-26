package testing

import (
	"encoding/json"
	"fmt"
)

type MockExecutor struct {
	Results    map[string]string
	Error      error
	CallCount  int
	LastTool   string
	LastParams json.RawMessage
}

func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		Results: make(map[string]string),
	}
}

func (m *MockExecutor) ExecuteTool(name string, params json.RawMessage) (string, error) {
	m.CallCount++
	m.LastTool = name
	m.LastParams = params

	if m.Error != nil {
		return "", m.Error
	}

	if result, ok := m.Results[name]; ok {
		return result, nil
	}

	return fmt.Sprintf("mock result for %s", name), nil
}
