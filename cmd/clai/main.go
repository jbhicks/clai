package main

import (
	"clai/internal/benchmark"
	"clai/internal/db"
	"clai/internal/llm"
	"clai/internal/logger"
	"clai/internal/ui"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

// Build information - set at build time with -ldflags
var (
	buildTime  string
	gitCommit  string
	buildCount string
	buildRand  string
)

func getStackTrace() string {
	return string(debug.Stack())
}

func getBuildIdentifier() string {
	// Build identifier for restart verification
	parts := []string{}

	if buildTime != "" {
		parts = append(parts, buildTime)
	}
	if gitCommit != "" {
		// Truncate commit hash to first 7 characters for display
		if len(gitCommit) > 7 {
			parts = append(parts, gitCommit[:7])
		} else {
			parts = append(parts, gitCommit)
		}
	}
	if buildCount != "" {
		parts = append(parts, "b"+buildCount)
	}
	if buildRand != "" {
		parts = append(parts, buildRand)
	}

	if len(parts) == 0 {
		return "dev"
	}

	return strings.Join(parts, "-")
}

func main() {
	// Check for subcommands first - these should run without TUI setup
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "debug":
			runDebugCommand(os.Args[2:])
			return
		case "models":
			runModelsCommand(os.Args[2:])
			return
		case "ask":
			runAskCommand(os.Args[2:])
			return
		case "benchmark":
			runBenchmarkCommand(os.Args[2:])
			return
		}
	}

	// Only for TUI mode - initialize logging and screen clearing
	startTime := time.Now()
	logger.Info("CLAI startup beginning")

	// Clear screen immediately on startup to remove any previous output (especially in tmux)
	fmt.Print("\x1b[2J\x1b[H")
	logger.Info("Screen cleared in %v", time.Since(startTime))

	defer func() {
		if r := recover(); r != nil {
			logger.Error("PANIC: %v", r)
			logger.Error("STACK TRACE:\n%s", getStackTrace())
			fmt.Fprintf(os.Stderr, "PANIC: %v\n", r)
			fmt.Fprintf(os.Stderr, "STACK TRACE:\n%s", getStackTrace())
			os.Exit(2)
		}
	}()
	// Check for TTY (interactive terminal) by trying to open /dev/tty
	// Skip TTY check in development environments if CLAI_DEV is set
	if os.Getenv("CLAI_DEV") != "1" {
		ttyCheck, err := os.Open("/dev/tty")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: This program requires an interactive terminal (TTY). Exiting.")
			os.Exit(1)
		}
		ttyCheck.Close()
	}
	logger.Info("TTY check completed in %v", time.Since(startTime))

	// Detect terminal size for TUI (works in tmux/screen)
	terminalWidth, terminalHeight := 80, 24
	if linesOut, err := exec.Command("tput", "lines").Output(); err == nil {
		if colsOut, err := exec.Command("tput", "cols").Output(); err == nil {
			if h, err := strconv.Atoi(strings.TrimSpace(string(linesOut))); err == nil {
				if w, err := strconv.Atoi(strings.TrimSpace(string(colsOut))); err == nil {
					terminalWidth, terminalHeight = w, h
				}
			}
		}
	}
	logger.Info("Terminal size detection completed in %v (size: %dx%d)", time.Since(startTime), terminalWidth, terminalHeight)
	// Log to debug.log, overwrite each run
	logFile, err := os.Create("debug.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open debug.log for writing: %v\n", err)
		os.Exit(1)
	}
	logger.Init(logFile)
	logger.Info("Logger initialization completed in %v", time.Since(startTime))

	// Handle SIGINT for clean shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Received signal, shutting down gracefully...")
		ui.StopDebugServer()
		logFile.Close()
		// Reset terminal state before exit
		fmt.Print("\x1b[2J\x1b[H\x1b[?1049l") // Clear screen, reset cursor, exit alt screen
		os.Exit(0)
	}()

	_ = godotenv.Load()
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "llama3.1-gpu:latest"
	}
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:8081"
	}
	systemPrompt := os.Getenv("SYSTEM_PROMPT")

	flag.Parse()
	llmClient := llm.NewClient(host, modelName, systemPrompt)
	logger.Info("LLM client creation completed in %v", time.Since(startTime))

	store, err := db.New()
	if err != nil {
		logger.Error("Failed to initialize database: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()
	logger.Info("Database initialization completed in %v", time.Since(startTime))

	modelInfo, err := llmClient.GetModelInfo()
	var assistantIntro string
	if err != nil {
		logger.Warn("Failed to get model info: %v", err)
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
	logger.Info("Model info fetching completed in %v", time.Since(startTime))

	availableThemes := ui.GetAvailableThemes()
	theme := availableThemes[0]

	chatInput := textarea.New()
	chatInput.Placeholder = "Type your message..."
	chatInput.CharLimit = 256
	chatInput.SetWidth(40)
	chatInput.SetHeight(3) // Multi-line input
	chatInput.ShowLineNumbers = false

	bgColor := lipgloss.Color(theme.Theme.Primary.Background)
	// Set comprehensive background styling for textarea
	chatInput.FocusedStyle.Base = lipgloss.NewStyle().Background(bgColor)
	chatInput.BlurredStyle.Base = lipgloss.NewStyle().Background(bgColor)
	chatInput.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(bgColor)
	chatInput.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Background(bgColor)
	chatInput.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Background(bgColor)
	chatInput.FocusedStyle.LineNumber = lipgloss.NewStyle().Background(bgColor)
	chatInput.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.DimForeground)).Background(bgColor)
	chatInput.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.DimForeground)).Background(bgColor)
	chatInput.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.Foreground)).Background(bgColor)
	chatInput.BlurredStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.Foreground)).Background(bgColor)
	chatInput.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Bright.Yellow)).Background(bgColor)
	chatInput.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Bright.Yellow)).Background(bgColor)
	chatInput.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFAA")).Bold(true).Underline(true).Background(bgColor)
	chatInput.Focus()

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	helpModel := help.New()
	helpModel.ShowAll = false
	helpModel.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.Foreground))
	helpModel.Styles.FullDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.DimForeground))
	helpModel.Styles.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.DimForeground))
	debugChan := make(chan ui.DebugServerMsg, 10) // buffered channel for debug commands

	// Calculate UI layout dimensions using constants from ui package
	chatWidth := int(float64(terminalWidth) * ui.ChatPaneWidthRatio)
	if chatWidth < ui.MinPaneWidth {
		chatWidth = ui.MinPaneWidth
	}
	statusWidth := terminalWidth - chatWidth
	if statusWidth < ui.MinPaneWidth {
		statusWidth = ui.MinPaneWidth
	}
	contentHeight := terminalHeight - ui.StatusBarHeight

	statusHeight := contentHeight / 2
	logHeight := contentHeight - statusHeight

	m := &ui.Model{
		Width:         terminalWidth,
		Height:        terminalHeight,
		Log:           ui.SetupLogComponent(statusWidth, logHeight),
		AgentStatus:   ui.SetupAgentStatusComponent(theme),
		Help:          helpModel,
		Keys:          ui.DefaultKeyMap,
		StatusBarText: "",
		ActivePane:    ui.ChatPane,
		ErrorBanner:   lipgloss.NewStyle().Background(lipgloss.Color("9")).Foreground(lipgloss.Color("15")).Padding(0, 1),
		Theme:         theme,
		DB:            store,
		Layout:        ui.DefaultLayoutConfig(),
	}

	// Re-setup textarea styling with the (potentially loaded) theme
	bgColor = lipgloss.Color(theme.Theme.Primary.Background)
	chatInput.FocusedStyle.Base = lipgloss.NewStyle().Background(bgColor)
	chatInput.BlurredStyle.Base = lipgloss.NewStyle().Background(bgColor)
	chatInput.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(bgColor)
	chatInput.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Background(bgColor)
	chatInput.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Background(bgColor)
	chatInput.FocusedStyle.LineNumber = lipgloss.NewStyle().Background(bgColor)
	chatInput.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.DimForeground)).Background(bgColor)
	chatInput.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.DimForeground)).Background(bgColor)
	chatInput.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.Foreground)).Background(bgColor)
	chatInput.BlurredStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.Foreground)).Background(bgColor)
	chatInput.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Bright.Yellow)).Background(bgColor)
	chatInput.BlurredStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Bright.Yellow)).Background(bgColor)
	chatInput.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFAA")).Bold(true).Underline(true).Background(bgColor)

	m.Agent = llm.NewAgent(llmClient)
	logger.Info("Agent mode enabled")

	conv, err := store.GetLatestConversation()
	if err != nil {
		logger.Warn("Failed to load latest conversation: %v", err)
	}
	if conv == nil {
		conv = &db.Conversation{
			Title:    "New Conversation",
			Messages: []llm.Message{{Role: "assistant", Content: assistantIntro}},
		}
		logger.Info("Starting new conversation")
	} else {
		logger.Info("Loaded conversation %d with %d messages", conv.ID, len(conv.Messages))
	}
	m.Conversation = conv
	logger.Info("Conversation loading completed in %v", time.Since(startTime))

	// Use the simplified UI setup functions
	chatHeight := terminalHeight - ui.StatusBarHeight

	chat := ui.SetupChatComponent(chatWidth, chatHeight, theme, llmClient)
	chat.AssistantName = "assistant"
	chat.Messages = conv.Messages
	chat.ContentDirty = true
	chat.NeedsInitialScroll = true

	for _, msg := range conv.Messages {
		if msg.Role == "user" {
			chat.QueryHistory = append(chat.QueryHistory, msg.Content)
		}
	}

	m.Chat = chat
	// Let Bubble Tea detect terminal size automatically via WindowSizeMsg
	logger.Info("Starting app, Bubble Tea will detect terminal size...")

	// Create Bubble Tea program - skip alternate screen in tmux for better pane clearing
	var opts []tea.ProgramOption
	if os.Getenv("TMUX") == "" {
		opts = append(opts, tea.WithAltScreen())
	}
	p := tea.NewProgram(m, opts...)

	// Start benchmark server in background
	logger.Info("Starting benchmark server in background...")
	logger.Info("DEBUG: ActivePane initialized to: %d", ui.ChatPane)
	benchmarkServer := benchmark.NewServer(store)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("BENCHMARK SERVER PANIC: %v", r)
				logger.Error("BENCHMARK STACK TRACE:\n%s", getStackTrace())
			}
		}()
		port, err := benchmarkServer.Start()
		if err != nil {
			logger.Error("Failed to start benchmark server: %v", err)
		} else {
			logger.Info("Benchmark server started on port %d", port)
			p.Send(ui.WebUIPortMsg{Port: port})
		}
	}()

	// Start goroutine to bridge debug channel to tea program
	go func() {
		for msg := range debugChan {
			logger.Info("[DEBUG] Sending DebugServerMsg to Bubble Tea")
			p.Send(msg)
			logger.Info("[DEBUG] DebugServerMsg sent to Bubble Tea")
		}
	}()

	if err := ui.StartDebugServer(debugChan); err != nil {
		logger.Warn("Failed to start debug server: %v", err)
	}

	// Clear screen immediately before starting TUI
	fmt.Print("\x1b[2J\x1b[H")
	logger.Info("All initialization completed in %v, starting TUI", time.Since(startTime))

	if _, err := p.Run(); err != nil {
		logger.Error("Fatal error: %v", err)
		ui.StopDebugServer()
		logFile.Close()
		os.Exit(1)
	}

	ui.StopDebugServer()
	logFile.Close()
}

// Trigger rebuild
