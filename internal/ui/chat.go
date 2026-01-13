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
	// Rebuild content before auto-scrolling to ensure TotalLineCount() is accurate
	if c.ContentDirty {
		c.rebuildContentIfDirty()
	}

	shouldAutoScroll := c.AutoScroll && !c.UserScrolled
	if shouldAutoScroll {
		c.Viewport.GotoBottom()
	}

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

	for i, msg := range c.Messages {
		var rendered string
		switch msg.Role {
		case "user":
			// Width() on bordered styles gives Width + 2, so subtract 2 to get desired total width
			bubbleWidth := c.Width - 2
			wrappedContent := msg.Content
			bubble := themeStyles.UserMessage.Width(bubbleWidth).Render(wrappedContent)
			// Pad each line to full chat width
			rendered = c.padLinesToWidth(bubble, c.Width)
		case "assistant":
			bubbleWidth := c.Width - 2
			cleanedContent := llm.StripTextBasedFunctionCalls(msg.Content)
			contentWithHighlighting := llm.RenderWithSyntaxHighlighting(cleanedContent, bubbleWidth, themeStyles.CodeBlockBadge, themeStyles.CodeBlockContainer)
			bubble := themeStyles.AssistantMessage.Width(bubbleWidth).Render(contentWithHighlighting)
			// Pad each line to full chat width
			rendered = c.padLinesToWidth(bubble, c.Width)
		case "tool":
			bubbleWidth := c.Width - 2
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
			bubble := themeStyles.ToolMessage.Width(bubbleWidth).Render(toolHeader + "\n" + displayContent)
			// Pad each line to full chat width
			rendered = c.padLinesToWidth(bubble, c.Width)
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

func (c *ChatModel) padLinesToWidth(content string, width int) string {
	themeStyles := GetThemeStyles(c.Theme)

	lines := strings.Split(content, "\n")

	for i, line := range lines {
		visualWidth := lipgloss.Width(line)
		if visualWidth < width {
			paddingSize := width - visualWidth
			paddedLine := themeStyles.BackgroundWrapper.Width(paddingSize).Render(line)
			lines[i] = paddedLine
		}
	}
	return strings.Join(lines, "\n")
}

func (c *ChatModel) View() string {
	c.rebuildContentIfDirty()

	themeStyles := GetThemeStyles(c.Theme)
	inputStyle := themeStyles.InputUnfocused
	if c.TextInput.Focused() {
		inputStyle = themeStyles.InputFocused
	}

	scrollIndicator := ""
	if c.Viewport.TotalLineCount() > 0 {
		if !c.Viewport.AtTop() && !c.Viewport.AtBottom() {
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
				scrollWrapper := themeStyles.BackgroundWrapper.Width(c.Width).Align(lipgloss.Center)
				scrollIndicator = scrollWrapper.Render(scrollText)
			}
		}
	}

	spinnerView := ""
	if c.Streaming {
		spinnerText := c.Spinner.View() + " Generating..."
		spinnerView = themeStyles.BackgroundWrapper.Width(c.Width).Render(spinnerText)
	}

	inputFieldRendered := inputStyle.Render(c.TextInput.View())
	inputWrapper := themeStyles.BackgroundWrapper.Width(c.Width).Align(lipgloss.Left)
	inputFieldRendered = inputWrapper.Render(inputFieldRendered)

	if !c.TextInput.Focused() {
		tooltip := themeStyles.Tooltip.Render("Ctrl+T: switch panes | Ctrl+H: help | Ctrl+D: theme | Ctrl+Q: quit")
		inputFieldRendered = lipgloss.JoinVertical(lipgloss.Left, inputFieldRendered, tooltip)
	}

	// Viewport height already set in Update() via updateViewportHeight()
	var joined string
	var parts []string

	viewportContent := c.Viewport.View()
	parts = append(parts, viewportContent)
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
