package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"unicode"
)

type DebugCommand struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args"`
}

func main() {
	conn, err := net.Dial("unix", "/tmp/clai.sock")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	cmd := DebugCommand{
		Command: "inspect",
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal: %v\n", err)
		os.Exit(1)
	}

	_, err = conn.Write(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write: %v\n", err)
		os.Exit(1)
	}

	buf := make([]byte, 200000)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read: %v\n", err)
		os.Exit(1)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(buf[:n], &response); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to unmarshal: %v\n", err)
		os.Exit(1)
	}

	if success, ok := response["success"].(bool); !ok || !success {
		fmt.Fprintf(os.Stderr, "Command failed: %+v\n", response)
		os.Exit(1)
	}

	dataField := response["data"].(map[string]interface{})

	fmt.Println("\n=== DIMENSIONS ===")
	fmt.Printf("Width: %v, Height: %v\n", dataField["width"], dataField["height"])
	fmt.Printf("Chat Width: %v, Chat Height: %v\n", dataField["chat_width"], dataField["chat_height"])

	if fullView, ok := dataField["full_chat_view"].(string); ok {
		fmt.Println("\n=== FULL CHAT VIEW (last 2000 chars) ===")
		// Print as-is for inspection
		r := []rune(fullView)
		if len(r) > 2000 {
			fullView = string(r[:2000])
		}
		fmt.Println(fullView)
	}

	fmt.Println("\n=== ANALYZING INPUT FIELD ===")
	if fullView, ok := dataField["full_chat_view"].(string); ok {
		lines := 0
		for _, c := range fullView {
			if c == '\n' {
				lines++
			}
		}
		fmt.Printf("Total lines in chat view: %d\n", lines)

		// Look for input field patterns (e.g., ">" prompt, border chars)
		hasPrompt := false
		hasBorder := false
		for _, c := range fullView {
			if c == '>' {
				hasPrompt = true
			}
			if unicode.IsSymbol(c) || unicode.IsPunct(c) {
				if c == '│' || c == '─' || c == '┌' || c == '┐' || c == '└' || c == '┘' {
					hasBorder = true
				}
			}
		}
		fmt.Printf("Has input prompt (>): %v\n", hasPrompt)
		fmt.Printf("Has border characters: %v\n", hasBorder)
	}
}
