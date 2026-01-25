package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"os"
)

// Basic full-screen TUI example
type model struct {
	width  int
	height int
	ready  bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, tea.ClearScreen
	}
	return m, nil
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Center content
	title := "Bubble Tea Full-Screen UI"
	subtitle := fmt.Sprintf("Terminal Size: %dx%d", m.width, m.height)

	titleStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height/2).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color("#50fa7b")).
		Background(lipgloss.Color("#282a36"))

	subtitleStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height/2).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color("#f8f8f2"))

	return titleStyle.Render(title) + subtitleStyle.Render(subtitle)
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
