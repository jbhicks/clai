package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectAPIFormat_Ollama(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := Response{
			Message: Message{
				Role:    "assistant",
				Content: "test response",
			},
			Done: true,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", "")
	
	if client.apiFormat != FormatOllama {
		t.Errorf("Expected FormatOllama, got %v", client.apiFormat)
	}
}

func TestDetectAPIFormat_OpenAI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := OpenAIStreamChunk{
			ID:      "test",
			Object:  "chat.completion",
			Created: 1234567890,
			Model:   "test-model",
			Choices: []OpenAIChoice{
				{
					Delta: OpenAIDelta{
						Content: "test response",
						Role:    "assistant",
					},
					Index: 0,
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", "")
	
	if client.apiFormat != FormatOpenAI {
		t.Errorf("Expected FormatOpenAI, got %v", client.apiFormat)
	}
}
