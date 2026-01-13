package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GetProjectRoot returns the absolute path to the project root directory
func getProjectRoot() string {
	cwd, _ := os.Getwd()
	// If we're in internal/llm, go up two levels
	if filepath.Base(cwd) == "llm" {
		return filepath.Dir(filepath.Dir(cwd))
	}
	return cwd
}

func TestBenchmarkDataFilesExist(t *testing.T) {
	root := getProjectRoot()
	tests := []struct {
		name     string
		filePath string
	}{
		{"sample.txt", filepath.Join(root, "internal/llm/sample.txt")},
		{"test_data.json", filepath.Join(root, "internal/llm/test_data.json")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := os.Stat(tt.filePath); os.IsNotExist(err) {
				t.Errorf("%s does not exist at path %s", tt.name, tt.filePath)
			}
		})
	}
}

func TestSampleTxtContent(t *testing.T) {
	root := getProjectRoot()
	filePath := filepath.Join(root, "internal/llm/sample.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read sample.txt: %v", err)
	}

	text := string(content)

	// Verify TOTAL_COUNT: 42 exists
	if !strings.Contains(text, "TOTAL_COUNT: 42") {
		t.Error("sample.txt should contain 'TOTAL_COUNT: 42'")
	}

	// Verify the word "Important" exists
	if !strings.Contains(text, "Important") {
		t.Error("sample.txt should contain 'Important'")
	}

	// Verify Line 3 and Line 4 exist (for the line counting test)
	line3Count := strings.Count(text, "Line 3:")
	line4Count := strings.Count(text, "Line 4:")
	if line3Count == 0 {
		t.Error("sample.txt should contain 'Line 3:'")
	}
	if line4Count == 0 {
		t.Error("sample.txt should contain 'Line 4:'")
	}

	// Count actual lines with "Line" in them
	lineCount := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "Line") {
			lineCount++
		}
	}
	if lineCount != 2 {
		t.Errorf("Expected 2 lines containing 'Line', got %d", lineCount)
	}
}

func TestTestDataJsonContent(t *testing.T) {
	root := getProjectRoot()
	filePath := filepath.Join(root, "internal/llm/test_data.json")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read test_data.json: %v", err)
	}

	text := string(content)

	// Verify users with role "engineer" - should be Alice and Charlie = 2
	engineerCount := strings.Count(text, `"role": "engineer"`)
	if engineerCount != 2 {
		t.Errorf("Expected 2 users with role 'engineer', got %d", engineerCount)
	}

	// Verify user with id 3 is Charlie
	if !strings.Contains(text, `"id": 3, "name": "Charlie"`) {
		t.Error("test_data.json should contain user with id=3 named Charlie")
	}

	// Verify all users exist
	for _, name := range []string{"Alice", "Bob", "Charlie", "Diana"} {
		if !strings.Contains(text, `"name": "`+name+`"`) {
			t.Errorf("test_data.json should contain user named %s", name)
		}
	}

	// Verify ages
	expectedAges := map[string]int{
		"Alice":   30,
		"Bob":     25,
		"Charlie": 35,
		"Diana":   28,
	}

	for name, expectedAge := range expectedAges {
		expected := `"name": "` + name + `", "age": ` + itoa(expectedAge)
		if !strings.Contains(text, expected) {
			t.Errorf("test_data.json should have %s with age %d", name, expectedAge)
		}
	}
}

func TestBenchmarkMathematicalExpectations(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "15 * 23 + 47 = 392",
			query:    "Calculate 15 * 23 + 47 and give me the result",
			expected: "392",
		},
		{
			name:     "Sum of squares 1-10 = 385",
			query:    "Use Python to find the sum of squares of numbers 1 through 10",
			expected: "385",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the mathematical expectation is correct
			switch tt.expected {
			case "392":
				// 15 * 23 + 47 = 345 + 47 = 392
				result := 15*23 + 47
				if result != 392 {
					t.Errorf("Math error: 15 * 23 + 47 = %d, expected 392", result)
				}
			case "385":
				// Sum of squares 1² + 2² + ... + 10² = 385
				sum := 0
				for i := 1; i <= 10; i++ {
					sum += i * i
				}
				if sum != 385 {
					t.Errorf("Math error: sum of squares 1-10 = %d, expected 385", sum)
				}
			}
		})
	}
}

func TestBenchmarkJSONAnalysisExpectations(t *testing.T) {
	// Test the JSON analysis expected values
	// Users: Alice(30), Bob(25), Charlie(35), Diana(28)
	// Average age: (30 + 25 + 35 + 28) / 4 = 118 / 4 = 29.5

	ages := []int{30, 25, 35, 28}
	sum := 0
	for _, age := range ages {
		sum += age
	}
	average := float64(sum) / float64(len(ages))

	if average != 29.5 {
		t.Errorf("Average age calculation error: (30+25+35+28)/4 = %f, expected 29.5", average)
	}

	// Verify rounding behavior (29.5 rounds to 30)
	rounded := int(average + 0.5)
	if rounded != 30 {
		t.Errorf("Rounding 29.5 should give 30, got %d", rounded)
	}
}

func TestBenchmarkEngineerCount(t *testing.T) {
	// Verify there are exactly 2 engineers
	roles := []string{"engineer", "designer", "engineer", "manager"}
	engineerCount := 0
	for _, role := range roles {
		if role == "engineer" {
			engineerCount++
		}
	}
	if engineerCount != 2 {
		t.Errorf("Expected 2 engineers, got %d", engineerCount)
	}
}

func TestBenchmarkFileExistsForAgentQuery(t *testing.T) {
	root := getProjectRoot()
	// Check that agent.go exists for the "Code File Analysis" test
	filePath := filepath.Join(root, "internal/llm/agent.go")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Also check the UI agent.go
		uiPath := filepath.Join(root, "internal/ui/agent.go")
		if _, err := os.Stat(uiPath); os.IsNotExist(err) {
			t.Error("Neither internal/llm/agent.go nor internal/ui/agent.go exists for Code File Analysis test")
		}
	}
}

func TestBenchmarkLineCountExpectations(t *testing.T) {
	root := getProjectRoot()
	filePath := filepath.Join(root, "internal/llm/sample.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read sample.txt: %v", err)
	}

	lines := strings.Split(string(content), "\n")

	// Count lines containing the word "Line"
	lineCount := 0
	for _, line := range lines {
		if strings.Contains(line, "Line") {
			lineCount++
		}
	}

	// The test expects "2" as the answer
	if lineCount != 2 {
		t.Errorf("sample.txt has %d lines containing 'Line', but test expects answer '2'", lineCount)
	}

	// Verify which lines have "Line"
	expectedLines := []string{"Line 3:", "Line 4:"}
	for _, expected := range expectedLines {
		found := false
		for _, line := range lines {
			if strings.Contains(line, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("sample.txt should contain '%s'", expected)
		}
	}
}

func TestBenchmarkCapitalFrance(t *testing.T) {
	// This is a general knowledge test, no file needed
	// Paris is the capital of France
	expected := "Paris"

	// Verify the expectation is correct
	if expected != "Paris" {
		t.Errorf("Capital of France is '%s', expected 'Paris'", expected)
	}
}

func TestBenchmarkNonexistentFileError(t *testing.T) {
	// The test "Error Handling" uses nonexistent_file_xyz_999.txt
	// which should NOT exist
	filePath := "nonexistent_file_xyz_999.txt"
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("File %s should NOT exist for Error Handling test, but it does", filePath)
	}
}

// Helper function for converting int to string without importing strconv
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	negative := i < 0
	if negative {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if negative {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

func TestBenchmarkSuiteCompleteness(t *testing.T) {
	root := getProjectRoot()
	// Verify all referenced files exist
	referencedFiles := map[string]string{
		"sample.txt":      filepath.Join(root, "internal/llm/sample.txt"),
		"test_data.json":  filepath.Join(root, "internal/llm/test_data.json"),
		"agent.go":        filepath.Join(root, "internal/llm/agent.go"),
		"nonexistent.txt": "nonexistent_file_xyz_999.txt", // Should NOT exist
	}

	for name, path := range referencedFiles {
		t.Run(name, func(t *testing.T) {
			_, err := os.Stat(path)
			if name == "nonexistent.txt" {
				// This file should NOT exist
				if err == nil {
					t.Error("nonexistent_file_xyz_999.txt should not exist")
				}
			} else {
				// Other files SHOULD exist
				if os.IsNotExist(err) {
					t.Errorf("Referenced file %s does not exist at %s", name, path)
				}
			}
		})
	}
}

func TestCountGoFilesInLlmDirectory(t *testing.T) {
	root := getProjectRoot()
	dir := filepath.Join(root, "internal/llm")

	var goCount int
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			goCount++
		}
		return nil
	})

	// The test expects a number greater than 0 and not "0"
	if goCount == 0 {
		t.Error("There should be at least one .go file in internal/llm directory")
	}

	t.Logf("Found %d .go files in internal/llm directory", goCount)
}
