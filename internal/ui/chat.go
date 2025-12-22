package ui

import (
	"clai/internal/llm"
	"clai/internal/logger"
	"fmt"
	"regexp"
	"strings"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

type ChatModel struct {
	Messages           []llm.Message
	TextInput          textinput.Model
	Viewport           viewport.Model
	LlmClient          llm.LLMClientInterface
	Spinner            spinner.Model
	Streaming          bool
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
		logger.Debug("[CHAT] Manual scroll detected: YOffset changed from %d to %d", oldYOffset, c.Viewport.YOffset)
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

	c.rebuildContentIfDirty()
	c.updateViewportHeight()

	return *c, tea.Batch(cmds...)
}

func (c *ChatModel) rebuildContentIfDirty() {
	if !c.ContentDirty {
		return
	}

	themeStyles := GetThemeStyles(c.Theme)
	logger.Debug("[CHAT] Rendering %d messages, c.Width=%d", len(c.Messages), c.Width)
	chatContent := ""
	maxBubbleWidth := int(float64(c.Width) * 0.95)
	maxInnerTextWidth := maxBubbleWidth - themeStyles.AssistantMessage.GetHorizontalFrameSize()

	for i, msg := range c.Messages {
		var rendered string
		switch msg.Role {
		case "user":
			userInnerWidth := maxBubbleWidth - themeStyles.UserMessage.GetHorizontalFrameSize()
			wrappedContent := lipgloss.NewStyle().Width(userInnerWidth).Render(msg.Content)
			bubble := themeStyles.UserMessage.Render(wrappedContent)
			wrapper := lipgloss.NewStyle().
				Width(c.Width).
				Align(lipgloss.Left)
			rendered = wrapper.Render(bubble)
		case "assistant":
			cleanedContent := llm.StripTextBasedFunctionCalls(msg.Content)
			contentWithHighlighting := llm.RenderWithSyntaxHighlighting(cleanedContent, maxInnerTextWidth, themeStyles.CodeBlockBadge, themeStyles.CodeBlockContainer)
			bubble := themeStyles.AssistantMessage.Render(contentWithHighlighting)
			wrapper := lipgloss.NewStyle().
				Width(c.Width).
				Align(lipgloss.Right)
			rendered = wrapper.Render(bubble)
		case "tool":
			toolInnerWidth := maxBubbleWidth - themeStyles.ToolMessage.GetHorizontalFrameSize()
			cleanContent := ansiRegex.ReplaceAllString(msg.Content, "")
			languageIcon := "⚙️"
			header := "Execution Result"
			if strings.Contains(cleanContent, "code block") && strings.Contains(cleanContent, "executed") {
				header = "Code Execution Complete"
				if strings.Contains(cleanContent, "(bash)") || strings.Contains(strings.ToLower(cleanContent), "bash") {
					languageIcon = "🐚"
				} else if strings.Contains(strings.ToLower(cleanContent), "python") {
					languageIcon = "🐍"
				} else if strings.Contains(strings.ToLower(cleanContent), "javascript") || strings.Contains(strings.ToLower(cleanContent), "node") {
					languageIcon = "📜"
				}
			}
			displayContent := cleanContent
			const maxToolOutputLength = 1000
			if len(displayContent) > maxToolOutputLength {
				displayContent = displayContent[:maxToolOutputLength] + "\n... (output truncated)"
			}
			toolHeader := themeStyles.ToolBadge.Render(languageIcon + " " + header)
			wrappedContent := lipgloss.NewStyle().Width(toolInnerWidth).Render(displayContent)
			bubble := themeStyles.ToolMessage.Render(toolHeader + "\n" + wrappedContent)
			rendered = bubble
		default:
			rendered = msg.Content
		}
		chatContent += rendered
		if i < len(c.Messages)-1 {
			chatContent += "\n"
		}
	}
	c.CachedContent = chatContent
	c.ContentDirty = false
	c.Viewport.SetContent(c.CachedContent)

	if c.AutoScroll && !c.UserScrolled {
		c.Viewport.GotoBottom()
		logger.Debug("[CHAT] Auto-scroll to bottom after content update: YOffset=%d, Height=%d, TotalLines=%d",
			c.Viewport.YOffset, c.Viewport.Height, c.Viewport.TotalLineCount())
	}
}

func (c *ChatModel) updateViewportHeight() {
	themeStyles := GetThemeStyles(c.Theme)

	// Calculate input height (always account for focused state to prevent height jumping)
	// When focused: border adds 2 lines (top + bottom) + 1 line for input = 3
	// When unfocused: 1 line for input + 1 line for tooltip = 2, but we'll use 3 for consistency
	inputHeight := 3

	// Calculate spinner height
	spinnerHeight := 0
	if c.Streaming {
		spinnerHeight = 1
	}

	// Calculate scroll indicator height
	scrollIndicatorHeight := 0
	if c.Viewport.TotalLineCount() > 0 {
		if !c.Viewport.AtTop() || !c.Viewport.AtBottom() {
			scrollIndicatorHeight = 1
		}
	}

	// Calculate and set viewport height
	viewportHeight := c.Height - inputHeight - spinnerHeight - scrollIndicatorHeight
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	c.Viewport.Height = viewportHeight

	// Perform initial scroll to bottom after first render
	if c.NeedsInitialScroll {
		c.Viewport.GotoBottom()
		c.NeedsInitialScroll = false
		logger.Debug("[CHAT] Initial scroll to bottom: YOffset=%d, Height=%d, TotalLines=%d",
			c.Viewport.YOffset, c.Viewport.Height, c.Viewport.TotalLineCount())
	}

	_ = themeStyles // Suppress unused warning if needed
}

func (c *ChatModel) View() string {
	themeStyles := GetThemeStyles(c.Theme)
	inputStyle := lipgloss.NewStyle()
	if c.TextInput.Focused() {
		inputStyle = inputStyle.Border(lipgloss.RoundedBorder(), true).BorderForeground(lipgloss.Color(c.Theme.Theme.Bright.Yellow))
	}

	scrollIndicator := ""
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
			scrollText := themeStyles.ScrollIndicator.Render(fmt.Sprintf(" %s %d%% ", arrow, scrollPct))
			scrollWrapper := lipgloss.NewStyle().
				Width(c.Width).
				Background(lipgloss.Color(c.Theme.Theme.Primary.Background)).
				Align(lipgloss.Center)
			scrollIndicator = scrollWrapper.Render(scrollText)
		}
	}

	spinnerView := ""
	if c.Streaming {
		spinnerText := c.Spinner.View() + " Generating..."
		spinnerView = lipgloss.NewStyle().
			Width(c.Width).
			Background(lipgloss.Color(c.Theme.Theme.Primary.Background)).
			Render(spinnerText)
	}

	inputFieldRendered := inputStyle.Render(c.TextInput.View())
	if lipgloss.Width(inputFieldRendered) < c.Width {
		inputWrapper := lipgloss.NewStyle().
			Width(c.Width).
			Background(lipgloss.Color(c.Theme.Theme.Primary.Background)).
			Align(lipgloss.Left)
		inputFieldRendered = inputWrapper.Render(inputFieldRendered)
	}

	if !c.TextInput.Focused() {
		tooltip := lipgloss.NewStyle().Background(lipgloss.Color(c.Theme.Theme.Bright.Blue)).Foreground(lipgloss.Color(c.Theme.Theme.Bright.White)).Padding(0, 1).Render("Ctrl+T: switch panes | Ctrl+H: help | Ctrl+D: theme | Ctrl+Q: quit")
		inputFieldRendered = lipgloss.JoinVertical(lipgloss.Left, inputFieldRendered, tooltip)
	}

	// Viewport height already set in Update() via updateViewportHeight()
	var joined string
	var parts []string

	// Wrap viewport content with background (no explicit height to avoid overflow)
	viewportContent := c.Viewport.View()
	viewportWrapper := lipgloss.NewStyle().
		Width(c.Width).
		Background(lipgloss.Color(c.Theme.Theme.Primary.Background))
	viewportContentFilled := viewportWrapper.Render(viewportContent)

	parts = append(parts, viewportContentFilled)
	if scrollIndicator != "" {
		parts = append(parts, scrollIndicator)
	}
	if c.Streaming && spinnerView != "" {
		parts = append(parts, spinnerView)
	}
	parts = append(parts, inputFieldRendered)
	joined = lipgloss.JoinVertical(lipgloss.Left, parts...)

	return joined
}
