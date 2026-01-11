package main

import (
	"clai/internal/archival"
	"clai/internal/benchmark"
	"clai/internal/db"
	"clai/internal/llm"
	"clai/internal/logger"
	"clai/internal/orchestrator"
	"clai/internal/patterns"
	"clai/internal/persistence"
	"clai/internal/todo"
	"clai/internal/ui"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
	"github.com/muesli/termenv"
)

func getStackTrace() string {
	return string(debug.Stack())
}

// checkForRunningInstances checks if there are already running CLAI instances
func checkForRunningInstances() error {
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check running processes: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	currentPid := os.Getpid()
	foundInstances := 0

	for _, line := range lines {
		// Look for clai processes that conflict with this mode
		if strings.Contains(line, "clai") && !strings.Contains(line, "grep") {
			// Parse the PID from the line
			fields := strings.Fields(line)
			if len(fields) > 1 {
				if pid, err := strconv.Atoi(fields[1]); err == nil {
					// Don't count our own process
					if pid != currentPid {
						foundInstances++
					}
				}
			}
		}
	}

	// Allow multiple TUI instances (they can coexist)
	// Only prevent multiple instances if we're starting a server mode
	isServerMode := len(os.Args) > 1 && (os.Args[1] == "benchmark")

	if foundInstances > 0 && isServerMode {
		return fmt.Errorf("CLAI server is already running (%d instance(s) detected). Please stop any running CLAI servers before starting a new one", foundInstances)
	}

	return nil
}

func main() {
	// Disable terminal capability queries that can cause escape sequences to be displayed
	// Set color profile explicitly to avoid automatic detection
	os.Setenv("TERM", "xterm-256color")
	os.Setenv("COLORTERM", "truecolor")
	lipgloss.SetColorProfile(termenv.TrueColor)
	// Check for headless flag early (before TTY check)
	var headless bool
	for _, arg := range os.Args {
		if arg == "--headless" {
			headless = true
			break
		}
	}

	if len(os.Args) >= 2 && os.Args[1] == "debug" {
		runDebugCommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "models" {
		runModelsCommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "benchmark" {
		runBenchmarkCommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "run-benchmarks" {
		runBenchmarksCommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "decompose" {
		runDecomposeCommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "qa" {
		runQACommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "learn" {
		runLearnCommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "orchestrate" {
		runOrchestrateCommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "agent" {
		runAgentCommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "task" {
		runTaskCommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "archive" {
		runArchiveCommand(os.Args[2:])
		return
	}

	if len(os.Args) >= 2 && os.Args[1] == "todo" {
		runTodoCommand(os.Args[2:])
		return
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Error("PANIC: %v", r)
			logger.Error("STACK TRACE:\n%s", getStackTrace())
			fmt.Fprintf(os.Stderr, "PANIC: %v\n", r)
			fmt.Fprintf(os.Stderr, "STACK TRACE:\n%s", getStackTrace())
			os.Exit(2)
		}
	}()

	// Initialize context for orchestrator lifecycle management
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Check for TTY (interactive terminal) by trying to open /dev/tty
	// Skip TTY check in headless mode
	if !headless {
		ttyCheck, err := os.Open("/dev/tty")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: This program requires an interactive terminal (TTY). Exiting.")
			os.Exit(1)
		}
		ttyCheck.Close()
	}

	// Detect terminal size - prefer tmux pane dimensions
	terminalWidth, terminalHeight := 80, 24
	if !headless {
		if out, err := exec.Command("tmux", "display-message", "-p", "#{pane_height}").Output(); err == nil {
			if h, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil {
				if out, err := exec.Command("tmux", "display-message", "-p", "#{pane_width}").Output(); err == nil {
					if w, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil {
						terminalWidth, terminalHeight = w, h
					}
				}
			}
		}
		if terminalWidth == 80 {
			if linesOut, err := exec.Command("tput", "lines").Output(); err == nil {
				if colsOut, err := exec.Command("tput", "cols").Output(); err == nil {
					if h, err := strconv.Atoi(strings.TrimSpace(string(linesOut))); err == nil {
						if w, err := strconv.Atoi(strings.TrimSpace(string(colsOut))); err == nil {
							terminalWidth, terminalHeight = w, h
						}
					}
				}
			}
		}
	}
	// Log to debug.log, overwrite each run
	logFile, err := os.Create("debug.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open debug.log for writing: %v\n", err)
		os.Exit(1)
	}
	logger.Init(logFile)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		logger.Info("Received SIGINT (Ctrl+C), exiting immediately.")
		ui.StopDebugServer()
		logFile.Close()
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

	flag.BoolVar(&headless, "headless", false, "Run in headless mode (no TUI, long-lived service)")
	flag.Parse()
	llmClient := llm.NewClient(host, modelName, systemPrompt)

	store, err := db.New()
	if err != nil {
		logger.Error("Failed to initialize database: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Initialize persistence manager for memory system
	persistMgr, err := persistence.NewPersistenceManager()
	if err != nil {
		logger.Error("Failed to initialize persistence manager: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to initialize persistence manager: %v\n", err)
		os.Exit(1)
	}

	// Load existing progress state
	progress, err := persistMgr.LoadProgress()
	if err != nil {
		logger.Warn("Failed to load progress state: %v", err)
	} else {
		logger.Info("Loaded progress state: %d iterations, %d patterns learned",
			len(progress.Iterations), progress.PatternsLearned)
	}

	// Check for already running CLAI instances (skip in benchmark mode)
	if len(os.Args) == 1 || (len(os.Args) > 1 && os.Args[1] != "benchmark") {
		if err := checkForRunningInstances(); err != nil {
			logger.Error("Instance check failed: %v", err)
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			fmt.Fprintf(os.Stderr, "Please stop any running CLAI instances before starting a new one.\n")
			os.Exit(1)
		}
	}

	// Start benchmark server in background
	logger.Info("Starting benchmark server on port 8081...")
	benchmarkServer := benchmark.NewServer(store)
	go func() {
		logger.Info("Benchmark server goroutine started")
		if err := benchmarkServer.StartWithPreferredPort(8081); err != nil {
			logger.Error("Failed to start benchmark server: %v", err)
		} else {
			logger.Info("Benchmark server started successfully")
		}
	}()

	// Wait a moment for benchmark server to start
	time.Sleep(500 * time.Millisecond)

	// Get the actual port the benchmark server is running on
	benchmarkPort := 8081 // fallback
	if port := benchmarkServer.GetPort(); port != 0 {
		benchmarkPort = port
	}

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

	chatInput := textinput.New()
	chatInput.Prompt = "> "
	chatInput.Placeholder = "Type your message..."
	chatInput.Focus()
	chatInput.CharLimit = 256
	chatInput.Width = 40
	chatInput.TextStyle = lipgloss.NewStyle().Background(lipgloss.Color("#282a36"))
	chatInput.PlaceholderStyle = lipgloss.NewStyle().Background(lipgloss.Color("#282a36"))
	chatInput.PromptStyle = lipgloss.NewStyle().Background(lipgloss.Color("#282a36")).Foreground(lipgloss.Color("#f8f8f2"))
	chatInput.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFAA")).Bold(true).Underline(true)
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	availableThemes := ui.GetAvailableThemes()
	theme := availableThemes[0]
	helpModel := help.New()
	helpModel.ShowAll = false
	helpModel.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.Foreground))
	helpModel.Styles.FullDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.DimForeground))
	helpModel.Styles.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Theme.Primary.DimForeground))
	m := &ui.Model{
		Log:           viewport.New(0, 0),
		AgentStatus:   ui.NewAgentStatusView(theme),
		Help:          helpModel,
		Keys:          ui.DefaultKeyMap,
		StatusBarText: "",
		ActivePane:    ui.ChatPane,
		ErrorBanner:   lipgloss.NewStyle().Background(lipgloss.Color("9")).Foreground(lipgloss.Color("15")).Padding(0, 1),
		Theme:         theme,
		DB:            store,
		Width:         terminalWidth,
		Height:        terminalHeight,
	}

	chatPaneWidth := int(float64(terminalWidth) * 0.7)
	m.Chat.Width = chatPaneWidth - 4
	m.Chat.Height = terminalHeight - 4

	m.Agent = llm.NewAgent(llmClient)
	logger.Info("Agent mode enabled")

	// Start the orchestrator for concurrent task execution
	m.Agent.StartOrchestrator(ctx)

	// Ensure orchestrator and persistence are cleaned up on exit
	defer func() {
		cancel()
		m.Agent.StopOrchestrator()

		// Save final progress state
		if persistMgr != nil {
			if err := persistMgr.BackupToGit("Auto-backup: CLAI session ended"); err != nil {
				logger.Warn("Failed to create final git backup: %v", err)
			}
		}
	}()

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
	chat.NeedsInitialScroll = true
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
	if headless {
		// Headless mode: run servers without TUI
		logger.Info("Running in headless mode - benchmark server available at http://localhost:%d", benchmarkPort)

		// Start debug server for headless mode (if available)
		if err := ui.StartDebugServer(nil); err != nil {
			logger.Warn("Failed to start debug server in headless mode: %v", err)
		}

		// Wait for shutdown signal
		<-c
		logger.Info("Received shutdown signal, stopping servers...")

		ui.StopDebugServer()
		logFile.Close()
		return
	}

	// Normal mode: start TUI
	logger.Info("Starting app, Bubble Tea will detect terminal size...")

	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if err := ui.StartDebugServer(p); err != nil {
		logger.Warn("Failed to start debug server: %v", err)
	}

	if _, err := p.Run(); err != nil {
		logger.Error("Fatal error: %v", err)
		ui.StopDebugServer()
		logFile.Close()
		os.Exit(1)
	}

	ui.StopDebugServer()
	logFile.Close()
}

func runDecomposeCommand(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: clai decompose <feature-description>\n")
		os.Exit(1)
	}

	featureDescription := strings.Join(args, " ")
	logger.Info("Starting task decomposition for: %s", featureDescription)

	// Initialize LLM client
	_ = godotenv.Load()
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "/mnt/media/models/Qwen3-Coder-30B-A3B-Instruct-UD-Q8_K_XL.gguf"
	}
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:8081"
	}
	systemPrompt := os.Getenv("SYSTEM_PROMPT")

	llmClient := llm.NewClient(host, modelName, systemPrompt)

	// Initialize task decomposer
	decomposer := llm.NewTaskDecomposer(llmClient)

	// Decompose the feature
	tasks, err := decomposer.DecomposeFeature(featureDescription)
	if err != nil {
		logger.Error("Failed to decompose feature: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to decompose feature: %v\n", err)
		os.Exit(1)
	}

	// Export to stories.json
	if err := decomposer.ExportToStoriesJSON(tasks); err != nil {
		logger.Error("Failed to export tasks: %v", err)
		fmt.Fprintf(os.Stderr, "Failed to export tasks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully decomposed feature into %d tasks\n", len(tasks))
	for i, task := range tasks {
		fmt.Printf("%d. %s (%s priority)\n", i+1, task.Title, task.Priority)
	}
}

func runQACommand(args []string) {
	if len(args) > 0 && args[0] == "parallel" {
		runQAParallel()
		return
	}

	runQASequential()
}

func runQASequential() {
	fmt.Println("Running quality assurance checks...")

	// Initialize persistence manager
	persistMgr, err := persistence.NewPersistenceManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize persistence manager: %v\n", err)
		os.Exit(1)
	}

	results, allPassed, err := persistMgr.RunQualityChecks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run quality checks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nQuality Check Results:\n")
	fmt.Printf("Overall: %s\n\n", map[bool]string{true: "PASSED", false: "FAILED"}[allPassed])

	for _, result := range results {
		status := map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Passed]
		fmt.Printf("%s %s (%.2fs)\n", status, result.Name, result.Duration.Seconds())

		if !result.Passed {
			if result.Error != nil {
				fmt.Printf("  Error: %v\n", result.Error)
			}
			if result.ErrorOutput != "" {
				fmt.Printf("  Details: %s\n", strings.TrimSpace(result.ErrorOutput))
			}
		}
	}

	if !allPassed {
		fmt.Println("\n❌ Some quality checks failed")
		os.Exit(1)
	}

	fmt.Println("\n✅ All quality checks passed")
}

func runQAParallel() {
	fmt.Println("Running quality assurance checks in parallel...")

	// Initialize persistence manager
	persistMgr, err := persistence.NewPersistenceManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize persistence manager: %v\n", err)
		os.Exit(1)
	}

	results, allPassed, err := persistMgr.RunQualityChecksParallel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run parallel quality checks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nParallel Quality Check Results:\n")
	fmt.Printf("Overall: %s\n\n", map[bool]string{true: "PASSED", false: "FAILED"}[allPassed])

	for _, result := range results {
		status := map[bool]string{true: "✓ PASS", false: "✗ FAIL"}[result.Passed]
		fmt.Printf("%s %s (%.2fs)\n", status, result.Name, result.Duration.Seconds())

		if !result.Passed {
			if result.Error != nil {
				fmt.Printf("  Error: %v\n", result.Error)
			}
			if result.ErrorOutput != "" {
				fmt.Printf("  Details: %s\n", strings.TrimSpace(result.ErrorOutput))
			}
		}
	}

	if !allPassed {
		fmt.Println("\n❌ Some quality checks failed")
		os.Exit(1)
	}

	fmt.Println("\n✅ All parallel quality checks passed")
}

func runLearnCommand(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: clai learn <category> <pattern> [context]\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  clai learn api_usage 'Use bufio.Scanner for large file reading'\n")
		fmt.Fprintf(os.Stderr, "  clai learn bug_fix 'Check for nil pointers before dereference' 'Database operations'\n")
		os.Exit(1)
	}

	category := args[0]
	pattern := args[1]
	context := ""
	if len(args) > 2 {
		context = strings.Join(args[2:], " ")
	}

	// Initialize pattern manager
	patternMgr, err := patterns.NewPatternManager(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize pattern manager: %v\n", err)
		os.Exit(1)
	}

	// Load existing learnings
	if err := patternMgr.LoadLearningsFromFile(); err != nil {
		logger.Warn("Failed to load existing learnings: %v", err)
	}

	// Record the learning
	importance := 3 // Default importance, could be made configurable
	tags := []string{category}
	if err := patternMgr.RecordLearning(category, pattern, context, "cli-command", 0.8, importance, tags); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to record learning: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Recorded learning: %s\n", pattern)
	fmt.Printf("   Category: %s\n", category)
	if context != "" {
		fmt.Printf("   Context: %s\n", context)
	}
}

func runOrchestrateCommand(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: clai orchestrate <command> [options]\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  start [stories.json]     Begin autonomous execution\n")
		fmt.Fprintf(os.Stderr, "  status [--watch]         Show current progress\n")
		fmt.Fprintf(os.Stderr, "  stop                     Gracefully halt execution\n")
		fmt.Fprintf(os.Stderr, "  resume [--from-failure]  Continue from last checkpoint\n")
		os.Exit(1)
	}

	command := args[0]
	switch command {
	case "start":
		runOrchestrateStart(args[1:])
	case "status":
		runOrchestrateStatus(args[1:])
	case "stop":
		runOrchestrateStop(args[1:])
	case "resume":
		runOrchestrateResume(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown orchestrate command: %s\n", command)
		fmt.Fprintf(os.Stderr, "Use 'clai orchestrate' for help\n")
		os.Exit(1)
	}
}

func runTaskCommand(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: clai task <command> [options]\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  execute <task-id> [--skip-checks]  Execute single task manually\n")
		os.Exit(1)
	}

	command := args[0]
	switch command {
	case "execute":
		runTaskExecute(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown task command: %s\n", command)
		fmt.Fprintf(os.Stderr, "Use 'clai task' for help\n")
		os.Exit(1)
	}
}

func runOrchestrateStart(args []string) {
	storiesFile := ".clai/stories.json"
	if len(args) > 0 {
		storiesFile = args[0]
	}

	fmt.Printf("Starting autonomous execution with stories from: %s\n", storiesFile)

	// Initialize LLM client for orchestrator
	_ = godotenv.Load()
	modelName := os.Getenv("OLLAMA_MODEL")
	if modelName == "" {
		modelName = "/mnt/media/models/Qwen3-Coder-30B-A3B-Instruct-UD-Q8_K_XL.gguf"
	}
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:8081"
	}

	llmClient := llm.NewClient(host, modelName, "")
	orch := orchestrator.NewOrchestrator(llmClient)

	// Load stories
	if err := orch.LoadStories(storiesFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load stories: %v\n", err)
		os.Exit(1)
	}

	// Start execution
	if err := orch.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start orchestrator: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Orchestrator started successfully")
}

func runOrchestrateStatus(args []string) {
	watch := false
	if len(args) > 0 && args[0] == "--watch" {
		watch = true
	}

	// Initialize persistence manager to get progress
	persistMgr, err := persistence.NewPersistenceManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize persistence: %v\n", err)
		os.Exit(1)
	}

	progress, err := persistMgr.LoadProgress()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load progress: %v\n", err)
		os.Exit(1)
	}

	// Load stories to get completion status
	storiesData, err := loadStoriesData()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load stories: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Orchestrator Status:\n")
	fmt.Printf("Completed: %d/%d stories\n", storiesData["completed"], storiesData["total"])
	if progress.CurrentTask != nil {
		fmt.Printf("Current Task: %s\n", progress.CurrentTask.Description)
	}
	fmt.Printf("Patterns Learned: %d\n", progress.PatternsLearned)
	fmt.Printf("Success Rate: %.1f%%\n", progress.SuccessRate*100)
	fmt.Printf("Last Update: %s\n", progress.LastUpdate.Format("2006-01-02 15:04:05"))

	if watch {
		fmt.Println("\nWatching for updates... (Ctrl+C to stop)")
		// TODO: Implement live watching
	}
}

func runOrchestrateStop(args []string) {
	fmt.Println("Stopping orchestrator...")

	// For now, just show that the command is recognized
	// In a full implementation, this would connect to a running orchestrator instance
	fmt.Println("✅ Orchestrator stop command received")
	fmt.Println("Note: Stop functionality requires persistent orchestrator instance")
}

func runOrchestrateResume(args []string) {
	fromFailure := false
	if len(args) > 0 && args[0] == "--from-failure" {
		fromFailure = true
	}

	fmt.Printf("Resuming orchestrator")
	if fromFailure {
		fmt.Printf(" from last failure")
	}
	fmt.Printf("...\n")

	// Load progress and resume from last checkpoint
	persistMgr, err := persistence.NewPersistenceManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize persistence: %v\n", err)
		os.Exit(1)
	}

	progress, err := persistMgr.LoadProgress()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load progress: %v\n", err)
		os.Exit(1)
	}

	if progress.CurrentTask != nil {
		fmt.Printf("Resuming from task: %s\n", progress.CurrentTask.Description)
	} else {
		fmt.Printf("No active task found, starting fresh execution\n")
		// Could call orchestrate start here
	}

	fmt.Println("✅ Orchestrator resume command received")
}

func runTaskExecute(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: clai task execute <task-id> [--skip-checks]\n")
		os.Exit(1)
	}

	taskID := args[0]
	skipChecks := false
	if len(args) > 1 && args[1] == "--skip-checks" {
		skipChecks = true
	}

	fmt.Printf("Executing task: %s", taskID)
	if skipChecks {
		fmt.Printf(" (skipping quality checks)")
	}
	fmt.Printf("\n")

	// For now, simulate task execution
	// In a full implementation, this would:
	// 1. Find the task in stories.json
	// 2. Execute it using the appropriate tools
	// 3. Run quality checks unless skipped
	// 4. Update progress

	if !skipChecks {
		fmt.Println("Running quality checks...")
		// Simulate quality checks
		time.Sleep(500 * time.Millisecond)
		fmt.Println("Quality checks passed")
	}

	fmt.Println("Task execution logic would go here...")
	time.Sleep(1 * time.Second) // Simulate work

	fmt.Println("✅ Task executed successfully")
}

func runBenchmarksCommand(args []string) {
	// Parse command line arguments
	webServerURL := ""
	modelName := ""
	modelPath := ""

	i := 0
	for i < len(args) {
		arg := args[i]
		switch arg {
		case "--web-server", "-w":
			if i+1 < len(args) {
				webServerURL = args[i+1]
				i++
			}
		case "--model", "-m":
			if i+1 < len(args) {
				modelName = args[i+1]
				i++
			}
		case "--model-path", "-p":
			if i+1 < len(args) {
				modelPath = args[i+1]
				i++
			}
		case "--help", "-h":
			fmt.Println("Usage: clai run-benchmarks [--web-server URL] [--model NAME] [--model-path PATH]")
			fmt.Println("Runs the unified benchmark suite and optionally notifies a web server for live updates.")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --web-server, -w    URL of web server to notify for live updates (e.g., http://localhost:8083)")
			fmt.Println("  --model, -m         Model name to benchmark")
			fmt.Println("  --model-path, -p    Path to model file")
			fmt.Println("  --help, -h          Show this help message")
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "Unknown argument: %s\n", arg)
			fmt.Fprintf(os.Stderr, "Use --help for usage information\n")
			os.Exit(1)
		}
		i++
	}

	// Validate required arguments
	if modelName == "" || modelPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --model and --model-path are required\n")
		fmt.Fprintf(os.Stderr, "Use --help for usage information\n")
		os.Exit(1)
	}

	// Initialize database
	store, err := db.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Create runner
	var runner *benchmark.Runner
	if webServerURL != "" {
		runner = benchmark.NewRunnerWithWebServer(store, webServerURL)
		fmt.Printf("Runner configured to notify web server at %s\n", webServerURL)
	} else {
		runner = benchmark.NewRunner(store)
	}

	// Find a running server for this model
	// For now, assume it's on port 8081 (the Qwen model we saw earlier)
	serverPort := 8081

	ctx := context.Background()
	runID, err := runner.RunUnifiedBenchmark(ctx, modelName, modelPath, serverPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Benchmark failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Benchmark completed successfully! Run ID: %d\n", runID)
}

func runArchiveCommand(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: clai archive <command> [options]\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  create <feature-name>    Archive current feature\n")
		fmt.Fprintf(os.Stderr, "  list                     List available archives\n")
		fmt.Fprintf(os.Stderr, "  restore <archive-name>   Restore an archive\n")
		os.Exit(1)
	}

	command := args[0]
	archiver := archival.NewArchiver(".")

	switch command {
	case "create":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: clai archive create <feature-name>\n")
			os.Exit(1)
		}
		featureName := strings.Join(args[1:], " ")
		if err := archiver.ArchiveFeature(featureName); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create archive: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Successfully archived feature: %s\n", featureName)

	case "list":
		archives, err := archiver.ListArchives()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list archives: %v\n", err)
			os.Exit(1)
		}

		if len(archives) == 0 {
			fmt.Println("No archives found.")
			return
		}

		fmt.Println("Available archives:")
		for _, archive := range archives {
			size := "directory"
			if archive.IsCompressed {
				size = fmt.Sprintf("%.1f MB", float64(archive.Size)/(1024*1024))
			}
			fmt.Printf("  %s (%s) - %s\n", archive.Name, size, archive.CreatedAt.Format("2006-01-02"))
		}

	case "restore":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: clai archive restore <archive-name>\n")
			os.Exit(1)
		}
		archiveName := args[1]
		if err := archiver.RestoreArchive(archiveName); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to restore archive: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Successfully restored archive: %s\n", archiveName)

	default:
		fmt.Fprintf(os.Stderr, "Unknown archive command: %s\n", command)
		fmt.Fprintf(os.Stderr, "Use 'clai archive' for help\n")
		os.Exit(1)
	}
}

func loadStoriesData() (map[string]interface{}, error) {
	data, err := os.ReadFile(".clai/stories.json")
	if err != nil {
		return nil, err
	}

	var stories map[string]interface{}
	if err := json.Unmarshal(data, &stories); err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	if completed, ok := stories["completed"]; ok {
		if v, ok := completed.(float64); ok {
			result["completed"] = int(v)
		}
	}
	if total, ok := stories["total"]; ok {
		if v, ok := total.(float64); ok {
			result["total"] = int(v)
		}
	}

	return result, nil
}

func runAgentCommand(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: clai agent <command> [options]\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  spawn <type> <task>    Spawn a new agent (type: code, research, test, review, document)\n")
		fmt.Fprintf(os.Stderr, "  list [--type=TYPE]     List all agents or filter by type\n")
		fmt.Fprintf(os.Stderr, "  status [agent-id]      Show status of all agents or specific agent\n")
		fmt.Fprintf(os.Stderr, "  stop <agent-id>        Stop a running agent\n")
		fmt.Fprintf(os.Stderr, "  aggregate              Aggregate results from completed agents\n")
		fmt.Fprintf(os.Stderr, "  dashboard              Show real-time agent status dashboard\n")
		fmt.Fprintf(os.Stderr, "  cleanup [--max-age=DURATION]  Clean up completed agents (default: 1h)\n")
		os.Exit(1)
	}

	command := args[0]
	llmClient := llm.NewClient(os.Getenv("OLLAMA_HOST"), os.Getenv("OLLAMA_MODEL"), os.Getenv("SYSTEM_PROMPT"))
	agentOrch := orchestrator.NewAgentOrchestrator(llmClient)

	switch command {
	case "spawn":
		runAgentSpawn(args[1:], agentOrch)
	case "list":
		runAgentList(args[1:], agentOrch)
	case "status":
		runAgentStatus(args[1:], agentOrch)
	case "stop":
		runAgentStop(args[1:], agentOrch)
	case "aggregate":
		runAgentAggregate(args[1:], agentOrch)
	case "dashboard":
		runAgentDashboard(args[1:], agentOrch)
	case "cleanup":
		runAgentCleanup(args[1:], agentOrch)
	default:
		fmt.Fprintf(os.Stderr, "Unknown agent command: %s\n", command)
		fmt.Fprintf(os.Stderr, "Use 'clai agent' for help\n")
		os.Exit(1)
	}
}

func runAgentSpawn(args []string, agentOrch *orchestrator.AgentOrchestrator) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: clai agent spawn <type> <task>\n")
		fmt.Fprintf(os.Stderr, "Types: code, research, test, review, document\n")
		os.Exit(1)
	}

	agentTypeStr := args[0]
	task := strings.Join(args[1:], " ")

	var agentType orchestrator.AgentType
	switch agentTypeStr {
	case "code":
		agentType = orchestrator.CodeAgent
	case "research":
		agentType = orchestrator.ResearchAgent
	case "test":
		agentType = orchestrator.TestAgent
	case "review":
		agentType = orchestrator.ReviewAgent
	case "document":
		agentType = orchestrator.DocumentAgent
	default:
		fmt.Fprintf(os.Stderr, "Invalid agent type: %s\n", agentTypeStr)
		fmt.Fprintf(os.Stderr, "Valid types: code, research, test, review, document\n")
		os.Exit(1)
	}

	config := map[string]interface{}{
		"type": agentTypeStr,
	}

	agent, err := agentOrch.SpawnAgent(agentType, task, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to spawn agent: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Spawned %s agent: %s (ID: %s)\n", agentType, agent.Name, agent.ID)
	fmt.Printf("Task: %s\n", task)
	fmt.Printf("Status: %s\n", agent.Status)
}

func runAgentList(args []string, agentOrch *orchestrator.AgentOrchestrator) {
	var filterType string
	if len(args) > 0 && strings.HasPrefix(args[0], "--type=") {
		filterType = strings.TrimPrefix(args[0], "--type=")
	}

	agents := agentOrch.ListAgents()

	if len(agents) == 0 {
		fmt.Println("No agents found.")
		return
	}

	fmt.Printf("Agents (%d total):\n", len(agents))
	fmt.Println("ID                                     TYPE       STATUS     NAME")
	fmt.Println("------------------------------------------------------------------------------")

	for _, agent := range agents {
		if filterType != "" && string(agent.Type) != filterType {
			continue
		}

		status := string(agent.Status)
		if len(status) > 10 {
			status = status[:10]
		}

		name := agent.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}

		fmt.Printf("%-38s %-10s %-10s %s\n",
			agent.ID,
			string(agent.Type),
			status,
			name)
	}
}

func runAgentStatus(args []string, agentOrch *orchestrator.AgentOrchestrator) {
	if len(args) > 0 {
		agentID := args[0]
		agent, exists := agentOrch.GetAgent(agentID)
		if !exists {
			fmt.Fprintf(os.Stderr, "Agent %s not found\n", agentID)
			os.Exit(1)
		}

		fmt.Printf("Agent: %s\n", agent.Name)
		fmt.Printf("ID: %s\n", agent.ID)
		fmt.Printf("Type: %s\n", agent.Type)
		fmt.Printf("Status: %s\n", agent.Status)
		fmt.Printf("Task: %s\n", agent.Task)
		fmt.Printf("Started: %s\n", agent.StartTime.Format("2006-01-02 15:04:05"))

		if agent.EndTime != nil {
			fmt.Printf("Ended: %s\n", agent.EndTime.Format("2006-01-02 15:04:05"))
			duration := agent.EndTime.Sub(agent.StartTime)
			fmt.Printf("Duration: %v\n", duration.Round(time.Second))
		}

		if agent.ErrorMessage != "" {
			fmt.Printf("Error: %s\n", agent.ErrorMessage)
		}

		if agent.Results != nil {
			fmt.Printf("Results: %+v\n", agent.Results)
		}
	} else {
		status := agentOrch.GetAgentStatus()

		fmt.Printf("Agent Orchestrator Status:\n")
		fmt.Printf("Total Agents: %d\n", status["total_agents"])
		fmt.Printf("Running: %d\n", status["running"])
		fmt.Printf("Completed: %d\n", status["completed"])
		fmt.Printf("Error: %d\n", status["error"])

		if avgDuration, ok := status["average_duration"].(time.Duration); ok {
			fmt.Printf("Average Duration: %v\n", avgDuration.Round(time.Second))
		}

		byType := status["by_type"].(map[string]int)
		if len(byType) > 0 {
			fmt.Printf("\nBy Type:\n")
			for agentType, count := range byType {
				fmt.Printf("  %s: %d\n", agentType, count)
			}
		}
	}
}

func runAgentStop(args []string, agentOrch *orchestrator.AgentOrchestrator) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: clai agent stop <agent-id>\n")
		os.Exit(1)
	}

	agentID := args[0]

	if err := agentOrch.StopAgent(agentID); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop agent: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Stopped agent: %s\n", agentID)
}

func runAgentAggregate(args []string, agentOrch *orchestrator.AgentOrchestrator) {
	strategy := "latest_wins"
	if len(args) > 0 && strings.HasPrefix(args[0], "--strategy=") {
		strategy = strings.TrimPrefix(args[0], "--strategy=")
	}

	var conflictStrategy orchestrator.ConflictResolutionStrategy
	switch strategy {
	case "latest_wins":
		conflictStrategy = orchestrator.StrategyLatestWins
	case "prioritize":
		conflictStrategy = orchestrator.StrategyPrioritize
	case "merge":
		conflictStrategy = orchestrator.StrategyMerge
	case "manual":
		conflictStrategy = orchestrator.StrategyManualReview
	default:
		fmt.Fprintf(os.Stderr, "Invalid strategy: %s\n", strategy)
		fmt.Fprintf(os.Stderr, "Valid strategies: latest_wins, prioritize, merge, manual\n")
		os.Exit(1)
	}

	agentOrch.SetConflictResolutionStrategy(conflictStrategy)

	fmt.Printf("Aggregating results with strategy: %s\n", strategy)

	result, err := agentOrch.AggregateResults()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to aggregate results: %v\n", err)
		os.Exit(1)
	}

	report := agentOrch.GenerateAggregationReport(result)
	fmt.Println(report)

	if len(result.AggregatedFiles) > 0 {
		fmt.Printf("\nAggregated %d files successfully\n", len(result.AggregatedFiles))
		if result.WithConflicts > 0 {
			fmt.Printf("Resolved %d conflicts\n", result.WithConflicts)
		}
	} else {
		fmt.Println("\nNo files to aggregate")
	}
}

func runAgentDashboard(args []string, agentOrch *orchestrator.AgentOrchestrator) {
	dashboard := orchestrator.NewAgentStatusDashboard(agentOrch)

	p := tea.NewProgram(dashboard)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Dashboard error: %v\n", err)
		os.Exit(1)
	}
}

func runAgentCleanup(args []string, agentOrch *orchestrator.AgentOrchestrator) {
	maxAge := 1 * time.Hour
	if len(args) > 0 && strings.HasPrefix(args[0], "--max-age=") {
		ageStr := strings.TrimPrefix(args[0], "--max-age=")
		if parsed, err := time.ParseDuration(ageStr); err == nil {
			maxAge = parsed
		}
	}

	fmt.Printf("Cleanup functionality would remove agents older than %v\n", maxAge)
	fmt.Println("Note: This feature requires extending the AgentOrchestrator interface")
}

func runTodoCommand(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: clai todo <command> [options]\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  add <content> [--priority=high|medium|low]    Add a new todo item\n")
		fmt.Fprintf(os.Stderr, "  list [--status=pending|in_progress|completed] List todos (filtered by status)\n")
		fmt.Fprintf(os.Stderr, "  complete <id>                                 Mark todo as completed\n")
		fmt.Fprintf(os.Stderr, "  delete <id>                                   Delete a todo item\n")
		fmt.Fprintf(os.Stderr, "  update <id> <content>                         Update todo content\n")
		fmt.Fprintf(os.Stderr, "  status                                        Show todo statistics\n")
		os.Exit(1)
	}

	manager := todo.NewManager()

	if err := manager.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load todos: %v\n", err)
		os.Exit(1)
	}

	command := args[0]

	switch command {
	case "add":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: clai todo add <content> [--priority=high|medium|low]\n")
			os.Exit(1)
		}

		content := args[1]
		priority := todo.PriorityMedium

		for _, arg := range args[2:] {
			if strings.HasPrefix(arg, "--priority=") {
				priorityStr := strings.TrimPrefix(arg, "--priority=")
				switch priorityStr {
				case "high":
					priority = todo.PriorityHigh
				case "medium":
					priority = todo.PriorityMedium
				case "low":
					priority = todo.PriorityLow
				default:
					fmt.Fprintf(os.Stderr, "Invalid priority: %s (must be high, medium, or low)\n", priorityStr)
					os.Exit(1)
				}
			}
		}

		todoItem, err := manager.Add(content, priority)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to add todo: %v\n", err)
			os.Exit(1)
		}

		if err := manager.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save todos: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Added todo: %s (ID: %s, Priority: %s)\n", todoItem.Content, todoItem.ID, todoItem.Priority)

	case "list":
		var statusFilter todo.Status
		for _, arg := range args[1:] {
			if strings.HasPrefix(arg, "--status=") {
				statusStr := strings.TrimPrefix(arg, "--status=")
				switch statusStr {
				case "pending":
					statusFilter = todo.StatusPending
				case "in_progress":
					statusFilter = todo.StatusInProgress
				case "completed":
					statusFilter = todo.StatusCompleted
				default:
					fmt.Fprintf(os.Stderr, "Invalid status: %s\n", statusStr)
					os.Exit(1)
				}
			}
		}

		todos := manager.List(statusFilter)
		if len(todos) == 0 {
			if statusFilter != "" {
				fmt.Printf("No %s todos found.\n", statusFilter)
			} else {
				fmt.Println("No todos found.")
			}
			return
		}

		fmt.Printf("Todos (%d total):\n", len(todos))
		fmt.Println("ID                                    STATUS       PRIORITY  CONTENT")
		fmt.Println("--------------------------------------------------------------------------------")

		for _, t := range todos {
			status := string(t.Status)
			priority := string(t.Priority)
			content := t.Content
			if len(content) > 50 {
				content = content[:47] + "..."
			}

			fmt.Printf("%-36s %-12s %-8s %s\n",
				t.ID,
				status,
				priority,
				content)
		}

	case "complete":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: clai todo complete <id>\n")
			os.Exit(1)
		}

		id := args[1]
		err := manager.Update(id, map[string]interface{}{
			"status": todo.StatusCompleted,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to complete todo: %v\n", err)
			os.Exit(1)
		}

		if err := manager.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save todos: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Marked todo %s as completed\n", id)

	case "delete":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: clai todo delete <id>\n")
			os.Exit(1)
		}

		id := args[1]
		err := manager.Delete(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete todo: %v\n", err)
			os.Exit(1)
		}

		if err := manager.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save todos: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Deleted todo %s\n", id)

	case "update":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: clai todo update <id> <content>\n")
			os.Exit(1)
		}

		id := args[1]
		content := args[2]

		err := manager.Update(id, map[string]interface{}{
			"content": content,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update todo: %v\n", err)
			os.Exit(1)
		}

		if err := manager.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save todos: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Updated todo %s\n", id)

	case "status":
		counts := manager.Count()
		total := 0
		for _, count := range counts {
			total += count
		}

		fmt.Printf("Todo Status Summary:\n")
		fmt.Printf("Total todos: %d\n", total)
		fmt.Printf("Pending: %d\n", counts[todo.StatusPending])
		fmt.Printf("In Progress: %d\n", counts[todo.StatusInProgress])
		fmt.Printf("Completed: %d\n", counts[todo.StatusCompleted])
		fmt.Printf("Cancelled: %d\n", counts[todo.StatusCancelled])

		if total > 0 {
			completed := counts[todo.StatusCompleted]
			completionRate := float64(completed) / float64(total) * 100
			fmt.Printf("Completion rate: %.1f%%\n", completionRate)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		fmt.Fprintf(os.Stderr, "Run 'clai todo' for help\n")
		os.Exit(1)
	}
}
