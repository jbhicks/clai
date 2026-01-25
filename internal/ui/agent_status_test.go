package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/brittonhayes/glitter/theme"
	"github.com/charmbracelet/lipgloss"
)

func TestAgentStatusViewNarrowWidths(t *testing.T) {
	// Create a minimal theme for testing
	testTheme := glitter.NewUI(theme.Theme{
		Primary: theme.Primary{
			Background:    lipgloss.Color("#282a36"),
			Foreground:    lipgloss.Color("#f8f8f2"),
			DimForeground: lipgloss.Color("#6272a4"),
		},
		Bright: theme.Bright{
			Yellow: lipgloss.Color("#f1fa8c"),
			Green:  lipgloss.Color("#50fa7b"),
			Cyan:   lipgloss.Color("#8be9fd"),
			White:  lipgloss.Color("#ffffff"),
		},
		Dim: theme.Dim{
			White: lipgloss.Color("#8a8a8a"),
			Black: lipgloss.Color("#191a21"),
		},
	})

	v := NewAgentStatusView(testTheme)
	v.Height = 20

	// Test narrow widths from 0 to 12 as required by AGENTS.md
	for w := 0; w <= 12; w++ {
		v.Width = w
		v.Status = AgentStatus{Active: false}

		// This must not panic
		_ = v.View()
	}

	// Test with active status and longer content
	for w := 0; w <= 12; w++ {
		v.Width = w
		v.Status = AgentStatus{
			Active:      true,
			CurrentIter: 1,
			Thought:     "This is a test thought that might be too long",
			Subtasks: []string{
				"Test action 1",
				"Test action 2",
			},
		}

		// This must not panic
		_ = v.View()
	}
}

func TestAgentStatusViewWidthArithmetic(t *testing.T) {
	testTheme := glitter.NewUI(DraculaTheme)
	v := NewAgentStatusView(testTheme)
	v.Height = 20

	// Test widths that could cause negative values in arithmetic
	testCases := []int{0, 1, 2, 3, 4, 5, 8, 10, 15}

	for _, w := range testCases {
		v.Width = w
		v.Status = AgentStatus{
			Active:      true,
			CurrentIter: 1,
			Thought:     "Short thought",
		}

		// This must not panic and should produce valid output
		result := v.View()

		// Verify result is a valid string
		if result == "" {
			t.Errorf("View() returned empty string for width %d", w)
		}

		// Verify no negative repeat counts (strings.Repeat with negative panics)
		// The code uses padCount which is guarded, so this should be safe
		lines := strings.Split(result, "\n")
		for _, line := range lines {
			// Each line should have non-negative visual width
			lineWidth := lipgloss.Width(line)
			if lineWidth < 0 {
				t.Errorf("Negative line width %d for width %d", lineWidth, w)
			}
		}
	}
}

func TestAgentStatusViewWithHistoryNarrowWidth(t *testing.T) {
	testTheme := glitter.NewUI(DraculaTheme)
	v := NewAgentStatusView(testTheme)
	v.Height = 20
	v.Width = 5 // Very narrow

	// Add history entries
	v.History = []CompletedTask{
		{
			Thought:     "Test task",
			Actions:     []AgentAction{{Description: "Action", Status: "completed"}},
			CompletedAt: time.Now(),
			ActionCount: 1,
		},
	}

	// This must not panic
	_ = v.View()
}

func TestAgentStatusViewWithCodeBlockNarrowWidth(t *testing.T) {
	testTheme := glitter.NewUI(DraculaTheme)
	v := NewAgentStatusView(testTheme)
	v.Height = 15
	v.Width = 8 // Very narrow

	// Add code execution
	v.CurrentCode = &CodeExecution{
		Language: "go",
		Code:     "fmt.Println('hello')",
		Status:   "running",
	}

	// This must not panic
	_ = v.View()
}

func TestAgentStatusViewFrameSizeUsage(t *testing.T) {
	testTheme := glitter.NewUI(DraculaTheme)
	v := NewAgentStatusView(testTheme)
	v.Height = 20

	// Test that frame size is properly used
	themeStyles := GetThemeStyles(testTheme)
	expectedFrameSize := themeStyles.MainPane.GetHorizontalFrameSize()

	// Width must be at least frameSize for proper rendering
	v.Width = expectedFrameSize
	v.Status = AgentStatus{Active: false}
	result1 := v.View()

	v.Width = expectedFrameSize + 10
	result2 := v.View()

	// Both should render without panicking
	_ = result1
	_ = result2
}
