package ui

import (
	"bufio"
	"clai/internal/db"
	"clai/internal/debug"
	"clai/internal/llm"
	"clai/internal/logger"
	"clai/internal/ralph"
	"clai/internal/tools"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ActivePane int

const (
	BriefingRoom ActivePane = iota
	ChatPane
	LogPane
)

type Stage int

const (
	StageIdle Stage = iota
	StageThinking
	StageExecuting
	StageVerifying
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatLogLine(line string, theme *glitter.UI, width int) string {
	if len(line) < 9 {
		return line
	}

	var level string
	var message string

	if len(line) > 15 && line[8] == ' ' {
		rest := line[9:]
		if len(rest) > 7 && rest[0] == '[' {
			if rest[1:8] == "DEBUG] " {
				level = "DEBUG"
				message = rest[8:]
			} else if len(rest) > 6 && rest[1:7] == "INFO] " {
				level = "INFO"
				message = rest[7:]
			} else if len(rest) > 6 && rest[1:7] == "WARN] " {
				level = "WARN"
				message = rest[7:]
			} else if len(rest) > 7 && rest[1:8] == "ERROR] " {
				level = "ERROR"
				message = rest[8:]
			} else {
				return line
			}
		} else {
			return line
		}
	} else {
		return line
	}

	var color lipgloss.Color
	switch level {
	case "DEBUG":
		color = lipgloss.Color(theme.Theme.Normal.Cyan)
	case "INFO":
		color = lipgloss.Color(theme.Theme.Normal.Green)
	case "WARN":
		color = lipgloss.Color(theme.Theme.Normal.Yellow)
	case "ERROR":
		color = lipgloss.Color(theme.Theme.Normal.Red)
	default:
		return line
	}

	style := lipgloss.NewStyle().
		Foreground(color).
		Background(lipgloss.Color(theme.Theme.Primary.Background))

	rendered := style.Render(message)

	if lipgloss.Width(rendered) < width {
		wrapper := lipgloss.NewStyle().
			Width(width).
			Background(lipgloss.Color(theme.Theme.Primary.Background))
		rendered = wrapper.Render(rendered)
	}

	return rendered
}

type Model struct {
	Chat          ChatModel
	Log           viewport.Model
	logBuffer     string
	AgentStatus   *AgentStatusView
	Err           error
	Width         int
	Height        int
	Help          help.Model
	Keys          KeyMap
	StatusBarText string
	WebUIPort     int
	ShowHelp      bool
	ActivePane    ActivePane
	ErrorBanner   lipgloss.Style
	ErrorMessage  string
	ShowError     bool
	Theme         *glitter.UI
	Layout        LayoutConfig
	cachedStyles  *ThemeStyles
	logChan       chan tea.Msg
	logDone       chan struct{}
	DB            interface{}
	Conversation  interface{}
	Agent         *llm.Agent
	statusChan    chan tea.Msg

	// Ralph orchestrator fields
	stage         Stage
	prd           *ralph.PRD
	logs          []string
	debugServer   *debug.Server
	activeStoryID string
	viewport      viewport.Model
	ctx           context.Context
	cancel        context.CancelFunc
	cursor        int
	llmHost       string
	llmModel      string
	llmHealth     bool
}

type (
	LogUpdateMsg       string
	LLMResponseMsg     struct{ Resp llm.Response }
	StreamUpdateMsg    string
	CodeBlockMsg       struct{ Blocks []llm.CodeBlock }
	TickMsg            struct{}
	HealthCheckMsg     struct{ Err error }
	HealthCheckDoneMsg struct{}
	errorMsg           struct{ err error }
	clearErrorMsg      struct{}
	smoothScrollMsg    struct{}
	AgentStatusMsg     struct {
		Status AgentStatus
		Code   string
	}
	WebUIPortMsg      struct{ Port int }
	prdLoadedMsg      *ralph.PRD
	prdErrorMsg       error
	prdFileChangedMsg struct{}
	healthMsg         bool
	logMsg            struct {
		storyID string
		content string
		gitHash string
	}
	patternMsg string
)

func executeCodeBlocksCmd(blocks []llm.CodeBlock) tea.Cmd {
	return func() tea.Msg {
		var results []string
		for i, block := range blocks {
			logger.Info("Executing code block %d/%d (language=%s)", i+1, len(blocks), block.Language)
			output, err := tools.ExecuteCode(block.Language, block.Code)

			if err != nil {
				results = append(results, fmt.Sprintf("Code block %d (error):\n```\n%s\n```\nError: %v", i+1, output, err))
			} else {
				truncated := tools.TruncateForHistory(output)
				results = append(results, fmt.Sprintf("Code block %d output:\n```\n%s\n```", i+1, truncated))
			}
		}

		combined := fmt.Sprintf("%d code block(s) executed:\n\n%s", len(blocks), fmt.Sprintf("%s", results[0]))
		if len(results) > 1 {
			for i := 1; i < len(results); i++ {
				combined += "\n\n" + results[i]
			}
		}

		return CodeBlockMsg{Blocks: blocks}
	}
}

func smoothScrollCmd() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg {
		return smoothScrollMsg{}
	})
}

func TailLogFileCmd(m *Model) tea.Cmd {
	m.logChan = make(chan tea.Msg)
	m.logDone = make(chan struct{})
	go tailLogFile(m.logChan, m.logDone)
	return readLogChanCmd(m.logChan)
}

func tailLogFile(logChan chan<- tea.Msg, done <-chan struct{}) {
	// Truncate debug.log on startup to avoid reading old content
	os.WriteFile("debug.log", []byte{}, 0644)

	f, err := os.Open("debug.log")
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			logChan <- LogUpdateMsg(scanner.Text())
		}
		f.Close()
	}
	t, err := os.Stat("debug.log")
	var offset int64 = 0
	if err == nil {
		offset = t.Size()
	}
	for {
		select {
		case <-done:
			return
		default:
			file, err := os.Open("debug.log")
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
			file.Seek(offset, 0)
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				offset += int64(len(line)) + 1
				select {
				case logChan <- LogUpdateMsg(line):
				case <-done:
					file.Close()
					return
				}
			}
			file.Close()
			time.Sleep(1 * time.Second)
		}
	}
}

func readLogChanCmd(logChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-logChan }
}

func readStatusChanCmd(statusChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-statusChan
	}
}

func StartsWithLLMError(s string) bool {
	return len(s) >= 11 && s[:11] == "[LLM ERROR]"
}

func getThemeName(currentTheme *glitter.UI) string {
	availableThemes := GetAvailableThemes()
	themeNames := GetAvailableThemeNames()
	for i, theme := range availableThemes {
		if theme == currentTheme {
			if i < len(themeNames) {
				return themeNames[i]
			}
			break
		}
	}
	return "Unknown"
}

func (m *Model) Init() tea.Cmd {
	m.statusChan = make(chan tea.Msg, 100)
	m.Layout = DefaultLayoutConfig()
	m.updateCachedStyles()
	return tea.Batch(
		TailLogFileCmd(m),
		m.Chat.Init(),
		tea.WindowSize(),
		m.loadStoriesCmd(),
		m.watchStoriesCmd(),
		m.checkHealthCmd(),
	)
}

func (m *Model) loadStoriesCmd() tea.Cmd {
	return func() tea.Msg {
		stories, err := ralph.LoadStories(".clai/stories.json")
		if err != nil {
			return prdErrorMsg(err)
		}
		return prdLoadedMsg(&ralph.PRD{
			Project:     "CLAI",
			BranchName:  "main",
			Description: "CLAI Development Tasks",
			UserStories: stories.Stories,
		})
	}
}

func (m *Model) watchStoriesCmd() tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement file watching for stories.json
		return nil
	}
}

func (m *Model) checkHealthCmd() tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement LLM health check
		return healthMsg(true)
	}
}

func (m *Model) updateDimensions() {
	chatPaneWidth := int(float64(m.Width) * m.Layout.ChatPaneWidthRatio)
	logPaneWidth := m.Width - chatPaneWidth

	themeStyles := GetThemeStyles(m.Theme)
	chatPaneStyle := themeStyles.MainPane
	logPaneStyle := themeStyles.MainPane

	contentHeight := m.Height - 1
	if m.ShowError && m.ErrorMessage != "" {
		contentHeight -= m.ErrorBanner.GetHeight()
	}

	m.Chat.Width = max(chatPaneWidth-chatPaneStyle.GetHorizontalFrameSize(), m.Layout.MinViewportWidth)

	// Log pane contains both agent status and log viewport
	logPaneInnerWidth := max(logPaneWidth-logPaneStyle.GetHorizontalFrameSize(), m.Layout.MinPaneWidth)
	logPaneInnerHeight := max(contentHeight-logPaneStyle.GetVerticalFrameSize(), m.Layout.MinViewportHeight)

	agentStatusHeight := m.Layout.AgentStatusPaneHeight
	m.AgentStatus.Width = logPaneInnerWidth
	m.AgentStatus.Height = agentStatusHeight

	m.Log.Width = logPaneInnerWidth
	m.Log.Height = max(logPaneInnerHeight-agentStatusHeight, m.Layout.MinViewportHeight)

	m.Chat.Height = max(contentHeight-chatPaneStyle.GetVerticalFrameSize(), m.Layout.MinViewportHeight)

	m.Chat.Viewport.Width = m.Chat.Width
	// Note: Viewport.Height is set by chat.updateViewportHeight() in chat.Update()
	// to account for input field, spinner, scroll indicator, etc.
	m.Chat.Textarea.SetWidth(m.Chat.Width - m.Layout.TextareaPadding)
}

func (m *Model) updateCachedStyles() {
	styles := GetThemeStyles(m.Theme)
	m.cachedStyles = &styles
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		cmds = append(cmds, m.handleWindowSizeMsg(msg))
	case tea.KeyMsg:
		keyStr := msg.String()
		if len(keyStr) > 2 && (keyStr[:3] == "alt" ||
			(keyStr[0] >= '0' && keyStr[0] <= '9' && len(keyStr) > 1 && keyStr[1] == ';')) {
			return m, nil
		}
		cmds = append(cmds, m.handleKeyMsg(msg))
	case StreamUpdateMsg:
		chunk := string(msg)
		if len(chunk) == 0 {
			m.Chat.Streaming = false

			// Save conversation after assistant response completes
			if conv, ok := m.Conversation.(*db.Conversation); ok {
				conv.Messages = m.Chat.Messages
				if store, ok := m.DB.(*db.Store); ok {
					if err := store.SaveConversation(conv); err != nil {
						logger.Info("[DB] Failed to save conversation: %v", err)
					}
				}
			}

			// Check if last message contains code blocks
			if len(m.Chat.Messages) > 0 {
				lastMsg := m.Chat.Messages[len(m.Chat.Messages)-1]
				if lastMsg.Role == "assistant" {
					blocks := llm.ParseCodeBlocks(lastMsg.Content)
					if len(blocks) > 0 {
						logger.Info("Detected %d code blocks in completed message", len(blocks))
						if len(blocks) > 0 {
							m.AgentStatus.SetExecutingCode(blocks[0].Language, blocks[0].Code)
						}
						return m, executeCodeBlocksCmd(blocks)
					}
				}
			}

			break
		}
		// Find the last assistant message (may not be the very last message if tool messages exist)
		lastAssistantIdx := -1
		for i := len(m.Chat.Messages) - 1; i >= 0; i-- {
			if m.Chat.Messages[i].Role == "assistant" {
				lastAssistantIdx = i
				break
			}
		}

		if lastAssistantIdx >= 0 && m.Chat.Streaming && !StartsWithLLMError(chunk) {
			m.Chat.Messages[lastAssistantIdx].Content += chunk
			m.Chat.ContentDirty = true
		} else if chunk != "" {
			m.Chat.Messages = append(m.Chat.Messages, llm.Message{Role: "assistant", Content: chunk})
			m.Chat.ContentDirty = true
		}
		if StartsWithLLMError(chunk) {
			m.Chat.Streaming = false
		}
	case CodeBlockMsg:
		logger.Debug("CodeBlockMsg received: %d blocks", len(msg.Blocks))

		// Strip code tags from last assistant message
		if len(m.Chat.Messages) > 0 && m.Chat.Messages[len(m.Chat.Messages)-1].Role == "assistant" {
			m.Chat.Messages[len(m.Chat.Messages)-1].Content = llm.StripCodeTags(m.Chat.Messages[len(m.Chat.Messages)-1].Content)
		}

		// Execute code blocks sequentially and collect results
		var results []string
		for i, block := range msg.Blocks {
			logger.Info("Executing code block %d/%d (language=%s)", i+1, len(msg.Blocks), block.Language)

			// Update agent status to show code execution
			if i > 0 {
				m.AgentStatus.SetExecutingCode(block.Language, block.Code)
			}

			output, err := tools.ExecuteCode(block.Language, block.Code)

			if err != nil {
				m.AgentStatus.FailCodeExecution(fmt.Sprintf("Error: %v\n%s", err, output))
				results = append(results, fmt.Sprintf("Code block %d (error):\n```\n%s\n```\nError: %v", i+1, output, err))
			} else {
				m.AgentStatus.CompleteCodeExecution(output)
				truncated := tools.TruncateForHistory(output)
				results = append(results, fmt.Sprintf("Code block %d output:\n```\n%s\n```", i+1, truncated))
			}
		}

		// Combine all results
		combined := fmt.Sprintf("%d code block(s) executed:\n\n%s", len(msg.Blocks), results[0])
		if len(results) > 1 {
			for i := 1; i < len(results); i++ {
				combined += "\n\n" + results[i]
			}
		}

		// Add code result as tool message
		m.Chat.Messages = append(m.Chat.Messages, llm.Message{
			Role:    "tool",
			Content: combined,
		})
		m.Chat.ContentDirty = true

		// Save conversation
		if conv, ok := m.Conversation.(*db.Conversation); ok {
			conv.Messages = m.Chat.Messages
			if store, ok := m.DB.(*db.Store); ok {
				if err := store.SaveConversation(conv); err != nil {
					logger.Info("[DB] Failed to save conversation: %v", err)
				}
			}
		}

		// Send updated conversation back to agent for final response
		m.Chat.Streaming = true
		return m, tea.Batch(RunAgentCmd(m.Agent, "continue", m.statusChan), readStatusChanCmd(m.statusChan))
	case smoothScrollMsg:
		if m.Chat.SmoothScrollActive {
			currentOffset := m.Chat.Viewport.YOffset
			targetOffset := m.Chat.SmoothScrollTarget

			if currentOffset == targetOffset {
				m.Chat.SmoothScrollActive = false
				return m, nil
			}

			diff := targetOffset - currentOffset
			step := 1
			if diff < 0 {
				step = -1
			}
			if diff > 5 || diff < -5 {
				step = diff / 5
			}

			newOffset := currentOffset + step
			m.Chat.Viewport.SetYOffset(newOffset)

			return m, smoothScrollCmd()
		}
		return m, nil
	case LogUpdateMsg:
		formattedLine := formatLogLine(string(msg), m.Theme, m.Log.Width)
		m.logBuffer += formattedLine + "\n"
		m.Log.SetContent(m.logBuffer)
		m.Log.GotoBottom()
		return m, readLogChanCmd(m.logChan)
	case errorMsg:
		m.ErrorMessage = msg.err.Error()
		m.ShowError = true
		return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return clearErrorMsg{} })
	case clearErrorMsg:
		m.ShowError = false
		m.ErrorMessage = ""
		return m, nil
	case DebugServerMsg:
		cmds = append(cmds, m.handleDebugCommand(msg))
	case AgentResponseMsg:
		m.Chat.Streaming = false
		if msg.Err != nil {
			logger.Error("[AGENT-RESPONSE] Agent error: %v", msg.Err)
			m.Chat.Messages = append(m.Chat.Messages, llm.Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Agent error: %v", msg.Err),
			})
			m.AgentStatus.FailCurrentAction()
			m.AgentStatus.CompleteCurrentTask()
		} else {
			logger.Info("[AGENT-RESPONSE] Agent completed: %s", msg.Response)
			m.Chat.Messages = append(m.Chat.Messages, llm.Message{
				Role:    "assistant",
				Content: msg.Response,
			})
			m.AgentStatus.CompleteCurrentAction()
			m.AgentStatus.CompleteCurrentTask()
		}
		m.Chat.ContentDirty = true

		// Save conversation after agent response
		if conv, ok := m.Conversation.(*db.Conversation); ok {
			conv.Messages = m.Chat.Messages
			if store, ok := m.DB.(*db.Store); ok {
				if err := store.SaveConversation(conv); err != nil {
					logger.Info("[DB] Failed to save conversation: %v", err)
				}
			}
		}
	case AgentStatusMsg:
		// Handle agent status updates
		logger.Debug("[AGENT-STATUS] Iteration: %d, Executing: %t", msg.Status.CurrentIter, msg.Status.ExecutingCode)
		m.AgentStatus.Update(msg.Status)
		if msg.Code != "" && msg.Status.ExecutingCode {
			m.AgentStatus.SetExecutingCode(msg.Status.CodeLanguage, msg.Code)
		}
		return m, readStatusChanCmd(m.statusChan)
	case prdLoadedMsg:
		m.prd = msg
		logger.Info("Loaded PRD with %d stories", len(msg.UserStories))
	case WebUIPortMsg:
		m.WebUIPort = msg.Port
		// Update status bar text with the new port
		themeName := getThemeName(m.Theme)
		m.StatusBarText = fmt.Sprintf("Model: %s | Host: %s | Format: %s | Theme: %s | Web UI: http://localhost:%d",
			m.Chat.LlmClient.Model(),
			m.Chat.LlmClient.Host(),
			m.Chat.LlmClient.APIFormatString(),
			themeName,
			m.WebUIPort)
	default:
		var cmd tea.Cmd
		updatedChat, cmd := m.Chat.Update(msg)
		m.Chat = updatedChat
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	var cmds []tea.Cmd

	keyStr := msg.String()
	if len(keyStr) > 0 && (keyStr[0] == '\x1b' ||
		(len(keyStr) > 2 && keyStr[:3] == "alt") ||
		(len(keyStr) > 1 && (keyStr[0] >= '0' && keyStr[0] <= '9') && keyStr[1] == ';') ||
		(len(keyStr) > 3 && keyStr[0] >= '0' && keyStr[0] <= '9' && keyStr[1] >= '0' && keyStr[1] <= '9' && keyStr[2] == ';')) {
		return nil
	}

	// Handle global hotkeys that work regardless of focus
	switch msg.String() {
	case "ctrl+c":
		// Signal log tailer to stop
		if m.logDone != nil {
			close(m.logDone)
		}
		return tea.Quit
	case "enter":
		if m.Chat.Textarea.Focused() {
			userMsg := m.Chat.Textarea.Value()
			if userMsg != "" {
				m.Chat.Messages = append(m.Chat.Messages, llm.Message{Role: "user", Content: userMsg})
				m.Chat.ContentDirty = true
				m.Chat.AutoScroll = true
				m.Chat.UserScrolled = false
				m.Chat.Viewport.GotoBottom()
				m.Chat.Textarea.SetValue("")
				m.Chat.Streaming = true
				m.Chat.QueryHistory = append(m.Chat.QueryHistory, userMsg)
				m.Chat.HistoryIndex = 0

				// Update conversation with new message
				if conv, ok := m.Conversation.(*db.Conversation); ok {
					conv.Messages = m.Chat.Messages
					if conv.Title == "" || conv.Title == "New Conversation" {
						conv.Title = db.GenerateConversationTitle(m.Chat.Messages)
					}
					if store, ok := m.DB.(*db.Store); ok {
						if err := store.SaveConversation(conv); err != nil {
							logger.Info("[DB] Failed to save conversation: %v", err)
						}
					}
				}

				if m.Agent != nil {
					logger.Info("[AGENT-MODE] Running query through agent: %s", userMsg)
					return tea.Batch(RunAgentCmd(m.Agent, userMsg, m.statusChan), readStatusChanCmd(m.statusChan))
				}

				logger.Warn("[AGENT-MODE] Agent is nil, cannot process query")
			}
		}
	}

	// Handle hotkeys with ctrl modifier (work even when input is focused)
	switch msg.String() {
	case "ctrl+q":
		// Save conversation before quitting
		if conv, ok := m.Conversation.(*db.Conversation); ok {
			if len(m.Chat.Messages) > 0 {
				conv.Messages = m.Chat.Messages
				if store, ok := m.DB.(*db.Store); ok {
					if err := store.SaveConversation(conv); err != nil {
						logger.Info("[DB] Failed to save conversation: %v", err)
					}
				}
			}
		}
		// Signal log tailer to stop
		if m.logDone != nil {
			close(m.logDone)
		}
		return tea.Quit
	case "ctrl+h":
		m.ShowHelp = !m.ShowHelp
		m.Help.ShowAll = true
		return nil
	case "ctrl+t":
		if m.ActivePane == BriefingRoom {
			m.ActivePane = ChatPane
		} else if m.ActivePane == ChatPane {
			m.ActivePane = BriefingRoom
		}
		return nil
	case "ctrl+l":
		if m.ActivePane == LogPane {
			m.ActivePane = BriefingRoom // Go back to main view
		} else {
			m.ActivePane = LogPane // Open log viewer
		}
		return nil
	case "ctrl+d":
		availableThemes := GetAvailableThemes()
		currentIdx := 0
		for i, theme := range availableThemes {
			if theme == m.Theme {
				currentIdx = i
				break
			}
		}
		nextIdx := (currentIdx + 1) % len(availableThemes)
		m.Theme = availableThemes[nextIdx]
		m.Chat.Theme = m.Theme
		m.Chat.ContentDirty = true
		m.updateCachedStyles()
		m.Help.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color(m.Theme.Theme.Primary.Foreground))
		m.Help.Styles.FullDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(m.Theme.Theme.Primary.DimForeground))
		m.Help.Styles.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color(m.Theme.Theme.Primary.DimForeground))
		themeName := getThemeName(m.Theme)
		m.StatusBarText = fmt.Sprintf("Model: %s | Host: %s | Format: %s | Theme: %s | Web UI: http://localhost:%d",
			m.Chat.LlmClient.Model(),
			m.Chat.LlmClient.Host(),
			m.Chat.LlmClient.APIFormatString(),
			themeName,
			m.WebUIPort)
		return nil
	case "ctrl+n":
		// Save current conversation before creating new one
		if conv, ok := m.Conversation.(*db.Conversation); ok {
			if len(m.Chat.Messages) > 0 {
				conv.Messages = m.Chat.Messages
				if store, ok := m.DB.(*db.Store); ok {
					if err := store.SaveConversation(conv); err != nil {
						logger.Info("[DB] Failed to save conversation: %v", err)
					}
				}
			}
		}

		// Create new conversation and save it immediately
		newConv := &db.Conversation{
			Title:    "New Conversation",
			Messages: []llm.Message{},
		}
		if store, ok := m.DB.(*db.Store); ok {
			if err := store.SaveConversation(newConv); err != nil {
				logger.Warn("[DB] Failed to save new conversation: %v", err)
			} else {
				logger.Info("[DB] Created new conversation with ID %d", newConv.ID)
			}
		}
		m.Conversation = newConv

		m.Chat.Messages = []llm.Message{}
		m.Chat.QueryHistory = []string{}
		m.Chat.HistoryIndex = 0
		m.Chat.ContentDirty = true
		m.Chat.Textarea.SetValue("")
		m.Chat.Viewport.GotoTop()
		m.Chat.UserScrolled = false
		m.Chat.AutoScroll = true

		// Force chat to rebuild content
		updatedChat, _ := m.Chat.Update(nil)
		m.Chat = updatedChat

		return nil
	case "up":
		if m.Chat.Textarea.Focused() {
			if len(m.Chat.QueryHistory) > 0 && m.Chat.HistoryIndex < len(m.Chat.QueryHistory) {
				m.Chat.HistoryIndex++
				m.Chat.Textarea.SetValue(m.Chat.QueryHistory[len(m.Chat.QueryHistory)-m.Chat.HistoryIndex])
				m.Chat.Textarea.CursorEnd()
			}
		} else {
			m.Chat.SmoothScrollTarget = max(m.Chat.Viewport.YOffset-1, 0)
			m.Chat.SmoothScrollActive = true
			m.Chat.UserScrolled = true
			m.Chat.AutoScroll = false
			return smoothScrollCmd()
		}
	case "down":
		if m.Chat.Textarea.Focused() {
			if m.Chat.HistoryIndex > 1 {
				m.Chat.HistoryIndex--
				m.Chat.Textarea.SetValue(m.Chat.QueryHistory[len(m.Chat.QueryHistory)-m.Chat.HistoryIndex])
				m.Chat.Textarea.CursorEnd()
			} else if m.Chat.HistoryIndex == 1 {
				m.Chat.HistoryIndex--
				m.Chat.Textarea.SetValue("")
			}
		} else {
			maxOffset := max(m.Chat.Viewport.TotalLineCount()-m.Chat.Viewport.Height, 0)
			m.Chat.SmoothScrollTarget = max(0, min(m.Chat.Viewport.YOffset+1, maxOffset))
			m.Chat.SmoothScrollActive = true
			if m.Chat.Viewport.AtBottom() {
				m.Chat.UserScrolled = false
				m.Chat.AutoScroll = true
			}
			return smoothScrollCmd()
		}
	case "k":
		if !m.Chat.Textarea.Focused() {
			m.Chat.SmoothScrollTarget = max(m.Chat.Viewport.YOffset-1, 0)
			m.Chat.SmoothScrollActive = true
			m.Chat.UserScrolled = true
			m.Chat.AutoScroll = false
			return smoothScrollCmd()
		}
	case "j":
		if !m.Chat.Textarea.Focused() {
			maxOffset := max(m.Chat.Viewport.TotalLineCount()-m.Chat.Viewport.Height, 0)
			m.Chat.SmoothScrollTarget = max(0, min(m.Chat.Viewport.YOffset+1, maxOffset))
			m.Chat.SmoothScrollActive = true
			if m.Chat.Viewport.AtBottom() {
				m.Chat.UserScrolled = false
				m.Chat.AutoScroll = true
			}
			return smoothScrollCmd()
		}
	case "pgup":
		if !m.Chat.Textarea.Focused() {
			m.Chat.SmoothScrollTarget = max(m.Chat.Viewport.YOffset-m.Chat.Viewport.Height, 0)
			m.Chat.SmoothScrollActive = true
			m.Chat.UserScrolled = true
			m.Chat.AutoScroll = false
			return smoothScrollCmd()
		}
	case "pgdown":
		if !m.Chat.Textarea.Focused() {
			maxOffset := max(m.Chat.Viewport.TotalLineCount()-m.Chat.Viewport.Height, 0)
			m.Chat.SmoothScrollTarget = max(0, min(m.Chat.Viewport.YOffset+m.Chat.Viewport.Height, maxOffset))
			m.Chat.SmoothScrollActive = true
			if m.Chat.Viewport.AtBottom() {
				m.Chat.UserScrolled = false
				m.Chat.AutoScroll = true
			}
			return smoothScrollCmd()
		}
	case "home", "g":
		if !m.Chat.Textarea.Focused() {
			m.Chat.SmoothScrollTarget = 0
			m.Chat.SmoothScrollActive = true
			m.Chat.UserScrolled = true
			m.Chat.AutoScroll = false
			return smoothScrollCmd()
		}
	case "end", "G":
		if !m.Chat.Textarea.Focused() {
			maxOffset := max(m.Chat.Viewport.TotalLineCount()-m.Chat.Viewport.Height, 0)
			m.Chat.SmoothScrollTarget = maxOffset
			m.Chat.SmoothScrollActive = true
			m.Chat.UserScrolled = false
			m.Chat.AutoScroll = true
			return smoothScrollCmd()
		}
	}

	// Pass all other keys to chat component (for text input)
	var cmd tea.Cmd
	updatedChat, cmd := m.Chat.Update(msg)
	m.Chat = updatedChat
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}

func (m *Model) handleDebugCommand(msg DebugServerMsg) tea.Cmd {
	logger.Debug("[DEBUG] Handling command: %s", msg.Cmd.Command)

	var resp DebugResponse

	switch msg.Cmd.Command {
	case "ping":
		resp = DebugResponse{
			Success: true,
			Data: map[string]interface{}{
				"status": "ok",
			},
		}

	case "inspect":
		chatView := m.Chat.Viewport.View()
		fullChatView := m.Chat.View()
		fullView := m.View()
		helpContent := ""
		if m.ShowHelp {
			helpContent = m.Help.View(m.Keys)
		}
		resp = DebugResponse{
			Success: true,
			Data: map[string]interface{}{
				"viewport_content": chatView,
				"full_chat_view":   fullChatView,
				"full_view":        fullView,
				"width":            m.Width,
				"height":           m.Height,
				"chat_width":       m.Chat.Width,
				"chat_height":      m.Chat.Height,
				"message_count":    len(m.Chat.Messages),
				"viewport_offset":  m.Chat.Viewport.YOffset,
				"viewport_height":  m.Chat.Viewport.Height,
				"total_lines":      m.Chat.Viewport.TotalLineCount(),
				"active_pane":      m.ActivePane,
				"theme":            "current",
				"show_help":        m.ShowHelp,
				"help_show_all":    m.Help.ShowAll,
				"help_content":     helpContent,
			},
		}

	case "inspect_styles":
		resp = DebugResponse{
			Success: true,
			Data: map[string]interface{}{
				"width":           m.Width,
				"height":          m.Height,
				"chat_width":      m.Chat.Width,
				"chat_height":     m.Chat.Height,
				"message_count":   len(m.Chat.Messages),
				"viewport_offset": m.Chat.Viewport.YOffset,
				"viewport_height": m.Chat.Viewport.Height,
				"total_lines":     m.Chat.Viewport.TotalLineCount(),
				"active_pane":     m.ActivePane,
				"theme":           "current",
				"show_help":       m.ShowHelp,
				"help_show_all":   m.Help.ShowAll,
			},
		}

	case "get_history":
		resp = DebugResponse{
			Success: true,
			Data: map[string]interface{}{
				"messages": m.Chat.Messages,
			},
		}

	case "switch_pane":
		if m.ActivePane == ChatPane {
			m.ActivePane = LogPane
		} else {
			m.ActivePane = ChatPane
		}
		resp = DebugResponse{
			Success: true,
			Data: map[string]interface{}{
				"active_pane": m.ActivePane,
			},
		}

	case "send_message":
		role, roleOk := msg.Cmd.Args["role"].(string)
		content, contentOk := msg.Cmd.Args["content"].(string)
		if !roleOk || !contentOk {
			resp = DebugResponse{
				Success: false,
				Error:   "Missing required args: role and content",
			}
		} else {
			m.Chat.Messages = append(m.Chat.Messages, llm.Message{
				Role:    role,
				Content: content,
			})
			m.Chat.ContentDirty = true
			m.Chat.AutoScroll = true
			m.Chat.UserScrolled = false

			// Manually trigger scrolling since debug commands bypass chat.Update()
			if m.Chat.AutoScroll && !m.Chat.UserScrolled && m.Chat.ContentDirty {
				m.Chat.Viewport.GotoBottom()
			}

			if role == "user" {
				m.Chat.QueryHistory = append(m.Chat.QueryHistory, content)
			}

			resp = DebugResponse{
				Success: true,
				Data: map[string]interface{}{
					"message_count": len(m.Chat.Messages),
				},
			}
		}

	case "send_key":
		key, keyOk := msg.Cmd.Args["key"].(string)
		if !keyOk {
			resp = DebugResponse{
				Success: false,
				Error:   "Missing required arg: key",
			}
		} else {
			keyMsg := tea.KeyMsg{}
			keyMsg.Type = tea.KeyRunes

			switch key {
			case "ctrl+h":
				keyMsg.Type = tea.KeyCtrlH
			case "ctrl+q":
				keyMsg.Type = tea.KeyCtrlQ
			case "ctrl+c":
				keyMsg.Type = tea.KeyCtrlC
			case "ctrl+t":
				keyMsg.Type = tea.KeyCtrlT
			case "ctrl+d":
				keyMsg.Type = tea.KeyCtrlD
			case "ctrl+n":
				keyMsg.Type = tea.KeyCtrlN
			case "enter":
				keyMsg.Type = tea.KeyEnter
			case "up":
				keyMsg.Type = tea.KeyUp
			case "down":
				keyMsg.Type = tea.KeyDown
			case "left":
				keyMsg.Type = tea.KeyLeft
			case "right":
				keyMsg.Type = tea.KeyRight
			case "pgup":
				keyMsg.Type = tea.KeyPgUp
			case "pgdown":
				keyMsg.Type = tea.KeyPgDown
			case "home":
				keyMsg.Type = tea.KeyHome
			case "end":
				keyMsg.Type = tea.KeyEnd
			default:
				resp = DebugResponse{
					Success: false,
					Error:   fmt.Sprintf("Unknown key: %s", key),
				}
				SendDebugResponse(msg.Conn, resp)
				msg.Conn.Close()
				return nil
			}

			m, cmd := m.Update(keyMsg)
			logger.Info("[DEBUG] cmd is nil: %v", cmd == nil)
			if cmd != nil {
				logger.Info("[DEBUG] executing cmd")
				msg := cmd()
				logger.Info("[DEBUG] cmd executed, msg type: %T", msg)
				if bm, ok := msg.([]tea.Msg); ok {
					logger.Info("[DEBUG] batch msg with %d msgs", len(bm))
					for _, subMsg := range bm {
						logger.Info("[DEBUG] processing sub msg: %T", subMsg)
						m, _ = m.Update(subMsg)
					}
				} else {
					logger.Info("[DEBUG] single msg: %T", msg)
					m, _ = m.Update(msg)
				}
			}

			resp = DebugResponse{
				Success: true,
				Data: map[string]interface{}{
					"key_sent": key,
				},
			}
		}

	case "send_window_size":
		width, widthOk := msg.Cmd.Args["width"].(float64)
		height, heightOk := msg.Cmd.Args["height"].(float64)
		if !widthOk || !heightOk {
			resp = DebugResponse{
				Success: false,
				Error:   "Missing required args: width and height (must be numbers)",
			}
		} else {
			// Create and process a WindowSizeMsg
			windowMsg := tea.WindowSizeMsg{
				Width:  int(width),
				Height: int(height),
			}
			m, cmd := m.Update(windowMsg)
			logger.Info("[DEBUG] Processed window resize to %dx%d", int(width), int(height))
			if cmd != nil {
				logger.Info("[DEBUG] Window resize command executed")
				msg := cmd()
				m, _ = m.Update(msg)
			}

			resp = DebugResponse{
				Success: true,
				Data: map[string]interface{}{
					"width":  int(width),
					"height": int(height),
				},
			}
		}

	case "send_mouse":
		x, xOk := msg.Cmd.Args["x"].(float64)
		y, yOk := msg.Cmd.Args["y"].(float64)
		button, buttonOk := msg.Cmd.Args["button"].(string)
		action, actionOk := msg.Cmd.Args["action"].(string)
		if !xOk || !yOk || !buttonOk || !actionOk {
			resp = DebugResponse{
				Success: false,
				Error:   "Missing required args: x, y, button, action",
			}
		} else {
			// Create mouse event based on button and action
			var mouseMsg tea.MouseMsg
			mouseMsg.X = int(x)
			mouseMsg.Y = int(y)

			switch button {
			case "left":
				mouseMsg.Button = tea.MouseButtonLeft
			case "right":
				mouseMsg.Button = tea.MouseButtonRight
			case "middle":
				mouseMsg.Button = tea.MouseButtonMiddle
			case "wheel_up":
				mouseMsg.Button = tea.MouseButtonWheelUp
			case "wheel_down":
				mouseMsg.Button = tea.MouseButtonWheelDown
			default:
				resp = DebugResponse{
					Success: false,
					Error:   fmt.Sprintf("Unknown button: %s", button),
				}
				SendDebugResponse(msg.Conn, resp)
				msg.Conn.Close()
				return nil
			}

			switch action {
			case "press":
				mouseMsg.Action = tea.MouseActionPress
			case "release":
				mouseMsg.Action = tea.MouseActionRelease
			case "motion":
				mouseMsg.Action = tea.MouseActionMotion
			default:
				resp = DebugResponse{
					Success: false,
					Error:   fmt.Sprintf("Unknown action: %s", action),
				}
				SendDebugResponse(msg.Conn, resp)
				msg.Conn.Close()
				return nil
			}

			m, cmd := m.Update(mouseMsg)
			logger.Info("[DEBUG] Processed mouse event at (%d,%d) button=%s action=%s", int(x), int(y), button, action)
			if cmd != nil {
				logger.Info("[DEBUG] Mouse event command executed")
				msg := cmd()
				m, _ = m.Update(msg)
			}

			resp = DebugResponse{
				Success: true,
				Data: map[string]interface{}{
					"x":      int(x),
					"y":      int(y),
					"button": button,
					"action": action,
				},
			}
		}

	case "type_text":
		text, textOk := msg.Cmd.Args["text"].(string)
		if !textOk {
			resp = DebugResponse{
				Success: false,
				Error:   "Missing required arg: text",
			}
		} else {
			// Send each character as a key event
			for _, ch := range text {
				keyMsg := tea.KeyMsg{
					Type:  tea.KeyRunes,
					Runes: []rune{ch},
				}
				m, cmd := m.Update(keyMsg)
				if cmd != nil {
					msg := cmd()
					m, _ = m.Update(msg)
				}
			}
			resp = DebugResponse{
				Success: true,
				Data: map[string]interface{}{
					"chars_sent": len(text),
					"text":       text,
				},
			}
		}

	default:
		resp = DebugResponse{
			Success: false,
			Error:   fmt.Sprintf("Unknown command: %s", msg.Cmd.Command),
		}
	}

	SendDebugResponse(msg.Conn, resp)
	msg.Conn.Close()
	logger.Debug("[DEBUG] Response sent and connection closed for command: %s", msg.Cmd.Command)
	return nil
}

func (m *Model) handleWindowSizeMsg(msg tea.WindowSizeMsg) tea.Cmd {
	logger.Debug("handleWindowSizeMsg: WindowSizeMsg received - Width: %d, Height: %d", msg.Width, msg.Height)

	// Only update if we receive non-zero dimensions
	// If WindowSizeMsg has 0x0, keep the manually detected size from main.go
	if msg.Width > 0 {
		m.Width = msg.Width
	}
	if msg.Height > 0 {
		m.Height = msg.Height
	}
	logger.Debug("handleWindowSizeMsg: Using Width: %d, Height: %d", m.Width, m.Height)

	themeStyles := GetThemeStyles(m.Theme)

	// Calculate total height available for content (excluding status bar and potential error banner)
	// Status bar always takes 1 row (don't use GetHeight() which returns 0)
	contentHeight := m.Height - 1
	logger.Debug("handleWindowSizeMsg: Initial contentHeight (after status bar): %d", contentHeight)
	if m.ShowError && m.ErrorMessage != "" {
		contentHeight -= m.ErrorBanner.GetHeight()
		logger.Debug("handleWindowSizeMsg: contentHeight after error banner: %d", contentHeight)
	}

	const minPaneWidth = 20
	const minPaneHeight = 5
	const minViewportWidth = 10
	const minViewportHeight = 3

	// Calculate pane widths (chat left, agent status + log right)
	chatPaneWidth := int(float64(m.Width) * 0.6)
	rightPaneWidth := m.Width - chatPaneWidth

	if chatPaneWidth < minPaneWidth {
		chatPaneWidth = minPaneWidth
	}
	if rightPaneWidth < minPaneWidth {
		rightPaneWidth = minPaneWidth
	}

	// Chat pane uses full content height (left side)
	m.Chat.Width = chatPaneWidth - themeStyles.MainPane.GetHorizontalFrameSize()
	m.Chat.Height = contentHeight - themeStyles.MainPane.GetVerticalFrameSize()

	// Right side has two separate bordered panes stacked vertically
	// Each pane needs its own border, so we calculate available space for both
	agentStatusPaneHeight := int(float64(contentHeight) * 0.5)
	logPaneHeight := contentHeight - agentStatusPaneHeight

	// Agent status pane (top-right, with its own border)
	m.AgentStatus.Width = rightPaneWidth - themeStyles.MainPane.GetHorizontalFrameSize()
	m.AgentStatus.Height = agentStatusPaneHeight - themeStyles.MainPane.GetVerticalFrameSize()

	// Log pane (bottom-right, with its own border)
	m.Log.Width = rightPaneWidth - themeStyles.MainPane.GetHorizontalFrameSize()
	m.Log.Height = logPaneHeight - themeStyles.MainPane.GetVerticalFrameSize()

	logger.Debug("handleWindowSizeMsg: m.Chat.Width: %d, m.Chat.Height: %d (contentHeight=%d, frameSize=%d)",
		m.Chat.Width, m.Chat.Height, contentHeight, themeStyles.MainPane.GetVerticalFrameSize())

	// Ensure minimum dimensions
	if m.Chat.Width < minViewportWidth {
		m.Chat.Width = minViewportWidth
	}
	if m.Chat.Height < minViewportHeight {
		m.Chat.Height = minViewportHeight
	}

	// Update chat components dimensions
	m.Chat.Viewport.Width = m.Chat.Width
	// Note: Viewport.Height is set by chat.updateViewportHeight() in chat.Update()
	// to account for input field, spinner, scroll indicator, etc.
	m.Chat.Textarea.SetWidth(m.Chat.Width - 8)

	// Mark content dirty so it re-renders, and flag for initial scroll
	m.Chat.ContentDirty = true
	m.Chat.NeedsInitialScroll = true

	// Update status bar text with theme name
	themeName := getThemeName(m.Theme)
	m.StatusBarText = fmt.Sprintf("Model: %s | Host: %s | Format: %s | Theme: %s | Web UI: http://localhost:%d",
		m.Chat.LlmClient.Model(),
		m.Chat.LlmClient.Host(),
		m.Chat.LlmClient.APIFormatString(),
		themeName,
		m.WebUIPort)
	return nil
}

func (m *Model) renderBriefingRoom() string {
	if m.prd == nil {
		return "Loading stories..."
	}

	var lines []string
	completed := 0
	total := len(m.prd.UserStories)
	for _, story := range m.prd.UserStories {
		if story.Passes {
			completed++
		}
	}
	lines = append(lines, fmt.Sprintf("📋 CLAI Development Tasks (%d/%d completed)", completed, total))
	lines = append(lines, "")

	for _, story := range m.prd.UserStories {
		status := "❌"
		if story.Passes {
			status = "✅"
		}
		lines = append(lines, fmt.Sprintf("%s %s: %s", status, story.ID, story.Title))
		if len(story.Description) > 0 {
			// Truncate description
			desc := story.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			lines = append(lines, fmt.Sprintf("   %s", desc))
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

func (m *Model) View() string {
	// Active pane styling
	themeStyles := GetThemeStyles(m.Theme)
	briefingPaneStyle := themeStyles.MainPane
	chatPaneStyle := themeStyles.MainPane
	logPaneStyle := themeStyles.MainPane

	if m.ActivePane == BriefingRoom {
		briefingPaneStyle = briefingPaneStyle.Copy().BorderForeground(lipgloss.Color(m.Theme.Theme.Bright.Yellow))
	} else if m.ActivePane == ChatPane {
		chatPaneStyle = chatPaneStyle.Copy().BorderForeground(lipgloss.Color(m.Theme.Theme.Bright.Yellow))
	}

	// Render briefing room pane (left, full height)
	briefingViewInner := m.renderBriefingRoom()
	briefingView := briefingPaneStyle.Render(briefingViewInner)

	// Render chat pane (right, full height)
	chatViewInner := m.Chat.View()
	chatView := chatPaneStyle.Render(chatViewInner)

	// For now, combine briefing and chat horizontally
	mainView := lipgloss.JoinHorizontal(lipgloss.Top, briefingView, chatView)

	// If log viewer is active (ctrl-l pressed), show it instead
	if m.ActivePane == LogPane {
		// Render log pane (full width, full height)
		logContent := m.Log.View()
		lines := strings.Split(logContent, "\n")
		var renderedLines []string
		for _, line := range lines {
			renderedLines = append(renderedLines, themeStyles.BackgroundWrapper.Width(m.Width).Render(line))
		}
		if m.Height > 0 {
			for len(renderedLines) < m.Height-1 { // -1 for status bar
				renderedLines = append(renderedLines, themeStyles.BackgroundWrapper.Width(m.Width).Render(""))
			}
		}
		logContentFilled := strings.Join(renderedLines, "\n")
		logPane := logPaneStyle.Copy().BorderForeground(lipgloss.Color(m.Theme.Theme.Bright.Yellow)).Render(logContentFilled)
		mainView = logPane
	}

	statusBarRendered := themeStyles.StatusBar.Width(m.Width).Render(m.StatusBarText)
	layout := lipgloss.JoinVertical(lipgloss.Left, mainView, statusBarRendered)

	if m.ShowError && m.ErrorMessage != "" {
		layout = lipgloss.JoinVertical(lipgloss.Left, layout, m.ErrorBanner.Width(m.Width).Render(m.ErrorMessage))
	}

	if m.ShowHelp {
		chatPaneWidth := int(float64(m.Width) * m.Layout.HelpPaneWidthRatio)
		themeStyles := GetThemeStyles(m.Theme)
		helpBox := themeStyles.HelpBox.Width(max(chatPaneWidth/2, 10)).Render(m.Help.View(m.Keys))
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, helpBox)
	}
	return layout

}
