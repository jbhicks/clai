package main

import (
	"fmt"
	"os"
)

func showMainHelp() {
	fmt.Println("🚀 CLAI - Command Line AI Assistant")
	fmt.Println("")
	fmt.Println("USAGE:")
	fmt.Println("  clai [COMMAND] [OPTIONS] [ARGS]")
	fmt.Println("")
	fmt.Println("COMMANDS:")
	fmt.Println("  query       Send a single query to the LLM")
	fmt.Println("  orchestrate  Run Ralph autonomous development loop")
	fmt.Println("  task        Manage and execute individual tasks")
	fmt.Println("  models      List available LLM models")
	fmt.Println("  benchmark   Run benchmark server")
	fmt.Println("  debug       Debug CLAI functionality")
	fmt.Println("")
	fmt.Println("TASK SUBCOMMANDS:")
	fmt.Println("  execute     Execute a single task by ID")
	fmt.Println("  decompose   Decompose a task into implementation steps")
	fmt.Println("")
	fmt.Println("EXAMPLES:")
	fmt.Println("  clai query \"What is the capital of France?\"")
	fmt.Println("  clai query --stream --format json \"Explain Go interfaces\"")
	fmt.Println("  clai orchestrate --single --model opencode/claude-opus-4-5")
	fmt.Println("  clai orchestrate --max-iterations 10 --watch")
	fmt.Println("  clai task execute TASK-123")
	fmt.Println("  clai task decompose \"Add user authentication\"")
	fmt.Println("  clai models")
	fmt.Println("  clai benchmark")
	fmt.Println("")
	fmt.Println("ENVIRONMENT VARIABLES:")
	fmt.Println("  OLLAMA_HOST      LLM server URL (default: http://localhost:8081)")
	fmt.Println("  OLLAMA_MODEL     Model to use (default: llama3.1-gpu:latest)")
	fmt.Println("  SYSTEM_PROMPT     Override system prompt")
	fmt.Println("")
	fmt.Println("For command-specific help, use:")
	fmt.Println("  clai [COMMAND] --help")
	fmt.Println("")
	fmt.Println("📖 Documentation: https://github.com/your-repo/clai")
}

func isHelpCommand(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func handleNoCommand() {
	if len(os.Args) == 1 {
		showMainHelp()
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "Error: Unknown command '%s'\n\n", os.Args[1])
	showMainHelp()
	os.Exit(1)
}

func commandExists(command string) bool {
	commands := []string{"query", "orchestrate", "task", "models", "benchmark", "debug"}
	for _, cmd := range commands {
		if cmd == command {
			return true
		}
	}
	return false
}
