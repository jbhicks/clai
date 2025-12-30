package llm

import (
	"testing"
)

// TestRealWorldScenarios tests actual patterns seen in model outputs
func TestRealWorldScenarios(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantBlocks   int
		wantLanguage string
		wantCode     string
	}{
		{
			name: "Hermes-3 incomplete simplified tag (actual failure case)",
			content: `I'll help you read the test_data.json file and count how many users have the role 'engineer'.

First, let me check if the file exists and then read it to count the engineers.
<code python
import json

with open('test_data.json', 'r') as f:
    data = json.load(f)

engineers = [user for user in data['users'] if user['role'] == 'engineer']
print(len(engineers))`,
			wantBlocks:   1,
			wantLanguage: "python",
			wantCode: `import json

with open('test_data.json', 'r') as f:
    data = json.load(f)

engineers = [user for user in data['users'] if user['role'] == 'engineer']
print(len(engineers))`,
		},
		{
			name: "Model outputs simplified format",
			content: `Let me calculate that for you.

<code python>print(42 + 58)</code>

The answer is 100.`,
			wantBlocks:   1,
			wantLanguage: "python",
			wantCode:     "print(42 + 58)",
		},
		{
			name: "Model outputs markdown format",
			content: `Here's how to do it:

` + "```python\nresult = 42 + 58\nprint(result)\n```" + `

That gives us 100.`,
			wantBlocks:   1,
			wantLanguage: "python",
			wantCode:     "result = 42 + 58\nprint(result)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := ParseCodeBlocks(tt.content)
			
			if len(blocks) != tt.wantBlocks {
				t.Fatalf("expected %d blocks, got %d", tt.wantBlocks, len(blocks))
			}
			
			if tt.wantBlocks > 0 {
				if blocks[0].Language != tt.wantLanguage {
					t.Errorf("language: expected %q, got %q", tt.wantLanguage, blocks[0].Language)
				}
				if blocks[0].Code != tt.wantCode {
					t.Errorf("code mismatch:\nexpected:\n%s\n\ngot:\n%s", tt.wantCode, blocks[0].Code)
				}
			}
		})
	}
}
