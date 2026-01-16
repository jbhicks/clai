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
		fmt.Fprintln(os.Stderr, "  inspect_styles          - Get UI layout dimensions")
		fmt.Fprintln(os.Stderr, "  get_theme_colors        - Get theme colors including textarea background")
		fmt.Fprintln(os.Stderr, "  switch_pane             - Switch between chat and log panes")
		fmt.Fprintln(os.Stderr, "  get_history             - Get conversation history")
		fmt.Fprintln(os.Stderr, "  send_message ROLE TEXT  - Send test message (role: user|assistant)")
		fmt.Fprintln(os.Stderr, "  send_key KEY            - Send keystroke (e.g., ctrl+h, enter, up)")
		fmt.Fprintln(os.Stderr, "  type_text TEXT          - Type text into input field")
		os.Exit(1)
	}

	command := args[0]

	switch command {
	case "ping":
		runPing()
	case "inspect":
		runInspect()
	case "inspect_styles":
		runInspectStyles()
	case "get_theme_colors":
		runGetThemeColors()
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
	case "send_key":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: clai debug send_key KEY")
			os.Exit(1)
		}
		runSendKey(args[1])
	case "type_text":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: clai debug type_text TEXT")
			os.Exit(1)
		}
		runTypeText(strings.Join(args[1:], " "))
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

func runInspectStyles() {
	resp, err := debug.SendCommand("inspect_styles", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Command failed: %s\n", resp.Error)
		os.Exit(1)
	}

	data := resp.Data
	fmt.Printf("Terminal: %vx%v\n", data["width"], data["height"])
	fmt.Printf("Chat Pane: %vx%v\n", data["chat_width"], data["chat_height"])
	fmt.Printf("Viewport: %v lines (offset %v)\n", data["viewport_height"], data["viewport_offset"])
	fmt.Printf("Messages: %v\n", data["message_count"])
	fmt.Printf("Active Pane: %v\n", data["active_pane"])
}

func runGetThemeColors() {
	resp, err := debug.SendCommand("get_theme_colors", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Command failed: %s\n", resp.Error)
		os.Exit(1)
	}

	data := resp.Data
	fmt.Println("=== Theme Colors ===")
	fmt.Printf("Textarea Background: %v\n", data["textarea_background"])
	fmt.Printf("Textarea Foreground: %v\n", data["textarea_foreground"])
	fmt.Printf("Primary Background: %v\n", data["primary_background"])
	fmt.Printf("Primary Foreground: %v\n", data["primary_foreground"])
	fmt.Println("=== All Colors ===")
	for k, v := range data {
		if k != "textarea_background" && k != "textarea_foreground" && k != "primary_background" && k != "primary_foreground" {
			fmt.Printf("%s: %v\n", k, v)
		}
	}
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
	fmt.Printf("Show Help: %v\n", data["show_help"])
	fmt.Printf("Help Show All: %v\n", data["help_show_all"])
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("VIEWPORT CONTENT (what user sees):")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println(data["viewport_content"])
	fmt.Println(strings.Repeat("=", 80))
	if data["show_help"] == true {
		fmt.Println("HELP CONTENT:")
		fmt.Println(strings.Repeat("=", 80))
		fmt.Println(data["help_content"])
		fmt.Println(strings.Repeat("=", 80))
	}
	fmt.Println("FULL CHAT VIEW (with borders and background):")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println(data["full_chat_view"])
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("FULL VIEW (complete UI):")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println(data["full_view"])
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

func runSendKey(key string) {
	args := map[string]interface{}{
		"key": key,
	}

	resp, err := debug.SendCommand("send_key", args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Command failed: %s\n", resp.Error)
		os.Exit(1)
	}

	fmt.Println("Keystroke sent")
}

func runTypeText(text string) {
	args := map[string]interface{}{
		"text": text,
	}

	resp, err := debug.SendCommand("type_text", args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !resp.Success {
		fmt.Fprintf(os.Stderr, "Command failed: %s\n", resp.Error)
		os.Exit(1)
	}

	fmt.Printf("Text typed: %q (%d chars)\n", text, resp.Data["chars_sent"])
}
