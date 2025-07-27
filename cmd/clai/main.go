package main

import (
	"clai/internal/llm"
	"clai/internal/ui"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
	"github.com/mattn/go-isatty"
)

func getStackTrace() string {
	return string(debug.Stack())
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: %v\n", r)
			log.Printf("STACK TRACE:\n%s", getStackTrace())
			fmt.Fprintf(os.Stderr, "PANIC: %v\n", r)
			fmt.Fprintf(os.Stderr, "STACK TRACE:\n%s", getStackTrace())
			os.Exit(2)
		}
	}()
	// Check for TTY (interactive terminal)
	if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "Error: This program requires an interactive terminal (TTY). Exiting.")
		os.Exit(1)
	}
	// Log to debug.log, overwrite each run
	logFile, err := os.Create("debug.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open debug.log for writing: %v\n", err)
		os.Exit(1)
	}
	log.SetOutput(logFile)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Received SIGINT (Ctrl+C), exiting immediately.")
		os.Exit(0)
	}()
	_ = godotenv.Load()
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "llama3.1-gpu:latest"
	}
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	systemPrompt := os.Getenv("SYSTEM_PROMPT")
	flag.Parse()
	llmClient := llm.NewClient(host, modelName, systemPrompt)
	chatInput := textinput.New()
	chatInput.Prompt = "> "
	chatInput.Placeholder = "Type your message..."
	chatInput.Focus()
	chatInput.CharLimit = 256
	chatInput.Width = 40
	chatInput.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFAA")).Bold(true).Underline(true)
	// Modern: no blinking cursor (Blink not supported in current bubbles/textinput)
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	help := help.New()
	help.ShowAll = false
	m := &ui.Model{
		Log:           viewport.New(0, 0),
		Help:          help,
		Keys:          ui.DefaultKeyMap,
		StatusBarText: "",
		ActivePane:    ui.ChatPane,
		ErrorBanner:   lipgloss.NewStyle().Background(lipgloss.Color("9")).Foreground(lipgloss.Color("15")).Padding(0, 1),
		Theme:         ui.DarkTheme,
	}
	m.Theme.ApplyStyles()
	chat := ui.ChatModel{
		TextInput: chatInput,
		LlmClient: llmClient,
		Spinner:   spin,
		Theme:     &m.Theme,
	}
	assistantIntro := "Hello! I am your AI assistant. I can use tools to help answer your questions."
	assistantName := "assistant"
	chat.AssistantName = assistantName
	chat.Messages = append(chat.Messages, llm.Message{Role: "assistant", Content: assistantIntro})
	items := []list.Item{}
	items = append(items, ui.Item(assistantIntro))
	chat.List = list.New(items, list.NewDefaultDelegate(), 0, 0)
	chat.Width = 80
	chat.Height = 20
	chat.Viewport = viewport.New(chat.Width, chat.Height)
	chat.Viewport.SetContent(assistantIntro)
	m.Chat = chat
	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}
	p := tea.NewProgram(m, opts...)
	if _, err := p.Run(); err != nil {
		log.Println("Fatal error:", err)
		os.Exit(1)
	}
}
