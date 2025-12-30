package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// TestGlamourBackgroundIssue recreates the issue where Glamour's background colors
// conflict with our custom backgrounds when trying to create consistent message bubbles
func TestGlamourBackgroundIssue(t *testing.T) {
	// Simulate what we're doing in chat.go for assistant messages
	content := `Here is some text before the code.

` + "```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```" + `

And some text after the code.`
	maxWidth := 80
	ourBackground := lipgloss.Color("#282a36") // Dracula primary background

	// Step 1: Render markdown with Glamour (what RenderWithSyntaxHighlighting does)
	// First test with "dark" style (used for code blocks)
	t.Log("--- Testing with glamour dark style (used for code blocks) ---")
	rendererDark, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(maxWidth),
	)
	if err != nil {
		t.Fatalf("Failed to create dark renderer: %v", err)
	}

	renderedDark, err := rendererDark.Render(content)
	if err != nil {
		t.Fatalf("Failed to render with dark: %v", err)
	}

	t.Logf("Rendered with dark style:\n%s", renderedDark)
	// Only log first 500 chars of ANSI to avoid spam
	if len(renderedDark) > 500 {
		t.Logf("Rendered with dark (first 500 chars with visible ANSI):\n%q", renderedDark[:500])
	} else {
		t.Logf("Rendered with dark (with visible ANSI):\n%q", renderedDark)
	}

	hasBackgroundCodesDark := strings.Contains(renderedDark, "\x1b[48;")
	t.Logf("Dark style output contains background ANSI codes: %v", hasBackgroundCodesDark)

	// Now test with auto style
	t.Log("\n--- Testing with glamour auto style (used for main content) ---")
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(maxWidth),
	)
	if err != nil {
		t.Fatalf("Failed to create glamour renderer: %v", err)
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		t.Fatalf("Failed to render markdown: %v", err)
	}

	t.Logf("Rendered markdown:\n%s", rendered)
	t.Logf("Rendered markdown (with visible ANSI):\n%q", rendered)

	// Step 2: Try to wrap it with our background (what chat.go does)
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#50fa7b")).
		Background(ourBackground).
		Foreground(lipgloss.Color("#f8f8f2")).
		Padding(0, 1)

	bubble := style.Render(strings.TrimSpace(rendered))

	t.Logf("Bubble:\n%s", bubble)
	t.Logf("Bubble (with visible ANSI):\n%q", bubble)

	// Step 3: Check if Glamour embedded background codes
	hasBackgroundCodes := strings.Contains(rendered, "\x1b[48;")
	t.Logf("Glamour output contains background ANSI codes (ESC[48;...m): %v", hasBackgroundCodes)

	if hasBackgroundCodes {
		t.Logf("PROBLEM: Glamour is adding its own background colors")
		t.Logf("When we wrap this with lipgloss background, the two backgrounds conflict")
		t.Logf("This creates the 'darker stripe' issue on the right side")
	}

	// Step 4: Try the "simpler" approach - use glamour with no styling
	t.Log("\n--- Testing with glamour.WithStylePath(\"notty\") ---")
	rendererNoStyle, err := glamour.NewTermRenderer(
		glamour.WithStylePath("notty"),
		glamour.WithWordWrap(maxWidth),
	)
	if err != nil {
		t.Fatalf("Failed to create no-style renderer: %v", err)
	}

	renderedNoStyle, err := rendererNoStyle.Render(content)
	if err != nil {
		t.Fatalf("Failed to render with notty: %v", err)
	}

	t.Logf("Rendered with notty:\n%s", renderedNoStyle)
	t.Logf("Rendered with notty (with visible ANSI):\n%q", renderedNoStyle)

	hasBackgroundCodesNoStyle := strings.Contains(renderedNoStyle, "\x1b[48;")
	t.Logf("No-style output contains background ANSI codes: %v", hasBackgroundCodesNoStyle)

	// Step 5: Maybe we should just not use Glamour for non-code content?
	t.Log("\n--- Testing without Glamour (plain text with manual bold) ---")
	// Simple approach: just add ANSI bold codes manually
	plainWithBold := strings.ReplaceAll(content, "**bold**", "\x1b[1mbold\x1b[0m")
	bubblePlain := style.Render(plainWithBold)

	t.Logf("Plain bubble:\n%s", bubblePlain)
	t.Logf("Plain bubble (with visible ANSI):\n%q", bubblePlain)
}

// TestLipglossBackgroundFill tests whether lipgloss properly fills backgrounds
// when content already has ANSI codes
func TestLipglossBackgroundFill(t *testing.T) {
	// Test if lipgloss background fills properly when content has existing ANSI codes
	ourBg := lipgloss.Color("#282a36")

	// Content with foreground color only
	contentFg := "\x1b[32mGreen text\x1b[0m"
	style := lipgloss.NewStyle().
		Width(40).
		Background(ourBg)

	rendered := style.Render(contentFg)
	t.Logf("With FG codes: %q", rendered)
	t.Logf("Width: %d", lipgloss.Width(rendered))

	// Content with background color
	contentBg := "\x1b[48;5;234mText with BG\x1b[0m"
	rendered2 := style.Render(contentBg)
	t.Logf("With BG codes: %q", rendered2)
	t.Logf("Width: %d", lipgloss.Width(rendered2))

	// Check if our background color is present
	hasBg := strings.Contains(rendered, "48;")
	t.Logf("Rendered output has background ANSI codes: %v", hasBg)

	// The question: does lipgloss override existing backgrounds or just fill empty space?
	if strings.Contains(contentBg, "\x1b[48;") {
		t.Log("INSIGHT: Content already has background codes")
		if strings.Contains(rendered2, "\x1b[48;5;234m") {
			t.Log("Result: Original background codes are PRESERVED")
			t.Log("This means lipgloss .Background() doesn't override - it just fills empty space")
		}
	}
}
