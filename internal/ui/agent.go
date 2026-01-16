package ui

import (
	"clai/internal/llm"
	"clai/internal/logger"
	"clai/internal/tools"

	tea "github.com/charmbracelet/bubbletea"
)

type AgentResponseMsg struct {
	Response string
	Err      error
}

type StreamChunkMsg struct {
	Chunk     string
	ToolCall  *tools.ToolCall
	CodeBlock *llm.CodeBlock
}

func RunAgentCmd(agent *llm.Agent, query string, statusChan chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		logger.Info("[AGENT-CMD] Starting agent.Run")
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

func RunStreamingAgentCmd(agent *llm.Agent, query string, statusChan chan tea.Msg) tea.Cmd {
	logger.Info("[AGENT-STREAM-CMD] RunStreamingAgentCmd called, creating command closure")
	logger.Info("[AGENT-STREAM-CMD] Agent nil check: %v, statusChan nil check: %v", agent == nil, statusChan == nil)
	return func() tea.Msg {
		logger.Info("[AGENT-STREAM-CMD] INSIDE GOROUTINE - Command execution started")
		logger.Info("[AGENT-STREAM-CMD] Starting streaming agent.Run")

		callback := func(chunk string, toolCall *tools.ToolCall, codeBlock *llm.CodeBlock) {
			logger.Info("[AGENT-STREAM-CALLBACK] Received - chunk: %q, toolCall: %v, codeBlock: %v", chunk, toolCall != nil, codeBlock != nil)
			select {
			case statusChan <- StreamUpdateMsg(chunk):
				logger.Info("[AGENT-STREAM-CALLBACK] Sent StreamUpdateMsg to statusChan")
			default:
				logger.Warn("[AGENT-STREAM-CMD] Status channel full, dropping stream chunk")
			}
		}

		result, err := agent.RunWithStreaming(query, callback)
		if err != nil {
			logger.Error("[AGENT-STREAM-CMD] Streaming agent error: %v", err)
			return AgentResponseMsg{Response: "", Err: err}
		}
		logger.Info("[AGENT-STREAM-CMD] Streaming agent completed: %s", result)
		return AgentResponseMsg{Response: result, Err: nil}
	}
}
