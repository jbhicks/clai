package main

import (
	"clai/internal/debug"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func runDebugCommand(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: clai debug COMMAND [ARGS...]")
		fmt.Fprintln(os.Stderr, "\nAvailable commands:")
		fmt.Fprintln(os.Stderr, "  ping                    - Test server connectivity")
		fmt.Fprintln(os.Stderr, "  inspect                 - Show viewport state and content")
		fmt.Fprintln(os.Stderr, "  switch_pane             - Switch between chat and log panes")
		fmt.Fprintln(os.Stderr, "  get_history             - Get conversation history")
		fmt.Fprintln(os.Stderr, "  send_message ROLE TEXT  - Send test message (role: user|assistant)")
		os.Exit(1)
	}

	command := args[0]

	switch command {
	case "ping":
		runPing()
	case "inspect":
		runInspect()
	case "switch_pane":
		runSwitchPane()
	case "get_history":
		runGetHistory()
	case "send_message":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: clai debug send_message ROLE TEXT")
			os.Exit(1)
		}
		runSendMessage(args[1], strings.Join(args[2:], " "))
	default:
		fmt.Fprintf(os.Stderr, "Unknown debug command: %s\n", command)
		os.Exit(1)
	}
}

func runPing() {
	resp, err := debug.SendCommand("ping", nil)
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
	resp, err := debug.SendCommand("inspect", nil)
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
	resp, err := debug.SendCommand("switch_pane", nil)
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
	resp, err := debug.SendCommand("get_history", nil)
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

	resp, err := debug.SendCommand("send_message", args)
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
