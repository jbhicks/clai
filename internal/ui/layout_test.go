package ui

import (
	"clai/internal/llm"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// TestJoinVerticalEmptyString verifies that we don't pass empty strings to
// lipgloss.JoinVertical since they count as 1 row and cause layout overflow.
func TestJoinVerticalEmptyString(t *testing.T) {
	// This is the bug we fixed: empty strings in JoinVertical count as 1 row
	a := "line1"
	b := ""
	c := "line2"

	joined := lipgloss.JoinVertical(lipgloss.Left, a, b, c)
	height := lipgloss.Height(joined)

	// Empty string counts as 1 row, so height is 3 not 2
	if height != 3 {
		t.Errorf("Expected height 3 with empty string, got %d", height)
	}

	// Correct approach: only join non-empty strings
	joinedCorrect := lipgloss.JoinVertical(lipgloss.Left, a, c)
	heightCorrect := lipgloss.Height(joinedCorrect)

	if heightCorrect != 2 {
		t.Errorf("Expected height 2 without empty string, got %d", heightCorrect)
	}
}

// TestChatModelLayoutDimensions verifies that ChatModel.View() respects height constraints.
func TestChatModelLayoutDimensions(t *testing.T) {
	theme := AvailableThemes[0] // Use Gruvbox theme

	ti := textinput.New()
	ti.Focus()

	chat := ChatModel{
		Width:     80,
		Height:    20,
		Theme:     theme,
		Messages:  []llm.Message{{Role: "assistant", Content: "test"}},
		TextInput: ti,
		Viewport:  viewport.New(80, 20),
	}
	_ = chat

	view := chat.View()
	actualHeight := lipgloss.Height(view)

	// The view should not exceed the allocated height
	if actualHeight > chat.Height {
		t.Errorf("ChatModel.View() exceeded height constraint: got %d, max %d", actualHeight, chat.Height)
	}
}

// TestMainLayoutDimensions verifies that the main View() doesn't exceed terminal size.
func TestMainLayoutDimensions(t *testing.T) {
	termWidth := 100
	termHeight := 40

	theme := AvailableThemes[0] // Use Gruvbox theme

	ti := textinput.New()
	ti.Focus()

	m := &Model{
		Width:       termWidth,
		Height:      termHeight,
		Theme:       theme,
		AgentStatus: NewAgentStatusView(theme),
	}

	// Simulate what handleWindowSizeMsg does:
	// contentHeight = termHeight - statusBar(1)
	// Component.Height = contentHeight - MainPane frame size
	themeStyles := GetThemeStyles(theme)
	contentHeight := termHeight - 1
	componentHeight := contentHeight - themeStyles.MainPane.GetVerticalFrameSize()

	chatPaneWidth := int(float64(termWidth) * 0.8)
	chatInnerWidth := chatPaneWidth - themeStyles.MainPane.GetHorizontalFrameSize()

	m.Chat.Width = chatInnerWidth
	m.Chat.Height = componentHeight
	m.Chat.Theme = theme
	m.Chat.TextInput = ti
	m.Chat.Viewport = viewport.New(chatInnerWidth, componentHeight)

	m.Log = viewport.New(termWidth, componentHeight)

	view := m.View()
	actualHeight := lipgloss.Height(view)

	// The final layout should not exceed terminal height
	if actualHeight > termHeight {
		t.Errorf("Model.View() exceeded terminal height: got %d, max %d", actualHeight, termHeight)
	}
}
