package main

import (
	"clai/internal/db"
	"clai/internal/llm"
	"clai/internal/ui"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

func getStackTrace() string {
	return string(debug.Stack())
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "debug" {
		runDebugCommand(os.Args[2:])
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC: %v\n", r)
			log.Printf("STACK TRACE:\n%s", getStackTrace())
			fmt.Fprintf(os.Stderr, "PANIC: %v\n", r)
			fmt.Fprintf(os.Stderr, "STACK TRACE:\n%s", getStackTrace())
			os.Exit(2)
		}
	}()
	// Check for TTY (interactive terminal) by trying to open /dev/tty
	ttyCheck, err := os.Open("/dev/tty")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: This program requires an interactive terminal (TTY). Exiting.")
		os.Exit(1)
	}
	ttyCheck.Close()
	// Log to debug.log, overwrite each run
	logFile, err := os.Create("debug.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open debug.log for writing: %v\n", err)
		os.Exit(1)
	}
	log.SetOutput(logFile)
	log.SetFlags(log.Ltime) // Only show time, not date

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

	store, err := db.New()
	if err != nil {
		log.Printf("Failed to initialize database: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	modelInfo, err := llmClient.GetModelInfo()
	var assistantIntro string
	if err != nil {
		log.Printf("Failed to get model info: %v", err)
		assistantIntro = fmt.Sprintf("Model: %s\nUnable to retrieve model details.", modelName)
	} else {
		assistantIntro = fmt.Sprintf("Model: %s", modelName)

		if modelInfo.Details.Family != "" {
			assistantIntro += fmt.Sprintf("\nFamily: %s", modelInfo.Details.Family)
		}
		if modelInfo.Details.ParameterSize != "" {
			assistantIntro += fmt.Sprintf("\nParameters: %s", modelInfo.Details.ParameterSize)
		}
		if modelInfo.Details.QuantizationLevel != "" {
			assistantIntro += fmt.Sprintf("\nQuantization: %s", modelInfo.Details.QuantizationLevel)
		}
		if modelInfo.Details.Format != "" {
			assistantIntro += fmt.Sprintf("\nFormat: %s", modelInfo.Details.Format)
		}
		if contextLen, ok := modelInfo.ModelInfo["llama.context_length"].(float64); ok {
			assistantIntro += fmt.Sprintf("\nContext Length: %.0f", contextLen)
		}
	}

	chatInput := textinput.New()
	chatInput.Prompt = "> "
	chatInput.Placeholder = "Type your message..."
	chatInput.Focus()
	chatInput.CharLimit = 256
	chatInput.Width = 40
	chatInput.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFAA")).Bold(true).Underline(true)
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
		Theme:         ui.AvailableThemes[0],
		DB:            store,
	}

	conv, err := store.GetLatestConversation()
	if err != nil {
		log.Printf("Failed to load latest conversation: %v", err)
	}
	if conv == nil {
		conv = &db.Conversation{
			Title:    "New Conversation",
			Messages: []llm.Message{{Role: "assistant", Content: assistantIntro}},
		}
		log.Printf("[DB] Starting new conversation")
	} else {
		log.Printf("[DB] Loaded conversation %d with %d messages", conv.ID, len(conv.Messages))
	}
	m.Conversation = conv

	chat := ui.ChatModel{
		TextInput:    chatInput,
		LlmClient:    llmClient,
		Spinner:      spin,
		Theme:        m.Theme,
		AutoScroll:   true,
		UserScrolled: false,
	}
	chat.AssistantName = "assistant"
	chat.Messages = conv.Messages
	chat.ContentDirty = true
	chat.Width = 80
	chat.Height = 20
	chat.Viewport = viewport.New(chat.Width, chat.Height)
	chat.Viewport.MouseWheelEnabled = true
	chat.Viewport.MouseWheelDelta = 3

	for _, msg := range conv.Messages {
		if msg.Role == "user" {
			chat.QueryHistory = append(chat.QueryHistory, msg.Content)
		}
	}

	m.Chat = chat
	// Let Bubble Tea detect terminal size automatically via WindowSizeMsg
	log.Printf("Starting app, Bubble Tea will detect terminal size...")

	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if err := ui.StartDebugServer(p); err != nil {
		log.Printf("Warning: Failed to start debug server: %v", err)
	}

	if _, err := p.Run(); err != nil {
		log.Println("Fatal error:", err)
		os.Exit(1)
	}
}
