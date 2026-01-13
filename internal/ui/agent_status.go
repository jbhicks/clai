package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/brittonhayes/glitter/glitter"
	"github.com/charmbracelet/lipgloss"
)

type AgentStatus struct {
	Active        bool
	CurrentIter   int
	Thought       string
	Subtasks      []string
	ExecutingCode bool
	CodeLanguage  string
}

type AgentAction struct {
	Description string
	Status      string
	Iteration   int
}

type CodeExecution struct {
	Language string
	Code     string
	Status   string
	Output   string
}

type CompletedTask struct {
	Thought     string
	Actions     []AgentAction
	CompletedAt time.Time
	ActionCount int
}

type AgentStatusView struct {
	Status      AgentStatus
	Actions     []AgentAction
	CurrentCode *CodeExecution
	CurrentTask *CompletedTask
	History     []CompletedTask
	Width       int
	Height      int
	Theme       *glitter.UI
	lastThought string
}

func NewAgentStatusView(theme *glitter.UI) *AgentStatusView {
	return &AgentStatusView{
		Status:  AgentStatus{},
		Actions: []AgentAction{},
		History: []CompletedTask{},
		Theme:   theme,
	}
}

func (v *AgentStatusView) Update(status AgentStatus) {
	if status.Thought != "" && status.Thought != v.lastThought {
		v.startNewTask(status.Thought)
		v.lastThought = status.Thought
	}
	v.Status = status
}

func (v *AgentStatusView) startNewTask(thought string) {
	if v.CurrentTask != nil && len(v.CurrentTask.Actions) > 0 {
		v.History = append(v.History, *v.CurrentTask)
		if len(v.History) > 10 {
			v.History = v.History[1:]
		}
	}

	v.CurrentTask = &CompletedTask{
		Thought: thought,
		Actions: make([]AgentAction, 0),
	}
	v.Actions = make([]AgentAction, 0)
	v.CurrentCode = nil
}

func (v *AgentStatusView) CompleteCurrentTask() {
	if v.CurrentTask != nil {
		v.CurrentTask.CompletedAt = time.Now()
		v.CurrentTask.ActionCount = len(v.CurrentTask.Actions)
		v.History = append(v.History, *v.CurrentTask)
		if len(v.History) > 10 {
			v.History = v.History[1:]
		}
		v.CurrentTask = nil
		v.Actions = make([]AgentAction, 0)
		v.CurrentCode = nil
	}
}

func (v *AgentStatusView) SetExecutingCode(language, code string) {
	v.CurrentCode = &CodeExecution{
		Language: language,
		Code:     code,
		Status:   "running",
	}
}

func (v *AgentStatusView) CompleteCodeExecution(output string) {
	if v.CurrentCode != nil {
		v.CurrentCode.Status = "completed"
		v.CurrentCode.Output = output
	}
}

func (v *AgentStatusView) FailCodeExecution(output string) {
	if v.CurrentCode != nil {
		v.CurrentCode.Status = "failed"
		v.CurrentCode.Output = output
	}
}

func (v *AgentStatusView) AddAction(description string, iteration int) {
	action := AgentAction{
		Description: description,
		Status:      "in_progress",
		Iteration:   iteration,
	}
	v.Actions = append(v.Actions, action)

	if v.CurrentTask != nil {
		v.CurrentTask.Actions = append(v.CurrentTask.Actions, action)
	}
}

func (v *AgentStatusView) CompleteCurrentAction() {
	if len(v.Actions) > 0 {
		v.Actions[len(v.Actions)-1].Status = "completed"
		if v.CurrentTask != nil && len(v.CurrentTask.Actions) > 0 {
			v.CurrentTask.Actions[len(v.CurrentTask.Actions)-1].Status = "completed"
		}
	}
}

func (v *AgentStatusView) FailCurrentAction() {
	if len(v.Actions) > 0 {
		v.Actions[len(v.Actions)-1].Status = "failed"
		if v.CurrentTask != nil && len(v.CurrentTask.Actions) > 0 {
			v.CurrentTask.Actions[len(v.CurrentTask.Actions)-1].Status = "failed"
		}
	}
}

func (v *AgentStatusView) View() string {
	themeStyles := GetThemeStyles(v.Theme)
	bg := lipgloss.Color(v.Theme.Theme.Primary.Background)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(v.Theme.Theme.Bright.Yellow)).
		Background(bg)

	var content strings.Builder

	if !v.Status.Active {
		header := themeStyles.ToolBadge.Render("🤖 Agent Idle")
		content.WriteString(header)
		content.WriteString("\n\n")
		content.WriteString(themeStyles.LogDim.Render("Waiting for task..."))
	} else {
		header := fmt.Sprintf("🤖 Agent Active - Iteration %d", v.Status.CurrentIter)
		content.WriteString(themeStyles.ToolBadge.Render(header))
		content.WriteString("\n")

		if v.Status.Thought != "" {
			maxLen := v.Width - 10
			thought := v.Status.Thought
			if maxLen > 3 && len(thought) > maxLen {
				thought = thought[:maxLen-3] + "..."
			}
			content.WriteString(labelStyle.Render("Task: "))
			content.WriteString(themeStyles.LogValue.Render(thought))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(labelStyle.Render("Current Actions:"))
	content.WriteString("\n")

	linesUsed := strings.Count(content.String(), "\n")
	remainingHeight := v.Height - linesUsed - 2

	var maxActionLines int
	if v.CurrentCode != nil {
		maxActionLines = max(remainingHeight/2, 2)
	} else {
		maxActionLines = max(remainingHeight-4, 3)
	}

	if len(v.Actions) == 0 {
		content.WriteString(themeStyles.LogDim.Render("  No actions yet"))
		content.WriteString("\n")
	} else {
		displayStart := 0
		if len(v.Actions) > maxActionLines {
			displayStart = len(v.Actions) - maxActionLines
		}

		for i := displayStart; i < len(v.Actions); i++ {
			action := v.Actions[i]
			var icon string
			var actionStyle lipgloss.Style

			switch action.Status {
			case "completed":
				icon = "✓"
				actionStyle = themeStyles.ActionCompleted
			case "failed":
				icon = "✗"
				actionStyle = themeStyles.ActionFailed
			case "in_progress":
				icon = "⟳"
				actionStyle = themeStyles.ActionInProgress
			default:
				icon = "•"
				actionStyle = themeStyles.LogDim
			}

			maxLen := v.Width - 8
			desc := action.Description
			if len(desc) > maxLen {
				desc = desc[:maxLen-3] + "..."
			}

			line := fmt.Sprintf("  %s %s", icon, actionStyle.Render(desc))
			content.WriteString(line)
			content.WriteString("\n")
		}
	}

	if v.CurrentCode != nil {
		content.WriteString("\n")
		linesUsedNow := strings.Count(content.String(), "\n")
		codeBlockHeight := v.Height - linesUsedNow - 2
		codeBlock := v.renderCodeBlock(codeBlockHeight)
		content.WriteString(codeBlock)
	}

	if len(v.History) > 0 && v.CurrentCode == nil {
		content.WriteString("\n")
		divider := strings.Repeat("━", v.Width-4)
		content.WriteString(themeStyles.LogDim.Render(divider))
		content.WriteString("\n")
		content.WriteString(labelStyle.Render("Recent Completions:"))
		content.WriteString("\n")

		displayCount := 2
		if len(v.History) < displayCount {
			displayCount = len(v.History)
		}

		for i := len(v.History) - 1; i >= len(v.History)-displayCount; i-- {
			task := v.History[i]
			elapsed := formatElapsed(time.Since(task.CompletedAt))

			maxLen := v.Width - 20
			thought := task.Thought
			if len(thought) > maxLen {
				thought = thought[:maxLen-3] + "..."
			}

			line := fmt.Sprintf("▸ %s (%d) - %s",
				themeStyles.LogDim.Render(thought),
				task.ActionCount,
				themeStyles.LogDim.Render(elapsed))
			content.WriteString(line)
			content.WriteString("\n")
		}
	}

	wrapperStyle := lipgloss.NewStyle().
		Width(v.Width).
		Height(v.Height).
		Background(bg).
		Padding(0, 1)

	return wrapperStyle.Render(content.String())
}

func (v *AgentStatusView) renderCodeBlock(maxHeight int) string {
	if v.CurrentCode == nil {
		return ""
	}

	bg := lipgloss.Color(v.Theme.Theme.Primary.Background)

	var borderColor lipgloss.Color
	var statusIcon string
	switch v.CurrentCode.Status {
	case "running":
		borderColor = lipgloss.Color(v.Theme.Theme.Bright.Yellow)
		statusIcon = "⏳"
	case "completed":
		borderColor = lipgloss.Color(v.Theme.Theme.Bright.Green)
		statusIcon = "✓"
	case "failed":
		borderColor = lipgloss.Color(v.Theme.Theme.Bright.Red)
		statusIcon = "✗"
	default:
		borderColor = lipgloss.Color(v.Theme.Theme.Primary.DimForeground)
		statusIcon = "○"
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(borderColor).
		Background(bg).
		Bold(true)
	headerText := headerStyle.Render(fmt.Sprintf("%s Executing %s", statusIcon, v.CurrentCode.Language))

	// Wrap header to full width with matching background
	headerWrapper := lipgloss.NewStyle().
		Width(v.Width - 4).
		Background(bg)
	header := headerWrapper.Render(headerText)

	codeLines := strings.Split(v.CurrentCode.Code, "\n")

	maxCodeLines := max(maxHeight-4, 3)
	truncated := false
	if len(codeLines) > maxCodeLines {
		codeLines = codeLines[:maxCodeLines]
		truncated = true
	}

	codeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(v.Theme.Theme.Primary.Foreground)).
		Background(bg)

	var codeContent strings.Builder
	for _, line := range codeLines {
		if len(line) > v.Width-10 {
			line = line[:v.Width-13] + "..."
		}
		codeContent.WriteString(codeStyle.Render(line))
		codeContent.WriteString("\n")
	}

	if truncated {
		codeContent.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(v.Theme.Theme.Primary.DimForeground)).
			Background(bg).
			Render("... (truncated)"))
		codeContent.WriteString("\n")
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Background(bg).
		Padding(0, 1).
		Width(v.Width - 4)

	parts := []string{header, boxStyle.Render(codeContent.String())}

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func formatElapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(d.Hours()))
}
