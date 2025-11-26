package ui

import (
	"clai/internal/llm"
	"log"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ChatModel struct {
	Messages       []llm.Message
	TextInput      textinput.Model
	Viewport       viewport.Model
	LlmClient      llm.LLMClientInterface
	Spinner        spinner.Model
	Streaming      bool
	SelectingTools bool
	Width          int
	Height         int
	AssistantName  string
	Theme          *glitter.UI
	CachedContent  string
	ContentDirty   bool
	QueryHistory   []string
	HistoryIndex   int
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
	themeStyles := GetThemeStyles(c.Theme)
	inputStyle := lipgloss.NewStyle()
	if c.TextInput.Focused() {
		inputStyle = inputStyle.Border(lipgloss.RoundedBorder(), true).BorderForeground(lipgloss.Color(c.Theme.Theme.Bright.Yellow))
	}

	// Only rebuild chat content if dirty (messages changed)
	if c.ContentDirty {
		log.Printf("[CHAT] Rendering %d messages, c.Width=%d", len(c.Messages), c.Width)
		chatContent := ""
		for i, msg := range c.Messages {
			var rendered string
			switch msg.Role {
			case "user":
				innerWidth := c.Width - themeStyles.UserMessage.GetHorizontalFrameSize()
				rendered = themeStyles.UserMessage.Width(innerWidth).Render(msg.Content)
			case "assistant":
				toolBadges := ""
				if len(msg.SelectedTools) > 0 && i > 0 && c.Messages[i-1].Role == "tool" {
					badge := themeStyles.ToolBadge.Render("🔧 " + c.Messages[i-1].Content)
					toolBadges = "\n  " + badge
				}
				innerWidth := c.Width - themeStyles.AssistantMessage.GetHorizontalFrameSize()
				rendered = themeStyles.AssistantMessage.Width(innerWidth).Render(msg.Content + toolBadges)
			case "tool":
				continue
			default:
				rendered = lipgloss.NewStyle().Width(c.Width).Render(msg.Content)
			}
			chatContent += rendered + "\n"
		}
		c.CachedContent = chatContent
		c.ContentDirty = false
	}
	c.Viewport.SetContent(c.CachedContent)

	spinnerView := ""
	spinnerHeight := 0
	if c.SelectingTools {
		spinnerView = c.Spinner.View() + " Selecting tools..."
		spinnerHeight = 1
	} else if c.Streaming {
		spinnerView = c.Spinner.View() + " Generating..."
		spinnerHeight = 1
	}

	inputFieldRendered := inputStyle.Render(c.TextInput.View())
	inputHeight := lipgloss.Height(inputFieldRendered)

	if !c.TextInput.Focused() {
		tooltip := lipgloss.NewStyle().Background(lipgloss.Color(c.Theme.Theme.Bright.Blue)).Foreground(lipgloss.Color(c.Theme.Theme.Bright.White)).Padding(0, 1).Render("Ctrl+T: switch panes | Ctrl+H: help | Ctrl+D: theme | Ctrl+Q: quit")
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

	var joined string
	if (c.Streaming || c.SelectingTools) && spinnerView != "" {
		joined = lipgloss.JoinVertical(lipgloss.Left, c.Viewport.View(), spinnerView, inputFieldRendered)
	} else {
		joined = lipgloss.JoinVertical(lipgloss.Left, c.Viewport.View(), inputFieldRendered)
	}

	return joined
}
