package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"os"
)

// Simple responsive layout with viewport
type model struct {
	width     int
	height    int
	content   string
	scrollPos int
}

func (m model) Init() tea.Cmd {
	m.content = `This is a responsive Bubble Tea TUI example.

Resize the terminal to see how the layout adapts automatically.

Key Controls:
- q/ctrl+c: Quit
- Up/Down: Scroll content (if it overflows)

Features demonstrated:
- Full-screen layout that fills terminal
- Window resize handling
- Responsive text wrapping
- Border styling with proper backgrounds`
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, tea.ClearScreen

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.scrollPos > 0 {
				m.scrollPos--
			}
		case "down":
			m.scrollPos++
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Create border style with proper background to prevent gaps
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#50fa7b")).
		BorderBackground(lipgloss.Color("#282a36")).
		Background(lipgloss.Color("#282a36")).
		Padding(1, 2)

	// Calculate inner dimensions
	frameSize := 6 // 1 padding left/right + 2 border chars + 1 padding left/right
	innerWidth := m.width - frameSize
	innerHeight := m.height - frameSize

	// Wrap text to fit inner width
	wrappedText := lipgloss.NewStyle().
		Width(innerWidth).
		Height(innerHeight).
		Foreground(lipgloss.Color("#f8f8f2")).
		Render(m.content)

	// Apply border
	panel := borderStyle.
		Width(m.width).
		Height(m.height).
		Render(wrappedText)

	// Add status line at bottom
	statusLine := lipgloss.NewStyle().
		Width(m.width).
		Height(1).
		Background(lipgloss.Color("#44475a")).
		Foreground(lipgloss.Color("#f8f8f2")).
		Align(lipgloss.Center).
		Render(fmt.Sprintf("Terminal: %dx%d | Press 'q' to quit", m.width, m.height))

	return panel + "\n" + statusLine
}

func main() {
	p := tea.NewProgram(
		&model{},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
