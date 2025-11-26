package ui

import (
	"github.com/brittonhayes/glitter/glitter"
	"github.com/brittonhayes/glitter/theme"
	"github.com/charmbracelet/lipgloss"
)

// AvailableThemes provides a list of pre-built glitter themes
var AvailableThemes = []*glitter.UI{
	glitter.NewUI(theme.Gruvbox),
	glitter.NewUI(theme.Monokai),
	glitter.NewUI(theme.Nord),
	glitter.NewUI(theme.OneBit),
}

// ThemeNames provides human-readable names for the themes
var ThemeNames = []string{
	"Gruvbox",
	"Monokai",
	"Nord",
	"OneBit",
}

// GetThemeStyles returns lipgloss styles compatible with the current glitter theme
func GetThemeStyles(ui *glitter.UI) ThemeStyles {
	return ThemeStyles{
		MainPane: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ui.Theme.Primary.Foreground)).
			Background(lipgloss.Color(ui.Theme.Primary.Background)),

		StatusBar: lipgloss.NewStyle().
			Background(lipgloss.Color(ui.Theme.Primary.Background)).
			Foreground(lipgloss.Color(ui.Theme.Primary.Foreground)).
			Bold(true).
			Padding(0, 2),

		UserMessage: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color(ui.Theme.Bright.White)).
			Background(lipgloss.Color(ui.Theme.Normal.White)).
			PaddingLeft(2).
			PaddingRight(1).
			MarginBottom(1).
			Bold(true),

		AssistantMessage: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color(ui.Theme.Bright.Yellow)).
			Background(lipgloss.Color(ui.Theme.Primary.Background)).
			PaddingLeft(1).
			PaddingRight(2).
			MarginLeft(2).
			MarginBottom(1),

		ToolMessage: lipgloss.NewStyle().
			Background(lipgloss.Color(ui.Theme.Dim.Black)).
			Foreground(lipgloss.Color(ui.Theme.Dim.White)).
			Italic(true).
			Padding(0, 1).
			MarginLeft(2),

		ToolBadge: lipgloss.NewStyle().
			Background(lipgloss.Color(ui.Theme.Bright.Blue)).
			Foreground(lipgloss.Color(ui.Theme.Bright.White)).
			Padding(0, 2),
	}
}

// ThemeStyles contains all the lipgloss styles for a theme
type ThemeStyles struct {
	MainPane         lipgloss.Style
	StatusBar        lipgloss.Style
	UserMessage      lipgloss.Style
	AssistantMessage lipgloss.Style
	ToolMessage      lipgloss.Style
	ToolBadge        lipgloss.Style
}
