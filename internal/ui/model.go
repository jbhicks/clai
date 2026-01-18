package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/jbhicks/clai/internal/debug"
	"github.com/jbhicks/clai/internal/llm"
	"github.com/jbhicks/clai/internal/ralph"
)

type Stage int

const (
	StageIdle Stage = iota
	StageThinking
	StageExecuting
	StageVerifying
)

type prdLoadedMsg *ralph.PRD
type prdErrorMsg error
type prdFileChangedMsg struct{}
type healthMsg bool

type logMsg struct {
	storyID string
	content string
	gitHash string
}
type patternMsg string

type Model struct {
	width         int
	height        int
	stage         Stage
	prd           *ralph.PRD
	logs          []string
	debugServer   *debug.Server
	err           error
	activeStoryID string
	viewport      viewport.Model
	ctx           context.Context
	cancel        context.CancelFunc
	cursor        int
	llmHost       string
	llmModel      string
	llmHealth     bool
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadPRDCmd(),
		m.watchPRDCmd(),
		m.checkHealthCmd(),
	)
}

func (m Model) loadPRDCmd() tea.Cmd {
	return func() tea.Msg {
		prd, err := ralph.LoadPRD(".clai/prd.json")
		if err != nil {
			return prdErrorMsg(err)
		}
		return prdLoadedMsg(prd)
	}
}

func (m Model) watchPRDCmd() tea.Cmd {
	return func() tea.Msg {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil // Silently fail watcher for now
		}
		// Note: We'd need to keep this watcher alive.
		// For a simple implementation, we can just spawn a goroutine.
		// But Bubble Tea prefers commands.
		// Actually, we can return a command that waits for a single event.
		err = watcher.Add(".clai/prd.json")
		if err != nil {
			watcher.Close()
			return nil
		}

		select {
		case event, ok := <-watcher.Events:
			watcher.Close()
			if !ok {
				return nil
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				return prdFileChangedMsg{}
			}
		case <-watcher.Errors:
			watcher.Close()
			return nil
		}
		return nil
	}
}

func (m Model) checkHealthCmd() tea.Cmd {
	return func() tea.Msg {
		client := llm.NewClient(m.llmHost, m.llmModel, "")
		err := client.HealthCheck()
		return healthMsg(err == nil)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.debugServer != nil {
		m.debugServer.SetModel(&m)
	}
	switch msg := msg.(type) {
	case healthMsg:
		m.llmHealth = bool(msg)
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Initialize or resize viewport
		missionWidth := m.width / 3
		workWidth := m.width - missionWidth
		occupiedHeight := 4 // Header(2) + Footer(2)
		paddingHeight := 2  // BodyStyle Padding(1, 2) top/bottom
		contentHeight := m.height - occupiedHeight

		if m.viewport.Width == 0 {
			m.viewport = viewport.New(workWidth-4, contentHeight-paddingHeight)
		} else {
			m.viewport.Width = workWidth - 4
			m.viewport.Height = contentHeight - paddingHeight
		}
	case prdLoadedMsg:
		m.prd = msg
		m.err = nil
	case prdErrorMsg:
		m.err = msg
	case prdFileChangedMsg:
		return m, tea.Batch(m.loadPRDCmd(), m.watchPRDCmd())
	case logMsg:
		m.logs = append(m.logs, fmt.Sprintf("[%s] %s", msg.storyID, msg.content))
		m.viewport.SetContent(strings.Join(m.logs, "\n"))
		m.viewport.GotoBottom()
		return m, func() tea.Msg {
			ralph.AppendLog(msg.storyID, msg.content, msg.gitHash)
			return nil
		}
	case patternMsg:
		m.logs = append(m.logs, fmt.Sprintf("PATTERN: %s", string(msg)))
		m.viewport.SetContent(strings.Join(m.logs, "\n"))
		m.viewport.GotoBottom()
		return m, func() tea.Msg {
			ralph.UpdatePatterns(string(msg))
			return nil
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			// Clean up debug server if it's there
			if m.debugServer != nil {
				m.debugServer.Stop()
			}
			return m, tea.Quit
		case "r":
			if m.stage == StageIdle && m.prd != nil {
				// Find next story
				var nextStory *ralph.UserStory
				for i := range m.prd.UserStories {
					if !m.prd.UserStories[i].Passes {
						nextStory = &m.prd.UserStories[i]
						break
					}
				}

				if nextStory == nil {
					m.logs = append(m.logs, "Nothing to do: All stories passed.")
					return m, nil
				}

				m.stage = StageThinking
				m.activeStoryID = nextStory.ID
				m.logs = append(m.logs, fmt.Sprintf("Thinking about: %s...", nextStory.Title))
				m.viewport.SetContent(strings.Join(m.logs, "\n"))
				m.viewport.GotoBottom()

				// Spawn sub-agent worker
				if m.cancel != nil {
					m.cancel()
				}
				m.ctx, m.cancel = context.WithCancel(context.Background())
				return m, m.runSubagentCmd(nextStory.ID)
			}
		case "enter":
			if m.stage == StageIdle && m.prd != nil && m.cursor < len(m.prd.UserStories) {
				story := m.prd.UserStories[m.cursor]
				m.stage = StageThinking
				m.activeStoryID = story.ID
				m.logs = append(m.logs, fmt.Sprintf("Starting selected task: %s...", story.Title))
				m.viewport.SetContent(strings.Join(m.logs, "\n"))
				m.viewport.GotoBottom()

				if m.cancel != nil {
					m.cancel()
				}
				m.ctx, m.cancel = context.WithCancel(context.Background())
				return m, m.runSubagentCmd(story.ID)
			}
		case "s", "esc":
			if m.stage != StageIdle {
				if m.cancel != nil {
					m.cancel()
				}
				m.stage = StageIdle
				m.activeStoryID = ""
				m.logs = append(m.logs, "Loop interrupted.")
				m.viewport.SetContent(strings.Join(m.logs, "\n"))
				m.viewport.GotoBottom()
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.prd != nil && m.cursor < len(m.prd.UserStories)-1 {
				m.cursor++
			}
		case "e":
			if m.prd != nil && m.cursor < len(m.prd.UserStories) {
				story := m.prd.UserStories[m.cursor]
				lineNumber := ralph.FindLineNumber(".clai/prd.json", story.ID)

				editor := os.Getenv("EDITOR")
				if editor == "" {
					editor = "nvim"
				}

				c := exec.Command(editor, fmt.Sprintf("+%d", lineNumber), ".clai/prd.json")
				return m, tea.ExecProcess(c, func(err error) tea.Msg {
					return nil // We rely on FSWatcher to reload
				})
			}
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)

	// Sync with debug server
	if m.debugServer != nil {
		m.debugServer.SetModel(m)
	}

	return m, cmd
}

func (m Model) runSubagentCmd(storyID string) tea.Cmd {
	return func() tea.Msg {
		// This is where real LLM sub-agent logic would go.
		// For now, we simulate concurrent activity.
		// In a real implementation, we would send multiple logMsgs back.
		return logMsg{
			storyID: storyID,
			content: "Sub-agent started work...",
			gitHash: "HEAD",
		}
	}
}

func (m Model) getWidth() int {
	return m.width
}

func (m Model) getHeight() int {
	return m.height
}

func (m Model) getLogs() []string {
	return m.logs
}

func (m Model) getViewportContent() string {
	if m.viewport.Width > 0 && m.viewport.Height > 0 {
		return m.viewport.View()
	}
	return strings.Join(m.logs, "\n")
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return BaseStyle.Width(m.width).Height(m.height).Render("Initializing Ralph Orchestrator...")
	}

	if m.err != nil {
		return BaseStyle.Width(m.width).Height(m.height).Render(fmt.Sprintf("Error: %v\n\nPress 'q' to quit.", m.err))
	}

	// 1. Render Header
	leftHeader := " CLAI MISSION CONTROL | THE RALPH METHOD"
	rightHeader := fmt.Sprintf("HOST: %s | MODEL: %s ", m.llmHost, m.llmModel)
	gap := m.width - lipgloss.Width(leftHeader) - lipgloss.Width(rightHeader) - 2
	if gap < 0 {
		gap = 0
	}
	header := HeaderStyle.Width(m.width).Render(leftHeader + strings.Repeat(" ", gap) + rightHeader)

	// 2. Render Footer
	footer := FooterStyle.Width(m.width).Render(" [Q]uit | [E]dit Story | [R]un Loop | [S]top")

	// 3. Main Content Area Sizing
	occupiedHeight := lipgloss.Height(header) + lipgloss.Height(footer)
	contentHeight := m.height - occupiedHeight
	if contentHeight < 0 {
		contentHeight = 0
	}

	// 4. Create Panes
	missionWidth := m.width / 3
	workWidth := m.width - missionWidth

	// Left Pane: Mission (PRD/Stories)
	missionContent := "MISSION (PRD)\n\n"
	if m.prd == nil {
		missionContent += "Loading PRD..."
	} else {
		missionContent += fmt.Sprintf("Project: %s\nBranch: %s\n\nSTORIES:\n", m.prd.Project, m.prd.BranchName)
		for i, s := range m.prd.UserStories {
			status := "○"
			content := fmt.Sprintf("[%s] %s", s.ID, s.Title)

			isCurrent := s.ID == m.activeStoryID
			isSelected := i == m.cursor

			if s.Passes {
				status = "●"
				content = PassStyle.Render(content)
			}

			prefix := "  "
			if isCurrent {
				status = "▶"
				content = ActiveStoryStyle.Render(content)
				prefix = "> "
			} else if isSelected {
				content = SelectedStoryStyle.Render(content)
				prefix = "• "
			}
			missionContent += fmt.Sprintf("%s%s %s\n", prefix, status, content)
		}
	}

	mission := BodyStyle.
		Width(missionWidth).
		Height(contentHeight).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(TNSelection).
		BorderRight(true).
		Render(missionContent)

	// Right Pane: Briefing Room (Idle) or Work Stream (Active)
	var rightPaneContent string
	if m.stage == StageIdle && m.prd != nil && m.cursor < len(m.prd.UserStories) {
		story := m.prd.UserStories[m.cursor]

		briefing := BriefingHeaderStyle.Render(fmt.Sprintf("BRIEFING: %s", story.ID)) + "\n"
		briefing += BriefingLabelStyle.Render("Title: ") + story.Title + "\n"
		briefing += BriefingLabelStyle.Render("Priority: ") + fmt.Sprintf("%d", story.Priority) + "\n\n"
		briefing += BriefingLabelStyle.Render("Description:") + "\n" + story.Description + "\n\n"

		if len(story.AcceptanceCriteria) > 0 {
			briefing += BriefingLabelStyle.Render("Acceptance Criteria:") + "\n"
			for _, ac := range story.AcceptanceCriteria {
				briefing += fmt.Sprintf("  • %s\n", ac)
			}
			briefing += "\n"
		}

		if story.Notes != "" {
			briefing += BriefingLabelStyle.Render("Notes:") + "\n" + story.Notes + "\n"
		}

		rightPaneContent = briefing
	} else {
		m.viewport.Width = workWidth - 4
		m.viewport.Height = contentHeight - 2
		m.viewport.Style = lipgloss.NewStyle().Background(TNBackground)
		m.viewport.SetContent(strings.Join(m.logs, "\n"))
		rightPaneContent = m.viewport.View()
	}

	work := BodyStyle.
		Width(workWidth).
		Height(contentHeight).
		Render(rightPaneContent)

	// 5. Join panes horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, mission, work)

	// 6. Join everything vertically
	finalView := lipgloss.JoinVertical(lipgloss.Left,
		header,
		content,
		footer,
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Left, lipgloss.Top,
		finalView,
		lipgloss.WithWhitespaceBackground(TNBackground),
	)
}

func Run() error {
	// Manual .env loading if it exists
	envVars := make(map[string]string)
	if data, err := os.ReadFile(".env"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				envVars[k] = v
			}
		}
	}

	m := Model{
		llmHost:  envVars["OLLAMA_HOST"],
		llmModel: envVars["OLLAMA_MODEL"],
	}

	if m.llmHost == "" {
		m.llmHost = os.Getenv("OLLAMA_HOST")
	}
	if m.llmModel == "" {
		m.llmModel = os.Getenv("OLLAMA_MODEL")
	}
	if m.llmHost == "" {
		m.llmHost = "http://localhost:11434"
	}
	if m.llmModel == "" {
		m.llmModel = "clai"
	}

	p := tea.NewProgram(m, tea.WithAltScreen())

	m.debugServer = debug.NewServer(p)
	m.debugServer.SetModel(m) // Set the initial model
	if err := m.debugServer.Start(); err != nil {
		return fmt.Errorf("failed to start debug server: %w", err)
	}

	_, err := p.Run()
	return err
}
