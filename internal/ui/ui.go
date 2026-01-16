package ui

import (
	"clai/internal/llm"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

const (
	ChatPaneWidthRatio = 0.6
	StatusBarHeight    = 1
	MinPaneWidth       = 20
	MinPaneHeight      = 5
	TextAreaPadding    = 8
	TextAreaHeight     = 3
	TextAreaMaxLength  = 256
	HelpPaneWidthRatio = 0.8
)

func SetupChatComponent(width, height int, theme *glitter.UI, llmClient llm.LLMClientInterface) ChatModel {
	viewport := viewport.New(width, height-TextAreaHeight-1)
	viewport.MouseWheelEnabled = true
	viewport.MouseWheelDelta = 3

	textArea := textarea.New()
	textArea.Placeholder = "Type your message..."
	textArea.CharLimit = TextAreaMaxLength
	textArea.SetWidth(width - TextAreaPadding)
	textArea.SetHeight(TextAreaHeight)
	textArea.ShowLineNumbers = false

	setupTextareaStyling(textArea, theme)

	s := spinner.New()
	s.Spinner = spinner.Dot
	textArea.Focus()

	return ChatModel{
		Viewport:     viewport,
		Textarea:     textArea,
		LlmClient:    llmClient,
		Spinner:      s,
		Theme:        theme,
		Streaming:    false,
		Width:        width,
		Height:       height,
		ContentDirty: true,
		Messages:     []llm.Message{},
		QueryHistory: []string{},
		HistoryIndex: -1,
		AutoScroll:   true,
		UserScrolled: false,
	}
}

func SetupAgentStatusComponent(theme *glitter.UI) *AgentStatusView {
	return NewAgentStatusView(theme)
}

func SetupLogComponent(width, height int) viewport.Model {
	log := viewport.New(width, height)
	log.MouseWheelEnabled = true
	log.MouseWheelDelta = 3
	return log
}

func setupTextareaStyling(ta textarea.Model, theme *glitter.UI) {
	if theme == nil {
		return
	}

	bgColor := lipgloss.Color(theme.Theme.Primary.Background)

	ta.FocusedStyle.Base = lipgloss.NewStyle().Background(bgColor)
	ta.BlurredStyle.Base = lipgloss.NewStyle().Background(bgColor)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(bgColor)
	ta.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Background(bgColor)
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Theme.Primary.DimForeground)).
		Background(bgColor)
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Theme.Primary.DimForeground)).
		Background(bgColor)
	ta.FocusedStyle.Text = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Theme.Primary.Foreground)).
		Background(bgColor)
	ta.BlurredStyle.Text = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Theme.Primary.Foreground)).
		Background(bgColor)
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Theme.Bright.Yellow)).
		Background(bgColor)
	ta.BlurredStyle.Prompt = lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Theme.Bright.Yellow)).
		Background(bgColor)
	ta.Cursor.Style = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFAA")).
		Bold(true).
		Underline(true).
		Background(bgColor)
}
