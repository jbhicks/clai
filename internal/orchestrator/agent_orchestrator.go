package orchestrator

import (
	"clai/internal/llm"
	"clai/internal/logger"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type AgentType string

const (
	CodeAgent     AgentType = "code"
	ResearchAgent AgentType = "research"
	TestAgent     AgentType = "test"
	ReviewAgent   AgentType = "review"
	DocumentAgent AgentType = "document"
)

type AgentStatus string

const (
	AgentIdle      AgentStatus = "idle"
	AgentStarting  AgentStatus = "starting"
	AgentRunning   AgentStatus = "running"
	AgentStopping  AgentStatus = "stopping"
	AgentError     AgentStatus = "error"
	AgentCompleted AgentStatus = "completed"
)

type Agent struct {
	ID           string                 `json:"id"`
	Type         AgentType              `json:"type"`
	Name         string                 `json:"name"`
	Task         string                 `json:"task"`
	Status       AgentStatus            `json:"status"`
	PID          int                    `json:"pid,omitempty"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      *time.Time             `json:"end_time,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Results      map[string]interface{} `json:"results,omitempty"`
}

type RalphLoopState struct {
	LoopID           string      `json:"loop_id"`
	ProjectName      string      `json:"project_name"`
	BranchName       string      `json:"branch_name"`
	CurrentIteration int         `json:"current_iteration"`
	MaxIterations    int         `json:"max_iterations"`
	CurrentTask      *RalphTask  `json:"current_task"`
	CompletedTasks   []RalphTask `json:"completed_tasks"`
	PendingTasks     []RalphTask `json:"pending_tasks"`
	OverallProgress  float64     `json:"overall_progress"`
	Status           string      `json:"status"`
	StartTime        time.Time   `json:"start_time"`
	LastUpdate       time.Time   `json:"last_update"`
	ErrorMessage     string      `json:"error_message,omitempty"`
	Learnings        []string    `json:"learnings"`
}

type RalphTask struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    string     `json:"priority"`
	Completed   bool       `json:"completed"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

type RalphLoopManager struct {
	agents           map[string]*Agent
	agentsMu         sync.RWMutex
	baseDir          string
	ohMyOpenCodePath string
	activeLoops      map[string]*RalphLoopState
	loopsMu          sync.RWMutex
}

func NewRalphLoopManager(baseDir string) *RalphLoopManager {
	return &RalphLoopManager{
		agents:           make(map[string]*Agent),
		baseDir:          baseDir,
		ohMyOpenCodePath: "bunx",
		activeLoops:      make(map[string]*RalphLoopState),
	}
}

func (rlm *RalphLoopManager) SpawnAgent(agentType AgentType, task string, config map[string]interface{}) (*Agent, error) {
	agentID := uuid.New().String()

	agent := &Agent{
		ID:        agentID,
		Type:      agentType,
		Name:      fmt.Sprintf("%s-agent-%s", agentType, agentID[:8]),
		Task:      task,
		Status:    AgentStarting,
		StartTime: time.Now(),
		Config:    config,
	}

	rlm.agentsMu.Lock()
	rlm.agents[agentID] = agent
	rlm.agentsMu.Unlock()

	go rlm.runAgent(agent)

	logger.Info("Spawned agent %s for task: %s", agent.Name, task)
	return agent, nil
}

func (rlm *RalphLoopManager) runAgent(agent *Agent) {
	defer func() {
		if r := recover(); r != nil {
			agent.Status = AgentError
			agent.ErrorMessage = fmt.Sprintf("Agent panicked: %v", r)
			logger.Error("Agent %s panicked: %v", agent.Name, r)
		}
	}()

	agentDir := filepath.Join(rlm.baseDir, "tmp", "agents", agent.ID)
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		agent.Status = AgentError
		agent.ErrorMessage = fmt.Sprintf("Failed to create agent directory: %v", err)
		return
	}

	prdPath := filepath.Join(agentDir, "prd.json")
	if err := rlm.createAgentPRD(agent, prdPath); err != nil {
		agent.Status = AgentError
		agent.ErrorMessage = fmt.Sprintf("Failed to create PRD: %v", err)
		return
	}

	oldDir, err := os.Getwd()
	if err != nil {
		agent.Status = AgentError
		agent.ErrorMessage = fmt.Sprintf("Failed to get current directory: %v", err)
		return
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(agentDir); err != nil {
		agent.Status = AgentError
		agent.ErrorMessage = fmt.Sprintf("Failed to change to agent directory: %v", err)
		return
	}

	agent.Status = AgentRunning
	logger.Info("Starting Ralph loop for agent %s", agent.Name)

	cmd := exec.Command(rlm.ohMyOpenCodePath, "oh-my-opencode", "run", fmt.Sprintf("/ralph-loop %s", agent.Task))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		agent.Status = AgentError
		agent.ErrorMessage = fmt.Sprintf("Failed to start Ralph loop: %v", err)
		return
	}

	agent.PID = cmd.Process.Pid

	if err := cmd.Wait(); err != nil {
		agent.Status = AgentError
		agent.ErrorMessage = fmt.Sprintf("Ralph loop exited with error: %v", err)
		logger.Error("Agent %s Ralph loop failed: %v", agent.Name, err)
	} else {
		agent.Status = AgentCompleted
		logger.Info("Agent %s completed successfully", agent.Name)
	}

	now := time.Now()
	agent.EndTime = &now

	rlm.collectAgentResults(agent, agentDir)
}

func (rlm *RalphLoopManager) createAgentPRD(agent *Agent, prdPath string) error {
	prd := map[string]interface{}{
		"project":     fmt.Sprintf("%s-%s", agent.Type, agent.ID[:8]),
		"branchName":  fmt.Sprintf("agent-%s-%s", agent.Type, agent.ID[:8]),
		"description": fmt.Sprintf("%s agent task: %s", strings.Title(string(agent.Type)), agent.Task),
		"userStories": []map[string]interface{}{
			{
				"id":          fmt.Sprintf("%s-001", agent.Type),
				"title":       agent.Task,
				"description": fmt.Sprintf("Execute %s task: %s", agent.Type, agent.Task),
				"acceptanceCriteria": []string{
					fmt.Sprintf("Complete the %s task successfully", agent.Type),
					"Verify results meet requirements",
					"Clean up any temporary resources",
				},
				"priority": "high",
				"passes":   false,
			},
		},
	}

	data, err := json.MarshalIndent(prd, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(prdPath, data, 0644)
}

func (rlm *RalphLoopManager) collectAgentResults(agent *Agent, agentDir string) {
	results := make(map[string]interface{})

	progressPath := filepath.Join(agentDir, "progress.txt")
	if data, err := os.ReadFile(progressPath); err == nil {
		results["progress"] = string(data)
	}

	files, err := os.ReadDir(agentDir)
	if err == nil {
		var fileList []string
		for _, file := range files {
			fileList = append(fileList, file.Name())
		}
		results["generated_files"] = fileList
	}

	agent.Results = results
}

func (rlm *RalphLoopManager) GetAgent(agentID string) (*Agent, bool) {
	rlm.agentsMu.RLock()
	defer rlm.agentsMu.RUnlock()
	agent, exists := rlm.agents[agentID]
	return agent, exists
}

func (rlm *RalphLoopManager) ListAgents() []*Agent {
	rlm.agentsMu.RLock()
	defer rlm.agentsMu.RUnlock()

	agents := make([]*Agent, 0, len(rlm.agents))
	for _, agent := range rlm.agents {
		agents = append(agents, agent)
	}
	return agents
}

func (rlm *RalphLoopManager) StopAgent(agentID string) error {
	rlm.agentsMu.Lock()
	defer rlm.agentsMu.Unlock()

	agent, exists := rlm.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	if agent.Status != AgentRunning {
		return fmt.Errorf("agent %s is not running", agentID)
	}

	agent.Status = AgentStopping

	if agent.PID > 0 {
		process, err := os.FindProcess(agent.PID)
		if err == nil {
			if err := process.Kill(); err != nil {
				logger.Warn("Failed to kill process %d: %v", agent.PID, err)
			}
		}
	}

	now := time.Now()
	agent.EndTime = &now
	agent.Status = AgentError
	agent.ErrorMessage = "Agent stopped by user"

	return nil
}

// Ralph Loop State Management Methods

func (rlm *RalphLoopManager) StartRalphLoop(projectName, branchName string, maxIterations int) *RalphLoopState {
	rlm.loopsMu.Lock()
	defer rlm.loopsMu.Unlock()

	loopID := fmt.Sprintf("ralph-%s-%d", branchName, time.Now().Unix())
	loopState := &RalphLoopState{
		LoopID:           loopID,
		ProjectName:      projectName,
		BranchName:       branchName,
		CurrentIteration: 0,
		MaxIterations:    maxIterations,
		OverallProgress:  0.0,
		Status:           "initializing",
		StartTime:        time.Now(),
		LastUpdate:       time.Now(),
		Learnings:        make([]string, 0),
	}

	rlm.activeLoops[loopID] = loopState
	logger.Info("Started Ralph loop %s for project %s", loopID, projectName)
	return loopState
}

func (rlm *RalphLoopManager) UpdateRalphLoop(loopID string, currentIteration int, currentTask *RalphTask, completedTasks []RalphTask, pendingTasks []RalphTask, learnings []string) {
	rlm.loopsMu.Lock()
	defer rlm.loopsMu.Unlock()

	loopState, exists := rlm.activeLoops[loopID]
	if !exists {
		logger.Warn("Attempted to update non-existent Ralph loop: %s", loopID)
		return
	}

	loopState.CurrentIteration = currentIteration
	loopState.CurrentTask = currentTask
	loopState.CompletedTasks = completedTasks
	loopState.PendingTasks = pendingTasks

	// Calculate overall progress
	totalTasks := len(completedTasks) + len(pendingTasks)
	if totalTasks > 0 {
		loopState.OverallProgress = float64(len(completedTasks)) / float64(totalTasks) * 100.0
	}

	loopState.Learnings = append(loopState.Learnings, learnings...)
	loopState.LastUpdate = time.Now()

	logger.Info("Updated Ralph loop %s: iteration %d, progress %.1f%%", loopID, currentIteration, loopState.OverallProgress)
}

func (rlm *RalphLoopManager) CompleteRalphLoop(loopID string) {
	rlm.loopsMu.Lock()
	defer rlm.loopsMu.Unlock()

	loopState, exists := rlm.activeLoops[loopID]
	if !exists {
		logger.Warn("Attempted to complete non-existent Ralph loop: %s", loopID)
		return
	}

	loopState.Status = "completed"
	loopState.OverallProgress = 100.0
	loopState.LastUpdate = time.Now()

	logger.Info("Completed Ralph loop %s", loopID)
}

func (rlm *RalphLoopManager) FailRalphLoop(loopID, errorMessage string) {
	rlm.loopsMu.Lock()
	defer rlm.loopsMu.Unlock()

	loopState, exists := rlm.activeLoops[loopID]
	if !exists {
		logger.Warn("Attempted to fail non-existent Ralph loop: %s", loopID)
		return
	}

	loopState.Status = "failed"
	loopState.ErrorMessage = errorMessage
	loopState.LastUpdate = time.Now()

	logger.Error("Ralph loop %s failed: %s", loopID, errorMessage)
}

func (rlm *RalphLoopManager) GetRalphLoop(loopID string) (*RalphLoopState, bool) {
	rlm.loopsMu.RLock()
	defer rlm.loopsMu.RUnlock()
	loopState, exists := rlm.activeLoops[loopID]
	return loopState, exists
}

func (rlm *RalphLoopManager) ListRalphLoops() []*RalphLoopState {
	rlm.loopsMu.RLock()
	defer rlm.loopsMu.RUnlock()

	loops := make([]*RalphLoopState, 0, len(rlm.activeLoops))
	for _, loopState := range rlm.activeLoops {
		loops = append(loops, loopState)
	}
	return loops
}

func (rlm *RalphLoopManager) CleanupCompletedRalphLoops(maxAge time.Duration) int {
	rlm.loopsMu.Lock()
	defer rlm.loopsMu.Unlock()

	cleaned := 0
	now := time.Now()

	for loopID, loopState := range rlm.activeLoops {
		if (loopState.Status == "completed" || loopState.Status == "failed") &&
			now.Sub(loopState.LastUpdate) > maxAge {
			delete(rlm.activeLoops, loopID)
			cleaned++
		}
	}

	if cleaned > 0 {
		logger.Info("RalphLoopManager cleaned up %d completed loops", cleaned)
	}

	return cleaned
}

type Message struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to,omitempty"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type AgentCommunicationBus struct {
	messages []Message
	msgsMu   sync.RWMutex
}

func NewAgentCommunicationBus() *AgentCommunicationBus {
	return &AgentCommunicationBus{
		messages: make([]Message, 0),
	}
}

func (acb *AgentCommunicationBus) Broadcast(message string, fromAgentID string) {
	msg := Message{
		ID:        uuid.New().String(),
		From:      fromAgentID,
		Content:   message,
		Timestamp: time.Now(),
	}

	acb.msgsMu.Lock()
	acb.messages = append(acb.messages, msg)
	acb.msgsMu.Unlock()

	logger.Info("Broadcast message from %s: %s", fromAgentID, message)
}

func (acb *AgentCommunicationBus) Send(toAgentID, message string, fromAgentID string) error {
	msg := Message{
		ID:        uuid.New().String(),
		From:      fromAgentID,
		To:        toAgentID,
		Content:   message,
		Timestamp: time.Now(),
	}

	acb.msgsMu.Lock()
	acb.messages = append(acb.messages, msg)
	acb.msgsMu.Unlock()

	logger.Info("Message from %s to %s: %s", fromAgentID, toAgentID, message)
	return nil
}

func (acb *AgentCommunicationBus) GetMessages(agentID string, since time.Time) []Message {
	acb.msgsMu.RLock()
	defer acb.msgsMu.RUnlock()

	var agentMessages []Message
	for _, msg := range acb.messages {
		if (msg.To == agentID || msg.To == "" || msg.From == agentID) && msg.Timestamp.After(since) {
			agentMessages = append(agentMessages, msg)
		}
	}
	return agentMessages
}

type AgentOrchestrator struct {
	*Orchestrator
	agentManager     *AgentManager
	communicationBus *AgentCommunicationBus
	resultAggregator *ResultAggregator
}

func NewAgentOrchestrator(llmClient llm.LLMClientInterface) *AgentOrchestrator {
	baseOrch := NewOrchestrator(llmClient)
	agentMgr := NewAgentManager(".")
	resultAgg := NewResultAggregator(".")

	return &AgentOrchestrator{
		Orchestrator:     baseOrch,
		agentManager:     agentMgr,
		communicationBus: NewAgentCommunicationBus(),
		resultAggregator: resultAgg,
	}
}

func (ao *AgentOrchestrator) SpawnAgent(agentType AgentType, task string, config map[string]interface{}) (*Agent, error) {
	return ao.agentManager.SpawnAgent(agentType, task, config)
}

func (ao *AgentOrchestrator) GetAgent(agentID string) (*Agent, bool) {
	return ao.agentManager.GetAgent(agentID)
}

func (ao *AgentOrchestrator) ListAgents() []*Agent {
	return ao.agentManager.ListAgents()
}

func (ao *AgentOrchestrator) StopAgent(agentID string) error {
	return ao.agentManager.StopAgent(agentID)
}

func (ao *AgentOrchestrator) BroadcastMessage(message string, fromAgentID string) {
	ao.communicationBus.Broadcast(message, fromAgentID)
}

func (ao *AgentOrchestrator) SendMessage(toAgentID, message string, fromAgentID string) error {
	return ao.communicationBus.Send(toAgentID, message, fromAgentID)
}

func (ao *AgentOrchestrator) AggregateResults() (*AggregationResult, error) {
	agents := ao.ListAgents()
	return ao.resultAggregator.AggregateResults(agents)
}

func (ao *AgentOrchestrator) SetConflictResolutionStrategy(strategy ConflictResolutionStrategy) {
	ao.resultAggregator.SetStrategy(strategy)
}

func (ao *AgentOrchestrator) GenerateAggregationReport(result *AggregationResult) string {
	return ao.resultAggregator.GenerateReport(result)
}

func (ao *AgentOrchestrator) GetAgentStatus() map[string]interface{} {
	stats := ao.agentManager.GetStats()
	agents := ao.ListAgents()

	agentDetails := make([]map[string]interface{}, 0, len(agents))
	for _, agent := range agents {
		agentStatus := map[string]interface{}{
			"id":         agent.ID,
			"type":       agent.Type,
			"name":       agent.Name,
			"task":       agent.Task,
			"status":     agent.Status,
			"start_time": agent.StartTime,
		}

		if agent.EndTime != nil {
			agentStatus["end_time"] = agent.EndTime
			agentStatus["duration"] = agent.EndTime.Sub(agent.StartTime)
		}

		if agent.ErrorMessage != "" {
			agentStatus["error"] = agent.ErrorMessage
		}

		if agent.Results != nil {
			agentStatus["results"] = agent.Results
		}

		agentDetails = append(agentDetails, agentStatus)
	}

	stats["agents"] = agentDetails
	return stats
}
