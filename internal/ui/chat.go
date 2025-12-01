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
		logger.Debug("[CHAT] Rendering %d messages, c.Width=%d", len(c.Messages), c.Width)
		chatContent := ""
		maxBubbleWidth := int(float64(c.Width) * 0.95)
		maxInnerTextWidth := maxBubbleWidth - themeStyles.AssistantMessage.GetHorizontalFrameSize()

		padLinesToWidth := func(text string, targetWidth int, bgColor lipgloss.Color) string {
			lines := strings.Split(text, "\n")
			var paddedLines []string
			for _, line := range lines {
				lineWidth := lipgloss.Width(line)
				if lineWidth < targetWidth {
					padStyle := lipgloss.NewStyle().
						Width(targetWidth).
						Background(bgColor).
						Align(lipgloss.Left)
					line = padStyle.Render(line)
				}
				paddedLines = append(paddedLines, line)
			}
			return strings.Join(paddedLines, "\n")
		}

		for i, msg := range c.Messages {
			var rendered string
			switch msg.Role {
			case "user":
				userInnerWidth := maxBubbleWidth - themeStyles.UserMessage.GetHorizontalFrameSize()
				wrappedContent := lipgloss.NewStyle().Width(userInnerWidth).Render(msg.Content)
				bubble := themeStyles.UserMessage.Render(wrappedContent)
				bubble = padLinesToWidth(bubble, c.Width, lipgloss.Color(c.Theme.Theme.Primary.Background))
				rendered = bubble
			case "assistant":
				contentWithHighlighting := llm.RenderWithSyntaxHighlighting(msg.Content, maxInnerTextWidth, themeStyles.CodeBlockBadge, themeStyles.CodeBlockContainer)
				bubble := themeStyles.AssistantMessage.Render(contentWithHighlighting)
				wrapper := lipgloss.NewStyle().
					Width(c.Width).
					Background(lipgloss.Color(c.Theme.Theme.Primary.Background)).
					Align(lipgloss.Right)
				rendered = wrapper.Render(bubble)
			case "tool":
				// Display tool execution results with styling
				toolInnerWidth := maxBubbleWidth - themeStyles.ToolMessage.GetHorizontalFrameSize()

				// Strip ANSI escape codes from tool message content
				cleanContent := ansiRegex.ReplaceAllString(msg.Content, "")

				// Extract language information if present
				languageIcon := "⚙️"
				header := "Execution Result"

				// Check if this is a code execution result
				if strings.Contains(cleanContent, "code block") && strings.Contains(cleanContent, "executed") {
					header = "Code Execution Complete"
					// Try to detect language from context
					if strings.Contains(cleanContent, "(bash)") || strings.Contains(strings.ToLower(cleanContent), "bash") {
						languageIcon = "🐚"
					} else if strings.Contains(strings.ToLower(cleanContent), "python") {
						languageIcon = "🐍"
					} else if strings.Contains(strings.ToLower(cleanContent), "javascript") || strings.Contains(strings.ToLower(cleanContent), "node") {
						languageIcon = "📜"
					}
				}

				// Render the content with markdown (for code blocks in output)
				displayContent := cleanContent
				const maxToolOutputLength = 1000
				if len(displayContent) > maxToolOutputLength {
					displayContent = displayContent[:maxToolOutputLength] + "\n... (output truncated)"
				}

				wrappedContent := lipgloss.NewStyle().Width(toolInnerWidth).Render(displayContent)
				toolHeader := themeStyles.ToolBadge.Render(languageIcon + " " + header)
				bubble := themeStyles.ToolMessage.Render(toolHeader + "\n" + wrappedContent)
				wrapper := lipgloss.NewStyle().
					Width(c.Width).
					Background(lipgloss.Color(c.Theme.Theme.Primary.Background)).
					Align(lipgloss.Right)
				rendered = wrapper.Render(bubble)
			default:
				rendered = msg.Content
			}
			if i > 0 {
				chatContent += "\n"
			}
			chatContent += rendered
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
		logger.Debug("ChatModel.View: Input NOT focused, tooltip added, new inputHeight=%d", inputHeight)
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
		logger.Debug("[CHAT] Initial scroll to bottom: YOffset=%d, Height=%d, TotalLines=%d",
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
