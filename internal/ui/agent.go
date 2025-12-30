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

func RunAgentCmd(agent *llm.Agent, query string, statusChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		logger.Info("[AGENT-CMD] Running agent with query: %s", query)

		agent.SetStatusCallback(func(iteration int, thought string, executingCode bool, language string, code string) {
			status := AgentStatus{
				Active:        iteration > 0,
				CurrentIter:   iteration,
				Thought:       thought,
				Subtasks:      []string{}, // Simplified - no more subtasks
				ExecutingCode: executingCode,
				CodeLanguage:  language,
			}
			select {
			case statusChan <- AgentStatusMsg{Status: status, Code: code}:
			default:
			}
		})

		result, err := agent.Run(query)
		if err != nil {
			logger.Error("[AGENT-CMD] Agent error: %v", err)
			return AgentResponseMsg{Response: "", Err: err}
		}
		logger.Info("[AGENT-CMD] Agent completed: %s", result)
		return AgentResponseMsg{Response: result, Err: nil}
	}
}
