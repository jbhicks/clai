package ui

import (
	"clai/internal/llm"
	"clai/internal/logger"

	tea "github.com/charmbracelet/bubbletea"
)

type AgentResponseMsg struct {
	Response string
	Err      error
}

func RunAgentCmd(agent *llm.Agent, query string) tea.Cmd {
	return func() tea.Msg {
		logger.Info("[AGENT-CMD] Running agent with query: %s", query)
		result, err := agent.Run(query)
		if err != nil {
			logger.Error("[AGENT-CMD] Agent error: %v", err)
			return AgentResponseMsg{Response: "", Err: err}
		}
		logger.Info("[AGENT-CMD] Agent completed: %s", result)
		return AgentResponseMsg{Response: result, Err: nil}
	}
}
