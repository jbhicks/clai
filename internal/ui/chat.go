package ui

import (
	"clai/internal/llm"
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/list"
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
	List          list.Model
	LlmClient     *llm.Client
	Spinner       spinner.Model
	Streaming     bool
	Width         int
	Height        int
	AssistantName string
	Theme         *Theme
}

func (c *ChatModel) Init() tea.Cmd {
	return nil // No blinking cursor, modern input
}

func (c *ChatModel) Update(msg tea.Msg) (ChatModel, tea.Cmd) {
	log.Printf("ChatModel.Update called with msg type: %T", msg)
	var cmds []tea.Cmd
	var cmd tea.Cmd
	c.TextInput, cmd = c.TextInput.Update(msg)
	cmds = append(cmds, cmd)
	c.Viewport, cmd = c.Viewport.Update(msg)
	cmds = append(cmds, cmd)
	c.Spinner, cmd = c.Spinner.Update(msg)
	cmds = append(cmds, cmd)
	c.List, cmd = c.List.Update(msg)
	cmds = append(cmds, cmd)
	return *c, tea.Batch(cmds...)
}

func (c *ChatModel) View() string {
	log.Printf("ChatModel.View called: Width=%d, Height=%d", c.Width, c.Height)

	inputStyle := lipgloss.NewStyle()
	if c.TextInput.Focused() {
		inputStyle = inputStyle.Border(lipgloss.RoundedBorder(), true).BorderForeground(c.Theme.Accent1)
	}

	chatContent := ""
	for _, msg := range c.Messages {
		var rendered string
		switch msg.Role {
		case "user":
			rendered = c.Theme.UserMessage.Width(c.Width).Render(fmt.Sprintf("user: %s", msg.Content))
		case "assistant":
			name := c.AssistantName
			if name == "" {
				name = "assistant"
			}
			rendered = c.Theme.AssistantMessage.Width(c.Width).Render(fmt.Sprintf("%s: %s", name, msg.Content))
		case "tool":
			rendered = c.Theme.ToolMessage.Width(c.Width).Render(fmt.Sprintf("tool: %s", msg.Content))
		default:
			rendered = lipgloss.NewStyle().Width(c.Width).Render(fmt.Sprintf("%s: %s", msg.Role, msg.Content))
		}
		chatContent += rendered + "\n\n"
	}
	c.Viewport.SetContent(chatContent)

	spinnerView := ""
	if c.Streaming {
		spinnerView = c.Spinner.View() + " Generating..."
	}

	inputFieldRendered := inputStyle.Render(c.TextInput.View())
	log.Printf("ChatModel.View: inputFieldRendered height: %d", lipgloss.Height(inputFieldRendered))

	tooltipHeight := 0
	if !c.TextInput.Focused() {
		tooltip := lipgloss.NewStyle().Background(c.Theme.Primary2).Foreground(c.Theme.Accent2).Padding(0, 1).Render("Press Enter to send, Tab to switch panes, ? for help")
		tooltipHeight = lipgloss.Height(tooltip)
		inputFieldRendered = lipgloss.JoinVertical(lipgloss.Left, inputFieldRendered, tooltip)
		log.Printf("ChatModel.View: tooltipHeight: %d, inputFieldRendered (with tooltip) height: %d", tooltipHeight, lipgloss.Height(inputFieldRendered))
	}

	// Calculate remaining height for the list
	remainingHeight := c.Height - lipgloss.Height(spinnerView) - lipgloss.Height(inputFieldRendered)
	if remainingHeight < 3 {
		remainingHeight = 3
	}
	log.Printf("ChatModel.View: spinnerView height: %d, inputFieldRendered height: %d, remainingHeight for list: %d", lipgloss.Height(spinnerView), lipgloss.Height(inputFieldRendered), remainingHeight)

	c.List.SetHeight(remainingHeight)
	c.List.SetWidth(c.Width)
	c.Viewport.Width = c.Width
	c.Viewport.Height = remainingHeight

	listView := c.List.View()
	log.Printf("ChatModel.View: listView rendered height: %d", lipgloss.Height(listView))
	log.Printf("ChatModel.View: spinnerView rendered height: %d", lipgloss.Height(spinnerView))
	log.Printf("ChatModel.View: inputFieldRendered rendered height: %d", lipgloss.Height(inputFieldRendered))

	joined := lipgloss.JoinVertical(lipgloss.Left, listView, spinnerView, inputFieldRendered)
	log.Printf("ChatModel.View: joined rendered height: %d", lipgloss.Height(joined))
	return joined
}
