package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Name        string
	Primary1    lipgloss.Color
	Primary2    lipgloss.Color
	Primary3    lipgloss.Color
	Accent1     lipgloss.Color
	Accent2     lipgloss.Color
	BgDark      lipgloss.Color
	BgLight     lipgloss.Color
	BorderCol   lipgloss.Color
	MainPane    lipgloss.Style
	StatusBar   lipgloss.Style
	UserMessage lipgloss.Style
	AssistantMessage lipgloss.Style
	ToolMessage lipgloss.Style
}

var ( 
	DarkTheme = Theme{
		Name:      "dark",
		Primary1:  lipgloss.Color("#5D5D81"), // Muted Purple
		Primary2:  lipgloss.Color("#7A7ABF"), // Medium Purple
		Primary3:  lipgloss.Color("#9B9BDC"), // Light Purple
		Accent1:   lipgloss.Color("#FFD700"), // Gold
		Accent2:   lipgloss.Color("#FFFFFF"), // White
		BgDark:    lipgloss.Color("#1A1A2E"), // Dark Blue-Purple
		BgLight:   lipgloss.Color("#2E2E50"), // Medium Blue-Purple
		BorderCol: lipgloss.Color("#7A7ABF"), // Medium Purple
	}

	LightTheme = Theme{
		Name:      "light",
		Primary1:  lipgloss.Color("#8B0000"), // Dark Red
		Primary2:  lipgloss.Color("#B22222"), // Firebrick
		Primary3:  lipgloss.Color("#CD5C5C"), // Indian Red
		Accent1:   lipgloss.Color("#FF8C00"), // Dark Orange
		Accent2:   lipgloss.Color("#000000"), // Black
		BgDark:    lipgloss.Color("#F5F5DC"), // Beige
		BgLight:   lipgloss.Color("#FFFFFF"), // White
		BorderCol: lipgloss.Color("#B22222"), // Firebrick
	}
)

func (t *Theme) ApplyStyles() {
	t.MainPane = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderCol).
		Background(t.BgDark).
		Padding(0, 2)

	t.StatusBar = lipgloss.NewStyle().
		Background(t.Primary1).
		Foreground(t.Accent2).
		Bold(true).
		Padding(0, 2)

	t.UserMessage = lipgloss.NewStyle().Background(t.BgLight).Foreground(t.BgDark).Bold(true).Padding(0, 1)
	t.AssistantMessage = lipgloss.NewStyle().Background(t.Primary1).Foreground(t.Accent2).Padding(0, 1)
	t.ToolMessage = lipgloss.NewStyle().Background(t.Primary3).Foreground(t.BgLight).Italic(true).Padding(0, 1)
}