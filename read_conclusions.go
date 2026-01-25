package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// BenchmarkConclusion represents the structure of each JSONL line
type BenchmarkConclusion struct {
	TestID      string    `json:"test_id"`
	TestName    string    `json:"test_name"`
	ModelName   string    `json:"model_name"`
	Category    string    `json:"category"`
	Performance string    `json:"performance"`
	Score       float64   `json:"score"`
	Duration    string    `json:"duration"`
	Timestamp   time.Time `json:"timestamp"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
}

// ReadBenchmarkConclusions reads a JSONL file line by line using bufio.Scanner
func ReadBenchmarkConclusions(filePath string) ([]BenchmarkConclusion, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var conclusions []BenchmarkConclusion
	scanner := bufio.NewScanner(file)

	// Configure scanner buffer for large lines if needed
	const maxCapacity = 512 * 1024 // 512KB max line length
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		var conclusion BenchmarkConclusion
		if err := json.Unmarshal([]byte(line), &conclusion); err != nil {
			log.Printf("Warning: Failed to parse line %d: %v\nLine content: %s", lineNum, err, line)
			continue
		}

		conclusions = append(conclusions, conclusion)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return conclusions, nil
}

// ProcessConclusions processes benchmark conclusions and generates insights
func ProcessConclusions(conclusions []BenchmarkConclusion) {
	fmt.Printf("Processed %d benchmark conclusions\n\n", len(conclusions))

	// Group by category
	categories := make(map[string]int)
	statuses := make(map[string]int)
	totalScore := 0.0

	for _, conclusion := range conclusions {
		categories[conclusion.Category]++
		statuses[conclusion.Status]++
		totalScore += conclusion.Score
	}

	// Print summary statistics
	fmt.Println("=== Summary Statistics ===")
	if len(conclusions) > 0 {
		avgScore := totalScore / float64(len(conclusions))
		fmt.Printf("Average Score: %.2f\n", avgScore)
	}

	fmt.Println("\nBy Category:")
	for category, count := range categories {
		fmt.Printf("  %s: %d\n", category, count)
	}

	fmt.Println("\nBy Status:")
	for status, count := range statuses {
		fmt.Printf("  %s: %d\n", status, count)
	}

	// Show top performing tests
	fmt.Println("\n=== Top 5 Performing Tests ===")
	for i, conclusion := range conclusions {
		if i >= 5 {
			break
		}
		fmt.Printf("%d. %s (%.2f) - %s\n",
			i+1, conclusion.TestName, conclusion.Score, conclusion.Conclusion)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run read_conclusions.go <jsonl_file_path>")
		fmt.Println("Example: go run read_conclusions.go benchmark_results/conclusions.jsonl")
		os.Exit(1)
	}

	filePath := os.Args[1]

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Fatalf("File does not exist: %s", filePath)
	}

	fmt.Printf("Reading benchmark conclusions from: %s\n", filePath)

	start := time.Now()
	conclusions, err := ReadBenchmarkConclusions(filePath)
	if err != nil {
		log.Fatalf("Failed to read conclusions: %v", err)
	}

	duration := time.Since(start)
	fmt.Printf("File read successfully in %v\n\n", duration)

	// Process and display insights
	ProcessConclusions(conclusions)

	// Optionally write results to a new file
	if len(os.Args) > 2 && os.Args[2] == "--export" {
		exportPath := filepath.Join(filepath.Dir(filePath), "conclusions_summary.json")
		if err := exportToJSON(conclusions, exportPath); err != nil {
			log.Printf("Warning: Failed to export summary: %v", err)
		} else {
			fmt.Printf("\nSummary exported to: %s\n", exportPath)
		}
	}
}

func exportToJSON(conclusions []BenchmarkConclusion, filePath string) error {
	data, err := json.MarshalIndent(conclusions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}
