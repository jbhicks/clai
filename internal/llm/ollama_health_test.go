package llm

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestOllamaHealth(t *testing.T) {
	resp, err := http.Get("http://localhost:11434/")
	if err != nil {
		t.Fatalf("Failed to connect to Ollama API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !strings.Contains(string(body), "Ollama is running") {
		t.Fatalf("Expected response to contain 'Ollama is running', got: %s", string(body))
	}
}
