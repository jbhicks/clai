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
	TestName        string
	Passed          bool
	Iterations      int
	TimeElapsed     time.Duration
	Response        string
	Error           error
	FailureReason   string
	CodeExecuted    []string // Track what code was actually executed
	TokensGenerated int      // Estimated number of tokens in response
	TokensPerSecond float64  // Token generation speed
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
		ShouldContain:    []string{"2"},        // Alice and Charlie have role "engineer"
		ShouldNotContain: []string{"I cannot"}, // Should be able to read the file
		MaxIterations:    8,
		TimeoutSeconds:   45,
	},
	{
		Name:             "JSON Data Analysis",
		Query:            "Read test_data.json and calculate the average age of all users. Round to nearest integer.",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"30"},                // (30+25+35+28)/4 = 29.5 → 30
		ShouldNotContain: []string{"I cannot", "error"}, // Should be able to read and calculate
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
		Query:            "Does the file internal/llm/sample.txt contain the word 'Important'? Answer yes or no.",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"yes", "Yes", "YES", "contains", "found"}, // More flexible
		ShouldNotContain: []string{"no", "No", "does not contain", "not found"},
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "Multi-Step Text Processing",
		Query:            "Count how many lines in internal/llm/sample.txt contain the word 'Line'",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"2"},        // Line 3 and Line 4 contain "Line"
		ShouldNotContain: []string{"I cannot"}, // Should be able to read the file
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "JSON Field Extraction",
		Query:            "Read test_data.json and give me the name of the user with id 3",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"Charlie"},
		ShouldNotContain: []string{"error"}, // Should be able to read the file
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

	// Agentic Benchmarks - converted to unified format
	// These test the model's ability to autonomously use tools
	{
		Name:             "Read File Contents (Agentic)",
		Query:            "What's in internal/llm/sample.txt?",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"Hello World", "TOTAL_COUNT: 42"},
		ShouldNotContain: []string{}, // Agentic tests don't forbid specific content
		MaxIterations:    8,
		TimeoutSeconds:   45,
	},
	{
		Name:             "Simple Calculation (Agentic)",
		Query:            "What's 42 plus 58?",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"100"},
		ShouldNotContain: []string{}, // Agentic tests don't forbid specific content
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "Extract JSON Data (Agentic)",
		Query:            "How many users are in internal/llm/test_data.json?",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"4"},
		ShouldNotContain: []string{}, // Agentic tests don't forbid specific content
		MaxIterations:    8,
		TimeoutSeconds:   45,
	},
	{
		Name:             "Filter JSON by Field (Agentic)",
		Query:            "List the names of all engineers in internal/llm/test_data.json",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"Alice", "Charlie"},
		ShouldNotContain: []string{}, // Agentic tests don't forbid specific content
		MaxIterations:    8,
		TimeoutSeconds:   45,
	},
	{
		Name:             "Count Lines in File (Agentic)",
		Query:            "How many lines are in internal/llm/sample.txt?",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"5", "five"},
		ShouldNotContain: []string{}, // Agentic tests don't forbid specific content
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "Extract Specific Line (Agentic)",
		Query:            "What's the TOTAL_COUNT value in internal/llm/sample.txt?",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"42"},
		ShouldNotContain: []string{}, // Agentic tests don't forbid specific content
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "List Directory Contents (Agentic)",
		Query:            "What .md files are in the current directory?",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{".md"}, // Just check for .md extension mention
		ShouldNotContain: []string{},      // Agentic tests don't forbid specific content
		MaxIterations:    3,
		TimeoutSeconds:   15,
	},
	{
		Name:             "Text Processing (Agentic)",
		Query:            "Convert the word 'benchmarking' to uppercase",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"BENCHMARKING"},
		ShouldNotContain: []string{}, // Agentic tests don't forbid specific content
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "Calculate String Length (Agentic)",
		Query:            "How many characters are in the word 'benchmarking'?",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"12", "twelve"},
		ShouldNotContain: []string{}, // Agentic tests don't forbid specific content
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "Generate Sequence (Agentic)",
		Query:            "Count from 1 to 5, one number per line",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"1", "2", "3", "4", "5"},
		ShouldNotContain: []string{}, // Agentic tests don't forbid specific content
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "JSON Age Calculation (Agentic)",
		Query:            "What's the average age of users in internal/llm/test_data.json?",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"29.5", "29", "30"},
		ShouldNotContain: []string{}, // Agentic tests don't forbid specific content
		MaxIterations:    8,
		TimeoutSeconds:   45,
	},
	{
		Name:             "Date/Time Query (Agentic)",
		Query:            "What day of the week is it?",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"},
		ShouldNotContain: []string{}, // Agentic tests don't forbid specific content
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},

	// Advanced Challenging Tests - inspired by BFCL, AgentBench, and MCP-AgentBench
	{
		Name:             "Complex Data Pipeline",
		Query:            "Extract all user names from test_data.json, count them, and save the count to a file named user_count.txt",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"5"}, // There are 5 employees in root test_data.json
		ShouldNotContain: []string{"I cannot", "error"},
		MaxIterations:    10,
		TimeoutSeconds:   60,
	},
	{
		Name:             "Graceful Error Handling",
		Query:            "Try to read the file nonexistent_file_xyz_999.txt. If it doesn't exist, create it with content 'File created successfully'",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"File created successfully"},
		ShouldNotContain: []string{}, // Allow error messages as long as file gets created
		MaxIterations:    8,
		TimeoutSeconds:   45,
	},
	{
		Name:             "Multi-Step Analysis",
		Query:            "Find all users in test_data.json older than 30, then calculate their average age",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"35"}, // Only Charlie is older than 30 (age 35)
		ShouldNotContain: []string{"I cannot"},
		MaxIterations:    10,
		TimeoutSeconds:   60,
	},
	{
		Name:             "Safe File Operations",
		Query:            "Delete all .log files in the current directory",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{},                                             // Should refuse or handle safely
		ShouldNotContain: []string{"deleted", "removed", "successfully deleted"}, // Should not actually delete
		MaxIterations:    5,
		TimeoutSeconds:   30,
	},
	{
		Name:             "Logical Problem Solving",
		Query:            "Sort the users in test_data.json by age descending, then show the top 2 names",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"Charlie", "Alice"}, // Charlie:35, Alice:30, Bob:25, Diana:28
		ShouldNotContain: []string{"I cannot"},
		MaxIterations:    10,
		TimeoutSeconds:   60,
	},

	// Ultra-Challenging Tests - designed to be extremely difficult even for large models
	{
		Name:             "Advanced Mathematical Reasoning",
		Query:            "Solve this system of equations and explain each step: 2x + 3y - z = 10, x - 2y + 4z = 8, 3x + y - 2z = 3. Then verify your solution by substitution and explain why this system has a unique solution.",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"x = 2", "y = 1", "z = -1"}, // Solution: x=2, y=1, z=-1
		ShouldNotContain: []string{"I cannot"},
		MaxIterations:    15,
		TimeoutSeconds:   120,
	},
	{
		Name:             "Complex Algorithm Implementation",
		Query:            "Implement a function that finds all prime numbers up to 1000 using the Sieve of Eratosthenes algorithm, then use it to find the 100th prime number. Explain the algorithm's time and space complexity.",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"541"}, // 100th prime is 541
		ShouldNotContain: []string{"I cannot"},
		MaxIterations:    15,
		TimeoutSeconds:   120,
	},
	{
		Name:             "Multi-Step Scientific Reasoning",
		Query:            "A rocket is launched vertically from Earth with an initial velocity of 8000 m/s. Ignoring air resistance, calculate: (a) the maximum height reached, (b) the time to reach maximum height, (c) the total time in flight, (d) the velocity just before impact. Show all work with proper physics equations and units.",
		ExpectedBehavior: "multi-step",
		ShouldContain:    []string{"3265306", "816", "1632"}, // Precise calculations with g=9.8
		ShouldNotContain: []string{"I cannot"},
		MaxIterations:    15,
		TimeoutSeconds:   120,
	},
	{
		Name:             "Advanced Code Architecture",
		Query:            "Design and implement a thread-safe LRU cache in Python with O(1) get and put operations. Include proper error handling, type hints, and comprehensive documentation. Explain your design choices and why this implementation is optimal.",
		ExpectedBehavior: "code",
		ShouldContain:    []string{"OrderedDict", "threading.Lock", "O(1)"}, // Key implementation elements
		ShouldNotContain: []string{"I cannot"},
		MaxIterations:    20,
		TimeoutSeconds:   180,
	},
}
