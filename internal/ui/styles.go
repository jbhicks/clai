package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Tokyo Night Palette
	TNBackground = lipgloss.Color("#1a1b26")
	TNForeground = lipgloss.Color("#a9b1d6")
	TNSelection  = lipgloss.Color("#33467c")
	TNComment    = lipgloss.Color("#565f89")
	TNRed        = lipgloss.Color("#f7768e")
	TNGreen      = lipgloss.Color("#9ece6a")
	TNYellow     = lipgloss.Color("#e0af68")
	TNBlue       = lipgloss.Color("#7aa2f7")
	TNMagenta    = lipgloss.Color("#bb9af7")
	TNCyan       = lipgloss.Color("#7dcfff")

	// Base style for the entire screen
	BaseStyle = lipgloss.NewStyle().
			Foreground(TNForeground).
			Background(TNBackground)

	// Header style
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(TNMagenta).
			Background(TNBackground).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(TNSelection).
			BorderBottom(true)

	// Body/Content area style
	BodyStyle = lipgloss.NewStyle().
			Background(TNBackground).
			Padding(1, 2)

	// Footer style
	FooterStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(TNComment).
			Background(TNBackground).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(TNSelection).
			BorderTop(true)

	ActiveStoryStyle = lipgloss.NewStyle().
				Foreground(TNBackground).
				Background(TNGreen).
				Bold(true)

	SelectedStoryStyle = lipgloss.NewStyle().
				Foreground(TNBackground).
				Background(TNBlue).
				Bold(true)

	PassStyle = lipgloss.NewStyle().
			Foreground(TNComment).
			Strikethrough(true)

	BriefingHeaderStyle = lipgloss.NewStyle().
				Foreground(TNCyan).
				Bold(true).
				Underline(true).
				MarginBottom(1)

	BriefingLabelStyle = lipgloss.NewStyle().
				Foreground(TNYellow).
				Bold(true)
)
