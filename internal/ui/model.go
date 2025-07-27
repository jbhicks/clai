package ui

import (
	"bufio"
	"clai/internal/llm"
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
}

type (
	ToolResultMsg      struct{ ToolName, Result string }
	LogUpdateMsg       string
	LLMResponseMsg     struct{ Resp llm.Response }
	StreamUpdateMsg    string
	TickMsg            struct{}
	HealthCheckMsg     struct{ Err error }
	HealthCheckDoneMsg struct{}
	errorMsg           struct{ err error }
	clearErrorMsg      struct{}
)

func StreamLLMResponseCmd(llmClient *llm.Client, messages []llm.Message) tea.Cmd {
	return func() tea.Msg {
		streamChan := make(chan string)
		go func() {
			_, err := llmClient.SendMessageStream(messages, streamChan)
			if err != nil {
				streamChan <- "[LLM ERROR] " + err.Error()
				close(streamChan)
			}
		}()
		for chunk := range streamChan {
			return StreamUpdateMsg(chunk)
		}
		return nil
	}
}

func TailLogFileCmd() tea.Cmd {
	return func() tea.Msg {
		logChan := make(chan tea.Msg)
		go tailLogFile(logChan)
		return readLogChanCmd(logChan)
	}
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
	return tea.Batch(TailLogFileCmd(), m.Chat.Init())
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Printf("model.Update called with msg type: %T", msg)
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		cmds = append(cmds, m.handleWindowSizeMsg(msg))
	case tea.KeyMsg:
		cmds = append(cmds, m.handleKeyMsg(msg))
	case StreamUpdateMsg:
		chunk := string(msg)
		if len(chunk) == 0 {
			m.Chat.Streaming = false
			break
		}
		if len(m.Chat.Messages) > 0 && m.Chat.Messages[len(m.Chat.Messages)-1].Role == "assistant" && m.Chat.Streaming && !StartsWithLLMError(chunk) {
			m.Chat.Messages[len(m.Chat.Messages)-1].Content += chunk
			// Update the last item in the list
			lastItemIndex := len(m.Chat.List.Items()) - 1
			if lastItemIndex >= 0 {
				lastItem := m.Chat.List.Items()[lastItemIndex].(Item)
				m.Chat.List.SetItem(lastItemIndex, Item(string(lastItem)+chunk))
			}
		} else {
			m.Chat.Messages = append(m.Chat.Messages, llm.Message{Role: "assistant", Content: chunk})
			m.Chat.List.InsertItem(len(m.Chat.List.Items()), Item(chunk))
		}
		if StartsWithLLMError(chunk) {
			m.Chat.Streaming = false
		}
	case LogUpdateMsg:
		m.Log.SetContent(m.Log.View() + string(msg) + "\n")
		m.Log.GotoBottom()
		return m, nil
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
	switch msg.String() {
	case "q", "ctrl+c":
		return tea.Quit
	case "?":
		m.ShowHelp = !m.ShowHelp
		return nil
	case "enter":
		if m.Chat.TextInput.Focused() {
			userMsg := m.Chat.TextInput.Value()
			if userMsg != "" {
				m.Chat.Messages = append(m.Chat.Messages, llm.Message{Role: "user", Content: userMsg})
				m.Chat.List.InsertItem(len(m.Chat.List.Items()), Item(userMsg))
				m.Chat.TextInput.SetValue("")
				m.Chat.Streaming = true
				return StreamLLMResponseCmd(m.Chat.LlmClient, m.Chat.Messages)
			}
		}
	case "tab":
		if m.ActivePane == ChatPane {
			m.ActivePane = LogPane
		} else {
			m.ActivePane = ChatPane
		}
		return nil
	case "t":
		if m.Theme.Name == DarkTheme.Name {
			m.Theme = LightTheme
		} else {
			m.Theme = DarkTheme
		}
		m.Theme.ApplyStyles()
		m.Chat.Theme = &m.Theme // Update ChatModel's theme pointer
		return nil
	}
	var cmd tea.Cmd
	updatedChat, cmd := m.Chat.Update(msg)
	m.Chat = updatedChat
	cmds = append(cmds, cmd)
	return tea.Batch(cmds...)
}

func (m *Model) handleWindowSizeMsg(msg tea.WindowSizeMsg) tea.Cmd {
	log.Printf("handleWindowSizeMsg: WindowSizeMsg received - Width: %d, Height: %d", msg.Width, msg.Height)
	m.Width = msg.Width
	m.Height = msg.Height

	// Reduce vertical size by half
	usableHeight := m.Height / 2
	// Calculate total height available for content (excluding status bar and potential error banner)
	contentHeight := usableHeight - m.Theme.StatusBar.GetHeight()
	log.Printf("handleWindowSizeMsg: Initial contentHeight (after status bar): %d", contentHeight)
	if m.ShowError && m.ErrorMessage != "" {
		contentHeight -= m.ErrorBanner.GetHeight()
		log.Printf("handleWindowSizeMsg: contentHeight after error banner: %d", contentHeight)
	}

	mainWidth := m.Width
	const minPaneWidth = 20
	const minPaneHeight = 5
	const minViewportWidth = 10
	const minViewportHeight = 3

	if mainWidth < minPaneWidth {
		mainWidth = minPaneWidth
	}

	// Distribute height to chat and log panes
	m.Chat.Width = mainWidth - m.Theme.MainPane.GetHorizontalFrameSize()
	m.Chat.Height = contentHeight
	log.Printf("handleWindowSizeMsg: m.Chat.Width: %d, m.Chat.Height: %d", m.Chat.Width, m.Chat.Height)

	// Ensure minimum heights
	if m.Chat.Width < minViewportWidth {
		m.Chat.Width = minViewportWidth
	}
	if m.Chat.Height < minViewportHeight {
		m.Chat.Height = minViewportHeight
	}

	// Update chat viewport dimensions based on chat pane's calculated height
	// This needs to account for the input field, spinner, and potential tooltip within the chat pane
	// The actual viewport height will be set in ChatModel.View()
	m.Chat.Viewport.Width = m.Chat.Width

	// Update status bar text
	m.StatusBarText = fmt.Sprintf("Model: %s | Host: %s", m.Chat.LlmClient.Model(), m.Chat.LlmClient.Host())
	return nil
}

func (m *Model) View() string {
	log.Printf("model.View called: Width=%d, Height=%d", m.Width, m.Height)

	chatPaneWidth := int(float64(m.Width) * 0.7)
	logPaneWidth := m.Width - chatPaneWidth
	log.Printf("model.View: chatPaneWidth=%d, logPaneWidth=%d", chatPaneWidth, logPaneWidth)

	// Active pane styling
	chatPaneStyle := m.Theme.MainPane
	logPaneStyle := m.Theme.MainPane
	if m.ActivePane == ChatPane {
		chatPaneStyle = chatPaneStyle.Copy().BorderForeground(m.Theme.Accent1) // Highlight active pane
	} else {
		logPaneStyle = logPaneStyle.Copy().BorderForeground(m.Theme.Accent1) // Highlight active pane
	}

	// Reduce vertical size by half
	usableHeight := m.Height / 2
	// Calculate total height available for content (excluding status bar and potential error banner)
	contentHeight := usableHeight - m.Theme.StatusBar.GetHeight()
	log.Printf("model.View: contentHeight (after status bar): %d", contentHeight)
	if m.ShowError && m.ErrorMessage != "" {
		contentHeight -= m.ErrorBanner.GetHeight()
		log.Printf("model.View: contentHeight after error banner: %d", contentHeight)
	}

	m.Chat.Width = max(chatPaneWidth-chatPaneStyle.GetHorizontalFrameSize(), 10)
	m.Log.Width = max(logPaneWidth-logPaneStyle.GetHorizontalFrameSize(), 10)
	m.Chat.Height = max(contentHeight, 3)
	m.Log.Height = max(contentHeight, 3)
	log.Printf("model.View: m.Chat.Width=%d, m.Chat.Height=%d, m.Log.Width=%d, m.Log.Height=%d", m.Chat.Width, m.Chat.Height, m.Log.Width, m.Log.Height)
	log.Printf("model.View: chatPaneStyle.GetHorizontalFrameSize()=%d, chatPaneStyle.GetVerticalFrameSize()=%d", chatPaneStyle.GetHorizontalFrameSize(), chatPaneStyle.GetVerticalFrameSize())
	log.Printf("model.View: logPaneStyle.GetHorizontalFrameSize()=%d, logPaneStyle.GetVerticalFrameSize()=%d", logPaneStyle.GetHorizontalFrameSize(), logPaneStyle.GetVerticalFrameSize())

	// Update Bubbles components with latest sizes
	m.Chat.List.SetWidth(m.Chat.Width)
	m.Chat.List.SetHeight(m.Chat.Height)
	m.Chat.Viewport.Width = m.Chat.Width
	m.Chat.Viewport.Height = m.Chat.Height
	m.Log.Width = max(m.Log.Width, 10)
	m.Log.Height = max(m.Log.Height, 3)

	chatView := chatPaneStyle.Width(chatPaneWidth).Height(max(m.Chat.Height-chatPaneStyle.GetVerticalFrameSize(), 3)).Render(m.Chat.View())
	log.Printf("model.View: chatView rendered height: %d", lipgloss.Height(chatView))
	logView := logPaneStyle.Width(logPaneWidth).Height(max(m.Log.Height-logPaneStyle.GetVerticalFrameSize(), 3)).Render(m.Log.View())
	log.Printf("model.View: logView rendered height: %d", lipgloss.Height(logView))

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, chatView, logView)
	log.Printf("model.View: mainView rendered height: %d", lipgloss.Height(mainView))
	statusBarRendered := m.Theme.StatusBar.Width(m.Width).Render(m.StatusBarText)
	log.Printf("model.View: statusBarRendered height: %d", lipgloss.Height(statusBarRendered))

	layout := lipgloss.JoinVertical(lipgloss.Left,
		mainView,
		statusBarRendered,
	)
	log.Printf("model.View: layout rendered height (before error/help): %d", lipgloss.Height(layout))

	if m.ShowError && m.ErrorMessage != "" {
		layout = lipgloss.JoinVertical(lipgloss.Left, layout, m.ErrorBanner.Width(m.Width).Render(m.ErrorMessage))
		log.Printf("model.View: layout rendered height (after error banner): %d", lipgloss.Height(layout))
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
		modal := lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, helpBox)
		log.Printf("model.View: help modal rendered height: %d", lipgloss.Height(modal))
		return lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Background(m.Theme.BgDark).Width(m.Width).Height(m.Height).Render(""),
			modal,
		)
	}
	log.Printf("model.View: final layout rendered height: %d", lipgloss.Height(layout))
	return layout

}
