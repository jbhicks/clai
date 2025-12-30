package llm

import "strings"

// AgenticBenchmarkTest represents a single test that requires the model to autonomously use code execution to complete a task
type AgenticBenchmarkTest struct {
	Name        string
	Description string
	Prompt      string
	Validator   func(output string) (bool, string) // Returns (passed, reason)
}

// AgenticBenchmarkSuite contains tests that verify a model can autonomously decide to use code execution to solve tasks
// These prompts are NATURAL task requests - the model must understand it needs to write code, choose the language, and execute it
var AgenticBenchmarkSuite = []AgenticBenchmarkTest{
	{
		Name:        "Read File Contents",
		Description: "Read and display contents of sample.txt",
		Prompt:      "What's in sample.txt?",
		Validator: func(output string) (bool, string) {
			if strings.Contains(output, "Hello World") && strings.Contains(output, "TOTAL_COUNT: 42") {
				return true, "Successfully read file contents"
			}
			return false, "Expected file contents with 'Hello World' and 'TOTAL_COUNT: 42' but got: " + output
		},
	},
	{
		Name:        "Simple Calculation",
		Description: "Perform arithmetic calculation",
		Prompt:      "What's 42 plus 58?",
		Validator: func(output string) (bool, string) {
			if strings.Contains(output, "100") {
				return true, "Correct calculation"
			}
			return false, "Expected answer to contain '100' but got: " + output
		},
	},
	{
		Name:        "Extract JSON Data",
		Description: "Parse JSON and extract specific field",
		Prompt:      "How many users are in test_data.json?",
		Validator: func(output string) (bool, string) {
			if strings.Contains(output, "4") {
				return true, "Correctly counted users from JSON"
			}
			return false, "Expected answer to contain '4' users but got: " + output
		},
	},
	{
		Name:        "Filter JSON by Field",
		Description: "Query JSON data with filtering",
		Prompt:      "List the names of all engineers in test_data.json",
		Validator: func(output string) (bool, string) {
			hasAlice := strings.Contains(output, "Alice")
			hasCharlie := strings.Contains(output, "Charlie")
			hasBob := strings.Contains(output, "Bob")
			hasDiana := strings.Contains(output, "Diana")

			if hasAlice && hasCharlie && !hasBob && !hasDiana {
				return true, "Correctly filtered engineers (Alice, Charlie)"
			}
			if hasAlice && hasCharlie {
				return false, "Found engineers but also included non-engineers in: " + output
			}
			return false, "Expected 'Alice' and 'Charlie' but got: " + output
		},
	},
	{
		Name:        "Count Lines in File",
		Description: "Count the number of lines in a file",
		Prompt:      "How many lines are in sample.txt?",
		Validator: func(output string) (bool, string) {
			if strings.Contains(output, "6") || strings.Contains(output, "six") {
				return true, "Correctly counted lines"
			}
			return false, "Expected '6' lines but got: " + output
		},
	},
	{
		Name:        "Extract Specific Line",
		Description: "Extract value from specific line in file",
		Prompt:      "What's the TOTAL_COUNT value in sample.txt?",
		Validator: func(output string) (bool, string) {
			if strings.Contains(output, "42") {
				return true, "Correctly extracted TOTAL_COUNT value"
			}
			return false, "Expected '42' but got: " + output
		},
	},
	{
		Name:        "List Directory Contents",
		Description: "List files in current directory",
		Prompt:      "Show me all .md files in the current directory",
		Validator: func(output string) (bool, string) {
			hasReadme := strings.Contains(output, "README.md")
			hasTodo := strings.Contains(output, "TODO.md")

			if hasReadme && hasTodo {
				return true, "Found markdown files"
			}
			return false, "Expected to find README.md and TODO.md but got: " + output
		},
	},
	{
		Name:        "Text Processing",
		Description: "Transform text to uppercase",
		Prompt:      "Convert the word 'benchmarking' to uppercase",
		Validator: func(output string) (bool, string) {
			if strings.Contains(output, "BENCHMARKING") {
				return true, "Correctly converted to uppercase"
			}
			return false, "Expected 'BENCHMARKING' but got: " + output
		},
	},
	{
		Name:        "Calculate String Length",
		Description: "Count characters in a string",
		Prompt:      "How many characters are in the word 'benchmarking'?",
		Validator: func(output string) (bool, string) {
			if strings.Contains(output, "12") || strings.Contains(output, "twelve") {
				return true, "Correct character count"
			}
			return false, "Expected '12' but got: " + output
		},
	},
	{
		Name:        "Generate Sequence",
		Description: "Generate numbered sequence",
		Prompt:      "Count from 1 to 5, one number per line",
		Validator: func(output string) (bool, string) {
			// Check for the sequence in order
			lines := strings.Split(strings.TrimSpace(output), "\n")
			expected := []string{"1", "2", "3", "4", "5"}

			// Look for all numbers in sequence somewhere in output
			hasAll := true
			for _, num := range expected {
				if !strings.Contains(output, num) {
					hasAll = false
					break
				}
			}

			if hasAll && len(lines) >= 5 {
				return true, "Generated correct sequence"
			}
			return false, "Expected sequence 1-5 on separate lines but got: " + output
		},
	},
	{
		Name:        "JSON Age Calculation",
		Description: "Calculate average from JSON data",
		Prompt:      "What's the average age of users in test_data.json?",
		Validator: func(output string) (bool, string) {
			// Average of 30, 25, 35, 28 = 29.5
			if strings.Contains(output, "29.5") || strings.Contains(output, "29") {
				return true, "Correctly calculated average age"
			}
			return false, "Expected average age ~29.5 but got: " + output
		},
	},
	{
		Name:        "Date/Time Query",
		Description: "Get current date information",
		Prompt:      "What day of the week is it?",
		Validator: func(output string) (bool, string) {
			// Accept any day name
			days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
			for _, day := range days {
				if strings.Contains(output, day) {
					return true, "Provided day of week"
				}
			}
			return false, "Expected a day of the week but got: " + output
		},
	},
}
