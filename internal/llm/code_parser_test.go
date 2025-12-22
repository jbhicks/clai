package llm

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestParseCodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "single bash block",
			content:  `Here's some text <code language="bash">ls -la</code> more text`,
			expected: 1,
		},
		{
			name:     "multiple blocks",
			content:  `<code language="bash">ls</code> and <code language="python">print("hello")</code>`,
			expected: 2,
		},
		{
			name:     "no code blocks",
			content:  "Just plain text",
			expected: 0,
		},
		{
			name: "multiline code",
			content: `<code language="python">
def hello():
    print("world")
</code>`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := ParseCodeBlocks(tt.content)
			if len(blocks) != tt.expected {
				t.Errorf("expected %d blocks, got %d", tt.expected, len(blocks))
			}
		})
	}
}

func TestParseCodeBlocksDetailed(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantLanguage string
		wantCode     string
	}{
		{
			name:         "bash with quotes",
			content:      `<code language="bash">echo "hello world"</code>`,
			wantLanguage: "bash",
			wantCode:     `echo "hello world"`,
		},
		{
			name:         "python with special chars",
			content:      `<code language="python">print('hello\nworld')</code>`,
			wantLanguage: "python",
			wantCode:     `print('hello\nworld')`,
		},
		{
			name:         "javascript with backticks",
			content:      "<code language=\"javascript\">console.log(`template ${var}`)</code>",
			wantLanguage: "javascript",
			wantCode:     "console.log(`template ${var}`)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := ParseCodeBlocks(tt.content)
			if len(blocks) != 1 {
				t.Fatalf("expected 1 block, got %d", len(blocks))
			}
			if blocks[0].Language != tt.wantLanguage {
				t.Errorf("language: expected %q, got %q", tt.wantLanguage, blocks[0].Language)
			}
			if blocks[0].Code != tt.wantCode {
				t.Errorf("code: expected %q, got %q", tt.wantCode, blocks[0].Code)
			}
		})
	}
}

func TestParseCodeBlocksEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "incomplete opening tag",
			content:  `<code language="bash">echo hello`,
			expected: 1, // Now matches incomplete blocks
		},
		{
			name:     "incomplete closing tag",
			content:  `<code language="bash">echo hello</cod`,
			expected: 1, // Now matches incomplete blocks
		},
		{
			name:     "mismatched quotes in language",
			content:  `<code language='bash">echo hello</code>`,
			expected: 0,
		},
		{
			name:     "empty code block",
			content:  `<code language="bash"></code>`,
			expected: 1, // Empty block is still a valid match
		},
		{
			name:     "whitespace only code",
			content:  `<code language="bash">   </code>`,
			expected: 1,
		},
		{
			name:     "nested angle brackets in code",
			content:  `<code language="bash">if [ "$x" -lt "$y" ]; then echo "less"; fi</code>`,
			expected: 1,
		},
		{
			name:     "multiple languages mixed",
			content:  `<code language="bash">ls</code> text <code language="python">print()</code> more <code language="javascript">console.log()</code>`,
			expected: 3,
		},
		{
			name:     "unicode in code",
			content:  `<code language="python">print("你好世界")</code>`,
			expected: 1,
		},
		{
			name:     "special characters in code",
			content:  `<code language="bash">grep -E '^\$[0-9]+' file.txt</code>`,
			expected: 1,
		},
		{
			name:     "newlines and tabs",
			content:  "<code language=\"python\">def func():\n\tprint(\"test\")\n\treturn True</code>",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := ParseCodeBlocks(tt.content)
			if len(blocks) != tt.expected {
				t.Errorf("expected %d blocks, got %d", tt.expected, len(blocks))
			}
		})
	}
}

func TestParseCodeBlocksLargeContent(t *testing.T) {
	largeCode := strings.Repeat("echo 'line'\n", 10000)
	content := `<code language="bash">` + largeCode + `</code>`

	blocks := ParseCodeBlocks(content)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	if len(blocks[0].Code) != len(strings.TrimSpace(largeCode)) {
		t.Errorf("code length mismatch: expected %d, got %d", len(strings.TrimSpace(largeCode)), len(blocks[0].Code))
	}
}

func TestStripCodeTags(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "remove single tag",
			content:  `text <code language="bash">ls</code> more`,
			expected: "text  more",
		},
		{
			name:     "remove multiple tags",
			content:  `<code language="bash">ls</code> and <code language="python">print()</code>`,
			expected: "and",
		},
		{
			name:     "no tags",
			content:  "plain text",
			expected: "plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripCodeTags(tt.content)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestStripCodeTagsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "incomplete tag at end",
			content:  `text <code language="bash">echo hello`,
			expected: "text",
		},
		{
			name:     "nested tags in text",
			content:  `text <code language="bash">ls</code> <code language="python">print()</code> end`,
			expected: "text   end",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
		{
			name:     "only code tags",
			content:  `<code language="bash">echo hello</code>`,
			expected: "",
		},
		{
			name:     "text before and after",
			content:  `start <code language="bash">ls</code> middle <code language="python">print()</code> end`,
			expected: "start  middle  end",
		},
		{
			name:     "unicode text preserved",
			content:  `你好 <code language="bash">ls</code> 世界`,
			expected: "你好  世界",
		},
		{
			name:     "multiline text preserved",
			content:  "line1\n<code language=\"bash\">ls</code>\nline2",
			expected: "line1\n\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripCodeTags(tt.content)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCodeBlockTrimming(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantCode string
	}{
		{
			name:     "leading whitespace",
			content:  `<code language="bash">   ls -la</code>`,
			wantCode: "ls -la",
		},
		{
			name:     "trailing whitespace",
			content:  `<code language="bash">ls -la   </code>`,
			wantCode: "ls -la",
		},
		{
			name:     "leading and trailing newlines",
			content:  "<code language=\"bash\">\n\nls -la\n\n</code>",
			wantCode: "ls -la",
		},
		{
			name:     "preserve internal whitespace",
			content:  `<code language="bash">ls    -la</code>`,
			wantCode: "ls    -la",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := ParseCodeBlocks(tt.content)
			if len(blocks) != 1 {
				t.Fatalf("expected 1 block, got %d", len(blocks))
			}
			if blocks[0].Code != tt.wantCode {
				t.Errorf("code: expected %q, got %q", tt.wantCode, blocks[0].Code)
			}
		})
	}
}

func TestStripTextBasedFunctionCalls(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "remove function call at start",
			content:  `<function=web_searchHello, I can help you`,
			expected: "Hello, I can help you",
		},
		{
			name:     "remove function call in middle",
			content:  `Hello! How can I assist you today?<function=web_searchI don't have access`,
			expected: "Hello! How can I assist you today?I don't have access",
		},
		{
			name:     "remove multiple function calls",
			content:  `<function=calculator<function=web_searchSome text`,
			expected: "Some text",
		},
		{
			name:     "no function calls",
			content:  "Just plain text",
			expected: "Just plain text",
		},
		{
			name:     "function call with closing bracket",
			content:  `<function=web_search>Result here`,
			expected: "Result here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripTextBasedFunctionCalls(tt.content)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRenderWithSyntaxHighlighting(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		maxWidth int
	}{
		{
			name:     "bash code block",
			content:  `Here is some code: <code language="bash">ls -la</code> end`,
			maxWidth: 80,
		},
		{
			name:     "python code block",
			content:  `<code language="python">print("hello world")</code>`,
			maxWidth: 80,
		},
		{
			name:     "no code blocks",
			content:  "Just plain text without any code",
			maxWidth: 80,
		},
		{
			name:     "multiple code blocks",
			content:  `First: <code language="bash">echo hi</code> Second: <code language="python">print()</code>`,
			maxWidth: 80,
		},
	}

	badgeStyle := lipgloss.NewStyle()
	containerStyle := lipgloss.NewStyle()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderWithSyntaxHighlighting(tt.content, tt.maxWidth, badgeStyle, containerStyle)

			if result == "" {
				t.Error("result should not be empty")
			}

			if len(ParseCodeBlocks(tt.content)) == 0 {
				// Content is now wrapped to maxWidth, so check that wrapping was applied
				if lipgloss.Width(result) > tt.maxWidth {
					t.Errorf("content without code blocks should be wrapped to maxWidth=%d, but width=%d", tt.maxWidth, lipgloss.Width(result))
				}
			} else {
				if strings.Contains(result, "<code language=") {
					t.Error("result should not contain XML code tags")
				}
			}
		})
	}
}
