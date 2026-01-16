package ui

import "github.com/charmbracelet/lipgloss"

const (
	DefaultViewportWidth  = 80
	DefaultViewportHeight = 20
	LogViewportWidth      = 50
	SmallViewportHeight   = 10
	MediumViewportHeight  = 15

	PaddingBlock      = 0
	PaddingInline     = 1
	PaddingInlineWide = 2

	AlignCenter = lipgloss.Center
	AlignLeft   = lipgloss.Left
)

// LayoutConfig contains all configurable layout ratios and dimensions
type LayoutConfig struct {
	ChatPaneWidthRatio    float64
	HelpPaneWidthRatio    float64
	AgentStatusPaneHeight int
	MinPaneWidth          int
	MinViewportWidth      int
	MinViewportHeight     int
	TextareaPadding       int
}

func DefaultLayoutConfig() LayoutConfig {
	return LayoutConfig{
		ChatPaneWidthRatio:    0.8,
		HelpPaneWidthRatio:    0.6,
		AgentStatusPaneHeight: 6,
		MinPaneWidth:          20,
		MinViewportWidth:      10,
		MinViewportHeight:     3,
		TextareaPadding:       8,
	}
}
