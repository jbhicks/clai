package mcpbridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PersistentState maintains a Python namespace across multiple turns.
//
// Unlike traditional agents where each turn is independent, Code Mode agents
// maintain state like in a real Python REPL session. Variables persist,
// imports remain available, and the agent can build complex state over time.
//
// Example:
//
//	# Turn 1: Initial setup
//	import servers.google_drive as gd
//	doc = gd.get_document("abc123")
//	processed = analyze(doc)  # Some analysis
//
//	# State after Turn 1:
//	# Variables: {doc: {...}, processed: {...}}
//	# Imports: ["servers.google_drive"]
//
//	# Turn 2: Continue from previous state
//	# Context prepended automatically:
//	"""
//	import servers.google_drive as gd
//	# === Session State ===
//	doc = {...}
//	processed = {...}
//	# === End State ===
//	"""
//
//	# Agent continues seamlessly:
//	print(f"Previously processed {len(processed)} items")
//	more_data = fetch_additional()
//	combined = processed + more_data
//
//	# State after Turn 2:
//	# Variables: {doc: {...}, processed: {...}, more_data: {...}, combined: {...}}
//
// Benefits:
// - Natural programming workflow (variables persist)
// - No need to re-fetch expensive data
// - Can resume long-running tasks after interruption
// - Agent can build complex state over time
type PersistentState struct {
	Variables    map[string]interface{} // Python namespace
	Files        map[string]string      // Saved workspace files
	Imports      []string               // Currently imported modules
	TurnCount    int                    // Number of turns in this session
	SessionID    string                 // Unique session identifier
	WorkspaceDir string                 // Directory for persistence

	mu sync.RWMutex // Protects all fields
}

// NewPersistentState creates a new persistent state
func NewPersistentState(sessionID, workspaceDir string) *PersistentState {
	return &PersistentState{
		Variables:    make(map[string]interface{}),
		Files:        make(map[string]string),
		Imports:      make([]string, 0),
		SessionID:    sessionID,
		WorkspaceDir: workspaceDir,
	}
}

// LoadPersistentState loads a previously saved state from disk
func LoadPersistentState(sessionID, workspaceDir string) (*PersistentState, error) {
	filepath := filepath.Join(workspaceDir, sessionID+"_state.json")

	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			// No previous state, create new
			return NewPersistentState(sessionID, workspaceDir), nil
		}
		return nil, fmt.Errorf("failed to read state: %w", err)
	}

	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Initialize maps if nil (in case of partial save)
	if state.Variables == nil {
		state.Variables = make(map[string]interface{})
	}
	if state.Files == nil {
		state.Files = make(map[string]string)
	}

	return &state, nil
}

// Serialize converts the state to Python code that recreates it
//
// This generates code that, when executed, restores the Python namespace
// to its current state. It's prepended to the agent's context each turn.
func (ps *PersistentState) Serialize() string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var code strings.Builder

	// Re-import modules
	if len(ps.Imports) > 0 {
		code.WriteString("# === Imports ===\n")
		for _, imp := range ps.Imports {
			code.WriteString(fmt.Sprintf("import %s\n", imp))
		}
		code.WriteString("\n")
	}

	// Restore variables
	if len(ps.Variables) > 0 {
		code.WriteString("# === Session State ===\n")
		for name, value := range ps.Variables {
			repr := toPythonRepr(value)
			code.WriteString(fmt.Sprintf("%s = %s\n", name, repr))
		}
		code.WriteString("# === End State ===\n\n")
	}

	return code.String()
}

// SerializeAsContext returns the state formatted as agent context
//
// This adds additional comments to help the agent understand the context
func (ps *PersistentState) SerializeAsContext() string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if len(ps.Variables) == 0 && len(ps.Imports) == 0 {
		return "# Starting fresh session\n"
	}

	var context strings.Builder

	context.WriteString(fmt.Sprintf("# Continuing session (Turn %d)\n", ps.TurnCount))

	// Show imported modules
	if len(ps.Imports) > 0 {
		context.WriteString(fmt.Sprintf("# Imported modules: %s\n", strings.Join(ps.Imports, ", ")))
	}

	// Show available variables
	if len(ps.Variables) > 0 {
		varNames := make([]string, 0, len(ps.Variables))
		for name := range ps.Variables {
			varNames = append(varNames, name)
		}
		context.WriteString(fmt.Sprintf("# Available variables: %s\n", strings.Join(varNames, ", ")))
	}

	context.WriteString("\n")
	context.WriteString(ps.Serialize())

	return context.String()
}

// Update merges new results into the state
//
// This is called after executing Python code to update the namespace
// with new variable assignments.
func (ps *PersistentState) Update(results map[string]interface{}, imports []string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Add new imports
	for _, imp := range imports {
		if !contains(ps.Imports, imp) {
			ps.Imports = append(ps.Imports, imp)
		}
	}

	// Update variables
	for name, value := range results {
		ps.Variables[name] = value
	}

	ps.TurnCount++

	// Persist to disk
	return ps.save()
}

// SetVariable sets a single variable in the namespace
func (ps *PersistentState) SetVariable(name string, value interface{}) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.Variables[name] = value
}

// GetVariable retrieves a variable from the namespace
func (ps *PersistentState) GetVariable(name string) (interface{}, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	val, ok := ps.Variables[name]
	return val, ok
}

// HasVariable checks if a variable exists
func (ps *PersistentState) HasVariable(name string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	_, ok := ps.Variables[name]
	return ok
}

// ListVariables returns all variable names
func (ps *PersistentState) ListVariables() []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	names := make([]string, 0, len(ps.Variables))
	for name := range ps.Variables {
		names = append(names, name)
	}
	return names
}

// AddImport adds an import to the state
func (ps *PersistentState) AddImport(imp string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !contains(ps.Imports, imp) {
		ps.Imports = append(ps.Imports, imp)
	}
}

// HasImport checks if a module is already imported
func (ps *PersistentState) HasImport(imp string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	return contains(ps.Imports, imp)
}

// ListImports returns all imported modules
func (ps *PersistentState) ListImports() []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	result := make([]string, len(ps.Imports))
	copy(result, ps.Imports)
	return result
}

// ClearVariables removes all variables (but keeps imports)
func (ps *PersistentState) ClearVariables() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.Variables = make(map[string]interface{})
}

// ClearImports removes all imports (but keeps variables)
func (ps *PersistentState) ClearImports() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.Imports = make([]string, 0)
}

// Clear removes all state
func (ps *PersistentState) Clear() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.Variables = make(map[string]interface{})
	ps.Imports = make([]string, 0)
	ps.TurnCount = 0
	ps.Files = make(map[string]string)
}

// save persists state to disk
func (ps *PersistentState) save() error {
	if ps.WorkspaceDir == "" {
		return nil // No persistence configured
	}

	// Ensure workspace directory exists
	if err := os.MkdirAll(ps.WorkspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Serialize state
	data, err := json.Marshal(ps)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to file
	filepath := filepath.Join(ps.WorkspaceDir, ps.SessionID+"_state.json")
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	return nil
}

// Save explicitly saves state to disk
func (ps *PersistentState) Save() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	return ps.save()
}

// GetSize returns an estimate of the state's token size
func (ps *PersistentState) GetSize() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Estimate based on variable count and serialized size
	data, _ := json.Marshal(ps.Variables)
	return estimateTokenSize(data)
}

// ToMap exports the state as a simple map for use by other components
func (ps *PersistentState) ToMap() map[string]interface{} {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	return map[string]interface{}{
		"session_id": ps.SessionID,
		"turn_count": ps.TurnCount,
		"variables":  ps.Variables,
		"imports":    ps.Imports,
		"files":      ps.Files,
	}
}

// Helper function to check if string is in slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// StateSnapshot captures a point-in-time view of the state
// Useful for debugging and logging
type StateSnapshot struct {
	Turn     int      `json:"turn"`
	Imports  []string `json:"imports"`
	Vars     []string `json:"variables"`
	VarCount int      `json:"variable_count"`
	Size     int      `json:"size_estimate"`
}

// Snapshot creates a snapshot of the current state
func (ps *PersistentState) Snapshot() StateSnapshot {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	varNames := make([]string, 0, len(ps.Variables))
	for name := range ps.Variables {
		varNames = append(varNames, name)
	}

	return StateSnapshot{
		Turn:     ps.TurnCount,
		Imports:  ps.Imports,
		Vars:     varNames,
		VarCount: len(ps.Variables),
		Size:     ps.GetSize(),
	}
}

// String returns a string representation of the snapshot
func (s StateSnapshot) String() string {
	return fmt.Sprintf(
		"Turn %d: %d vars (%s), %d imports",
		s.Turn,
		s.VarCount,
		strings.Join(s.Vars, ", "),
		len(s.Imports),
	)
}
