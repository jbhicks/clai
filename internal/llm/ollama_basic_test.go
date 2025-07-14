package llm_test

import (
"bytes"
"encoding/json"
"strings"
"net/http"
"os"
"testing"
"clai/internal/llm"
)
// Basic test to verify local Ollama API responds to a 'Hello, world!' prompt
func TestOllamaHelloWorld(t *testing.T) {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "llama3.1-gpu:latest"
	}
	client := llm.NewClient(host, model, "")

	messages := []llm.Message{{Role: "user", Content: "Hello, world!"}}
	resp, err := client.SendMessage(messages)
	if err != nil {
		t.Fatalf("Ollama API error: %v", err)
	}
	if resp.Message.Content == "" {
		t.Errorf("Expected non-empty response, got: %q", resp.Message.Content)
	}
}

func TestOllamaPrompt(t *testing.T) {
 host := os.Getenv("OLLAMA_HOST")
 if host == "" {
	 host = "http://localhost:11434"
 }
 model := os.Getenv("OLLAMA_MODEL")
 if model == "" {
	 model = "llama3.1-gpu:latest"
 }
 url := host + "/api/generate"
 prompt := map[string]interface{}{
	 "model": model,
	 "prompt": "Hello, Ollama!",
 }
 body, err := json.Marshal(prompt)
 if err != nil {
	 t.Fatalf("Failed to marshal prompt: %v", err)
 }

 resp, err := http.Post(url, "application/json", bytes.NewReader(body))
 if err != nil {
	 t.Fatalf("HTTP request failed: %v", err)
 }
 defer resp.Body.Close()

 if resp.StatusCode != http.StatusOK {
	 t.Fatalf("Unexpected status code: %d", resp.StatusCode)
 }

 // Parse streaming JSON chunks and check for expected content
 foundHello := false
 decoder := json.NewDecoder(resp.Body)
 for decoder.More() {
	 var chunk map[string]interface{}
	 if err := decoder.Decode(&chunk); err != nil {
		 t.Fatalf("Failed to decode chunk: %v", err)
	 }
	 if respText, ok := chunk["response"].(string); ok {
		 if strings.Contains(respText, "Hello") {
			 foundHello = true
		 }
	 }
 }
 if !foundHello {
	 t.Errorf("Response does not contain expected text 'Hello'")
 }
}
