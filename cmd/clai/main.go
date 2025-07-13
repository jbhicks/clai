package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"clai/internal/llm"
	"clai/internal/tools"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hpcloud/tail"
	"github.com/mattn/go-isatty"
)

var (
	logPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)
)

type model struct {
	messages    []llm.Message
	err         error
	historyFile string
	llmClient   *llm.Client
	streaming   bool

	mainViewport  viewport.Model // New left box
	logViewport   viewport.Model
	logLines      []string // Store multiple log lines
	width         int
	height        int
	statusBarText string

	logChan chan tea.Msg // Channel for log updates
}

type llmResponseMsg struct{ resp llm.Response }
type streamUpdateMsg string
type toolResultMsg struct {
	toolName string
	result   string
}
type errorMsg struct{ err error }

type logUpdateMsg string
type tickMsg struct{}
type healthCheckMsg struct{ err error }
type healthCheckDoneMsg struct{}

func performHealthCheckCmd(llmClient *llm.Client) tea.Cmd {
	return func() tea.Msg {
		err := llmClient.HealthCheck()
		// Log the health check result immediately
		if err != nil {
			log.Printf("Ollama health check failed: %v", err)
		} else {
			log.Println("Ollama health check successful.")
		}
		time.Sleep(100 * time.Millisecond) // Give time for logs to be written
		// Then send a message to trigger a log update
		return healthCheckDoneMsg{}
	}
}

func tailLogFileCmd(logChan chan<- tea.Msg) {
	t, err := tail.TailFile("debug.log", tail.Config{Follow: true, ReOpen: true, Poll: true, Location: &tail.SeekInfo{Offset: 0, Whence: 0}})
	if err != nil {
		logChan <- errorMsg{err}
		return
	}
	for line := range t.Lines {
		logChan <- logUpdateMsg(line.Text)
	}
}

func initialModel(historyFile string, llmClient *llm.Client) *model {
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	}
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Logging initialized")

	mainVP := viewport.New(1, 1)
	mainVP.SetContent("Main box (70%)\nThis is your new left pane.")

	logVP := viewport.New(1, 1)
	logVP.SetContent("Initializing...")

	m := &model{
		messages:      []llm.Message{{Role: "assistant", Content: "Welcome to clai! Ask me anything..."}},
		historyFile:   historyFile,
		llmClient:     llmClient,
		mainViewport:  mainVP,
		logViewport:   logVP,
		statusBarText: fmt.Sprintf("Model: %s | Host: %s", llmClient.Model(), llmClient.Host()),
	}

	return m
}

func readLogChanCmd(logChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-logChan
	}
}

func (m *model) Init() tea.Cmd {
	logChan := make(chan tea.Msg)
	go tailLogFileCmd(logChan)
	m.logChan = logChan // Save to model for later use

	return tea.Batch(
		tea.Tick(5*time.Second, func(t time.Time) tea.Msg { return tickMsg{} }),
		performHealthCheckCmd(m.llmClient),
		readLogChanCmd(m.logChan),
	)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Viewports should not receive key messages
	if _, ok := msg.(tea.KeyMsg); !ok {
		m.logViewport.Update(msg)
	}

	// 2. Handle logic in the parent model
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		paneHeight := m.height - 2 // Account for status bar
		if paneHeight < 1 {
			paneHeight = 1
		}
		m.logViewport.Width = m.width - logPaneStyle.GetHorizontalFrameSize()
		m.logViewport.Height = paneHeight - logPaneStyle.GetVerticalFrameSize()
		m.statusBarText = fmt.Sprintf("Model: %s | Host: %s", m.llmClient.Model(), m.llmClient.Host())

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case llmResponseMsg:
		m.messages = append(m.messages, msg.resp.Message)
		if len(msg.resp.Message.ToolCalls) > 0 {
			for _, toolCall := range msg.resp.Message.ToolCalls {
				cmds = append(cmds, m.executeTool(toolCall))
			}
		}

	case streamUpdateMsg:
		m.streaming = false
		m.messages = append(m.messages, llm.Message{Role: "assistant", Content: string(msg)})

	case toolResultMsg:
		m.messages = append(m.messages, llm.Message{
			Role:    "tool",
			Content: msg.result,
		})
		cmds = append(cmds, m.streamFromLLM())

	case errorMsg:
		m.err = msg.err
		m.messages = append(m.messages, llm.Message{Role: "assistant", Content: "Error: " + msg.err.Error()})

	case logUpdateMsg:
		// Append new log line, keep last 100
		m.logLines = append(m.logLines, string(msg))
		if len(m.logLines) > 100 {
			m.logLines = m.logLines[len(m.logLines)-100:]
		}
		// Join last N lines for viewport
		content := ""
		for _, line := range m.logLines {
			content += line + "\n"
		}
		m.logViewport.SetContent(content)
		if m.logViewport.Width > 0 && m.logViewport.Height > 0 {
			m.logViewport.GotoBottom()
		}
		// Re-issue log channel read to keep UI updating
		cmds = append(cmds, readLogChanCmd(m.logChan))

	case tickMsg:
		log.Println("Tick!")
		return m, nil

	case healthCheckMsg:
		if msg.err != nil {
			log.Printf("Ollama health check failed: %v", msg.err)
		} else {
			log.Println("Ollama health check successful.")
		}
		return m, nil

	case healthCheckDoneMsg:
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	// Calculate widths
	mainWidth := int(float64(m.width) * 0.7)
	logWidth := m.width - mainWidth
	if mainWidth < 1 {
		mainWidth = 1
	}
	if logWidth < 1 {
		logWidth = 1
	}

	// Set viewport dimensions
	m.mainViewport.Width = mainWidth - logPaneStyle.GetHorizontalFrameSize()
	m.mainViewport.Height = m.height - 2 - logPaneStyle.GetVerticalFrameSize()
	m.logViewport.Width = logWidth - logPaneStyle.GetHorizontalFrameSize()
	m.logViewport.Height = m.height - 2 - logPaneStyle.GetVerticalFrameSize()

	mainPaneStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1)

	mainPane := mainPaneStyle.Width(mainWidth).Render(m.mainViewport.View())
	logPane := logPaneStyle.Width(logWidth).Render(m.logViewport.View())

	// Render the status bar
	statusBarRendered := statusBarStyle.Width(m.width).Render(m.statusBarText)

	// Layout: main pane (left), log pane (right), then status bar
	row := lipgloss.JoinHorizontal(lipgloss.Top, mainPane, logPane)
	return fmt.Sprintf("%s\n%s", row, statusBarRendered)
}

func (m *model) streamFromLLM() tea.Cmd {
	m.streaming = true
	return func() tea.Msg {
		streamChan := make(chan string)
		go func() {
			defer close(streamChan)
			_, err := m.llmClient.SendMessageStream(m.messages, streamChan)
			if err != nil {
				log.Printf("error from SendMessageStream: %v", err)
			}
		}()

		var fullResponse string
		for chunk := range streamChan {
			fullResponse += chunk
		}
		return streamUpdateMsg(fullResponse)
	}
}

func (m *model) executeTool(toolCall llm.ToolCall) tea.Cmd {
	return func() tea.Msg {
		log.Printf("executing tool %s with params %v", toolCall.Name, toolCall.Parameters)
		result, err := tools.ExecuteTool(toolCall.Name, toolCall.Parameters)
		if err != nil {
			log.Printf("tool execution error: %v", err)
			return errorMsg{err}
		}
		log.Printf("tool result: %s", result)
		return toolResultMsg{toolName: toolCall.Name, result: result}
	}
}

func main() {
	historyFile := flag.String("history", "", "file to save conversation history")
	modelName := flag.String("model", "llama3.1", "Ollama model to use")
	host := flag.String("host", "http://localhost:11434", "Ollama host")
	systemPrompt := flag.String("system-prompt", "", "Custom system prompt")
	flag.Parse()

	// Start a goroutine to write a tick to debug.log every 2 seconds
	go func() {
		for {
			f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				fmt.Fprintf(f, "Tick: %s\n", time.Now().Format(time.RFC3339))
				f.Close()
			}
			time.Sleep(2 * time.Second)
		}
	}()

	llmClient := llm.NewClient(*host, *modelName, *systemPrompt)
	m := initialModel(*historyFile, llmClient)
	opts := []tea.ProgramOption{}

	if isatty.IsTerminal(os.Stdin.Fd()) {
		opts = append(opts, tea.WithAltScreen())
	}
	p := tea.NewProgram(m, opts...)
	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}
