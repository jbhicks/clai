package orchestrator

import (
	"clai/internal/logger"
	"fmt"
	"sync"
	"time"
)

type AgentHandler interface {
	CanHandle(agentType AgentType) bool
	SpawnAgent(task string, config map[string]interface{}) (*Agent, error)
	GetAgentStatus(agentID string) (AgentStatus, error)
	StopAgent(agentID string) error
	ListAgents() []*Agent
}

type RalphAgentHandler struct {
	ralphLoopManager *RalphLoopManager
}

func NewRalphAgentHandler(baseDir string) *RalphAgentHandler {
	return &RalphAgentHandler{
		ralphLoopManager: NewRalphLoopManager(baseDir),
	}
}

func (rah *RalphAgentHandler) CanHandle(agentType AgentType) bool {
	switch agentType {
	case CodeAgent, ResearchAgent, TestAgent, ReviewAgent, DocumentAgent:
		return true
	default:
		return false
	}
}

func (rah *RalphAgentHandler) SpawnAgent(task string, config map[string]interface{}) (*Agent, error) {
	agentType := CodeAgent
	if configType, ok := config["type"].(string); ok {
		agentType = AgentType(configType)
	}

	return rah.ralphLoopManager.SpawnAgent(agentType, task, config)
}

func (rah *RalphAgentHandler) GetAgentStatus(agentID string) (AgentStatus, error) {
	if agent, exists := rah.ralphLoopManager.GetAgent(agentID); exists {
		return agent.Status, nil
	}
	return "", fmt.Errorf("agent %s not found", agentID)
}

func (rah *RalphAgentHandler) StopAgent(agentID string) error {
	return rah.ralphLoopManager.StopAgent(agentID)
}

func (rah *RalphAgentHandler) ListAgents() []*Agent {
	return rah.ralphLoopManager.ListAgents()
}

type AgentManager struct {
	handlers []AgentHandler
	agents   map[string]*Agent
	agentsMu sync.RWMutex
	baseDir  string
}

func NewAgentManager(baseDir string) *AgentManager {
	am := &AgentManager{
		handlers: make([]AgentHandler, 0),
		agents:   make(map[string]*Agent),
		baseDir:  baseDir,
	}

	am.registerHandler(NewRalphAgentHandler(baseDir))

	return am
}

func (am *AgentManager) registerHandler(handler AgentHandler) {
	am.handlers = append(am.handlers, handler)
}

func (am *AgentManager) SpawnAgent(agentType AgentType, task string, config map[string]interface{}) (*Agent, error) {
	for _, handler := range am.handlers {
		if handler.CanHandle(agentType) {
			agent, err := handler.SpawnAgent(task, config)
			if err != nil {
				return nil, err
			}

			am.agentsMu.Lock()
			am.agents[agent.ID] = agent
			am.agentsMu.Unlock()

			logger.Info("Agent manager spawned %s agent: %s", agentType, agent.Name)
			return agent, nil
		}
	}

	return nil, fmt.Errorf("no handler available for agent type: %s", agentType)
}

func (am *AgentManager) GetAgent(agentID string) (*Agent, bool) {
	am.agentsMu.RLock()
	defer am.agentsMu.RUnlock()
	agent, exists := am.agents[agentID]
	return agent, exists
}

func (am *AgentManager) ListAgents() []*Agent {
	am.agentsMu.RLock()
	defer am.agentsMu.RUnlock()

	agents := make([]*Agent, 0, len(am.agents))
	for _, agent := range am.agents {
		agents = append(agents, agent)
	}
	return agents
}

func (am *AgentManager) ListAgentsByType(agentType AgentType) []*Agent {
	allAgents := am.ListAgents()
	filtered := make([]*Agent, 0)

	for _, agent := range allAgents {
		if agent.Type == agentType {
			filtered = append(filtered, agent)
		}
	}
	return filtered
}

func (am *AgentManager) StopAgent(agentID string) error {
	am.agentsMu.Lock()
	defer am.agentsMu.Unlock()

	agent, exists := am.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	for _, handler := range am.handlers {
		if handler.CanHandle(agent.Type) {
			if err := handler.StopAgent(agentID); err != nil {
				return err
			}

			agent.Status = AgentError
			now := time.Now()
			agent.EndTime = &now
			agent.ErrorMessage = "Agent stopped by user"

			logger.Info("Agent manager stopped agent: %s", agent.Name)
			return nil
		}
	}

	return fmt.Errorf("no handler available for agent type: %s", agent.Type)
}

func (am *AgentManager) GetAgentStatus(agentID string) (AgentStatus, error) {
	for _, handler := range am.handlers {
		if status, err := handler.GetAgentStatus(agentID); err == nil {
			return status, nil
		}
	}
	return "", fmt.Errorf("agent %s not found", agentID)
}

func (am *AgentManager) CleanupCompletedAgents(maxAge time.Duration) int {
	am.agentsMu.Lock()
	defer am.agentsMu.Unlock()

	cleaned := 0
	now := time.Now()

	for id, agent := range am.agents {
		if agent.Status == AgentCompleted || agent.Status == AgentError {
			if agent.EndTime != nil && now.Sub(*agent.EndTime) > maxAge {
				delete(am.agents, id)
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		logger.Info("Agent manager cleaned up %d completed agents", cleaned)
	}

	return cleaned
}

func (am *AgentManager) GetStats() map[string]interface{} {
	agents := am.ListAgents()

	stats := map[string]interface{}{
		"total_agents":     len(agents),
		"running":          0,
		"completed":        0,
		"error":            0,
		"by_type":          make(map[string]int),
		"average_duration": 0,
	}

	totalDuration := time.Duration(0)
	durationCount := 0

	for _, agent := range agents {
		switch agent.Status {
		case AgentRunning:
			stats["running"] = stats["running"].(int) + 1
		case AgentCompleted:
			stats["completed"] = stats["completed"].(int) + 1
		case AgentError:
			stats["error"] = stats["error"].(int) + 1
		}

		byType := stats["by_type"].(map[string]int)
		byType[string(agent.Type)]++

		if agent.EndTime != nil {
			duration := agent.EndTime.Sub(agent.StartTime)
			totalDuration += duration
			durationCount++
		}
	}

	if durationCount > 0 {
		stats["average_duration"] = totalDuration / time.Duration(durationCount)
	}

	return stats
}
