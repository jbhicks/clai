package ui

import (
	"clai/internal/llm"
	"strings"
	"testing"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/charmbracelet/lipgloss"
)

// TestAssistantMessageRendering tests the complete flow of rendering an assistant message
// to ensure there are no background color mismatches
func TestAssistantMessageRendering(t *testing.T) {
	// Setup theme
	theme := glitter.NewUI(DraculaTheme)
	themeStyles := GetThemeStyles(theme)

	// Simulate assistant message with markdown
	content := `Here is my response with **bold** text and code:

` + "```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```" + `

And more text after.`

	// Simulate what chat.go does
	chatWidth := 100
	maxBubbleWidth := int(float64(chatWidth) * 0.95) // 95
	assistantInnerWidth := maxBubbleWidth - themeStyles.AssistantMessage.GetHorizontalFrameSize()

	t.Logf("chatWidth: %d", chatWidth)
	t.Logf("maxBubbleWidth: %d", maxBubbleWidth)
	t.Logf("assistantInnerWidth: %d", assistantInnerWidth)
	t.Logf("AssistantMessage frame size: %d", themeStyles.AssistantMessage.GetHorizontalFrameSize())

	// Render with syntax highlighting (same as chat.go line 117)
	contentWithHighlighting := llm.RenderWithSyntaxHighlighting(
		content,
		assistantInnerWidth,
		themeStyles.CodeBlockBadge,
		themeStyles.CodeBlockContainer,
	)

	// Check for issues
	lines := strings.Split(contentWithHighlighting, "\n")
	hasTrailingSpaces := false
	hasBackgroundCodes := false

	for i, line := range lines {
		if strings.HasSuffix(line, " ") {
			hasTrailingSpaces = true
			t.Logf("Line %d has trailing spaces: %q", i, line)
		}
		if strings.Contains(line, "\x1b[48;") {
			hasBackgroundCodes = true
			t.Logf("Line %d has background codes: %q", i, line)
		}
	}

	if hasTrailingSpaces {
		t.Error("FAIL: Content has trailing spaces - will cause background color mismatch")
	}
	if hasBackgroundCodes {
		t.Error("FAIL: Content has ANSI background codes - will conflict with our theme background")
	}

	// Render the bubble (same as chat.go line 119)
	bubble := themeStyles.AssistantMessage.Render(contentWithHighlighting)

	t.Logf("\nRendered bubble:\n%s", bubble)

	// Verify the bubble doesn't overflow the expected width
	// Account for AssistantMessage padding (1 on each side)
	maxLineWidth := maxBubbleWidth + themeStyles.AssistantMessage.GetHorizontalFrameSize() + 5
	bubbleLines := strings.Split(bubble, "\n")
	for i, line := range bubbleLines {
		width := lipgloss.Width(line)
		if width > maxLineWidth {
			t.Errorf("Line %d exceeds max width: %d > %d", i, width, maxLineWidth)
		}
	}

	if !hasTrailingSpaces && !hasBackgroundCodes {
		t.Log("SUCCESS: Content is clean - no trailing spaces or background codes")
	}
}
