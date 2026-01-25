package llm

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"clai/internal/logger"
)

// Note: ModelBenchmarkTest, ModelBenchmarkResult, and ModelBenchmarkSuite
// are now defined in benchmarks.go (non-test file) so they can be used
// by the benchmark web server package.

// RunModelBenchmark executes all benchmark tests against a given model
func RunModelBenchmark(t *testing.T, client LLMClientInterface) []ModelBenchmarkResult {
	results := make([]ModelBenchmarkResult, 0, len(ModelBenchmarkSuite))

	for _, test := range ModelBenchmarkSuite {
		t.Run(test.Name, func(t *testing.T) {
			result := runSingleBenchmark(t, client, test)
			results = append(results, result)

			// Log result
			if result.Passed {
				t.Logf("✓ PASS: %s (%.2fs, %d iterations)",
					test.Name, result.TimeElapsed.Seconds(), result.Iterations)
			} else {
				t.Errorf("✗ FAIL: %s - %s (%.2fs, %d iterations)",
					test.Name, result.FailureReason, result.TimeElapsed.Seconds(), result.Iterations)
				t.Logf("Response: %s", truncate(result.Response, 200))
			}
		})
	}

	return results
}

func runSingleBenchmark(t *testing.T, client LLMClientInterface, test ModelBenchmarkTest) ModelBenchmarkResult {
	result := ModelBenchmarkResult{
		TestName: test.Name,
		Passed:   false,
	}

	agent := NewAgent(client)

	// CRITICAL: Add system prompt so the agent knows to use <code> blocks
	systemPrompt := `You are a free agent AI with full code execution capabilities. You can execute bash, python, and javascript code directly on the system.

**Critical rules:**
1. When you need to read files, execute commands, or perform system operations, use code execution by wrapping code in XML tags:
    <code language="bash">cat /path/to/file</code>
    <code language="python">print("Hello")</code>
    <code language="javascript">console.log("Hello")</code>

2. **Efficiency requirements:**
   - Complete tasks in minimum number of steps (ideally 1-2 iterations)
   - Combine multiple operations into single code blocks when possible
   - Avoid unnecessary file exploration or directory listings
   - Read files directly when you know the path - don't search for alternatives

3. **File handling:**
   - When a task mentions a specific file (e.g., "test_data.json"), use that exact file
   - Don't search for other files unless the specified file doesn't exist
   - Prefer the specified file even if other similar files exist

4. **Error handling:**
   - If a tool execution fails, fix the specific issue and retry
   - Don't abandon the approach after one error
   - Check JSON string escaping for Python code

5. **Code execution:**
   - DO NOT use echo/print/console.log to narrate your thinking. Only use them for actual task output.
   - Keep code blocks focused and purposeful.
   - Validate code syntax before execution.

Answer questions clearly and execute code when needed to provide accurate information.`

	agent.AddMessage("system", systemPrompt)

	// Set up timeout
	done := make(chan bool)
	start := time.Now()

	go func() {
		response, err := agent.Run(test.Query)
		result.TimeElapsed = time.Since(start)
		result.Response = response
		result.Error = err
		result.Iterations = len(agent.messages) / 2 // Rough iteration count

		if err != nil {
			result.FailureReason = fmt.Sprintf("Error: %v", err)
			done <- true
			return
		}

		// Check expected behavior
		responseLower := strings.ToLower(response)

		// Check ShouldContain
		for _, expected := range test.ShouldContain {
			if !strings.Contains(responseLower, strings.ToLower(expected)) {
				result.FailureReason = fmt.Sprintf("Response missing expected content: '%s'", expected)
				done <- true
				return
			}
		}

		// Check ShouldNotContain
		for _, forbidden := range test.ShouldNotContain {
			if strings.Contains(responseLower, strings.ToLower(forbidden)) {
				result.FailureReason = fmt.Sprintf("Response contains forbidden content: '%s'", forbidden)
				done <- true
				return
			}
		}

		// Check iteration count
		if result.Iterations > test.MaxIterations {
			result.FailureReason = fmt.Sprintf("Too many iterations: %d (max: %d)", result.Iterations, test.MaxIterations)
			done <- true
			return
		}

		// All checks passed
		result.Passed = true
		done <- true
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		return result
	case <-time.After(time.Duration(test.TimeoutSeconds) * time.Second):
		result.FailureReason = fmt.Sprintf("Timeout after %d seconds", test.TimeoutSeconds)
		result.TimeElapsed = time.Since(start)
		return result
	}
}

// PrintBenchmarkSummary prints a formatted summary of benchmark results
func PrintBenchmarkSummary(results []ModelBenchmarkResult) string {
	var summary strings.Builder

	passed := 0
	failed := 0
	totalTime := time.Duration(0)
	totalIterations := 0

	summary.WriteString("\n" + strings.Repeat("=", 80) + "\n")
	summary.WriteString("MODEL BENCHMARK SUMMARY\n")
	summary.WriteString(strings.Repeat("=", 80) + "\n\n")

	for _, result := range results {
		if result.Passed {
			passed++
			summary.WriteString(fmt.Sprintf("✓ %-40s %6.2fs  %2d iter\n",
				result.TestName, result.TimeElapsed.Seconds(), result.Iterations))
		} else {
			failed++
			summary.WriteString(fmt.Sprintf("✗ %-40s %6.2fs  %2d iter  %s\n",
				result.TestName, result.TimeElapsed.Seconds(), result.Iterations, result.FailureReason))
		}
		totalTime += result.TimeElapsed
		totalIterations += result.Iterations
	}

	summary.WriteString("\n" + strings.Repeat("-", 80) + "\n")
	summary.WriteString(fmt.Sprintf("TOTAL: %d tests, %d passed, %d failed\n",
		len(results), passed, failed))
	summary.WriteString(fmt.Sprintf("Total time: %.2fs, Avg time: %.2fs\n",
		totalTime.Seconds(), totalTime.Seconds()/float64(len(results))))
	summary.WriteString(fmt.Sprintf("Total iterations: %d, Avg iterations: %.1f\n",
		totalIterations, float64(totalIterations)/float64(len(results))))
	summary.WriteString(fmt.Sprintf("Success rate: %.1f%%\n",
		float64(passed)/float64(len(results))*100))
	summary.WriteString(strings.Repeat("=", 80) + "\n")

	return summary.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ExportBenchmarkHTML generates an HTML report of benchmark results
func ExportBenchmarkHTML(results []ModelBenchmarkResult, modelName, outputPath string) error {
	passed := 0
	failed := 0
	totalTime := time.Duration(0)
	totalIterations := 0

	for _, result := range results {
		if result.Passed {
			passed++
		} else {
			failed++
		}
		totalTime += result.TimeElapsed
		totalIterations += result.Iterations
	}

	successRate := float64(passed) / float64(len(results)) * 100
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Model Benchmark Report - %s</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
            padding: 30px;
            border-radius: 10px;
            margin-bottom: 30px;
        }
        .header h1 {
            margin: 0 0 10px 0;
        }
        .header .meta {
            opacity: 0.9;
            font-size: 14px;
        }
        .summary {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .summary-card {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .summary-card .label {
            color: #666;
            font-size: 12px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .summary-card .value {
            font-size: 32px;
            font-weight: bold;
            margin-top: 5px;
        }
        .success-rate {
            color: %s;
        }
        table {
            width: 100%%;
            background: white;
            border-radius: 8px;
            overflow: hidden;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            border-collapse: collapse;
        }
        th {
            background: #f8f9fa;
            padding: 15px;
            text-align: left;
            font-weight: 600;
            color: #333;
            border-bottom: 2px solid #dee2e6;
        }
        td {
            padding: 15px;
            border-bottom: 1px solid #dee2e6;
        }
        tr:hover {
            background-color: #f8f9fa;
        }
        .status-pass {
            color: #10b981;
            font-weight: bold;
        }
        .status-fail {
            color: #ef4444;
            font-weight: bold;
        }
        .response-box {
            background: #f8f9fa;
            padding: 10px;
            border-radius: 4px;
            font-family: monospace;
            font-size: 12px;
            max-height: 100px;
            overflow-y: auto;
            white-space: pre-wrap;
            word-wrap: break-word;
        }
        .failure-reason {
            color: #dc2626;
            font-size: 12px;
            font-style: italic;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Model Benchmark Report</h1>
        <div class="meta">
            <div>Model: %s</div>
            <div>Generated: %s</div>
        </div>
    </div>

    <div class="summary">
        <div class="summary-card">
            <div class="label">Total Tests</div>
            <div class="value">%d</div>
        </div>
        <div class="summary-card">
            <div class="label">Passed</div>
            <div class="value" style="color: #10b981;">%d</div>
        </div>
        <div class="summary-card">
            <div class="label">Failed</div>
            <div class="value" style="color: #ef4444;">%d</div>
        </div>
        <div class="summary-card">
            <div class="label">Success Rate</div>
            <div class="value success-rate">%.1f%%%%</div>
        </div>
        <div class="summary-card">
            <div class="label">Total Time</div>
            <div class="value">%.1fs</div>
        </div>
        <div class="summary-card">
            <div class="label">Avg Iterations</div>
            <div class="value">%.1f</div>
        </div>
    </div>

    <table>
        <thead>
            <tr>
                <th>Test Name</th>
                <th>Status</th>
                <th>Time</th>
                <th>Iterations</th>
                <th>Details</th>
            </tr>
        </thead>
        <tbody>
`, modelName, getSuccessRateColor(successRate), modelName, timestamp,
		len(results), passed, failed, successRate, totalTime.Seconds(),
		float64(totalIterations)/float64(len(results)))

	for _, result := range results {
		status := "PASS"
		statusClass := "status-pass"
		details := ""

		if !result.Passed {
			status = "FAIL"
			statusClass = "status-fail"
			if result.FailureReason != "" {
				details = fmt.Sprintf(`<div class="failure-reason">%s</div>`, html.EscapeString(result.FailureReason))
			}
			if result.Response != "" {
				truncatedResponse := truncate(result.Response, 500)
				details += fmt.Sprintf(`<div class="response-box">%s</div>`, html.EscapeString(truncatedResponse))
			}
		} else {
			if result.Response != "" {
				truncatedResponse := truncate(result.Response, 300)
				details = fmt.Sprintf(`<div class="response-box">%s</div>`, html.EscapeString(truncatedResponse))
			}
		}

		htmlContent += fmt.Sprintf(`
            <tr>
                <td><strong>%s</strong></td>
                <td class="%s">%s</td>
                <td>%.2fs</td>
                <td>%d</td>
                <td>%s</td>
            </tr>
`, html.EscapeString(result.TestName), statusClass, status,
			result.TimeElapsed.Seconds(), result.Iterations, details)
	}

	htmlContent += `
        </tbody>
    </table>
</body>
</html>`

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write HTML file
	if err := os.WriteFile(outputPath, []byte(htmlContent), 0644); err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}

	return nil
}

func getSuccessRateColor(rate float64) string {
	if rate >= 70 {
		return "#10b981" // green
	} else if rate >= 50 {
		return "#f59e0b" // orange
	}
	return "#ef4444" // red
}

// TestModelBenchmark_CurrentModel runs the full benchmark suite against the current model
// Run with: go test -v -run TestModelBenchmark_CurrentModel ./internal/llm
func TestMain(m *testing.M) {
	// Initialize logger for benchmark tests
	logger.Init(os.Stdout)
}

func TestModelBenchmark_CurrentModel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark test in short mode")
	}

	client := NewClient("http://localhost:8081", "test-model", "")

	t.Logf("Testing model at: %s", client.Host())
	t.Logf("Running %d benchmark tests...\n", len(ModelBenchmarkSuite))

	results := RunModelBenchmark(t, client)

	summary := PrintBenchmarkSummary(results)
	t.Log(summary)

	// Export HTML report
	htmlPath := filepath.Join("../../model_test_results",
		fmt.Sprintf("benchmark_%s.html", time.Now().Format("20060102_150405")))
	if err := ExportBenchmarkHTML(results, "Hermes-3-Llama-3.1-8B", htmlPath); err != nil {
		t.Logf("Warning: Failed to export HTML report: %v", err)
	} else {
		t.Logf("HTML report generated: %s", htmlPath)
	}

	// Optionally fail if success rate is too low
	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}
	successRate := float64(passed) / float64(len(results)) * 100

	if successRate < 50.0 {
		t.Errorf("Model success rate too low: %.1f%% (expected at least 50%%)", successRate)
	}
}
