package ui

import (
	"bufio"
	"clai/internal/db"
	"clai/internal/llm"
	"clai/internal/logger"
	"clai/internal/tools"
	"fmt"
	"os"
	"time"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ActivePane int

const (
	ChatPane ActivePane = iota
	LogPane
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

type Model struct {
	Chat          ChatModel
	Log           viewport.Model
	logBuffer     string
	Err           error
	Width         int
	Height        int
	Help          help.Model
	Keys          KeyMap
	StatusBarText string
	ShowHelp      bool
	ActivePane    ActivePane
	ErrorBanner   lipgloss.Style
	ErrorMessage  string
	ShowError     bool
	Theme         *glitter.UI
	logChan       chan tea.Msg
	logDone       chan struct{}
	DB            interface{}
	Conversation  interface{}
	Agent         *llm.Agent
}

type (
	LogUpdateMsg       string
	LLMResponseMsg     struct{ Resp llm.Response }
	StreamUpdateMsg    string
	CodeBlockMsg       struct{ Blocks []llm.CodeBlock }
	CodeResultMsg      struct{ Result string }
	TickMsg            struct{}
	HealthCheckMsg     struct{ Err error }
	HealthCheckDoneMsg struct{}
	errorMsg           struct{ err error }
	clearErrorMsg      struct{}
	smoothScrollMsg    struct{}
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

		return CodeResultMsg{Result: combined}
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
	return tea.Batch(TailLogFileCmd(m), m.Chat.Init(), tea.WindowSize())
}

func (m *Model) updateDimensions() {
	chatPaneWidth := int(float64(m.Width) * 0.8)
	logPaneWidth := m.Width - chatPaneWidth

	themeStyles := GetThemeStyles(m.Theme)
	chatPaneStyle := themeStyles.MainPane
	logPaneStyle := themeStyles.MainPane

	contentHeight := m.Height - 1
	if m.ShowError && m.ErrorMessage != "" {
		contentHeight -= m.ErrorBanner.GetHeight()
	}

	m.Chat.Width = max(chatPaneWidth-chatPaneStyle.GetHorizontalFrameSize(), 10)
	m.Log.Width = max(logPaneWidth-logPaneStyle.GetHorizontalFrameSize(), 10)
	m.Chat.Height = max(contentHeight-chatPaneStyle.GetVerticalFrameSize(), 3)
	m.Log.Height = max(contentHeight-logPaneStyle.GetVerticalFrameSize(), 3)

	m.Chat.Viewport.Width = m.Chat.Width
	// Note: Viewport.Height is set by chat.updateViewportHeight() in chat.Update()
	// to account for input field, spinner, scroll indicator, etc.
	m.Chat.TextInput.Width = m.Chat.Width - 8
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
	case CodeResultMsg:
		logger.Debug("CodeResultMsg received: %s", msg.Result)

		// Strip code tags from last assistant message
		if len(m.Chat.Messages) > 0 && m.Chat.Messages[len(m.Chat.Messages)-1].Role == "assistant" {
			m.Chat.Messages[len(m.Chat.Messages)-1].Content = llm.StripCodeTags(m.Chat.Messages[len(m.Chat.Messages)-1].Content)
		}

		// Add code result as tool message
		m.Chat.Messages = append(m.Chat.Messages, llm.Message{
			Role:    "tool",
			Content: msg.Result,
		})
		m.Chat.ContentDirty = true

		// Send updated conversation back to agent for final response
		m.Chat.Streaming = true
		return m, RunAgentCmd(m.Agent, "continue")
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
		m.logBuffer += string(msg) + "\n"
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
		} else {
			logger.Info("[AGENT-RESPONSE] Agent completed: %s", msg.Response)
			m.Chat.Messages = append(m.Chat.Messages, llm.Message{
				Role:    "assistant",
				Content: msg.Response,
			})
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
		if m.Chat.TextInput.Focused() {
			userMsg := m.Chat.TextInput.Value()
			if userMsg != "" {
				m.Chat.Messages = append(m.Chat.Messages, llm.Message{Role: "user", Content: userMsg})
				m.Chat.ContentDirty = true
				m.Chat.AutoScroll = true
				m.Chat.UserScrolled = false
				m.Chat.Viewport.GotoBottom()
				m.Chat.TextInput.SetValue("")
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
					return RunAgentCmd(m.Agent, userMsg)
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
		if m.ActivePane == ChatPane {
			m.ActivePane = LogPane
		} else {
			m.ActivePane = ChatPane
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
		m.Help.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color(m.Theme.Theme.Primary.Foreground))
		m.Help.Styles.FullDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(m.Theme.Theme.Primary.DimForeground))
		m.Help.Styles.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color(m.Theme.Theme.Primary.DimForeground))
		themeName := getThemeName(m.Theme)
		m.StatusBarText = fmt.Sprintf("Model: %s | Host: %s | Format: %s | Theme: %s",
			m.Chat.LlmClient.Model(),
			m.Chat.LlmClient.Host(),
			m.Chat.LlmClient.APIFormatString(),
			themeName)
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
		m.Chat.TextInput.SetValue("")
		m.Chat.Viewport.GotoTop()
		m.Chat.UserScrolled = false
		m.Chat.AutoScroll = true
		return nil
	case "up":
		if m.Chat.TextInput.Focused() {
			if len(m.Chat.QueryHistory) > 0 && m.Chat.HistoryIndex < len(m.Chat.QueryHistory) {
				m.Chat.HistoryIndex++
				m.Chat.TextInput.SetValue(m.Chat.QueryHistory[len(m.Chat.QueryHistory)-m.Chat.HistoryIndex])
				m.Chat.TextInput.CursorEnd()
			}
		} else {
			m.Chat.SmoothScrollTarget = max(m.Chat.Viewport.YOffset-1, 0)
			m.Chat.SmoothScrollActive = true
			m.Chat.UserScrolled = true
			m.Chat.AutoScroll = false
			return smoothScrollCmd()
		}
	case "down":
		if m.Chat.TextInput.Focused() {
			if m.Chat.HistoryIndex > 1 {
				m.Chat.HistoryIndex--
				m.Chat.TextInput.SetValue(m.Chat.QueryHistory[len(m.Chat.QueryHistory)-m.Chat.HistoryIndex])
				m.Chat.TextInput.CursorEnd()
			} else if m.Chat.HistoryIndex == 1 {
				m.Chat.HistoryIndex--
				m.Chat.TextInput.SetValue("")
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
		if !m.Chat.TextInput.Focused() {
			m.Chat.SmoothScrollTarget = max(m.Chat.Viewport.YOffset-1, 0)
			m.Chat.SmoothScrollActive = true
			m.Chat.UserScrolled = true
			m.Chat.AutoScroll = false
			return smoothScrollCmd()
		}
	case "j":
		if !m.Chat.TextInput.Focused() {
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
		if !m.Chat.TextInput.Focused() {
			m.Chat.SmoothScrollTarget = max(m.Chat.Viewport.YOffset-m.Chat.Viewport.Height, 0)
			m.Chat.SmoothScrollActive = true
			m.Chat.UserScrolled = true
			m.Chat.AutoScroll = false
			return smoothScrollCmd()
		}
	case "pgdown":
		if !m.Chat.TextInput.Focused() {
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
		if !m.Chat.TextInput.Focused() {
			m.Chat.SmoothScrollTarget = 0
			m.Chat.SmoothScrollActive = true
			m.Chat.UserScrolled = true
			m.Chat.AutoScroll = false
			return smoothScrollCmd()
		}
	case "end", "G":
		if !m.Chat.TextInput.Focused() {
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

			m.Update(keyMsg)

			resp = DebugResponse{
				Success: true,
				Data: map[string]interface{}{
					"key_sent": key,
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

	// Calculate pane widths
	chatPaneWidth := int(float64(m.Width) * 0.6)
	logPaneWidth := m.Width - chatPaneWidth

	if chatPaneWidth < minPaneWidth {
		chatPaneWidth = minPaneWidth
	}
	if logPaneWidth < minPaneWidth {
		logPaneWidth = minPaneWidth
	}

	// Distribute dimensions to chat and log panes
	// Subtract frame sizes because MainPane style will add border around the content
	m.Chat.Width = chatPaneWidth - themeStyles.MainPane.GetHorizontalFrameSize()
	m.Chat.Height = contentHeight - themeStyles.MainPane.GetVerticalFrameSize()
	m.Log.Width = logPaneWidth - themeStyles.MainPane.GetHorizontalFrameSize()
	m.Log.Height = contentHeight - themeStyles.MainPane.GetVerticalFrameSize()

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
	m.Chat.TextInput.Width = m.Chat.Width - 8
	m.Log.Width = logPaneWidth - themeStyles.MainPane.GetHorizontalFrameSize()
	m.Log.Height = contentHeight - themeStyles.MainPane.GetVerticalFrameSize()

	// Mark content dirty so it re-renders, and flag for initial scroll
	m.Chat.ContentDirty = true
	m.Chat.NeedsInitialScroll = true

	// Update status bar text with theme name
	themeName := getThemeName(m.Theme)
	m.StatusBarText = fmt.Sprintf("Model: %s | Host: %s | Format: %s | Theme: %s",
		m.Chat.LlmClient.Model(),
		m.Chat.LlmClient.Host(),
		m.Chat.LlmClient.APIFormatString(),
		themeName)
	return nil
}

func (m *Model) View() string {
	chatPaneWidth := int(float64(m.Width) * 0.8)

	// Active pane styling
	themeStyles := GetThemeStyles(m.Theme)
	chatPaneStyle := themeStyles.MainPane
	logPaneStyle := themeStyles.MainPane
	if m.ActivePane == ChatPane {
		chatPaneStyle = chatPaneStyle.Copy().BorderForeground(lipgloss.Color(m.Theme.Theme.Bright.Yellow))
	} else {
		logPaneStyle = logPaneStyle.Copy().BorderForeground(lipgloss.Color(m.Theme.Theme.Bright.Yellow))
	}

	// Render panes (dimensions already set in Update())
	chatViewInner := m.Chat.View()
	chatView := chatPaneStyle.Render(chatViewInner)

	// Wrap log viewport content with background to fill the entire pane
	logContent := m.Log.View()
	logWrapper := lipgloss.NewStyle().
		Width(m.Log.Width).
		Height(m.Log.Height).
		Background(lipgloss.Color(m.Theme.Theme.Primary.Background))
	logContentFilled := logWrapper.Render(logContent)
	logView := logPaneStyle.Render(logContentFilled)

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, chatView, logView)
	statusBarRendered := themeStyles.StatusBar.Width(m.Width).Render(m.StatusBarText)

	layout := lipgloss.JoinVertical(lipgloss.Left, mainView, statusBarRendered)

	if m.ShowError && m.ErrorMessage != "" {
		layout = lipgloss.JoinVertical(lipgloss.Left, layout, m.ErrorBanner.Width(m.Width).Render(m.ErrorMessage))
	}

	if m.ShowHelp {
		helpBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Background(lipgloss.Color(m.Theme.Theme.Primary.Background)).
			Foreground(lipgloss.Color(m.Theme.Theme.Bright.White)).
			Width(max(chatPaneWidth/2, 10)).
			Align(lipgloss.Center).
			Render(m.Help.View(m.Keys))
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, helpBox)
	}
	return layout

}
