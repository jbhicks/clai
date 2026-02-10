package benchmark

import (
	"clai/internal/llm"
	"clai/internal/logger"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// BenchmarkVisualizer provides clean, readable output for benchmark tests
type BenchmarkVisualizer struct {
	indent     string
	testNumber int
	totalTests int
}

// NewBenchmarkVisualizer creates a new visualizer
func NewBenchmarkVisualizer(testNumber, totalTests int) *BenchmarkVisualizer {
	return &BenchmarkVisualizer{
		indent:     "  ",
		testNumber: testNumber,
		totalTests: totalTests,
	}
}

// PrintTestHeader prints a clean test header
func (v *BenchmarkVisualizer) PrintTestHeader(name, query string) {
	fmt.Printf("\n%s┌──────────────────────────────────────────────────────────────────────┐%s\n", Blue, Reset)
	fmt.Printf("%s│ Test %d/%d: %-56s │%s\n", Blue, v.testNumber, v.totalTests, truncateString(name, 56), Reset)
	fmt.Printf("%s└──────────────────────────────────────────────────────────────────────┘%s\n", Blue, Reset)
	fmt.Printf("\n%sQuery:%s %s\n", Cyan, Reset, query)
	fmt.Printf("%s%s\n", v.indent, strings.Repeat("─", 70))
}

// PrintThinking prints when the LLM is reasoning
func (v *BenchmarkVisualizer) PrintThinking(text string) {
	if text == "" {
		return
	}
	// Don't print raw JSON or tool call fragments
	if isJSONFragment(text) || isToolCallFragment(text) {
		logger.Debug("[BENCH-THINKING-FILTERED] %s", truncateForLog(text, 100))
		return
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	fmt.Printf("\n%s%s🤔 Thinking:%s %s\n", v.indent, Yellow, Reset, truncateString(text, 60))
}

// PrintToolCall prints a formatted tool call
func (v *BenchmarkVisualizer) PrintToolCall(toolCall *llm.ToolCall) {
	if toolCall == nil || toolCall.Function.Name == "" {
		return
	}

	fmt.Printf("\n%s%s🔧 Tool Call:%s %s%s%s\n",
		v.indent,
		Green, Reset,
		Bold, toolCall.Function.Name, Reset)

	// Try to parse and format arguments
	args := toolCall.Function.Arguments
	if args != "" {
		// Try to pretty-print JSON arguments
		var argsMap map[string]interface{}
		if err := json.Unmarshal([]byte(args), &argsMap); err == nil {
			v.printFormattedArgs(argsMap)
		} else {
			// Fallback: print raw but truncated
			fmt.Printf("%s%s  args:%s %s\n", v.indent, v.indent, Reset, truncateString(args, 80))
		}
	}
}

// printFormattedArgs prints formatted arguments
func (v *BenchmarkVisualizer) printFormattedArgs(args map[string]interface{}) {
	for key, val := range args {
		valStr := fmt.Sprintf("%v", val)
		// Truncate long values
		if len(valStr) > 50 {
			valStr = valStr[:47] + "..."
		}
		// Format code specially
		if key == "code" || key == "command" {
			fmt.Printf("%s%s  %s:%s\n", v.indent, v.indent, key, Reset)
			lines := strings.Split(valStr, "\n")
			for i, line := range lines {
				if i >= 3 { // Only show first 3 lines
					fmt.Printf("%s%s    ... (%d more lines)\n", v.indent, v.indent, len(lines)-3)
					break
				}
				fmt.Printf("%s%s    %s%s%s\n", v.indent, v.indent, Dim, truncateString(line, 60), Reset)
			}
		} else {
			fmt.Printf("%s%s  %s:%s %s%s%s\n", v.indent, v.indent, key, Reset, Dim, valStr, Reset)
		}
	}
}

// PrintCodeExecution prints formatted code execution
func (v *BenchmarkVisualizer) PrintCodeExecution(language, code string) {
	fmt.Printf("\n%s%s💻 Executing %s code%s\n", v.indent, Magenta, language, Reset)

	lines := strings.Split(code, "\n")
	lineCount := len(lines)

	// Show first 5 lines
	for i, line := range lines {
		if i >= 5 {
			fmt.Printf("%s%s  ... (%d more lines)%s\n", v.indent, Dim, lineCount-5, Reset)
			break
		}
		fmt.Printf("%s%s  %s%s%s\n", v.indent, v.indent, Dim, truncateString(line, 65), Reset)
	}
}

// PrintToolResult prints tool execution result
func (v *BenchmarkVisualizer) PrintToolResult(result string, err error) {
	if err != nil {
		fmt.Printf("%s%s❌ Error:%s %v\n", v.indent, Red, Reset, err)
	} else {
		result = strings.TrimSpace(result)
		if len(result) > 100 {
			result = result[:97] + "..."
		}
		fmt.Printf("%s%s✓ Result:%s %s%s%s\n", v.indent, Green, Reset, Dim, result, Reset)
	}
}

// PrintResponseChunk prints a response chunk (filtered)
func (v *BenchmarkVisualizer) PrintResponseChunk(chunk string) {
	if chunk == "" {
		return
	}

	// Filter out JSON fragments and tool call noise
	if isJSONFragment(chunk) || isToolCallFragment(chunk) {
		logger.Debug("[BENCH-CHUNK-FILTERED] %q", truncateForLog(chunk, 100))
		return
	}

	// Filter out specific noise patterns
	chunk = filterNoisePatterns(chunk)

	if chunk != "" {
		fmt.Print(chunk)
	}
}

// PrintIterationComplete prints iteration summary
func (v *BenchmarkVisualizer) PrintIterationComplete(iteration int) {
	fmt.Printf("\n%s%s⏱️  Iteration %d complete%s\n", v.indent, Blue, iteration, Reset)
}

// PrintTestResult prints final test result
func (v *BenchmarkVisualizer) PrintTestResult(result llm.ModelBenchmarkResult) {
	fmt.Printf("\n%s%s\n", v.indent, strings.Repeat("─", 70))

	status := "✅ PASSED"
	statusColor := Green
	if !result.Passed {
		status = "❌ FAILED"
		statusColor = Red
	}

	fmt.Printf("\n%s%s%s %s%s\n", v.indent, statusColor, status, Reset, result.TestName)

	if result.FailureReason != "" {
		fmt.Printf("%s%sReason:%s %s\n", v.indent, Yellow, Reset, result.FailureReason)
	}

	fmt.Printf("%s%sTime:%s %.2fs  %sTokens:%s %d  %sSpeed:%s %.1f tok/s\n",
		v.indent,
		Cyan, Reset, result.TimeElapsed.Seconds(),
		Cyan, Reset, result.TokensGenerated,
		Cyan, Reset, result.TokensPerSecond)

	// Print executed actions summary
	if len(result.CodeExecuted) > 0 {
		fmt.Printf("\n%s%sActions executed:%s\n", v.indent, Blue, Reset)
		for i, action := range result.CodeExecuted {
			if i >= 5 {
				fmt.Printf("%s%s  ... and %d more%s\n", v.indent, v.indent, len(result.CodeExecuted)-5, Reset)
				break
			}

			// Format the action
			if strings.HasPrefix(action, "tool:") {
				parts := strings.SplitN(action[5:], " ", 2)
				toolName := parts[0]
				fmt.Printf("%s%s  •%s Called %s%s%s\n", v.indent, v.indent, Reset, Bold, toolName, Reset)
			} else if strings.HasPrefix(action, "code:") {
				fmt.Printf("%s%s  •%s Executed %s%s%s\n", v.indent, v.indent, Reset, Bold, action[5:], Reset)
			}
		}
	}
}

// PrintSummary prints final benchmark summary
func (v *BenchmarkVisualizer) PrintSummary(passed, total int, totalTime time.Duration) {
	fmt.Printf("\n%s%s╔══════════════════════════════════════════════════════════════════════╗%s\n", Bold, Blue, Reset)
	fmt.Printf("%s%s║                         BENCHMARK SUMMARY                            ║%s\n", Bold, Blue, Reset)
	fmt.Printf("%s%s╚══════════════════════════════════════════════════════════════════════╝%s\n", Bold, Blue, Reset)

	percent := float64(passed) / float64(total) * 100

	var statusColor string
	if percent == 100 {
		statusColor = Green
	} else if percent >= 80 {
		statusColor = Yellow
	} else {
		statusColor = Red
	}

	fmt.Printf("\n%sResults:%s %s%d/%d (%.1f%%)%s\n",
		Bold, Reset,
		statusColor, passed, total, percent, Reset)
	fmt.Printf("%sTotal Time:%s %.2fs\n", Bold, Reset, totalTime.Seconds())
	fmt.Printf("%sAverage Time/Test:%s %.2fs\n", Bold, Reset, totalTime.Seconds()/float64(total))
}

// Helper functions

func isJSONFragment(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") ||
		strings.Contains(s, `"id":`) || strings.Contains(s, `"type":`) ||
		strings.Contains(s, `"function":`) || strings.Contains(s, `"arguments":`)
}

func isToolCallFragment(s string) bool {
	return strings.Contains(s, `Tool Call:`) || strings.Contains(s, `"tool_call"`) ||
		strings.Contains(s, `execute_code`) && strings.Contains(s, `"language"`)
}

func filterNoisePatterns(s string) string {
	// Filter out partial JSON from streaming
	patterns := []string{
		`{"id":`,
		`"type":"function"`,
		`"function":{`,
		`"name":"`,
		`"arguments":"`,
		`Tool Call: execute_code`,
	}

	for _, pattern := range patterns {
		if strings.Contains(s, pattern) && len(s) < 50 {
			return ""
		}
	}

	return s
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Color codes
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
)

// ToolCallSummary represents a summarized tool call for display
type ToolCallSummary struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}
