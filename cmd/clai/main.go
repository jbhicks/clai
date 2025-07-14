package main

import (
	"bufio"
	"clai/internal/llm"
	"clai/internal/tools"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hpcloud/tail"
	"github.com/joho/godotenv"
)

// Set global logger output before any other imports
func init() {
	// Open the log file in append mode for reliable tailing
	f, err := os.OpenFile("debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	log.SetOutput(f)
}

type keyMap struct {
	Quit key.Binding
	Help key.Binding
	Tab  key.Binding
}

var defaultKeyMap = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q/ctrl+c", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch view"),
	),
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab, k.Help, k.Quit},
	}
}

var (
	focusedStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	blurredStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
)

type sessionState uint

const (
	chatView sessionState = iota
	logView
)

// Sub-model for chat functionality
type chatModel struct {
	messages  []llm.Message
	textInput textinput.Model
	viewport  viewport.Model
	llmClient *llm.Client
	streaming bool
	width     int
	height    int
}

// Sub-model for log functionality
type logModel struct {
	viewport viewport.Model
	logLines []string
	width    int
	height   int
}

// Master model that composes sub-models
type model struct {
	state       sessionState
	chat        chatModel
	logs        logModel
	err         error
	historyFile string
	width       int
	height      int
	help        help.Model
	keys        keyMap
	logChan     chan tea.Msg
}

// Chat model methods
func (c *chatModel) Init() tea.Cmd {
	return textinput.Blink
}

func (c *chatModel) Update(msg tea.Msg) (chatModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Update text input
	c.textInput, cmd = c.textInput.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	c.viewport, cmd = c.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return *c, tea.Batch(cmds...)
}

func (c *chatModel) View() string {
	// Show messages in viewport
	content := ""
	for _, msg := range c.messages {
		content += fmt.Sprintf("%s: %s\n\n", msg.Role, msg.Content)
	}
	c.viewport.SetContent(content)

	return lipgloss.JoinVertical(lipgloss.Left,
		c.viewport.View(),
		c.textInput.View(),
	)
}

// Simple word wrap for log lines
func wordWrap(s string, width int) string {
	if width < 1 {
		return s
	}
	var out strings.Builder
	var lineLen int
	for _, word := range strings.Fields(s) {
		if lineLen+len(word)+1 > width {
			out.WriteString("\n")
			lineLen = 0
		}
		if lineLen > 0 {
			out.WriteString(" ")
			lineLen++
		}
		out.WriteString(word)
		lineLen += len(word)
	}
	return out.String()
}

// Log model methods
func (l *logModel) Init() tea.Cmd {
	return nil
}

func (l *logModel) Update(msg tea.Msg) (logModel, tea.Cmd) {
	var cmd tea.Cmd
	l.viewport, cmd = l.viewport.Update(msg)
	return *l, cmd
}

func (l *logModel) View() string {
	return l.viewport.View()
}

type (
	llmResponseMsg  struct{ resp llm.Response }
	streamUpdateMsg string
	toolResultMsg   struct {
		toolName string
		result   string
	}
	errorMsg           struct{ err error }
	logUpdateMsg       string
	tickMsg            struct{}
	healthCheckMsg     struct{ err error }
	healthCheckDoneMsg struct{}
)

func performHealthCheckCmd(llmClient *llm.Client) tea.Cmd {
	return func() tea.Msg {
		err := llmClient.HealthCheck()
		if err != nil {
			log.Printf("Ollama health check failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
		return healthCheckDoneMsg{}
	}
}

func readExistingLogLines(logChan chan<- tea.Msg) {
	f, err := os.Open("debug.log")
	if err != nil {
		logChan <- errorMsg{err}
		return
	}
	defer f.Close()
	var lastLine string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lastLine = scanner.Text()
	}
	if lastLine != "" {
		logChan <- logUpdateMsg(lastLine)
	}
	if err := scanner.Err(); err != nil {
		logChan <- errorMsg{err}
	}
}

func tailLogFileCmd(logChan chan<- tea.Msg) {
	// Start tailing from the end of the file to avoid duplicate lines
	fi, err := os.Stat("debug.log")
	var offset int64 = 0
	if err == nil {
		offset = fi.Size()
	}
	t, err := tail.TailFile("debug.log", tail.Config{Follow: true, ReOpen: true, Poll: true, Location: &tail.SeekInfo{Offset: offset, Whence: 0}})
	if err != nil {
		logChan <- errorMsg{err}
		return
	}
	for line := range t.Lines {
		logChan <- logUpdateMsg(line.Text)
	}
}

func initialModel(historyFile string, llmClient *llm.Client) *model {
	log.SetFlags(log.Ltime)

	// Initialize chat input
	ti := textinput.New()
	ti.Placeholder = "Ask me anything..."
	ti.Focus()

	// Initialize chat model
	chatVP := viewport.New(0, 0)
	chat := chatModel{
		messages:  []llm.Message{{Role: "assistant", Content: "Welcome to clai! Ask me anything..."}},
		textInput: ti,
		viewport:  chatVP,
		llmClient: llmClient,
	}

	// Initialize log model
	logVP := viewport.New(0, 0)
	logs := logModel{
		viewport: logVP,
		logLines: []string{},
	}
	// Synchronously read last N lines from debug.log
	f, err := os.Open("debug.log")
	if err == nil {
		scanner := bufio.NewScanner(f)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		f.Close()
		if len(lines) > 100 {
			lines = lines[len(lines)-100:]
		}
		logs.logLines = lines
	}

	h := help.New()
	h.ShowAll = false

	m := &model{
		state:       chatView,
		chat:        chat,
		logs:        logs,
		historyFile: historyFile,
		help:        h,
		keys:        defaultKeyMap,
	}
	return m
}

func readLogChanCmd(logChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-logChan
	}
}

func (m *model) Init() tea.Cmd {
	logChan := make(chan tea.Msg, 100)
	go readExistingLogLines(logChan)
	go tailLogFileCmd(logChan)
	m.logChan = logChan
	// Request initial window size explicitly
	return tea.Batch(
		tea.WindowSize(),
		performHealthCheckCmd(m.chat.llmClient),
		readLogChanCmd(m.logChan),
	)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height - 5

		// Update sub-models with new dimensions
		// Border thickness for Lip Gloss boxes
		borderWidth := 2  // left + right
		borderHeight := 2 // top + bottom

		// Calculate pane sizes, subtracting borders
		usableWidth := max(m.width-borderWidth*2, 10)
		leftPaneWidth := usableWidth / 2
		rightPaneWidth := usableWidth - leftPaneWidth

		// Calculate available height, subtracting borders and help/model bars
		helpBarHeight := 1  // estimate, or use lipgloss.Height(m.help.View(m.keys))
		modelBarHeight := 1 // estimate, or use lipgloss.Height(modelBar)
		availableHeight := max(m.height-borderHeight-helpBarHeight-modelBarHeight, 5)
		textInputHeight := lipgloss.Height(m.chat.textInput.View())
		chatViewportHeight := max(availableHeight-textInputHeight, 1)

		m.chat.width = leftPaneWidth
		m.chat.height = availableHeight
		m.chat.viewport = viewport.New(leftPaneWidth, chatViewportHeight)

		m.logs.width = rightPaneWidth
		m.logs.height = availableHeight
		m.logs.viewport = viewport.New(rightPaneWidth, availableHeight)
		// Always set log content after resizing
		content := strings.Join(m.logs.logLines, "\n")
		m.logs.viewport.SetContent(content)
		if rightPaneWidth > 0 && availableHeight > 0 {
			m.logs.viewport.GotoBottom()
		}

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if msg.String() == "tab" {
			// Switch focus between views
			if m.state == chatView {
				m.state = logView
				m.chat.textInput.Blur()
			} else {
				m.state = chatView
				m.chat.textInput.Focus()
			}
			return m, nil
		}
		// If chat view is focused, delegate all keys to text input
		if m.state == chatView && m.chat.textInput.Focused() {
			var cmd tea.Cmd
			m.chat, cmd = m.chat.Update(msg)
			cmds = append(cmds, cmd)
			// Handle Enter key after text input updates
			if msg.Type == tea.KeyEnter {
				inputText := m.chat.textInput.Value()
				if strings.TrimSpace(inputText) != "" {
					m.chat.messages = append(m.chat.messages, llm.Message{Role: "user", Content: inputText})
					m.chat.textInput.SetValue("")
					// Add your async LLM call here if needed
				}
			}
			return m, tea.Batch(cmds...)
		}
		// Only handle global keys if input is NOT focused
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Tab):
			// Switch focus between views
			if m.state == chatView {
				m.state = logView
				m.chat.textInput.Blur()
			} else {
				m.state = chatView
				m.chat.textInput.Focus()
			}
		}

		// Handle Enter key in chat view
		if m.state == chatView && msg.Type == tea.KeyEnter {
			inputText := m.chat.textInput.Value()
			if strings.TrimSpace(inputText) == "" {
				// Ignore empty input
			} else {
				// Add user message to chat
				m.chat.messages = append(m.chat.messages, llm.Message{Role: "user", Content: inputText})
				// Pretty-print the outgoing request (user messages)
				// ...existing code...
				// Clear input
				m.chat.textInput.SetValue("")
				// Send to local API (llmClient) asynchronously
				cmds = append(cmds, func() tea.Msg {
					var resp llm.Response
					var err error
					var errJson error
					// Log outgoing LLM request
					reqJson, _ := json.MarshalIndent(m.chat.messages, "", "  ")
					log.Printf("[LLM Request] Sending messages: %s", string(reqJson))
					done := make(chan struct{})
					go func() {
						resp, err = m.chat.llmClient.SendMessage(m.chat.messages)
						close(done)
					}()
					<-done
					if err != nil {
						return errorMsg{err}
					}
					_, errJson = json.MarshalIndent(resp, "", "  ")
					if errJson != nil {
						return errorMsg{errJson}
					}
					return llmResponseMsg{resp: resp}
				})
			}
		}

		// Delegate to focused sub-model
		var cmd tea.Cmd
		switch m.state {
		case chatView:
			m.chat, cmd = m.chat.Update(msg)
			cmds = append(cmds, cmd)
		case logView:
			m.logs, cmd = m.logs.Update(msg)
			cmds = append(cmds, cmd)
		}
	case tea.MouseMsg:
		var cmd tea.Cmd
		switch m.state {
		case chatView:
			m.chat, cmd = m.chat.Update(msg) // Pass to chatModel's Update
			cmds = append(cmds, cmd)
		case logView:
			m.logs, cmd = m.logs.Update(msg) // Pass to logModel's Update
			cmds = append(cmds, cmd)
		}

	case llmResponseMsg:
		m.chat.messages = append(m.chat.messages, msg.resp.Message)
		if len(msg.resp.Message.ToolCalls) > 0 {
			for _, toolCall := range msg.resp.Message.ToolCalls {
				cmds = append(cmds, m.executeTool(toolCall))
			}
		}

	case streamUpdateMsg:
		m.chat.streaming = false
		m.chat.messages = append(m.chat.messages, llm.Message{Role: "assistant", Content: string(msg)})

	case toolResultMsg:
		m.chat.messages = append(m.chat.messages, llm.Message{
			Role:    "tool",
			Content: msg.result,
		})

	case errorMsg:
		m.err = msg.err
		m.chat.messages = append(m.chat.messages, llm.Message{Role: "assistant", Content: "Error: " + msg.err.Error()})

	case logUpdateMsg:
		// Show all log messages, no filtering
		logLine := string(msg)
		if len(m.logs.logLines) == 0 || m.logs.logLines[len(m.logs.logLines)-1] != logLine {
			m.logs.logLines = append(m.logs.logLines, logLine)
			if len(m.logs.logLines) > 100 {
				m.logs.logLines = m.logs.logLines[len(m.logs.logLines)-100:]
			}
			// Update log viewport simply
			content := strings.Join(m.logs.logLines, "\n")
			m.logs.viewport.SetContent(content)
			if m.logs.viewport.Width > 0 && m.logs.viewport.Height > 0 {
				m.logs.viewport.GotoBottom()
			}
		}
		// Re-issue log channel read
		cmds = append(cmds, readLogChanCmd(m.logChan))

	case tickMsg:
		return m, nil

	case healthCheckDoneMsg:
		return m, nil
	default:
		// Only handle errors and unexpected types
		if _, ok := msg.(errorMsg); !ok {
			// skip for normal messages
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	// Create border styles similar to minimal_testing.go
	chatBoxStyle := focusedStyle
	logsBoxStyle := blurredStyle
	if m.state == logView {
		chatBoxStyle = blurredStyle
		logsBoxStyle = focusedStyle
	}

	// Help bar
	helpBar := m.help.View(m.keys)

	modelBarStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("229")).
		Bold(true).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 2).
		MarginTop(1).
		MarginRight(2)

	// Fix: call Model() if it's a method
	var modelName string
	switch v := any(m.chat.llmClient.Model).(type) {
	case func() string:
		modelName = v()
	case string:
		modelName = v
	default:
		modelName = fmt.Sprintf("%v", v)
	}
	modelBarText := fmt.Sprintf("Model: %s", modelName)
	modelBar := modelBarStyle.Render(modelBarText)

	// Re-set chat viewport content
	chatContent := ""
	for _, msg := range m.chat.messages {
		chatContent += fmt.Sprintf("%s: %s\n\n", msg.Role, msg.Content)
	}
	m.chat.viewport.SetContent(chatContent)

	// Use Lipgloss layouts for clean arrangement
	leftPane := chatBoxStyle.Width(m.chat.width).Height(m.chat.height).Render(
		lipgloss.JoinVertical(lipgloss.Left, m.chat.viewport.View(), m.chat.textInput.View()),
	)
	rightPane := logsBoxStyle.Width(m.logs.width).Height(m.logs.height).Render(m.logs.View())
	row := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	return lipgloss.JoinVertical(lipgloss.Left, row, helpBar, modelBar)
}

func (m *model) executeTool(toolCall llm.ToolCall) tea.Cmd {
	return func() tea.Msg {
		result, err := tools.ExecuteTool(toolCall.Name, toolCall.Parameters)
		if err != nil {
			return errorMsg{err}
		}
		return toolResultMsg{toolName: toolCall.Name, result: result}
	}
}

func main() {
	// Ensure Ctrl+C always exits, regardless of Bubble Tea focus
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Received SIGINT (Ctrl+C), exiting immediately.")
		os.Exit(0)
	}()

	// Set up logging to debug.log for the entire program lifetime

	log.Println("[Test] clai app started")

	// Bubble Tea debug logging disabled to prevent UI log flooding.

	// Load environment variables from .env file
	_ = godotenv.Load()

	// Get config from env, fallback to defaults
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "llama3.1-gpu:latest"
	}
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	systemPrompt := os.Getenv("SYSTEM_PROMPT")

	historyFile := flag.String("history", "", "file to save conversation history")
	flag.Parse()

	llmClient := llm.NewClient(host, modelName, systemPrompt)
	m := initialModel(*historyFile, llmClient)
	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Enable mouse support
	}

	p := tea.NewProgram(m, opts...)
	if _, err := p.Run(); err != nil {
		log.Println("Fatal error:", err)
		os.Exit(1)
	}
}
