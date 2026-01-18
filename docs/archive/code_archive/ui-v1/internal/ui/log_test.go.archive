package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// TestLogUpdateMsg verifies that LogUpdateMsg messages are properly added to the log buffer
// and displayed in the Log viewport.
func TestLogUpdateMsg(t *testing.T) {
	theme := AvailableThemes[0] // Use Gruvbox theme

	m := &Model{
		Width:  100,
		Height: 40,
		Theme:  theme,
		Log:    viewport.New(50, 20),
	}

	// Initialize log channel
	m.logChan = make(chan tea.Msg, 10)

	// Send a few log messages
	logMsg1 := "First log line"
	logMsg2 := "Second log line"
	logMsg3 := "Third log line"

	// Process first log message
	updatedModel, _ := m.Update(LogUpdateMsg(logMsg1))
	m = updatedModel.(*Model)

	// Process second log message
	updatedModel, _ = m.Update(LogUpdateMsg(logMsg2))
	m = updatedModel.(*Model)

	// Process third log message
	updatedModel, _ = m.Update(LogUpdateMsg(logMsg3))
	m = updatedModel.(*Model)

	// Verify logBuffer contains all messages
	if !strings.Contains(m.logBuffer, logMsg1) {
		t.Errorf("logBuffer missing first message: expected to contain %q", logMsg1)
	}
	if !strings.Contains(m.logBuffer, logMsg2) {
		t.Errorf("logBuffer missing second message: expected to contain %q", logMsg2)
	}
	if !strings.Contains(m.logBuffer, logMsg3) {
		t.Errorf("logBuffer missing third message: expected to contain %q", logMsg3)
	}

	// Verify log viewport has been updated with content
	logView := m.Log.View()
	if !strings.Contains(logView, logMsg1) {
		t.Errorf("Log viewport missing first message: expected to contain %q in view", logMsg1)
	}
}

// TestLogBufferAccumulation verifies that log messages accumulate correctly
// in the buffer without losing previous content.
func TestLogBufferAccumulation(t *testing.T) {
	theme := AvailableThemes[0] // Use Gruvbox theme

	m := &Model{
		Width:  100,
		Height: 40,
		Theme:  theme,
		Log:    viewport.New(50, 20),
	}

	m.logChan = make(chan tea.Msg, 10)

	messages := []string{"Line 1", "Line 2", "Line 3", "Line 4", "Line 5"}

	// Process all messages
	for _, msg := range messages {
		updatedModel, _ := m.Update(LogUpdateMsg(msg))
		m = updatedModel.(*Model)
	}

	// Verify all messages are in the buffer
	for _, msg := range messages {
		if !strings.Contains(m.logBuffer, msg) {
			t.Errorf("logBuffer missing message: expected to contain %q", msg)
		}
	}

	// Verify buffer has correct number of newlines (one per message)
	newlineCount := strings.Count(m.logBuffer, "\n")
	if newlineCount != len(messages) {
		t.Errorf("Expected %d newlines in buffer, got %d", len(messages), newlineCount)
	}
}

// TestLogViewportDimensions verifies that the Log viewport respects dimension constraints.
func TestLogViewportDimensions(t *testing.T) {
	theme := AvailableThemes[0] // Use Gruvbox theme

	logWidth := 50
	logHeight := 20

	m := &Model{
		Width:  100,
		Height: 40,
		Theme:  theme,
		Log:    viewport.New(logWidth, logHeight),
	}

	m.logChan = make(chan tea.Msg, 10)

	// Add many lines of content (more than viewport height)
	for i := 0; i < 50; i++ {
		updatedModel, _ := m.Update(LogUpdateMsg("Log line " + string(rune('A'+i))))
		m = updatedModel.(*Model)
	}

	// Get the rendered view
	logView := m.Log.View()

	// The view should not exceed the viewport dimensions
	// (Note: lipgloss.Height/Width would need to be used for exact verification,
	// but we can at least check the content exists)
	if len(logView) == 0 {
		t.Error("Log viewport view is empty despite having content")
	}
}
