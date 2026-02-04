package benchmark

import (
	"clai/internal/db"
	"clai/internal/llm"
	"clai/internal/logger"
	"clai/internal/tools"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode"
)

// Runner executes benchmark tests and saves results to the database
type Runner struct {
	store     *db.Store
	llmClient *llm.Client
}

// NewRunner creates a new benchmark runner
func NewRunner(store *db.Store, llmClient *llm.Client) *Runner {
	return &Runner{
		store:     store,
		llmClient: llmClient,
	}
}

// RunBenchmark executes the full benchmark suite for a model and saves results
func (r *Runner) RunBenchmark(ctx context.Context, modelName, modelURL string) (int64, error) {
	startTime := time.Now()

	// Create benchmark run record
	run := &db.BenchmarkRun{
		ModelName:  modelName,
		ModelURL:   modelURL,
		TotalTests: len(llm.ModelBenchmarkSuite),
		StartedAt:  startTime,
	}

	// Execute all tests
	results := make([]db.BenchmarkResult, 0, len(llm.ModelBenchmarkSuite))
	passed := 0
	totalIterations := 0

	for _, test := range llm.ModelBenchmarkSuite {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		// Execute single test
		result := r.runSingleTest(test)

		// Convert to db.BenchmarkResult
		dbResult := db.BenchmarkResult{
			TestName:        result.TestName,
			Query:           test.Query,
			Passed:          result.Passed,
			Iterations:      result.Iterations,
			TimeSeconds:     result.TimeElapsed.Seconds(),
			Response:        result.Response,
			FailureReason:   result.FailureReason,
			CodeExecuted:    strings.Join(result.CodeExecuted, "\n---\n"),
			TokensGenerated: result.TokensGenerated,
			TokensPerSecond: result.TokensPerSecond,
		}

		results = append(results, dbResult)

		if result.Passed {
			passed++
		}
		totalIterations += result.Iterations
	}

	// Calculate summary statistics
	completedAt := time.Now()
	run.PassedTests = passed
	run.FailedTests = run.TotalTests - passed
	run.SuccessRate = float64(passed) / float64(run.TotalTests) * 100
	run.TotalTimeSeconds = completedAt.Sub(startTime).Seconds()
	run.AvgIterations = float64(totalIterations) / float64(run.TotalTests)
	run.CompletedAt = completedAt

	// Save run to database
	runID, err := r.store.SaveBenchmarkRun(run)
	if err != nil {
		return 0, fmt.Errorf("failed to save benchmark run: %w", err)
	}

	// Save all results
	for i := range results {
		results[i].RunID = int(runID)
		if err := r.store.SaveBenchmarkResult(&results[i]); err != nil {
			log.Printf("Failed to save result for test '%s': %v", results[i].TestName, err)
		}
	}

	// Clean up test-created files
	cleanupTestFiles()

	return runID, nil
}

// runSingleTest executes a single benchmark test (based on runSingleBenchmark from model_benchmark_test.go)
func (r *Runner) runSingleTest(test llm.ModelBenchmarkTest) llm.ModelBenchmarkResult {
	result := llm.ModelBenchmarkResult{
		TestName:     test.Name,
		CodeExecuted: []string{},
	}

	// Print test info
	fmt.Printf("\n🧪 Running: %s\n", test.Name)
	fmt.Printf("   Query: %s\n", test.Query)
	fmt.Printf("   Streaming response...\n")

	agent := llm.NewAgent(r.llmClient)

	// Note: System prompt is already configured in the LLM client
	// Do not override it here

	// Collect the full response for evaluation
	var fullResponse strings.Builder
	var inToolCall bool

	// Run the test with streaming
	start := time.Now()
	response, err := agent.RunWithStreaming(test.Query, func(chunk string, toolCall *llm.ToolCall, codeBlock *llm.CodeBlock) {
		if toolCall != nil {
			// Print tool call info and suppress text chunks until tool completes
			fmt.Printf("\n[Tool Call: %s]\n", toolCall.Function.Name)
			fmt.Printf("[Tool Args: %s]\n", toolCall.Function.Arguments)
			inToolCall = true
			result.CodeExecuted = append(result.CodeExecuted, fmt.Sprintf("tool:%s %s", toolCall.Function.Name, toolCall.Function.Arguments))
			logger.Debug("[BENCH-STREAM] tool_call name=%s args=%s", toolCall.Function.Name, truncateForLog(toolCall.Function.Arguments, 200))
		} else if codeBlock != nil {
			// Print code execution info
			fmt.Printf("\n[Executing %s code]\n", codeBlock.Language)
			logger.Debug("[BENCH-STREAM] code_block language=%s", codeBlock.Language)
		} else if chunk != "" && !inToolCall {
			// Only print text chunks when not in a tool call
			fmt.Print(chunk)
			fullResponse.WriteString(chunk)
			logger.Debug("[BENCH-STREAM] chunk len=%d content=%q", len(chunk), truncateForLog(chunk, 200))
		} else if chunk != "" {
			// Still collect all chunks for evaluation, even during tool calls
			fullResponse.WriteString(chunk)
			logger.Debug("[BENCH-STREAM] chunk(in tool) len=%d content=%q", len(chunk), truncateForLog(chunk, 200))
		}
	})
	result.TimeElapsed = time.Since(start)
	result.Response = response
	result.Error = err
	logger.Debug("[BENCH-RESULT] response_len=%d full_len=%d err=%v", len(response), fullResponse.Len(), err)

	// Simple iteration count (could be improved to track actual iterations)
	result.Iterations = 1

	// Calculate token metrics
	if response != "" {
		result.TokensGenerated = estimateTokenCount(response)
		result.TokensPerSecond = float64(result.TokensGenerated) / result.TimeElapsed.Seconds()
	}

	if response == "" && fullResponse.Len() > 0 {
		result.Response = fullResponse.String()
	}

	if err != nil {
		result.FailureReason = fmt.Sprintf("Error: %v", err)
		return result
	}

	if result.Response != "" {
		toolCalls := agent.ParseToolCallsForBenchmark(result.Response)
		if len(toolCalls) > 0 {
			logger.Debug("[BENCH-TOOLS] parsed_tool_calls=%d", len(toolCalls))
			var toolOutput strings.Builder
			for _, toolCall := range toolCalls {
				output, toolErr := tools.ExecuteTool(toolCall)
				result.CodeExecuted = append(result.CodeExecuted, fmt.Sprintf("tool:%s %s", toolCall.Function.Name, toolCall.Function.Arguments))
				if toolErr != nil {
					toolOutput.WriteString(fmt.Sprintf("Tool execution error: %v\n", toolErr))
					logger.Debug("[BENCH-TOOLS] tool_error name=%s err=%v", toolCall.Function.Name, toolErr)
				} else {
					toolOutput.WriteString(output)
					if !strings.HasSuffix(output, "\n") {
						toolOutput.WriteString("\n")
					}
					logger.Debug("[BENCH-TOOLS] tool_output name=%s len=%d", toolCall.Function.Name, len(output))
				}
			}
			result.Response = toolOutput.String()
		}
	}

	// Check if test passed
	responseLower := strings.ToLower(result.Response)

	// Check ShouldNotContain first
	for _, forbidden := range test.ShouldNotContain {
		if strings.Contains(responseLower, strings.ToLower(forbidden)) {
			result.FailureReason = fmt.Sprintf("Response contains forbidden content: '%s'", forbidden)
			return result
		}
	}

	// Check ShouldContain
	if len(test.ShouldContain) > 0 {
		foundAny := false
		for _, required := range test.ShouldContain {
			if strings.Contains(responseLower, strings.ToLower(required)) {
				foundAny = true
				break
			}
		}
		if !foundAny {
			result.FailureReason = fmt.Sprintf("Response missing required content. Expected one of: %v", test.ShouldContain)
			return result
		}
	}

	// Test passed!
	result.Passed = true
	return result
}

func truncateForLog(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

// estimateTokenCount provides a rough estimate of token count in text
// This is a simplified approximation: ~4 characters per token for English text
func estimateTokenCount(text string) int {
	if text == "" {
		return 0
	}

	// Count alphanumeric characters and spaces
	charCount := 0
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			charCount++
		}
	}

	// Rough approximation: 4 characters per token
	// This is a simplification - actual tokenization is more complex
	tokens := charCount / 4
	if tokens == 0 && charCount > 0 {
		tokens = 1 // At least 1 token for non-empty text
	}

	return tokens
}

// cleanupTestFiles removes files that are known to be created by benchmark tests
func cleanupTestFiles() {
	filesToClean := []string{
		"user_count.txt",
		"nonexistent_file_xyz_999.txt",
		// Add other test-created files here as needed
	}

	for _, filename := range filesToClean {
		if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: failed to clean up test file '%s': %v", filename, err)
		}
	}
}
