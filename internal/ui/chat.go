package ui

import (
	"clai/internal/llm"
	"fmt"
	"log"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ChatModel struct {
	Messages           []llm.Message
	TextInput          textinput.Model
	Viewport           viewport.Model
	LlmClient          llm.LLMClientInterface
	Spinner            spinner.Model
	Streaming          bool
	SelectingTools     bool
	Width              int
	Height             int
	AssistantName      string
	Theme              *glitter.UI
	CachedContent      string
	ContentDirty       bool
	QueryHistory       []string
	HistoryIndex       int
	AutoScroll         bool
	UserScrolled       bool
	SmoothScrollTarget int
	SmoothScrollActive bool
	NeedsInitialScroll bool
}

func (c *ChatModel) Init() tea.Cmd {
	return nil // No blinking cursor, modern input
}

func (c *ChatModel) Update(msg tea.Msg) (ChatModel, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	oldYOffset := c.Viewport.YOffset

	c.TextInput, cmd = c.TextInput.Update(msg)
	cmds = append(cmds, cmd)
	c.Viewport, cmd = c.Viewport.Update(msg)
	cmds = append(cmds, cmd)
	c.Spinner, cmd = c.Spinner.Update(msg)
	cmds = append(cmds, cmd)

	if c.Viewport.YOffset != oldYOffset && !c.Streaming {
		c.UserScrolled = true
		c.AutoScroll = false
		log.Printf("[CHAT] Manual scroll detected: YOffset changed from %d to %d", oldYOffset, c.Viewport.YOffset)
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelUp {
			c.UserScrolled = true
			c.AutoScroll = false
			c.SmoothScrollActive = false
		} else if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonWheelDown {
			if c.Viewport.AtBottom() {
				c.UserScrolled = false
				c.AutoScroll = true
			}
			c.SmoothScrollActive = false
		}
	}

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
		maxBubbleWidth := int(float64(c.Width) * 0.8)
		maxInnerTextWidth := maxBubbleWidth - themeStyles.AssistantMessage.GetHorizontalFrameSize()

		for i, msg := range c.Messages {
			var rendered string
			switch msg.Role {
			case "user":
				wrappedContent := lipgloss.NewStyle().Width(maxInnerTextWidth).Render(msg.Content)
				bubble := themeStyles.UserMessage.Render(wrappedContent)
				paddedLine := lipgloss.NewStyle().
					Width(c.Width).
					Background(lipgloss.Color(c.Theme.Theme.Primary.Background)).
					Align(lipgloss.Left).
					Render(bubble)
				rendered = paddedLine
			case "assistant":
				toolBadges := ""
				if len(msg.SelectedTools) > 0 && i > 0 && c.Messages[i-1].Role == "tool" {
					badge := themeStyles.ToolBadge.Render("🔧 " + c.Messages[i-1].Content)
					toolBadges = "\n  " + badge
				}

				contentWithHighlighting := llm.RenderWithSyntaxHighlighting(msg.Content, maxInnerTextWidth)
				bubble := themeStyles.AssistantMessage.Render(contentWithHighlighting + toolBadges)
				paddedLine := lipgloss.NewStyle().
					Width(c.Width).
					Background(lipgloss.Color(c.Theme.Theme.Primary.Background)).
					Align(lipgloss.Right).
					Render(bubble)
				rendered = paddedLine
			case "tool":
				continue
			default:
				rendered = msg.Content
			}
			chatContent += rendered + "\n"
		}
		c.CachedContent = chatContent
		c.ContentDirty = false
		c.Viewport.SetContent(c.CachedContent)
	}

	scrollIndicator := ""
	scrollIndicatorHeight := 0
	if c.Viewport.TotalLineCount() > 0 {
		scrollPct := int(c.Viewport.ScrollPercent() * 100)
		arrow := ""
		if !c.Viewport.AtTop() && !c.Viewport.AtBottom() {
			arrow = "↕"
		} else if !c.Viewport.AtTop() {
			arrow = "↑"
		} else if !c.Viewport.AtBottom() {
			arrow = "↓"
		}
		if arrow != "" {
			scrollIndicator = themeStyles.ScrollIndicator.Render(fmt.Sprintf(" %s %d%% ", arrow, scrollPct))
			scrollIndicatorHeight = 1
		}
	}

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
	viewportHeight := c.Height - inputHeight - spinnerHeight - scrollIndicatorHeight
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	c.Viewport.Height = viewportHeight

	// Perform initial scroll to bottom after first render (when height is set correctly)
	if c.NeedsInitialScroll {
		c.Viewport.GotoBottom()
		c.NeedsInitialScroll = false
		log.Printf("[CHAT] Initial scroll to bottom: YOffset=%d, Height=%d, TotalLines=%d",
			c.Viewport.YOffset, c.Viewport.Height, c.Viewport.TotalLineCount())
	}

	var joined string
	var parts []string
	parts = append(parts, c.Viewport.View())
	if scrollIndicator != "" {
		parts = append(parts, scrollIndicator)
	}
	if (c.Streaming || c.SelectingTools) && spinnerView != "" {
		parts = append(parts, spinnerView)
	}
	parts = append(parts, inputFieldRendered)
	joined = lipgloss.JoinVertical(lipgloss.Left, parts...)

	return joined
}
