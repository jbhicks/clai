package main

import (
	"clai/internal/db"
	"clai/internal/llm"
	"clai/internal/logger"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

func runQueryCommand(args []string) {
	// Check for help flag before parsing
	if isHelpCommand(args) {
		fmt.Println("clai query - Send a single query to the LLM")
		fmt.Println("")
		fmt.Println("Usage: clai query [OPTIONS] \"your question here\"")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --model MODEL          Override model selection")
		fmt.Println("  --format FORMAT       Output format (text, json)")
		fmt.Println("  --system-prompt PROMPT Override system prompt")
		fmt.Println("  --stream              Enable streaming output")
		fmt.Println("  --no-history         Don't save to conversation history")
		fmt.Println("  --verbose, -v         Show timing and debug info")
		os.Exit(0)
	}

	// Parse CLI flags
	var (
		model        = flag.String("model", "", "Override the default model (defaults to OLLAMA_MODEL env var)")
		format       = flag.String("format", "text", "Output format (text, json)")
		systemPrompt = flag.String("system-prompt", "", "Override the system prompt for this query")
		stream       = flag.Bool("stream", false, "Enable streaming output (real-time response)")
		noHistory    = flag.Bool("no-history", false, "Don't save this query to conversation history")
		verbose      = flag.Bool("verbose", false, "Show debug info (timing, token counts, etc.)")
	)

	flag.CommandLine.Parse(args)

	// Get the query from remaining arguments
	query := strings.Join(flag.Args(), " ")
	if query == "" {
		fmt.Fprintln(os.Stderr, "Error: No query provided")
		fmt.Fprintln(os.Stderr, "Usage: clai query [OPTIONS] \"your question here\"")
		flag.CommandLine.PrintDefaults()
		os.Exit(1)
	}

	// Load environment variables
	_ = godotenv.Load()

	// Set up configuration from environment and flags
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:8081"
	}

	llmModel := os.Getenv("OLLAMA_MODEL")
	if llmModel == "" {
		llmModel = "llama3.1-gpu:latest"
	}
	if *model != "" {
		llmModel = *model
	}

	llmSystemPrompt := os.Getenv("SYSTEM_PROMPT")
	if *systemPrompt != "" {
		llmSystemPrompt = *systemPrompt
	}

	// Initialize logger
	logFile, err := os.Create("debug.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open debug.log: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	logger.Init(logFile)

	if *verbose {
		logger.SetLevel(logger.LevelDebug)
	}

	// Initialize LLM client
	llmClient := llm.NewClient(host, llmModel, llmSystemPrompt)

	// Initialize database if needed
	var store *db.Store
	if !*noHistory {
		store, err = db.New()
		if err != nil {
			logger.Error("Failed to initialize database: %v", err)
			if *verbose {
				fmt.Fprintf(os.Stderr, "Warning: Failed to initialize database, query will not be saved: %v\n", err)
			}
		} else {
			defer store.Close()
		}
	}

	// Prepare the query message
	messages := []llm.Message{
		{Role: "user", Content: query},
	}

	startTime := time.Now()

	// Execute the query
	if *stream {
		runStreamingQuery(llmClient, messages, *format, *verbose)
	} else {
		runNonStreamingQuery(llmClient, messages, *format, *verbose)
	}

	duration := time.Since(startTime)

	if *verbose {
		fmt.Fprintf(os.Stderr, "\nQuery completed in %.2fs\n", duration.Seconds())
	}

	// Save to database if enabled
	if store != nil && !*noHistory {
		// We would need to save the conversation, but for simplicity in this CLI version,
		// we'll skip the database saving for now to keep it simple
		if *verbose {
			fmt.Fprintf(os.Stderr, "Note: Database saving not implemented in CLI mode\n")
		}
	}
}

func runStreamingQuery(client llm.LLMClientInterface, messages []llm.Message, format string, verbose bool) {
	streamChan := make(chan string, 100)

	_, err := client.SendMessageStreamNoTools(messages, streamChan, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting query: %v\n", err)
		os.Exit(1)
	}

	var fullResponse strings.Builder

	for chunk := range streamChan {
		if format == "json" {
			// For JSON format, collect all chunks first
			fullResponse.WriteString(chunk)
		} else {
			// For text format, print chunks as they arrive
			fmt.Print(chunk)
		}
	}

	if format == "json" {
		// Output JSON format
		response := map[string]interface{}{
			"query":     messages[0].Content,
			"response":  fullResponse.String(),
			"model":     client.Model(),
			"timestamp": time.Now().Format(time.RFC3339),
		}

		jsonBytes, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting JSON response: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Println() // Add newline after streaming output
	}
}

func runNonStreamingQuery(client llm.LLMClientInterface, messages []llm.Message, format string, verbose bool) {
	// For non-streaming, we need to make a direct HTTP request
	// Since the LLM client only supports streaming, we'll use a simplified approach
	streamChan := make(chan string, 1000) // Large buffer for non-streaming

	_, err := client.SendMessageStreamNoTools(messages, streamChan, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting query: %v\n", err)
		os.Exit(1)
	}

	var fullResponse strings.Builder
	timeout := time.After(30 * time.Second) // 30 second timeout

	for {
		select {
		case chunk, ok := <-streamChan:
			if !ok {
				// Channel closed, response complete
				if format == "json" {
					response := map[string]interface{}{
						"query":     messages[0].Content,
						"response":  fullResponse.String(),
						"model":     client.Model(),
						"timestamp": time.Now().Format(time.RFC3339),
					}

					jsonBytes, err := json.MarshalIndent(response, "", "  ")
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error formatting JSON response: %v\n", err)
						os.Exit(1)
					}
					fmt.Println(string(jsonBytes))
				} else {
					fmt.Println(fullResponse.String())
				}
				return
			}
			fullResponse.WriteString(chunk)

		case <-timeout:
			fmt.Fprintf(os.Stderr, "Query timed out after 30 seconds\n")
			os.Exit(1)
		}
	}
}
