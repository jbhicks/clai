package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

const SocketPath = "/tmp/clai.sock"

type Response struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

func SendCommand(command string, args map[string]interface{}) (*Response, error) {
	conn, err := net.Dial("unix", SocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", SocketPath, err)
	}
	defer conn.Close()

	cmd := map[string]interface{}{
		"command": command,
	}
	if args != nil {
		cmd["args"] = args
	}

	cmdJSON, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	cmdJSON = append(cmdJSON, '\n')
	if _, err := conn.Write(cmdJSON); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	buf := make([]byte, 1024*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [args...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nAvailable commands:\n")
		fmt.Fprintf(os.Stderr, "  ping                    - Test server connectivity\n")
		fmt.Fprintf(os.Stderr, "  inspect                 - Show viewport state and content\n")
		fmt.Fprintf(os.Stderr, "  get_history             - Get conversation history\n")
		fmt.Fprintf(os.Stderr, "  switch_pane             - Switch between chat and log panes\n")
		fmt.Fprintf(os.Stderr, "  send_message ROLE TEXT  - Send test message (role: user|assistant)\n")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "ping":
		runPing()
	case "inspect":
		runInspect()
	case "get_history":
		runGetHistory()
	case "switch_pane":
		runSwitchPane()
	case "send_message":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Usage: %s send_message ROLE TEXT\n", os.Args[0])
			os.Exit(1)
		}
		runSendMessage(os.Args[2], os.Args[3])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func runPing() {
	resp, err := SendCommand("ping", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Command failed: %s\n", resp.Error)
		os.Exit(1)
	}

	fmt.Println("pong")
}

func runInspect() {
	resp, err := SendCommand("inspect", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Command failed: %s\n", resp.Error)
		os.Exit(1)
	}

	data := resp.Data
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("DEBUG INSPECT OUTPUT")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Terminal Size: %vx%v\n", data["width"], data["height"])
	fmt.Printf("Chat Pane Size: %vx%v\n", data["chat_width"], data["chat_height"])
	fmt.Printf("Viewport Height: %v\n", data["viewport_height"])
	fmt.Printf("Viewport Offset: %v\n", data["viewport_offset"])
	fmt.Printf("Total Lines: %v\n", data["total_lines"])
	fmt.Printf("Message Count: %v\n", data["message_count"])
	fmt.Printf("Active Pane: %v\n", data["active_pane"])
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("VIEWPORT CONTENT (what user sees):")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println(data["viewport_content"])
	fmt.Println(strings.Repeat("=", 80))
}

func runSwitchPane() {
	resp, err := SendCommand("switch_pane", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Command failed: %s\n", resp.Error)
		os.Exit(1)
	}

	fmt.Println("Switched pane")
}

func runGetHistory() {
	resp, err := SendCommand("get_history", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Command failed: %s\n", resp.Error)
		os.Exit(1)
	}

	output, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to format response: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(output))
}

func runSendMessage(role, content string) {
	args := map[string]interface{}{
		"role":    role,
		"content": content,
	}

	resp, err := SendCommand("send_message", args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Command failed: %s\n", resp.Error)
		os.Exit(1)
	}

	fmt.Printf("Message sent. Total messages: %v\n", resp.Data["message_count"])
}
