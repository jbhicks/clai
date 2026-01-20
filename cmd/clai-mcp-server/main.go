package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"

	"clai/internal/mcp"
)

var socketPath = flag.String("socket", "/tmp/clai.sock", "Path to CLAI debug socket")

func main() {
	flag.Parse()

	// Check if CLAI debug socket exists
	if _, err := os.Stat(*socketPath); os.IsNotExist(err) {
		log.Fatalf("CLAI debug server socket not found: %s\nMake sure CLAI is running with debug server enabled.", *socketPath)
	}

	// Create MCP server
	server := mcp.NewServer(*socketPath)

	// Start processing JSON-RPC requests from stdin
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		request := scanner.Text()
		response := server.HandleRequest(request)
		if response != "" {
			fmt.Println(response)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading stdin: %v", err)
	}
}
