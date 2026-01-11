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

	shouldAutoScroll := c.AutoScroll && !c.UserScrolled && c.ContentDirty
	// Also auto-scroll during streaming to keep latest content visible
	shouldAutoScroll = shouldAutoScroll || (c.AutoScroll && !c.UserScrolled && c.Streaming)

	c.rebuildContentIfDirty()
	c.updateViewportHeight()

	// Auto-scroll after viewport height is finalized
	if shouldAutoScroll {
		c.Viewport.GotoBottom()
	}

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

	for i, msg := range c.Messages {
		rendered := c.renderMessage(msg, maxBubbleWidth, themeStyles)
		chatContent += rendered
		if i < len(c.Messages)-1 {
			chatContent += "\n"
		}
	}
	c.CachedContent = chatContent
	c.ContentDirty = false
	c.Viewport.SetContent(c.CachedContent)
}

func (c *ChatModel) updateViewportHeight() {
	// Calculate input height
	// When focused: border adds 2 lines (top + bottom) + 1 line for input = 3
	// When unfocused: 1 line for input + 1 line for tooltip = 2
	inputHeight := 3
	if !c.TextInput.Focused() {
		inputHeight = 2
	}

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
}

func (c *ChatModel) renderMessage(msg llm.Message, maxBubbleWidth int, themeStyles ThemeStyles) string {
	var bubbleStyle lipgloss.Style
	var innerWidth int
	var processedContent string
	var wrapperBg lipgloss.Color

	switch msg.Role {
	case "user":
		bubbleStyle = themeStyles.UserMessage
		innerWidth = maxBubbleWidth - bubbleStyle.GetHorizontalFrameSize()
		processedContent = msg.Content
		wrapperBg = lipgloss.Color(c.Theme.Theme.Primary.Background)
	case "assistant":
		bubbleStyle = themeStyles.AssistantMessage
		innerWidth = maxBubbleWidth - bubbleStyle.GetHorizontalFrameSize()
		cleanedContent := llm.StripTextBasedFunctionCalls(msg.Content)
		processedContent = llm.RenderWithSyntaxHighlighting(cleanedContent, innerWidth, themeStyles.CodeBlockBadge, themeStyles.CodeBlockContainer)
		wrapperBg = lipgloss.Color(c.Theme.Theme.Primary.Background)
	case "tool":
		bubbleStyle = themeStyles.ToolMessage
		innerWidth = maxBubbleWidth - bubbleStyle.GetHorizontalFrameSize()
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
		processedContent = toolHeader + "\n" + displayContent
		wrapperBg = lipgloss.Color(c.Theme.Theme.Dim.Black)
	default:
		return msg.Content
	}

	wrapperStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Background(wrapperBg)
	wrappedContent := wrapperStyle.Render(processedContent)
	bubble := bubbleStyle.Render(wrappedContent)
	return c.padLinesToWidth(bubble, c.Width)
}

func (c *ChatModel) padLinesToWidth(content string, width int) string {
	bg := lipgloss.Color(c.Theme.Theme.Primary.Background)
	bgStyle := lipgloss.NewStyle().Background(bg)
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < width {
			// Add background-colored spaces to fill the remaining width
			padding := width - lineWidth
			lines[i] = line + bgStyle.Render(strings.Repeat(" ", padding))
		}
	}
	return strings.Join(lines, "\n")
}

func (c *ChatModel) View() string {
	c.rebuildContentIfDirty()

	themeStyles := GetThemeStyles(c.Theme)
	inputStyle := lipgloss.NewStyle().Background(lipgloss.Color(c.Theme.Theme.Primary.Background))
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
