package ui

import (
	"bufio"
	"clai/internal/llm"
	"clai/internal/tools"
	"fmt"
	"log"
	"os"
	"time"

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
	Theme         Theme
	streamChan    chan string
	toolCallChan  chan []llm.ToolCall
	logChan       chan tea.Msg
}

type (
	LogUpdateMsg       string
	LLMResponseMsg     struct{ Resp llm.Response }
	StreamUpdateMsg    string
	ToolCallMsg        struct{ ToolCalls []llm.ToolCall }
	ToolResultMsg      struct{ ToolName, Result string }
	TickMsg            struct{}
	HealthCheckMsg     struct{ Err error }
	HealthCheckDoneMsg struct{}
	errorMsg           struct{ err error }
	clearErrorMsg      struct{}
)

func executeToolCmd(toolCall llm.ToolCall) tea.Cmd {
	return func() tea.Msg {
		log.Printf("Executing tool: %s with params: %s", toolCall.Name, string(toolCall.Parameters))
		result, err := tools.ExecuteTool(toolCall.Name, toolCall.Parameters)
		if err != nil {
			log.Printf("Tool execution error: %v", err)
			return ToolResultMsg{
				ToolName: toolCall.Name,
				Result:   fmt.Sprintf("Error: %v", err),
			}
		}
		log.Printf("Tool execution result: %s", result)
		return ToolResultMsg{
			ToolName: toolCall.Name,
			Result:   result,
		}
	}
}

func StreamLLMResponseCmd(llmClient *llm.Client, messages []llm.Message) tea.Cmd {
	return func() tea.Msg {
		streamChan := make(chan string, 100)
		toolCallChan := make(chan []llm.ToolCall, 1)

		_, err := llmClient.SendMessageStream(messages, streamChan, toolCallChan)
		if err != nil {
			return errorMsg{err: err}
		}

		// Return wrapper that will spawn commands for each chunk
		return streamStartMsg{streamChan: streamChan, toolCallChan: toolCallChan}
	}
}

type streamStartMsg struct {
	streamChan   chan string
	toolCallChan chan []llm.ToolCall
}

func waitForStreamChunk(streamChan <-chan string, toolCallChan <-chan []llm.ToolCall) tea.Cmd {
	return func() tea.Msg {
		// Non-blocking check for tool calls first
		select {
		case toolCalls, ok := <-toolCallChan:
			if ok && len(toolCalls) > 0 {
				return ToolCallMsg{ToolCalls: toolCalls}
			}
		default:
		}

		// Blocking read from stream
		chunk, ok := <-streamChan
		if !ok {
			return StreamUpdateMsg("")
		}
		return StreamUpdateMsg(chunk)
	}
}

func TailLogFileCmd(m *Model) tea.Cmd {
	m.logChan = make(chan tea.Msg)
	go tailLogFile(m.logChan)
	return readLogChanCmd(m.logChan)
}

func tailLogFile(logChan chan<- tea.Msg) {
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
			logChan <- LogUpdateMsg(line)
		}
		file.Close()
		time.Sleep(1 * time.Second)
	}
}

func readLogChanCmd(logChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-logChan }
}

func StartsWithLLMError(s string) bool {
	return len(s) >= 11 && s[:11] == "[LLM ERROR]"
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(TailLogFileCmd(m), m.Chat.Init(), tea.WindowSize())
}

func (m *Model) updateDimensions() {
	chatPaneWidth := int(float64(m.Width) * 0.8)
	logPaneWidth := m.Width - chatPaneWidth

	chatPaneStyle := m.Theme.MainPane
	logPaneStyle := m.Theme.MainPane

	contentHeight := m.Height - 1
	if m.ShowError && m.ErrorMessage != "" {
		contentHeight -= m.ErrorBanner.GetHeight()
	}

	m.Chat.Width = max(chatPaneWidth-chatPaneStyle.GetHorizontalFrameSize(), 10)
	m.Log.Width = max(logPaneWidth-logPaneStyle.GetHorizontalFrameSize(), 10)
	m.Chat.Height = max(contentHeight-chatPaneStyle.GetVerticalFrameSize(), 3)
	m.Log.Height = max(contentHeight-logPaneStyle.GetVerticalFrameSize(), 3)

	m.Chat.Viewport.Width = m.Chat.Width
	m.Chat.Viewport.Height = m.Chat.Height
	m.Chat.TextInput.Width = m.Chat.Width - 8
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		cmds = append(cmds, m.handleWindowSizeMsg(msg))
	case tea.KeyMsg:
		// Filter out terminal escape sequences before processing
		keyStr := msg.String()
		if len(keyStr) > 2 && (keyStr[:3] == "alt" ||
			(keyStr[0] >= '0' && keyStr[0] <= '9' && len(keyStr) > 1 && keyStr[1] == ';')) {
			log.Printf("Update: Filtering escape sequence: %s", keyStr)
			return m, nil
		}
		cmds = append(cmds, m.handleKeyMsg(msg))
	case streamStartMsg:
		m.streamChan = msg.streamChan
		m.toolCallChan = msg.toolCallChan
		return m, waitForStreamChunk(m.streamChan, m.toolCallChan)
	case StreamUpdateMsg:
		chunk := string(msg)
		if len(chunk) == 0 {
			m.Chat.Streaming = false
			m.streamChan = nil
			m.toolCallChan = nil
			break
		}
		if len(m.Chat.Messages) > 0 && m.Chat.Messages[len(m.Chat.Messages)-1].Role == "assistant" && m.Chat.Streaming && !StartsWithLLMError(chunk) {
			m.Chat.Messages[len(m.Chat.Messages)-1].Content += chunk
			m.Chat.ContentDirty = true
		} else {
			m.Chat.Messages = append(m.Chat.Messages, llm.Message{Role: "assistant", Content: chunk})
			m.Chat.ContentDirty = true
		}
		m.Chat.Viewport.GotoBottom()
		if StartsWithLLMError(chunk) {
			m.Chat.Streaming = false
			m.streamChan = nil
			m.toolCallChan = nil
		} else if m.streamChan != nil {
			return m, waitForStreamChunk(m.streamChan, m.toolCallChan)
		}
	case ToolCallMsg:
		log.Printf("ToolCallMsg received with %d tool calls", len(msg.ToolCalls))
		m.Chat.Streaming = false
		m.streamChan = nil
		m.toolCallChan = nil

		// Execute all tool calls and collect results
		var toolResultCmds []tea.Cmd
		for _, tc := range msg.ToolCalls {
			toolResultCmds = append(toolResultCmds, executeToolCmd(tc))
		}
		return m, tea.Batch(toolResultCmds...)
	case ToolResultMsg:
		log.Printf("ToolResultMsg received: %s -> %s", msg.ToolName, msg.Result)
		// Add tool result to messages
		m.Chat.Messages = append(m.Chat.Messages, llm.Message{
			Role:    "tool",
			Content: fmt.Sprintf("%s: %s", msg.ToolName, msg.Result),
		})
		m.Chat.ContentDirty = true
		m.Chat.Viewport.GotoBottom()

		// Send updated conversation back to LLM for final response
		m.Chat.Streaming = true
		return m, StreamLLMResponseCmd(m.Chat.LlmClient, m.Chat.Messages)
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

	log.Printf("handleKeyMsg: key=%s, focused=%v, value=%s", msg.String(), m.Chat.TextInput.Focused(), m.Chat.TextInput.Value())

	// Filter out terminal escape sequences that appear at startup
	keyStr := msg.String()
	if len(keyStr) > 0 && (keyStr[0] == '\x1b' ||
		(len(keyStr) > 2 && keyStr[:3] == "alt") ||
		(len(keyStr) > 1 && (keyStr[0] >= '0' && keyStr[0] <= '9') && keyStr[1] == ';') ||
		(len(keyStr) > 3 && keyStr[0] >= '0' && keyStr[0] <= '9' && keyStr[1] >= '0' && keyStr[1] <= '9' && keyStr[2] == ';')) {
		log.Printf("handleKeyMsg: Ignoring escape sequence: %s", keyStr)
		return nil
	}

	// Handle global hotkeys that work regardless of focus
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit
	case "enter":
		log.Printf("handleKeyMsg: Enter pressed, focused=%v, value=%s", m.Chat.TextInput.Focused(), m.Chat.TextInput.Value())
		if m.Chat.TextInput.Focused() {
			userMsg := m.Chat.TextInput.Value()
			if userMsg != "" {
				log.Printf("handleKeyMsg: Submitting message: %s", userMsg)
				m.Chat.Messages = append(m.Chat.Messages, llm.Message{Role: "user", Content: userMsg})
				m.Chat.ContentDirty = true
				m.Chat.Viewport.GotoBottom()
				m.Chat.TextInput.SetValue("")
				m.Chat.Streaming = true
				return StreamLLMResponseCmd(m.Chat.LlmClient, m.Chat.Messages)
			} else {
				log.Printf("handleKeyMsg: Empty message, not submitting")
			}
		} else {
			log.Printf("handleKeyMsg: TextInput not focused")
		}
	}

	// Handle hotkeys with ctrl modifier (work even when input is focused)
	switch msg.String() {
	case "ctrl+q":
		return tea.Quit
	case "ctrl+h":
		m.ShowHelp = !m.ShowHelp
		return nil
	case "ctrl+t":
		if m.ActivePane == ChatPane {
			m.ActivePane = LogPane
		} else {
			m.ActivePane = ChatPane
		}
		return nil
	case "ctrl+d":
		if m.Theme.Name == DarkTheme.Name {
			m.Theme = LightTheme
		} else {
			m.Theme = DarkTheme
		}
		m.Theme.ApplyStyles()
		m.Chat.Theme = &m.Theme // Update ChatModel's theme pointer
		return nil
	}

	// Pass all other keys to chat component (for text input)
	var cmd tea.Cmd
	updatedChat, cmd := m.Chat.Update(msg)
	m.Chat = updatedChat
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}

func (m *Model) handleWindowSizeMsg(msg tea.WindowSizeMsg) tea.Cmd {
	log.Printf("handleWindowSizeMsg: WindowSizeMsg received - Width: %d, Height: %d", msg.Width, msg.Height)

	// Only update if we receive non-zero dimensions
	// If WindowSizeMsg has 0x0, keep the manually detected size from main.go
	if msg.Width > 0 {
		m.Width = msg.Width
	}
	if msg.Height > 0 {
		m.Height = msg.Height
	}
	log.Printf("handleWindowSizeMsg: Using Width: %d, Height: %d", m.Width, m.Height)

	// Calculate total height available for content (excluding status bar and potential error banner)
	// Status bar always takes 1 row (don't use GetHeight() which returns 0)
	contentHeight := m.Height - 1
	log.Printf("handleWindowSizeMsg: Initial contentHeight (after status bar): %d", contentHeight)
	if m.ShowError && m.ErrorMessage != "" {
		contentHeight -= m.ErrorBanner.GetHeight()
		log.Printf("handleWindowSizeMsg: contentHeight after error banner: %d", contentHeight)
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
	m.Chat.Width = chatPaneWidth - m.Theme.MainPane.GetHorizontalFrameSize()
	m.Chat.Height = contentHeight - m.Theme.MainPane.GetVerticalFrameSize()
	m.Log.Width = logPaneWidth - m.Theme.MainPane.GetHorizontalFrameSize()
	m.Log.Height = contentHeight - m.Theme.MainPane.GetVerticalFrameSize()
	log.Printf("handleWindowSizeMsg: m.Chat.Width: %d, m.Chat.Height: %d (contentHeight=%d, frameSize=%d)",
		m.Chat.Width, m.Chat.Height, contentHeight, m.Theme.MainPane.GetVerticalFrameSize())

	// Ensure minimum dimensions
	if m.Chat.Width < minViewportWidth {
		m.Chat.Width = minViewportWidth
	}
	if m.Chat.Height < minViewportHeight {
		m.Chat.Height = minViewportHeight
	}

	// Update chat components dimensions
	m.Chat.Viewport.Width = m.Chat.Width
	m.Chat.Viewport.Height = m.Chat.Height
	m.Chat.TextInput.Width = m.Chat.Width - 8
	m.Log.Width = logPaneWidth - m.Theme.MainPane.GetHorizontalFrameSize()

	// Update status bar text
	m.StatusBarText = fmt.Sprintf("Model: %s | Host: %s | Format: %s",
		m.Chat.LlmClient.Model(),
		m.Chat.LlmClient.Host(),
		m.Chat.LlmClient.APIFormatString())
	return nil
}

func (m *Model) View() string {
	chatPaneWidth := int(float64(m.Width) * 0.8)

	// Active pane styling
	chatPaneStyle := m.Theme.MainPane
	logPaneStyle := m.Theme.MainPane
	if m.ActivePane == ChatPane {
		chatPaneStyle = chatPaneStyle.Copy().BorderForeground(m.Theme.Accent1)
	} else {
		logPaneStyle = logPaneStyle.Copy().BorderForeground(m.Theme.Accent1)
	}

	// Render panes (dimensions already set in Update())
	chatViewInner := m.Chat.View()
	chatView := chatPaneStyle.Render(chatViewInner)

	logView := logPaneStyle.Render(m.Log.View())

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, chatView, logView)
	statusBarRendered := m.Theme.StatusBar.Width(m.Width).Render(m.StatusBarText)

	layout := lipgloss.JoinVertical(lipgloss.Left, mainView, statusBarRendered)

	if m.ShowError && m.ErrorMessage != "" {
		layout = lipgloss.JoinVertical(lipgloss.Left, layout, m.ErrorBanner.Width(m.Width).Render(m.ErrorMessage))
	}

	if m.ShowHelp {
		helpBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Background(m.Theme.BgDark).
			Foreground(m.Theme.Accent1).
			Width(max(chatPaneWidth/2, 10)).
			Align(lipgloss.Center).
			Render(m.Help.View(m.Keys))
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, helpBox)
	}
	return layout

}
