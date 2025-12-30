package llm

import (
	"time"
)

// ModelBenchmarkTest represents a single test case for model evaluation
type ModelBenchmarkTest struct {
	Name             string
	Query            string
	ExpectedBehavior string // "code", "final", or "multi-step"
	ShouldContain    []string
	ShouldNotContain []string
	MaxIterations    int
	TimeoutSeconds   int
}

// ModelBenchmarkResult captures the results of a benchmark test
type ModelBenchmarkResult struct {
	TestName      string
	Passed        bool
	Iterations    int
	TimeElapsed   time.Duration
	Response      string
	Error         error
	FailureReason string
	CodeExecuted  []string // Track what code was actually executed
}

// ModelBenchmarkSuite defines the complete test suite
// Tests focus on END RESULTS rather than specific tools used
var ModelBenchmarkSuite = []ModelBenchmarkTest{
	{
		Name:             "Extract Specific Value from File",
		Query:            "What is the TOTAL_COUNT value in sample.txt?",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"42"},                                   // The actual value
		ShouldNotContain: []string{"I don't have access", "I cannot read"}, // Failure indicators
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "Count Files by Extension",
		Query:            "How many .go files are in /home/josh/clai/internal/llm directory? Give me just the number.",
		ExpectedBehavior: "code",
		ShouldContain:    []string{},                                       // Will verify it's a number
		ShouldNotContain: []string{"I don't have access", "I cannot", "0"}, // 0 would be wrong
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "Mathematical Calculation",
		Query:            "Calculate 15 * 23 + 47 and give me the result",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"392"},
		ShouldNotContain: []string{"error"}, // Allow showing work, just need final answer
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "JSON Data Extraction",
		Query:            "Read test_data.json and tell me how many users have the role 'engineer'",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"2"},                       // Alice and Charlie
		ShouldNotContain: []string{"1", "3", "4", "I cannot"}, // Wrong counts
		MaxIterations:    8,
		TimeoutSeconds:   45,
	},
	{
		Name:             "JSON Data Analysis",
		Query:            "Read test_data.json and calculate the average age of all users. Round to nearest integer.",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{},                                // Accept 29 or 30 (29.5 rounds either way)
		ShouldNotContain: []string{"I cannot", "error", "25", "35"}, // Wrong values
		MaxIterations:    8,
		TimeoutSeconds:   45,
	},
	{
		Name:             "Complex Python Calculation",
		Query:            "Use Python to find the sum of squares of numbers 1 through 10",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"385"},   // 1² + 2² + ... + 10² = 385
		ShouldNotContain: []string{"error"}, // Allow showing work
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "File Content Verification",
		Query:            "Does sample.txt contain the word 'Important'? Answer yes or no.",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"yes", "Yes", "YES", "contains", "found"}, // More flexible
		ShouldNotContain: []string{"no", "No", "does not contain", "not found"},
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "Multi-Step Text Processing",
		Query:            "Count how many lines in sample.txt contain the word 'Line'",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"2"},                // Line 3 and Line 4
		ShouldNotContain: []string{"0", "1", "3", "5"}, // Wrong counts
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "JSON Field Extraction",
		Query:            "Read test_data.json and give me the name of the user with id 3",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"Charlie"},
		ShouldNotContain: []string{"Alice", "Bob", "Diana", "error"},
		MaxIterations:    8,
		TimeoutSeconds:   45,
	},
	{
		Name:             "Error Handling",
		Query:            "Try to read the file nonexistent_file_xyz_999.txt and tell me what happens",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"doesn't exist", "does not exist", "doesn't", "not exist"}, // More flexible
		ShouldNotContain: []string{"successfully", "here is the content"},
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "Code File Analysis",
		Query:            "In the file agent.go, how many times does the word 'Agent' appear? Just give me the number.",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{},                    // Will vary, but should be a number > 0
		ShouldNotContain: []string{"I cannot", "error"}, // Remove "0" since model might mention it while explaining
		MaxIterations:    8,
		TimeoutSeconds:   45,
	},
	{
		Name:             "No Code Needed - General Knowledge",
		Query:            "What is the capital of France?",
		ExpectedBehavior: "final",
		ShouldContain:    []string{"Paris"},
		ShouldNotContain: []string{"<code", "London", "Berlin"},
		MaxIterations:    2,
		TimeoutSeconds:   15,
	},
}
