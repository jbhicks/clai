package ui

import (
	"fmt"
	"strings"
	"time"

	"clai/internal/log"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogModel is a Bubble Tea model for viewing logs
type LogModel struct {
	Entries  []log.LogEntry
	Viewport viewport.Model
	Selected int
	Styles   ThemeStyles
	Width    int
	Height   int
}

func NewLogModel(entries []log.LogEntry, styles ThemeStyles) *LogModel {
	vm := viewport.New(80, 20)
	return &LogModel{Entries: entries, Viewport: vm, Selected: 0, Styles: styles}
}

func (m *LogModel) Init() tea.Cmd {
	return nil
}

func (m *LogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		innerW := safeClamp(m.Width, m.Styles.MainPane.GetHorizontalFrameSize())
		m.Viewport.Width = innerW
		m.Viewport.Height = safeClamp(m.Height-3, m.Styles.MainPane.GetVerticalFrameSize())
		m.Viewport.SetContent(m.renderContent())
	}
	return m, nil
}

func (m *LogModel) View() string {
	return m.Viewport.View()
}

func (m *LogModel) renderContent() string {
	var b strings.Builder
	for i, e := range m.Entries {
		header := fmt.Sprintf("%s %-5s %s:%d", e.Ts.Format(time.RFC3339), strings.ToUpper(e.Level), e.File, e.Line)
		body := e.Body
		if body == "" {
			body = e.Msg
		}
		// Glamour render only when flagged
		bodyRendered := glamorRenderIfNeeded(body, e.BodyFormat, m.Viewport.Width)
		// pad with background style from theme
		headerLine := padLineWithBackground(header, m.Viewport.Width, m.Styles.MainPane)
		bodyLine := padLineWithBackground(bodyRendered, m.Viewport.Width, m.Styles.MainPane)
		b.WriteString(headerLine)
		b.WriteString("\n")
		b.WriteString(bodyLine)
		if i < len(m.Entries)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
