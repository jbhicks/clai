package ui

import (
	"clai/internal/llm"
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ChatModel struct {
	Messages      []llm.Message
	TextInput     textinput.Model
	Viewport      viewport.Model
	LlmClient     *llm.Client
	Spinner       spinner.Model
	Streaming     bool
	Width         int
	Height        int
	AssistantName string
	Theme         *Theme
	CachedContent string
	ContentDirty  bool
}

func (c *ChatModel) Init() tea.Cmd {
	return nil // No blinking cursor, modern input
}

func (c *ChatModel) Update(msg tea.Msg) (ChatModel, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	c.TextInput, cmd = c.TextInput.Update(msg)
	cmds = append(cmds, cmd)
	c.Viewport, cmd = c.Viewport.Update(msg)
	cmds = append(cmds, cmd)
	c.Spinner, cmd = c.Spinner.Update(msg)
	cmds = append(cmds, cmd)
	return *c, tea.Batch(cmds...)
}

func (c *ChatModel) View() string {
	inputStyle := lipgloss.NewStyle()
	if c.TextInput.Focused() {
		inputStyle = inputStyle.Border(lipgloss.RoundedBorder(), true).BorderForeground(c.Theme.Accent1)
	}

	// Only rebuild chat content if dirty (messages changed)
	if c.ContentDirty {
		chatContent := ""
		for _, msg := range c.Messages {
			var rendered string
			switch msg.Role {
			case "user":
				rendered = c.Theme.UserMessage.Width(c.Width).Render(fmt.Sprintf("> %s", msg.Content))
			case "assistant":
				rendered = c.Theme.AssistantMessage.Width(c.Width).Render(fmt.Sprintf("> %s", msg.Content))
			case "tool":
				rendered = c.Theme.ToolMessage.Width(c.Width).Render(fmt.Sprintf("> %s", msg.Content))
			default:
				rendered = lipgloss.NewStyle().Width(c.Width).Render(fmt.Sprintf("> %s", msg.Content))
			}
			chatContent += rendered + "\n\n"
		}
		c.CachedContent = chatContent
		c.ContentDirty = false
	}
	c.Viewport.SetContent(c.CachedContent)

	spinnerView := ""
	spinnerHeight := 0
	if c.Streaming {
		spinnerView = c.Spinner.View() + " Generating..."
		spinnerHeight = 1
	}

	inputFieldRendered := inputStyle.Render(c.TextInput.View())
	inputHeight := lipgloss.Height(inputFieldRendered)

	if !c.TextInput.Focused() {
		tooltip := lipgloss.NewStyle().Background(c.Theme.Primary2).Foreground(c.Theme.Accent2).Padding(0, 1).Render("Ctrl+T: switch panes | Ctrl+H: help | Ctrl+D: theme | Ctrl+Q: quit")
		inputFieldRendered = lipgloss.JoinVertical(lipgloss.Left, inputFieldRendered, tooltip)
		inputHeight = lipgloss.Height(inputFieldRendered)
		log.Printf("ChatModel.View: Input NOT focused, tooltip added, new inputHeight=%d", inputHeight)
	}

	// Calculate viewport height
	viewportHeight := c.Height - inputHeight - spinnerHeight
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	c.Viewport.Height = viewportHeight

	// Join components - only include spinner if streaming
	var joined string
	if c.Streaming && spinnerView != "" {
		joined = lipgloss.JoinVertical(lipgloss.Left, c.Viewport.View(), spinnerView, inputFieldRendered)
	} else {
		joined = lipgloss.JoinVertical(lipgloss.Left, c.Viewport.View(), inputFieldRendered)
	}

	return joined
}
