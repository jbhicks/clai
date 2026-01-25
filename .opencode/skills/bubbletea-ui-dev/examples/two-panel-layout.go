package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"os"
)

type model struct {
	width  int
	height int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, tea.ClearScreen
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Calculate panel dimensions (70% left, 30% right)
	leftWidth := int(float64(m.width) * 0.7)
	rightWidth := m.width - leftWidth

	// Style definitions with proper border backgrounds
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#6272a4")).
		BorderBackground(lipgloss.Color("#282a36")).
		Background(lipgloss.Color("#282a36"))

	leftStyle := panelStyle.Copy().
		Foreground(lipgloss.Color("#f8f8f2"))

	rightStyle := panelStyle.Copy().
		Foreground(lipgloss.Color("#6272a4"))

	// Create content
	leftContent := leftStyle.
		Width(leftWidth).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render("Main Content\n\nThis panel takes 70%\nof the terminal width")

	rightContent := rightStyle.
		Width(rightWidth).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render("Sidebar\n\nThis panel takes 30%\nof the terminal width")

	// Join panels horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, leftContent, rightContent)
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
