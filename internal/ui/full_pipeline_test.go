package ui

import (
	"clai/internal/llm"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestFullAssistantMessagePipeline replicates EXACTLY what happens in chat.go lines 114-121
func TestFullAssistantMessagePipeline(t *testing.T) {
	// Setup: same as chat.go
	chatWidth := 85
	maxBubbleWidth := int(float64(chatWidth) * 0.95) // 80
	theme := GetAvailableThemes()[0]
	themeStyles := GetThemeStyles(theme)

	assistantInnerWidth := maxBubbleWidth - themeStyles.AssistantMessage.GetHorizontalFrameSize()
	t.Logf("chatWidth=%d, maxBubbleWidth=%d, frameSize=%d, assistantInnerWidth=%d",
		chatWidth, maxBubbleWidth,
		themeStyles.AssistantMessage.GetHorizontalFrameSize(),
		assistantInnerWidth)

	// Test content: simple markdown with code block
	content := `Here is some text.

<code language="bash">
echo "hello"
</code>

And more text.`

	// Step 1: Strip function calls (like chat.go:116)
	cleanedContent := llm.StripTextBasedFunctionCalls(content)

	// Step 2: Render with syntax highlighting (like chat.go:117)
	contentWithHighlighting := llm.RenderWithSyntaxHighlighting(
		cleanedContent,
		assistantInnerWidth,
		themeStyles.CodeBlockBadge,
		themeStyles.CodeBlockContainer,
	)

	// Step 3: Wrap in AssistantMessage style (like chat.go:119)
	bubble := themeStyles.AssistantMessage.Render(contentWithHighlighting)

	// Step 4: Pad lines to full chat width (like chat.go:121)
	bg := lipgloss.Color(theme.Theme.Primary.Background)
	lines := strings.Split(bubble, "\n")
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < chatWidth {
			padStyle := lipgloss.NewStyle().
				Width(chatWidth).
				Background(bg)
			lines[i] = padStyle.Render(line)
		}
	}
	rendered := strings.Join(lines, "\n")

	// Output for visual inspection
	t.Logf("\n=== FINAL RENDERED OUTPUT ===\n%s\n", rendered)

	// Check for background codes
	hasBackground := strings.Contains(rendered, "\x1b[48;")
	t.Logf("Contains background ANSI codes: %v", hasBackground)

	// Show raw ANSI for first 3 lines
	outputLines := strings.Split(rendered, "\n")
	maxLines := 3
	if len(outputLines) < maxLines {
		maxLines = len(outputLines)
	}
	for i := 0; i < maxLines; i++ {
		t.Logf("Line %d (raw): %q", i, outputLines[i])
		t.Logf("Line %d (width): %d", i, lipgloss.Width(outputLines[i]))
	}
}
