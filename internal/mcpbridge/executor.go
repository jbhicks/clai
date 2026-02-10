package mcpbridge

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Executor is the main execution engine for MCP Code Bridge.
//
// It orchestrates the entire flow:
// 1. Receive user query
// 2. Generate Hermes 3 system prompt
// 3. Execute multi-turn agent loop
// 4. Handle tool calls (python, search_modules, inspect_module)
// 5. Parse Python code and execute tool calls via MCP
// 6. Manage state persistence and result storage
//
// Example:
//
//	executor := NewExecutor(vfs)
//	executor.RegisterLLM(llmClient)
//
//	result, err := executor.Run(ctx, "Get my meeting notes from Google Drive")
//	// Executes full workflow with progressive discovery
//
// Architecture:
//
//	┌─────────────┐     ┌──────────────┐     ┌─────────────┐
//	│   User      │────▶│  Executor    │────▶│  Hermes 3   │
//	│   Query     │     │  (this file) │     │   LLM       │
//	└─────────────┘     └──────────────┘     └─────────────┘
//	                           │                     │
//	                           ▼                     ▼
//	                    ┌──────────────┐     ┌─────────────┐
//	                    │  VirtualFS   │     │ Tool Calls  │
//	                    │  Discovery   │     │             │
//	                    └──────────────┘     └──────┬──────┘
//	                                                  │
//	                           ┌─────────────────────┘
//	                           ▼
//	                    ┌──────────────┐
//	                    │   MCP        │
//	                    │   Servers    │
//	                    └──────────────┘
type Executor struct {
	vfs           *VirtualFS
	translator    *CodeTranslator
	hermes        *HermesIntegration
	discovery     *DiscoveryEngine
	resultManager *ResultManager
	state         *PersistentState

	llm    LLMClientInterface
	config *ExecutorConfig

	maxTurns    int
	turnTimeout time.Duration
}

// ExecutorConfig contains configuration for the executor
type ExecutorConfig struct {
	MaxTurns          int
	TurnTimeout       time.Duration
	WorkspaceDir      string
	ResultThreshold   int
	EnablePersistence bool
	SessionID         string
}

// DefaultExecutorConfig returns sensible defaults
func DefaultExecutorConfig() *ExecutorConfig {
	return &ExecutorConfig{
		MaxTurns:          50,
		TurnTimeout:       5 * time.Minute,
		WorkspaceDir:      "./workspace",
		ResultThreshold:   1000,
		EnablePersistence: true,
		SessionID:         generateSessionID(),
	}
}

// LLMClientInterface defines the interface for LLM clients
type LLMClientInterface interface {
	// SendMessage sends a message to the LLM and returns the response
	SendMessage(ctx context.Context, messages []Message) (*LLMResponse, error)

	// SendMessageWithTools sends a message with tool definitions
	SendMessageWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (*LLMResponse, error)
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant", "tool"
	Content string `json:"content"` // Message content
}

// LLMResponse represents a response from the LLM
type LLMResponse struct {
	Content   string     // Text content
	ToolCalls []ToolCall // Tool calls in the response
}

// NewExecutor creates a new MCP Code Bridge executor
func NewExecutor(vfs *VirtualFS, config *ExecutorConfig) *Executor {
	if config == nil {
		config = DefaultExecutorConfig()
	}

	return &Executor{
		vfs:           vfs,
		translator:    NewCodeTranslator(vfs),
		hermes:        NewHermesIntegration(vfs),
		discovery:     NewDiscoveryEngine(vfs),
		resultManager: NewResultManager(config.WorkspaceDir, config.ResultThreshold),
		config:        config,
		maxTurns:      config.MaxTurns,
		turnTimeout:   config.TurnTimeout,
	}
}

// WithLLM sets the LLM client
func (e *Executor) WithLLM(llm LLMClientInterface) *Executor {
	e.llm = llm
	return e
}

// WithState sets or loads a persistent state
func (e *Executor) WithState(state *PersistentState) *Executor {
	e.state = state
	return e
}

// LoadOrCreateState loads existing state or creates new state
func (e *Executor) LoadOrCreateState(sessionID string) error {
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	var state *PersistentState
	var err error

	if e.config.EnablePersistence {
		state, err = LoadPersistentState(sessionID, e.config.WorkspaceDir)
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}
	} else {
		state = NewPersistentState(sessionID, e.config.WorkspaceDir)
	}

	e.state = state
	return nil
}

// Run executes the full agent workflow
//
// This is the main entry point. It handles:
// 1. Progressive tool discovery
// 2. Python code execution
// 3. Multi-turn agent loop
// 4. State management
func (e *Executor) Run(ctx context.Context, userQuery string) (string, error) {
	if e.state == nil {
		if err := e.LoadOrCreateState(e.config.SessionID); err != nil {
			return "", err
		}
	}

	if e.llm == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	// Build initial messages
	messages := e.buildInitialMessages(userQuery)

	// Agent loop
	for turn := 0; turn < e.maxTurns; turn++ {
		// Create timeout context for this turn
		turnCtx, cancel := context.WithTimeout(ctx, e.turnTimeout)
		defer cancel()

		// Send to LLM
		response, err := e.llm.SendMessage(turnCtx, messages)
		if err != nil {
			return "", fmt.Errorf("turn %d: LLM request failed: %w", turn+1, err)
		}

		// Check if done (no tool calls)
		if len(response.ToolCalls) == 0 {
			// Final answer
			return response.Content, nil
		}

		// Process tool calls
		results, continueLoop, err := e.processToolCalls(turnCtx, response.ToolCalls)
		if err != nil {
			return "", fmt.Errorf("turn %d: tool execution failed: %w", turn+1, err)
		}

		// Update messages with tool results
		messages = append(messages,
			Message{Role: "assistant", Content: response.Content})

		for _, result := range results {
			messages = append(messages, Message{
				Role:    "tool",
				Content: result,
			})
		}

		// Update state
		if err := e.updateState(results); err != nil {
			return "", fmt.Errorf("turn %d: state update failed: %w", turn+1, err)
		}

		// Check if we should continue
		if !continueLoop {
			return response.Content, nil
		}
	}

	return "", fmt.Errorf("max turns (%d) exceeded", e.maxTurns)
}

// buildInitialMessages creates the initial message sequence
func (e *Executor) buildInitialMessages(userQuery string) []Message {
	// Generate system prompt
	systemPrompt := e.hermes.GenerateSystemPrompt()

	// Add state context if continuing
	if e.state != nil && e.state.TurnCount > 0 {
		stateContext := e.state.SerializeAsContext()
		systemPrompt += "\n" + stateContext
	}

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userQuery},
	}

	return messages
}

// processToolCalls executes tool calls and returns results
//
// Returns:
// - results: tool results as strings
// - continueLoop: whether to continue the agent loop
// - error: any execution error
func (e *Executor) processToolCalls(ctx context.Context, toolCalls []ToolCall) ([]string, bool, error) {
	var results []string
	continueLoop := false

	for _, tc := range toolCalls {
		var result string
		var err error

		switch tc.Name {
		case "python":
			result, err = e.executePython(ctx, tc.Arguments)
			continueLoop = true // Usually need another turn after python execution

		case "search_available_modules":
			result, err = e.executeSearchModules(tc.Arguments)
			continueLoop = true

		case "inspect_module":
			result, err = e.executeInspectModule(tc.Arguments)
			continueLoop = true

		default:
			// Unknown tool - return error
			err = fmt.Errorf("unknown tool: %s", tc.Name)
		}

		if err != nil {
			result = fmt.Sprintf("Error: %v", err)
		}

		results = append(results, result)
	}

	return results, continueLoop, nil
}

// executePython handles the python tool call
//
// This is the core of MCP Code Bridge. It:
// 1. Extracts Python code from arguments
// 2. Parses the code to find imports and tool calls
// 3. Executes tool calls via MCP
// 4. Updates state with results
// 5. Returns Python representation of results
func (e *Executor) executePython(ctx context.Context, arguments map[string]interface{}) (string, error) {
	// Extract code from arguments
	code, ok := arguments["code"].(string)
	if !ok {
		return "", fmt.Errorf("python tool requires 'code' argument")
	}

	// Parse Python code
	parseResult, err := e.translator.ParsePythonCode(code, e.state.Variables)
	if err != nil {
		return "", fmt.Errorf("failed to parse Python code: %w", err)
	}

	// Extract imports from code and add to state
	for alias, server := range parseResult.Imports {
		modulePath := fmt.Sprintf("servers.%s", sanitizeModuleName(server))
		if !e.state.HasImport(modulePath) {
			e.state.AddImport(modulePath)
		}

		// Also track the alias mapping
		e.state.SetVariable("__import_"+alias, server)
	}

	// Execute tool calls
	if len(parseResult.ToolCalls) > 0 {
		executionResult, err := e.translator.ExecuteWithDependencies(
			ctx, parseResult.ToolCalls, e.state.Variables)

		if err != nil {
			return "", err
		}

		// Update state with results
		e.state.Update(executionResult.Results, []string{})

		// Store large results
		for varName, value := range executionResult.Results {
			if estimateTokenSize(value) > e.resultManager.threshold {
				stored := e.resultManager.StoreResult(value)
				// Replace with reference in state
				e.state.SetVariable(varName, stored)
			}
		}
	}

	// Return success message with available variables
	var response strings.Builder
	response.WriteString("Code executed successfully.\n\n")

	if len(parseResult.ToolCalls) > 0 {
		response.WriteString("Executed tool calls:\n")
		for _, call := range parseResult.ToolCalls {
			response.WriteString(fmt.Sprintf("- %s.%s\n", call.Server, call.Tool))
		}
		response.WriteString("\n")
	}

	response.WriteString("Updated variables:\n")
	for name := range parseResult.Variables {
		value, _ := e.state.GetVariable(name)
		repr := e.resultManager.GetPythonRepresentation(name)
		if repr == "" {
			repr = toPythonRepr(value)
		}
		response.WriteString(fmt.Sprintf("%s = %s\n", name, repr))
	}

	return response.String(), nil
}

// executeSearchModules handles the search_available_modules tool call
func (e *Executor) executeSearchModules(arguments map[string]interface{}) (string, error) {
	keyword := ""
	if k, ok := arguments["keyword"].(string); ok {
		keyword = k
	}

	modules := e.discovery.SearchModules(keyword)
	return FormatSearchResults(modules), nil
}

// executeInspectModule handles the inspect_module tool call
func (e *Executor) executeInspectModule(arguments map[string]interface{}) (string, error) {
	moduleName, ok := arguments["module_name"].(string)
	if !ok {
		return "", fmt.Errorf("inspect_module requires 'module_name' argument")
	}

	detailLevel := "signatures"
	if d, ok := arguments["detail_level"].(string); ok {
		detailLevel = d
	}

	doc, err := e.discovery.InspectModule(moduleName, detailLevel)
	if err != nil {
		return "", err
	}

	return FormatInspectResult(moduleName, detailLevel, doc), nil
}

// updateState updates the persistent state after tool execution
func (e *Executor) updateState(results []string) error {
	if !e.config.EnablePersistence {
		return nil
	}

	return e.state.Save()
}

// generateSessionID creates a unique session identifier
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

// GetStateSnapshot returns a snapshot of the current state
func (e *Executor) GetStateSnapshot() StateSnapshot {
	if e.state == nil {
		return StateSnapshot{}
	}
	return e.state.Snapshot()
}

// GetStats returns execution statistics
func (e *Executor) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"turn_count":     e.state.TurnCount,
		"session_id":     e.state.SessionID,
		"variable_count": len(e.state.Variables),
		"import_count":   len(e.state.Imports),
		"stored_results": len(e.resultManager.refs),
	}
}
