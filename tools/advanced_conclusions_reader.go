package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// StreamProcessor handles streaming JSONL processing with parallel workers
type StreamProcessor struct {
	BatchSize   int
	Workers     int
	Conclusions []BenchmarkConclusion
	mu          sync.Mutex
	Processed   int
	Errors      []ProcessingError
	errorMu     sync.Mutex
}

// ProcessingError tracks errors during file processing
type ProcessingError struct {
	LineNum int
	Line    string
	Error   string
}

// ProcessJSONLStream processes a JSONL file with streaming and parallel processing
func (sp *StreamProcessor) ProcessJSONLStream(ctx context.Context, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create buffered scanner for efficient line reading
	scanner := bufio.NewScanner(file)

	// Configure scanner for large lines
	const maxCapacity = 1024 * 1024 // 1MB max line length
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	// Channel for lines to be processed
	lineChan := make(chan LineData, sp.BatchSize)

	// Channel for processed conclusions
	resultChan := make(chan ProcessedResult, sp.BatchSize)

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < sp.Workers; i++ {
		wg.Add(1)
		go sp.worker(ctx, &wg, lineChan, resultChan)
	}

	// Start result collector
	go sp.collector(resultChan)

	lineNum := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			close(lineChan)
			wg.Wait()
			return ctx.Err()
		default:
			lineNum++
			line := scanner.Text()
			if line == "" {
				continue
			}
			lineChan <- LineData{LineNum: lineNum, Content: line}
		}
	}

	close(lineChan)
	wg.Wait()
	close(resultChan)

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	return nil
}

// LineData represents a line to be processed
type LineData struct {
	LineNum int
	Content string
}

// ProcessedResult represents processing results
type ProcessedResult struct {
	Conclusion BenchmarkConclusion
	Err        ProcessingError
	Success    bool
}

// worker processes lines in parallel
func (sp *StreamProcessor) worker(ctx context.Context, wg *sync.WaitGroup, lineChan <-chan LineData, resultChan chan<- ProcessedResult) {
	defer wg.Done()

	for lineData := range lineChan {
		select {
		case <-ctx.Done():
			return
		default:
			var conclusion BenchmarkConclusion
			err := json.Unmarshal([]byte(lineData.Content), &conclusion)

			if err != nil {
				resultChan <- ProcessedResult{
					Err: ProcessingError{
						LineNum: lineData.LineNum,
						Line:    lineData.Content,
						Error:   err.Error(),
					},
					Success: false,
				}
			} else {
				resultChan <- ProcessedResult{
					Conclusion: conclusion,
					Success:    true,
				}
			}
		}
	}
}

// collector collects processed results
func (sp *StreamProcessor) collector(resultChan <-chan ProcessedResult) {
	for result := range resultChan {
		if result.Success {
			sp.mu.Lock()
			sp.Conclusions = append(sp.Conclusions, result.Conclusion)
			sp.Processed++
			sp.mu.Unlock()
		} else {
			sp.errorMu.Lock()
			sp.Errors = append(sp.Errors, result.Err)
			sp.errorMu.Unlock()
		}
	}
}

// ProcessLargeJSONL handles very large JSONL files efficiently
func ProcessLargeJSONL(ctx context.Context, filePath string, workers int) (*StreamProcessor, error) {
	sp := &StreamProcessor{
		BatchSize: 1000,
		Workers:   workers,
	}

	// Process the file
	if err := sp.ProcessJSONLStream(ctx, filePath); err != nil {
		return nil, err
	}

	return sp, nil
}

// FilterConclusions applies filters to conclusions
func FilterConclusions(conclusions []BenchmarkConclusion, filters map[string]interface{}) []BenchmarkConclusion {
	var filtered []BenchmarkConclusion

	for _, conclusion := range conclusions {
		match := true

		// Check each filter
		for key, value := range filters {
			switch key {
			case "category":
				if conclusion.Category != value.(string) {
					match = false
				}
			case "status":
				if conclusion.Status != value.(string) {
					match = false
				}
			case "min_score":
				if conclusion.Score < value.(float64) {
					match = false
				}
			case "model_name":
				if conclusion.ModelName != value.(string) {
					match = false
				}
			}
		}

		if match {
			filtered = append(filtered, conclusion)
		}
	}

	return filtered
}

// GenerateReport creates a detailed analysis report
func GenerateReport(conclusions []BenchmarkConclusion, outputDir string) error {
	if len(conclusions) == 0 {
		return fmt.Errorf("no conclusions to analyze")
	}

	// Calculate statistics
	stats := calculateStats(conclusions)

	// Create HTML report
	htmlReport := generateHTMLReport(stats, conclusions)
	htmlPath := filepath.Join(outputDir, "analysis_report.html")
	if err := os.WriteFile(htmlPath, []byte(htmlReport), 0644); err != nil {
		return fmt.Errorf("failed to write HTML report: %w", err)
	}

	// Create JSON report
	jsonReport := generateJSONReport(stats)
	jsonPath := filepath.Join(outputDir, "analysis_summary.json")
	if err := os.WriteFile(jsonPath, jsonReport, 0644); err != nil {
		return fmt.Errorf("failed to write JSON report: %w", err)
	}

	fmt.Printf("Reports generated:\n")
	fmt.Printf("  HTML: %s\n", htmlPath)
	fmt.Printf("  JSON: %s\n", jsonPath)

	return nil
}

type Statistics struct {
	TotalTests        int                   `json:"total_tests"`
	AverageScore      float64               `json:"average_score"`
	CategoryBreakdown map[string]int        `json:"category_breakdown"`
	StatusBreakdown   map[string]int        `json:"status_breakdown"`
	TopPerformers     []BenchmarkConclusion `json:"top_performers"`
	BottomPerformers  []BenchmarkConclusion `json:"bottom_performers"`
}

func calculateStats(conclusions []BenchmarkConclusion) Statistics {
	stats := Statistics{
		CategoryBreakdown: make(map[string]int),
		StatusBreakdown:   make(map[string]int),
	}

	totalScore := 0.0

	// Sort by score (simple selection sort for demonstration)
	sorted := make([]BenchmarkConclusion, len(conclusions))
	copy(sorted, conclusions)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Score > sorted[i].Score {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	for i, conclusion := range sorted {
		totalScore += conclusion.Score
		stats.CategoryBreakdown[conclusion.Category]++
		stats.StatusBreakdown[conclusion.Status]++

		if i < 5 {
			stats.TopPerformers = append(stats.TopPerformers, conclusion)
		}
		if i >= len(sorted)-5 {
			stats.BottomPerformers = append(stats.BottomPerformers, conclusion)
		}
	}

	stats.TotalTests = len(conclusions)
	if len(conclusions) > 0 {
		stats.AverageScore = totalScore / float64(len(conclusions))
	}

	return stats
}

func generateHTMLReport(stats Statistics, conclusions []BenchmarkConclusion) string {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Benchmark Analysis Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .stat-box { background: #f0f0f0; padding: 15px; margin: 10px 0; border-radius: 5px; border-left: 4px solid #007bff; }
        .performance-table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        .performance-table th, .performance-table td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        .performance-table th { background-color: #007bff; color: white; }
        .score-high { background-color: #d4edda; }
        .score-medium { background-color: #fff3cd; }
        .score-low { background-color: #f8d7da; }
        .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
        .chart { background: #e9ecef; padding: 20px; border-radius: 5px; margin: 10px 0; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Benchmark Analysis Report</h1>
        
        <div class="stat-box">
            <h2>Summary Statistics</h2>
            <p><strong>Total Tests:</strong> ` + fmt.Sprintf("%d", stats.TotalTests) + `</p>
            <p><strong>Average Score:</strong> ` + fmt.Sprintf("%.2f", stats.AverageScore) + `</p>
        </div>

        <div class="grid">
            <div class="stat-box">
                <h2>Category Breakdown</h2>
`
	for category, count := range stats.CategoryBreakdown {
		html += fmt.Sprintf("        <p><strong>%s:</strong> %d</p>\n", category, count)
	}
	html += `            </div>

            <div class="stat-box">
                <h2>Status Breakdown</h2>
`
	for status, count := range stats.StatusBreakdown {
		html += fmt.Sprintf("        <p><strong>%s:</strong> %d</p>\n", status, count)
	}
	html += `            </div>
        </div>

        <h2>Top Performers</h2>
        <table class="performance-table">
            <tr><th>Test Name</th><th>Category</th><th>Score</th><th>Duration</th><th>Conclusion</th></tr>
`
	for _, perf := range stats.TopPerformers {
		scoreClass := "score-medium"
		if perf.Score >= 85 {
			scoreClass = "score-high"
		} else if perf.Score < 75 {
			scoreClass = "score-low"
		}
		html += fmt.Sprintf(`        <tr class="%s">
            <td>%s</td><td>%s</td><td>%.2f</td><td>%s</td><td>%s</td>
        </tr>`, scoreClass, perf.TestName, perf.Category, perf.Score, perf.Duration, perf.Conclusion)
	}
	html += `    </table>

    </div>
</body>
</html>`
	return html
}

func generateJSONReport(stats Statistics) []byte {
	data, _ := json.MarshalIndent(stats, "", "  ")
	return data
}

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
		fmt.Println("Usage: go run advanced_conclusions_reader.go <jsonl_file_path> [options]")
		fmt.Println("Options:")
		fmt.Println("  --export      Export detailed reports (HTML + JSON)")
		fmt.Println("  --filter key=value Apply filters (e.g., category=reasoning)")
		fmt.Println("  --workers N    Set number of parallel workers (default: 4)")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  go run advanced_conclusions_reader.go data/conclusions.jsonl --export")
		fmt.Println("  go run advanced_conclusions_reader.go data/conclusions.jsonl --category=reasoning --min_score=80")
		os.Exit(1)
	}

	filePath := os.Args[1]

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Printf("Error: File does not exist: %s\n", filePath)
		os.Exit(1)
	}

	// Parse arguments
	workers := 4
	export := false
	filters := make(map[string]interface{})

	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "--export":
			export = true
		case arg == "--workers" && i+1 < len(os.Args):
			fmt.Sscanf(os.Args[i+1], "%d", &workers)
			i++ // Skip next argument
		case strings.Contains(arg, "=") && !strings.HasPrefix(arg, "--"):
			parts := strings.SplitN(arg, "=", 2)
			if parts[0] == "min_score" {
				var score float64
				fmt.Sscanf(parts[1], "%f", &score)
				filters[parts[0]] = score
			} else {
				filters[parts[0]] = parts[1]
			}
		}
	}

	ctx := context.Background()

	// Process the file
	fmt.Printf("Processing benchmark conclusions from: %s\n", filePath)
	fmt.Printf("Using %d parallel workers\n", workers)
	start := time.Now()

	processor, err := ProcessLargeJSONL(ctx, filePath, workers)
	if err != nil {
		fmt.Printf("Error processing conclusions: %v\n", err)
		os.Exit(1)
	}

	duration := time.Since(start)
	fmt.Printf("File processed successfully in %v\n", duration)
	fmt.Printf("Processed: %d lines\n", processor.Processed)
	if len(processor.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(processor.Errors))
		fmt.Println("\nProcessing Errors:")
		for _, procErr := range processor.Errors {
			fmt.Printf("Line %d: %s\n", procErr.LineNum, procErr.Error)
		}
	}

	// Apply filters
	conclusions := processor.Conclusions
	if len(filters) > 0 {
		conclusions = FilterConclusions(conclusions, filters)
		fmt.Printf("Filtered to %d conclusions\n", len(conclusions))
	}

	// Generate reports
	if export {
		outputDir := filepath.Dir(filePath)
		if err := GenerateReport(conclusions, outputDir); err != nil {
			fmt.Printf("Error generating reports: %v\n", err)
		}
	}

	// Display summary
	ProcessConclusions(conclusions)
}
