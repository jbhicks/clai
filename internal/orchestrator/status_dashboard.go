package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AgentStatusModel struct {
	agentOrch  *AgentOrchestrator
	table      table.Model
	spinner    spinner.Model
	loading    bool
	lastUpdate time.Time
}

type statusUpdateMsg struct{}

func NewAgentStatusDashboard(agentOrch *AgentOrchestrator) *AgentStatusModel {
	columns := []table.Column{
		{Title: "ID", Width: 12},
		{Title: "Type", Width: 10},
		{Title: "Status", Width: 12},
		{Title: "Name", Width: 20},
		{Title: "Task", Width: 30},
		{Title: "Duration", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	return &AgentStatusModel{
		agentOrch:  agentOrch,
		table:      t,
		spinner:    s,
		lastUpdate: time.Now(),
	}
}

func (m *AgentStatusModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.updateStatus(),
	)
}

func (m *AgentStatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			return m, m.updateStatus()
		}
	case statusUpdateMsg:
		m.loading = false
		m.lastUpdate = time.Now()
		m.refreshTable()
		return m, m.updateStatus()
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *AgentStatusModel) View() string {
	var b strings.Builder

	b.WriteString("🤖 Agent Orchestrator Dashboard\n\n")

	if m.loading {
		b.WriteString(fmt.Sprintf("%s Refreshing status...\n\n", m.spinner.View()))
	} else {
		b.WriteString(fmt.Sprintf("✅ Last updated: %s\n\n", m.lastUpdate.Format("15:04:05")))
	}

	b.WriteString(m.table.View())
	b.WriteString("\n\n")

	status := m.agentOrch.GetAgentStatus()
	b.WriteString(fmt.Sprintf("📊 Summary: %d total, %d running, %d completed, %d failed\n",
		status["total_agents"], status["running"], status["completed"], status["error"]))

	b.WriteString("\nControls:\n")
	b.WriteString("  r - Refresh status\n")
	b.WriteString("  q - Quit\n")

	return b.String()
}

func (m *AgentStatusModel) updateStatus() tea.Cmd {
	return tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
		return statusUpdateMsg{}
	})
}

func (m *AgentStatusModel) refreshTable() {
	agents := m.agentOrch.ListAgents()

	rows := make([]table.Row, len(agents))
	for i, agent := range agents {
		duration := ""
		if agent.EndTime != nil && agent.StartTime.Before(*agent.EndTime) {
			d := agent.EndTime.Sub(agent.StartTime)
			duration = fmt.Sprintf("%.1fs", d.Seconds())
		} else if agent.Status == AgentRunning {
			d := time.Since(agent.StartTime)
			duration = fmt.Sprintf("%.1fs", d.Seconds())
		}

		status := string(agent.Status)
		if len(status) > 12 {
			status = status[:9] + "..."
		}

		task := agent.Task
		if len(task) > 30 {
			task = task[:27] + "..."
		}

		name := agent.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}

		rows[i] = table.Row{
			agent.ID[:12],
			string(agent.Type),
			status,
			name,
			task,
			duration,
		}
	}

	m.table.SetRows(rows)
}
