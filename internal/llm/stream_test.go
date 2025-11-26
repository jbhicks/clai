package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSendMessageStream_OpenAI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "chat") {
			reqBody := make(map[string]interface{})
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				t.Errorf("Failed to decode request: %v", err)
				return
			}

			isStream, _ := reqBody["stream"].(bool)

			if !isStream {
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
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("Expected flusher")
				return
			}

			chunks := []string{"Hello", " ", "world", "!"}
			for i, chunk := range chunks {
				response := OpenAIStreamChunk{
					ID:      fmt.Sprintf("chunk-%d", i),
					Object:  "chat.completion.chunk",
					Created: 1234567890,
					Model:   "test-model",
					Choices: []OpenAIChoice{
						{
							Delta: OpenAIDelta{
								Content: chunk,
							},
							Index: 0,
						},
					},
				}
				jsonBytes, _ := json.Marshal(response)
				fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
				flusher.Flush()
			}

			finishChunk := OpenAIStreamChunk{
				ID:      "chunk-final",
				Object:  "chat.completion.chunk",
				Created: 1234567890,
				Model:   "test-model",
				Choices: []OpenAIChoice{
					{
						Delta:        OpenAIDelta{},
						FinishReason: "stop",
						Index:        0,
					},
				},
			}
			jsonBytes, _ := json.Marshal(finishChunk)
			fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-model", "")
	client.apiFormat = FormatOpenAI

	streamChan := make(chan string, 10)
	toolCallChan := make(chan []ToolCall, 1)

	messages := []Message{{Role: "user", Content: "test"}}
	_, err := client.SendMessageStream(messages, streamChan, toolCallChan)
	if err != nil {
		t.Fatalf("SendMessageStream failed: %v", err)
	}

	var received []string
	timeout := time.After(2 * time.Second)

loop:
	for {
		select {
		case chunk, ok := <-streamChan:
			if !ok {
				break loop
			}
			received = append(received, chunk)
		case <-timeout:
			t.Fatal("Timeout waiting for stream chunks")
		}
	}

	expected := []string{"Hello", " ", "world", "!"}
	if len(received) != len(expected) {
		t.Errorf("Expected %d chunks, got %d: %v", len(expected), len(received), received)
	}

	for i, chunk := range expected {
		if i >= len(received) || received[i] != chunk {
			t.Errorf("Chunk %d: expected %q, got %q", i, chunk, received[i])
		}
	}
}
