package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runModelsCommand(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: clai models COMMAND [ARGS...]")
		fmt.Fprintln(os.Stderr, "\nAvailable commands:")
		fmt.Fprintln(os.Stderr, "  list                  - List downloaded models")
		fmt.Fprintln(os.Stderr, "  download <name>       - Download a specific model")
		fmt.Fprintln(os.Stderr, "  download --recommended - Download top 3 models (Hermes3, Llama3.1, Mistral)")
		fmt.Fprintln(os.Stderr, "  download --small      - Download all models < 10GB")
		fmt.Fprintln(os.Stderr, "  download --all        - Download all 6 models (~134GB)")
		fmt.Fprintln(os.Stderr, "  test <model.gguf>     - Test a specific model")
		fmt.Fprintln(os.Stderr, "  test --all            - Test all downloaded models")
		fmt.Fprintln(os.Stderr, "  available             - Show available models for download")
		os.Exit(1)
	}

	command := args[0]

	switch command {
	case "list":
		runModelsList()
	case "download":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: clai models download <name|--recommended|--small|--all>")
			os.Exit(1)
		}
		runModelsDownload(args[1])
	case "test":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: clai models test <model.gguf|--all>")
			os.Exit(1)
		}
		runModelsTest(args[1])
	case "available":
		runModelsAvailable()
	default:
		fmt.Fprintf(os.Stderr, "Unknown models command: %s\n", command)
		os.Exit(1)
	}
}

func runModelsList() {
	modelsDir := os.Getenv("MODELS_DIR")
	if modelsDir == "" {
		modelsDir = filepath.Join(os.Getenv("HOME"), "models")
	}

	fmt.Println("Downloaded models:")
	fmt.Println()

	files, err := filepath.Glob(filepath.Join(modelsDir, "*.gguf"))
	if err != nil || len(files) == 0 {
		fmt.Println("  No models found in", modelsDir)
		fmt.Println()
		fmt.Println("Download models with: clai models download --recommended")
		return
	}

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		size := info.Size()
		sizeStr := formatSize(size)
		name := filepath.Base(file)

		fmt.Printf("  • %s (%s)\n", name, sizeStr)
	}

	fmt.Println()
	fmt.Printf("Total: %d models\n", len(files))
}

func runModelsDownload(target string) {
	scriptPath := "./auto_download_models.sh"

	// Check if script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Error: auto_download_models.sh not found")
		fmt.Fprintln(os.Stderr, "Run this command from the clai project directory")
		os.Exit(1)
	}

	cmd := exec.Command(scriptPath, target)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		os.Exit(1)
	}
}

func runModelsTest(target string) {
	scriptPath := "./test_models.sh"

	// Check if script exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Error: test_models.sh not found")
		fmt.Fprintln(os.Stderr, "Run this command from the clai project directory")
		os.Exit(1)
	}

	cmd := exec.Command(scriptPath, target)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Test failed: %v\n", err)
		os.Exit(1)
	}
}

func runModelsAvailable() {
	models := []struct {
		Name string
		Size string
		Desc string
	}{
		{"hermes3", "4.9GB", "Best for tool calling (Nous Research)"},
		{"llama31-8b", "4.9GB", "Meta official tool calling support"},
		{"mistral-nemo", "7.1GB", "128k context, agent-focused"},
		{"qwen25-14b", "8.5GB", "General purpose with tools"},
		{"gpt-oss-120b", "68GB", "OpenAI reasoning (fast on Strix Halo!)"},
		{"llama31-70b", "40GB", "Most capable general model"},
	}

	fmt.Println("Available models for download:")
	fmt.Println()

	for _, m := range models {
		fmt.Printf("  %-15s %7s  %s\n", m.Name, m.Size, m.Desc)
	}

	fmt.Println()
	fmt.Println("Download examples:")
	fmt.Println("  clai models download hermes3           # Download Hermes 3")
	fmt.Println("  clai models download --recommended     # Download top 3")
	fmt.Println("  clai models download --all             # Download all 6")
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
