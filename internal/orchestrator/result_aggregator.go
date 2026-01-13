package orchestrator

import (
	"clai/internal/logger"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ConflictResolutionStrategy string

const (
	StrategyMerge        ConflictResolutionStrategy = "merge"
	StrategyPrioritize   ConflictResolutionStrategy = "prioritize"
	StrategyManualReview ConflictResolutionStrategy = "manual_review"
	StrategyLatestWins   ConflictResolutionStrategy = "latest_wins"
)

type FileConflict struct {
	FilePath    string               `json:"file_path"`
	Agents      []string             `json:"agents"`
	ModTimes    []time.Time          `json:"mod_times"`
	Resolutions []ConflictResolution `json:"resolutions"`
	Status      ConflictStatus       `json:"status"`
}

type ConflictResolution struct {
	Strategy      ConflictResolutionStrategy `json:"strategy"`
	PriorityAgent string                     `json:"priority_agent,omitempty"`
	ResolvedAt    time.Time                  `json:"resolved_at"`
	Resolution    string                     `json:"resolution"`
}

type ConflictStatus string

const (
	ConflictUnresolved ConflictStatus = "unresolved"
	ConflictResolved   ConflictStatus = "resolved"
	ConflictMerged     ConflictStatus = "merged"
)

type AggregationResult struct {
	TotalAgents     int               `json:"total_agents"`
	Successful      int               `json:"successful"`
	Failed          int               `json:"failed"`
	WithConflicts   int               `json:"with_conflicts"`
	ResolvedFiles   []string          `json:"resolved_files"`
	Conflicts       []FileConflict    `json:"conflicts"`
	AggregatedFiles map[string]string `json:"aggregated_files"`
	AgentSummaries  []AgentSummary    `json:"agent_summaries"`
	AggregationTime time.Time         `json:"aggregation_time"`
}

type AgentSummary struct {
	AgentID       string                 `json:"agent_id"`
	AgentName     string                 `json:"agent_name"`
	AgentType     AgentType              `json:"agent_type"`
	Status        AgentStatus            `json:"status"`
	FilesCreated  []string               `json:"files_created"`
	FilesModified []string               `json:"files_modified"`
	Results       map[string]interface{} `json:"results"`
}

type ResultAggregator struct {
	baseDir       string
	strategy      ConflictResolutionStrategy
	priorityOrder map[string]int
}

func NewResultAggregator(baseDir string) *ResultAggregator {
	return &ResultAggregator{
		baseDir:  baseDir,
		strategy: StrategyMerge,
		priorityOrder: map[string]int{
			string(CodeAgent):     1,
			string(TestAgent):     2,
			string(ResearchAgent): 3,
			string(ReviewAgent):   4,
			string(DocumentAgent): 5,
		},
	}
}

func (ra *ResultAggregator) SetStrategy(strategy ConflictResolutionStrategy) {
	ra.strategy = strategy
}

func (ra *ResultAggregator) AggregateResults(agents []*Agent) (*AggregationResult, error) {
	logger.Info("Starting result aggregation for %d agents", len(agents))

	result := &AggregationResult{
		TotalAgents:     len(agents),
		AggregatedFiles: make(map[string]string),
		AgentSummaries:  make([]AgentSummary, 0, len(agents)),
		AggregationTime: time.Now(),
	}

	for _, agent := range agents {
		summary := ra.createAgentSummary(agent)
		result.AgentSummaries = append(result.AgentSummaries, summary)

		switch agent.Status {
		case AgentCompleted:
			result.Successful++
		case AgentError:
			result.Failed++
		}
	}

	allFiles := make(map[string][]*Agent)

	for _, agent := range agents {
		if agent.Status != AgentCompleted {
			continue
		}

		agentDir := filepath.Join(ra.baseDir, "tmp", "agents", agent.ID)
		files, err := ra.collectAgentFiles(agentDir)
		if err != nil {
			logger.Warn("Failed to collect files for agent %s: %v", agent.Name, err)
			continue
		}

		for _, file := range files {
			relPath, err := filepath.Rel(ra.baseDir, file)
			if err != nil {
				relPath = file
			}

			allFiles[relPath] = append(allFiles[relPath], agent)
		}
	}

	conflicts := ra.processConflicts(allFiles, result)

	result.Conflicts = conflicts
	result.WithConflicts = len(conflicts)

	err := ra.generateAggregatedFiles(allFiles, result)
	if err != nil {
		return result, fmt.Errorf("failed to generate aggregated files: %w", err)
	}

	logger.Info("Result aggregation complete: %d successful, %d failed, %d conflicts",
		result.Successful, result.Failed, result.WithConflicts)

	return result, nil
}

func (ra *ResultAggregator) createAgentSummary(agent *Agent) AgentSummary {
	summary := AgentSummary{
		AgentID:   agent.ID,
		AgentName: agent.Name,
		AgentType: agent.Type,
		Status:    agent.Status,
		Results:   agent.Results,
	}

	if agent.Status == AgentCompleted {
		agentDir := filepath.Join(ra.baseDir, "tmp", "agents", agent.ID)
		files, err := ra.collectAgentFiles(agentDir)
		if err == nil {
			for _, file := range files {
				relPath, _ := filepath.Rel(ra.baseDir, file)
				if ra.isNewFile(file) {
					summary.FilesCreated = append(summary.FilesCreated, relPath)
				} else {
					summary.FilesModified = append(summary.FilesModified, relPath)
				}
			}
		}
	}

	return summary
}

func (ra *ResultAggregator) collectAgentFiles(agentDir string) ([]string, error) {
	var files []string

	err := filepath.Walk(agentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(path, "prd.json") || strings.HasSuffix(path, "progress.txt") {
			return nil
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

func (ra *ResultAggregator) isNewFile(filePath string) bool {
	baseFile := filepath.Join(ra.baseDir, filepath.Base(filePath))
	_, err := os.Stat(baseFile)
	return os.IsNotExist(err)
}

func (ra *ResultAggregator) processConflicts(allFiles map[string][]*Agent, result *AggregationResult) []FileConflict {
	var conflicts []FileConflict

	for filePath, agents := range allFiles {
		if len(agents) <= 1 {
			continue
		}

		conflict := FileConflict{
			FilePath: filePath,
			Agents:   make([]string, len(agents)),
			ModTimes: make([]time.Time, len(agents)),
			Status:   ConflictUnresolved,
		}

		for i, agent := range agents {
			conflict.Agents[i] = agent.Name
			if agent.EndTime != nil {
				conflict.ModTimes[i] = *agent.EndTime
			} else {
				conflict.ModTimes[i] = time.Now()
			}
		}

		resolution := ra.resolveConflict(conflict, agents)
		conflict.Resolutions = []ConflictResolution{resolution}

		if resolution.Strategy != StrategyManualReview {
			conflict.Status = ConflictResolved
			result.ResolvedFiles = append(result.ResolvedFiles, filePath)
		}

		conflicts = append(conflicts, conflict)
	}

	return conflicts
}

func (ra *ResultAggregator) resolveConflict(conflict FileConflict, agents []*Agent) ConflictResolution {
	resolution := ConflictResolution{
		Strategy:   ra.strategy,
		ResolvedAt: time.Now(),
	}

	switch ra.strategy {
	case StrategyLatestWins:
		latestIdx := 0
		var latestTime time.Time
		if agents[0].EndTime != nil {
			latestTime = *agents[0].EndTime
		} else {
			latestTime = time.Now()
		}
		for i, agent := range agents {
			if agent.EndTime != nil && agent.EndTime.After(latestTime) {
				latestTime = *agent.EndTime
				latestIdx = i
			}
		}
		resolution.Resolution = fmt.Sprintf("Using version from %s (latest)", agents[latestIdx].Name)

	case StrategyPrioritize:
		bestAgent := agents[0]
		bestPriority := ra.priorityOrder[string(bestAgent.Type)]
		for _, agent := range agents {
			priority := ra.priorityOrder[string(agent.Type)]
			if priority < bestPriority {
				bestAgent = agent
				bestPriority = priority
			}
		}
		resolution.PriorityAgent = bestAgent.Name
		resolution.Resolution = fmt.Sprintf("Using version from %s (%s agent - highest priority)",
			bestAgent.Name, bestAgent.Type)

	case StrategyMerge:
		resolution.Resolution = "Merged versions from all agents (requires manual review)"

	case StrategyManualReview:
		resolution.Resolution = "Requires manual review - multiple agents modified same file"
	}

	return resolution
}

func (ra *ResultAggregator) generateAggregatedFiles(allFiles map[string][]*Agent, result *AggregationResult) error {
	for filePath, agents := range allFiles {
		if len(agents) == 1 {
			content, err := ra.readAgentFile(agents[0], filePath)
			if err != nil {
				logger.Warn("Failed to read file %s from agent %s: %v", filePath, agents[0].Name, err)
				continue
			}
			result.AggregatedFiles[filePath] = content
		} else {
			content, err := ra.getResolvedContent(filePath, agents, result.Conflicts)
			if err != nil {
				logger.Warn("Failed to resolve content for %s: %v", filePath, err)
				continue
			}
			result.AggregatedFiles[filePath] = content
		}
	}

	return nil
}

func (ra *ResultAggregator) readAgentFile(agent *Agent, filePath string) (string, error) {
	agentFilePath := filepath.Join(ra.baseDir, "tmp", "agents", agent.ID, filepath.Base(filePath))
	content, err := os.ReadFile(agentFilePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (ra *ResultAggregator) getResolvedContent(filePath string, agents []*Agent, conflicts []FileConflict) (string, error) {
	var conflict *FileConflict
	for _, c := range conflicts {
		if c.FilePath == filePath {
			conflict = &c
			break
		}
	}

	if conflict == nil {
		return "", fmt.Errorf("no conflict found for file %s", filePath)
	}

	if len(conflict.Resolutions) > 0 {
		resolution := conflict.Resolutions[0]

		switch resolution.Strategy {
		case StrategyLatestWins:
			sort.Slice(agents, func(i, j int) bool {
				timeI := time.Now()
				timeJ := time.Now()
				if agents[i].EndTime != nil {
					timeI = *agents[i].EndTime
				}
				if agents[j].EndTime != nil {
					timeJ = *agents[j].EndTime
				}
				return timeI.After(timeJ)
			})
			return ra.readAgentFile(agents[0], filePath)

		case StrategyPrioritize:
			bestAgent := agents[0]
			bestPriority := ra.priorityOrder[string(bestAgent.Type)]
			for _, agent := range agents {
				priority := ra.priorityOrder[string(agent.Type)]
				if priority < bestPriority {
					bestAgent = agent
					bestPriority = priority
				}
			}
			return ra.readAgentFile(bestAgent, filePath)

		default:
			return ra.readAgentFile(agents[0], filePath)
		}
	}

	return ra.readAgentFile(agents[0], filePath)
}

func (ra *ResultAggregator) GenerateReport(result *AggregationResult) string {
	var report strings.Builder

	report.WriteString(fmt.Sprintf("=== AGENT ORCHESTRATION REPORT ===\n"))
	report.WriteString(fmt.Sprintf("Generated: %s\n\n", result.AggregationTime.Format("2006-01-02 15:04:05")))

	report.WriteString(fmt.Sprintf("SUMMARY:\n"))
	report.WriteString(fmt.Sprintf("  Total Agents: %d\n", result.TotalAgents))
	report.WriteString(fmt.Sprintf("  Successful: %d\n", result.Successful))
	report.WriteString(fmt.Sprintf("  Failed: %d\n", result.Failed))
	report.WriteString(fmt.Sprintf("  With Conflicts: %d\n", result.WithConflicts))
	report.WriteString(fmt.Sprintf("\n"))

	if len(result.Conflicts) > 0 {
		report.WriteString(fmt.Sprintf("CONFLICTS:\n"))
		for _, conflict := range result.Conflicts {
			report.WriteString(fmt.Sprintf("  %s (%d agents)\n", conflict.FilePath, len(conflict.Agents)))
			if len(conflict.Resolutions) > 0 {
				res := conflict.Resolutions[0]
				report.WriteString(fmt.Sprintf("    Resolution: %s\n", res.Resolution))
			}
		}
		report.WriteString(fmt.Sprintf("\n"))
	}

	report.WriteString(fmt.Sprintf("AGENT DETAILS:\n"))
	for _, summary := range result.AgentSummaries {
		status := "✅"
		if summary.Status == AgentError {
			status = "❌"
		}

		report.WriteString(fmt.Sprintf("  %s %s (%s) - %s\n",
			status, summary.AgentName, summary.AgentType, summary.Status))

		if len(summary.FilesCreated) > 0 {
			report.WriteString(fmt.Sprintf("    Created: %s\n", strings.Join(summary.FilesCreated, ", ")))
		}
		if len(summary.FilesModified) > 0 {
			report.WriteString(fmt.Sprintf("    Modified: %s\n", strings.Join(summary.FilesModified, ", ")))
		}
	}

	if len(result.AggregatedFiles) > 0 {
		report.WriteString(fmt.Sprintf("\nAGGREGATED FILES: %d\n", len(result.AggregatedFiles)))
	}

	return report.String()
}
