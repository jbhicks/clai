package parallel

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"clai/internal/logger"
	"golang.org/x/sync/errgroup"
)

// Command represents a single command to execute
type Command struct {
	Name         string            `json:"name"`
	Args         []string          `json:"args"`
	WorkingDir   string            `json:"workingDir,omitempty"`
	Environment  map[string]string `json:"environment,omitempty"`
	Timeout      time.Duration     `json:"timeout,omitempty"`
	Priority     int               `json:"priority"` // 1-10, higher = more important
	Dependencies []string          `json:"dependencies,omitempty"`
}

// CommandResult represents the result of executing a command
type CommandResult struct {
	Command   Command       `json:"command"`
	Success   bool          `json:"success"`
	Output    string        `json:"output"`
	Error     string        `json:"error"`
	Duration  time.Duration `json:"duration"`
	StartTime time.Time     `json:"startTime"`
	EndTime   time.Time     `json:"endTime"`
	ExitCode  int           `json:"exitCode"`
}

// ParallelExecutor manages parallel command execution
type ParallelExecutor struct {
	maxConcurrency  int
	resourceMonitor *ResourceMonitor
}

// NewParallelExecutor creates a new parallel executor
func NewParallelExecutor(maxConcurrency int) *ParallelExecutor {
	return &ParallelExecutor{
		maxConcurrency:  maxConcurrency,
		resourceMonitor: NewResourceMonitor(),
	}
}

// ExecuteCommands executes a set of commands with dependency management
func (pe *ParallelExecutor) ExecuteCommands(ctx context.Context, commands []Command) ([]CommandResult, error) {
	logger.Info("Starting parallel execution of %d commands", len(commands))

	// Build dependency graph
	dependencyGraph := pe.buildDependencyGraph(commands)

	// Execute commands in dependency order
	results := make([]CommandResult, 0, len(commands))
	executed := make(map[string]bool)

	for len(executed) < len(commands) {
		// Find commands ready to execute (all dependencies satisfied)
		readyCommands := pe.findReadyCommands(commands, executed, dependencyGraph)

		if len(readyCommands) == 0 {
			return nil, fmt.Errorf("circular dependency detected or no commands ready to execute")
		}

		// Execute ready commands in parallel (up to maxConcurrency)
		batchResults, err := pe.executeCommandBatch(ctx, readyCommands)
		if err != nil {
			return nil, fmt.Errorf("failed to execute command batch: %w", err)
		}

		results = append(results, batchResults...)

		// Mark successful commands as executed
		for _, result := range batchResults {
			if result.Success {
				executed[result.Command.Name] = true
			}
		}
	}

	logger.Info("Completed parallel execution of %d commands", len(commands))
	return results, nil
}

// executeCommandBatch executes a batch of commands in parallel
func (pe *ParallelExecutor) executeCommandBatch(ctx context.Context, commands []Command) ([]CommandResult, error) {
	results := make([]CommandResult, len(commands))
	resultsMutex := &sync.Mutex{}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(pe.maxConcurrency)

	for i, cmd := range commands {
		i, cmd := i, cmd // Capture loop variables
		g.Go(func() error {
			// Check resource availability
			if !pe.resourceMonitor.CanExecuteCommand(cmd) {
				logger.Warn("Skipping command %s due to resource constraints", cmd.Name)
				resultsMutex.Lock()
				results[i] = CommandResult{
					Command:  cmd,
					Success:  false,
					Error:    "Resource constraints prevent execution",
					ExitCode: -1,
				}
				resultsMutex.Unlock()
				return nil
			}

			result := pe.executeSingleCommand(gctx, cmd)
			resultsMutex.Lock()
			results[i] = result
			resultsMutex.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// executeSingleCommand executes a single command
func (pe *ParallelExecutor) executeSingleCommand(ctx context.Context, cmd Command) CommandResult {
	startTime := time.Now()

	// Notify resource monitor that command is starting
	pe.resourceMonitor.CommandStarted(cmd)

	// Prepare the command
	execCmd := exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)
	if cmd.WorkingDir != "" {
		execCmd.Dir = cmd.WorkingDir
	}

	// Set environment variables
	if cmd.Environment != nil {
		execCmd.Env = make([]string, 0, len(cmd.Environment))
		for key, value := range cmd.Environment {
			execCmd.Env = append(execCmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Execute the command
	output, err := execCmd.CombinedOutput()

	duration := time.Since(startTime)
	exitCode := 0

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result := CommandResult{
		Command:   cmd,
		Success:   err == nil,
		Output:    string(output),
		Error:     "",
		Duration:  duration,
		StartTime: startTime,
		EndTime:   time.Now(),
		ExitCode:  exitCode,
	}

	if err != nil {
		result.Error = err.Error()
	}

	// Notify resource monitor that command finished
	pe.resourceMonitor.CommandFinished(cmd)

	logger.Info("Command %s completed in %v (success: %v)", cmd.Name, duration, result.Success)
	return result
}

// buildDependencyGraph builds a map of command dependencies
func (pe *ParallelExecutor) buildDependencyGraph(commands []Command) map[string][]string {
	graph := make(map[string][]string)
	for _, cmd := range commands {
		graph[cmd.Name] = cmd.Dependencies
	}
	return graph
}

// findReadyCommands finds commands that are ready to execute (all dependencies satisfied)
func (pe *ParallelExecutor) findReadyCommands(commands []Command, executed map[string]bool, dependencies map[string][]string) []Command {
	var ready []Command

	for _, cmd := range commands {
		if executed[cmd.Name] {
			continue // Already executed
		}

		// Check if all dependencies are satisfied
		readyToExecute := true
		for _, dep := range dependencies[cmd.Name] {
			if !executed[dep] {
				readyToExecute = false
				break
			}
		}

		if readyToExecute {
			ready = append(ready, cmd)
		}
	}

	// Sort by priority (higher priority first)
	for i := 0; i < len(ready)-1; i++ {
		for j := i + 1; j < len(ready); j++ {
			if ready[i].Priority < ready[j].Priority {
				ready[i], ready[j] = ready[j], ready[i]
			}
		}
	}

	return ready
}

// GetExecutionStats returns statistics about command execution
func (pe *ParallelExecutor) GetExecutionStats(results []CommandResult) map[string]interface{} {
	stats := map[string]interface{}{
		"total_commands": len(results),
		"successful":     0,
		"failed":         0,
		"total_duration": 0.0,
		"avg_duration":   0.0,
	}

	totalDuration := time.Duration(0)
	for _, result := range results {
		if result.Success {
			stats["successful"] = stats["successful"].(int) + 1
		} else {
			stats["failed"] = stats["failed"].(int) + 1
		}
		totalDuration += result.Duration
	}

	stats["total_duration"] = totalDuration.Seconds()
	if len(results) > 0 {
		stats["avg_duration"] = totalDuration.Seconds() / float64(len(results))
	}

	return stats
}

// ResourceMonitor monitors system resources for command execution
type ResourceMonitor struct {
	maxConcurrentCommands int
	currentRunning        int
	maxMemoryUsage        uint64 // Maximum memory usage in MB
	currentMemoryUsage    uint64 // Current memory usage in MB
	mutex                 sync.Mutex
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		maxConcurrentCommands: 4, // Default limit
		currentRunning:        0,
		maxMemoryUsage:        2048, // Default 2GB limit
		currentMemoryUsage:    0,
	}
}

// CanExecuteCommand checks if a command can be executed based on resource constraints
func (rm *ResourceMonitor) CanExecuteCommand(cmd Command) bool {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	// Check concurrent command limit
	if rm.currentRunning >= rm.maxConcurrentCommands {
		return false
	}

	// Check memory usage limit
	estimatedMemory := rm.estimateCommandMemory(cmd)
	if rm.currentMemoryUsage+estimatedMemory > rm.maxMemoryUsage {
		return false
	}

	return true
}

// estimateCommandMemory estimates memory usage for a command
func (rm *ResourceMonitor) estimateCommandMemory(cmd Command) uint64 {
	// Simple estimation based on command type
	// In a real implementation, this would be more sophisticated
	baseMemory := uint64(50) // 50MB base

	switch cmd.Args[0] {
	case "go":
		if len(cmd.Args) > 1 && cmd.Args[1] == "build" {
			return baseMemory + 200 // Build commands use more memory
		}
		if len(cmd.Args) > 1 && cmd.Args[1] == "test" {
			return baseMemory + 150 // Test commands use moderate memory
		}
	case "make":
		return baseMemory + 100 // Make commands can use significant memory
	}

	return baseMemory
}

// UpdateMemoryUsage updates the current memory usage tracking
func (rm *ResourceMonitor) UpdateMemoryUsage(delta uint64) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if delta > 0 {
		rm.currentMemoryUsage += delta
	} else {
		if rm.currentMemoryUsage >= -delta {
			rm.currentMemoryUsage += delta // delta is negative
		} else {
			rm.currentMemoryUsage = 0
		}
	}
}

// GetMemoryStats returns current memory usage statistics
func (rm *ResourceMonitor) GetMemoryStats() map[string]interface{} {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	return map[string]interface{}{
		"max_memory_mb":        rm.maxMemoryUsage,
		"current_memory_mb":    rm.currentMemoryUsage,
		"available_memory_mb":  rm.maxMemoryUsage - rm.currentMemoryUsage,
		"memory_usage_percent": float64(rm.currentMemoryUsage) / float64(rm.maxMemoryUsage) * 100,
	}
}

// CommandStarted should be called when a command starts
func (rm *ResourceMonitor) CommandStarted(cmd Command) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	rm.currentRunning++

	memoryUsed := rm.estimateCommandMemory(cmd)
	rm.currentMemoryUsage += memoryUsed
}

// CommandFinished should be called when a command finishes
func (rm *ResourceMonitor) CommandFinished(cmd Command) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	if rm.currentRunning > 0 {
		rm.currentRunning--
	}

	memoryUsed := rm.estimateCommandMemory(cmd)
	if rm.currentMemoryUsage >= memoryUsed {
		rm.currentMemoryUsage -= memoryUsed
	} else {
		rm.currentMemoryUsage = 0
	}
}

// SetMaxConcurrentCommands sets the maximum number of concurrent commands
func (rm *ResourceMonitor) SetMaxConcurrentCommands(max int) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	rm.maxConcurrentCommands = max
}

// SetMaxMemoryUsage sets the maximum memory usage limit in MB
func (rm *ResourceMonitor) SetMaxMemoryUsage(maxMB uint64) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	rm.maxMemoryUsage = maxMB
}
