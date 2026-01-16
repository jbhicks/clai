package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// TestTrailingSpaceIssue tests if Glamour's trailing spaces cause background issues
func TestTrailingSpaceIssue(t *testing.T) {
	content := "Short line"
	maxWidth := 80

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(maxWidth),
	)
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		t.Fatalf("Failed to render: %v", err)
	}

	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		t.Logf("Line %d: %q (width: %d)", i, line, len(line))
		if strings.HasSuffix(line, " ") {
			t.Logf("  → Has trailing spaces!")
			// Count trailing spaces
			trimmed := strings.TrimRight(line, " ")
			trailingCount := len(line) - len(trimmed)
			t.Logf("  → %d trailing spaces", trailingCount)
		}
	}

	// Now wrap it in a border with background
	ourBackground := lipgloss.Color("#282a36")
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Background(ourBackground).
		BorderBackground(ourBackground).
		Padding(0, 1)

	bubble := style.Render(strings.TrimSpace(rendered))
	t.Logf("\nBubble:\n%s", bubble)

	// The issue: if Glamour adds trailing spaces to pad to maxWidth,
	// those spaces don't have our background color applied.
	// They show as transparent/terminal-default background.
}
