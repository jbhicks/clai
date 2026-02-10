package mcpbridge

import (
	"context"
	"strings"
	"testing"
)

// TestVirtualFS tests the virtual filesystem functionality
func TestVirtualFS(t *testing.T) {
	vfs := NewVirtualFS()

	// Test basic operations
	vfs.RegisterServer("test-server", &mockMCPClient{
		tools: []ToolDefinition{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        "test_tool",
					Description: "A test tool",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"input": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
		},
	})

	// Test list servers
	servers := vfs.ListServers()
	if len(servers) != 1 || servers[0] != "test-server" {
		t.Errorf("Expected [test-server], got %v", servers)
	}

	// Test generate module
	module, err := vfs.GenerateModule("test-server")
	if err != nil {
		t.Errorf("GenerateModule failed: %v", err)
	}

	if module == nil {
		t.Error("Expected module, got nil")
	}

	// Check module name
	if module.Name != "test_server" {
		t.Errorf("Expected module name 'test_server', got '%s'", module.Name)
	}

	// Check Python code contains function
	if module.PythonCode == "" {
		t.Error("Expected Python code, got empty string")
	}

	if !strings.Contains(module.PythonCode, "def test_tool") {
		t.Error("Expected Python code to contain function definition")
	}
}

// TestCodeTranslator tests the code translator
func TestCodeTranslator(t *testing.T) {
	vfs := NewVirtualFS()
	vfs.RegisterServer("google-drive", &mockMCPClient{
		tools: []ToolDefinition{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        "get_document",
					Description: "Get a document",
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"document_id": map[string]interface{}{
								"type": "string",
							},
						},
						"required": []string{"document_id"},
					},
				},
			},
		},
	})

	translator := NewCodeTranslator(vfs)

	// Test simple code parsing
	code := `
import servers.google_drive as gd
doc = gd.get_document("abc123")
`

	result, err := translator.ParsePythonCode(code, nil)
	if err != nil {
		t.Errorf("ParsePythonCode failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Check imports
	if len(result.Imports) != 1 {
		t.Errorf("Expected 1 import, got %d", len(result.Imports))
	}

	if result.Imports["gd"] != "google-drive" {
		t.Errorf("Expected import 'gd' -> 'google-drive', got %v", result.Imports)
	}

	// Check tool calls
	if len(result.ToolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(result.ToolCalls))
	}

	if len(result.ToolCalls) > 0 {
		call := result.ToolCalls[0]
		if call.Server != "google-drive" {
			t.Errorf("Expected server 'google-drive', got '%s'", call.Server)
		}
		if call.Tool != "get_document" {
			t.Errorf("Expected tool 'get_document', got '%s'", call.Tool)
		}
		if call.ReturnVar != "doc" {
			t.Errorf("Expected return var 'doc', got '%s'", call.ReturnVar)
		}
		// Note: For positional arguments, key is "_arg_0", "_arg_1", etc.
		// For keyword arguments, key is the parameter name
		if call.Arguments["_arg_0"] != "abc123" {
			t.Errorf("Expected _arg_0 'abc123', got %v", call.Arguments["_arg_0"])
		}
	}
}

// TestHermesIntegration tests Hermes 3 prompt generation
func TestHermesIntegration(t *testing.T) {
	vfs := NewVirtualFS()
	vfs.RegisterServer("test-server", &mockMCPClient{})

	hermes := NewHermesIntegration(vfs)

	// Test prompt generation
	prompt := hermes.GenerateSystemPrompt()

	if prompt == "" {
		t.Error("Expected non-empty prompt")
	}

	// Check Hermes 3 format markers
	if !strings.Contains(prompt, "<<|im_start|>>system") {
		t.Error("Expected Hermes 3 system marker")
	}

	if !strings.Contains(prompt, "<tools>") {
		t.Error("Expected <tools> tag")
	}

	if !strings.Contains(prompt, "</tools>") {
		t.Error("Expected </tools> tag")
	}

	// Check core tools are present (check for the names, JSON formatting may vary)
	if !strings.Contains(prompt, `"name"`) || !strings.Contains(prompt, `"python"`) {
		t.Error("Expected python tool")
	}

	if !strings.Contains(prompt, `"search_available_modules"`) {
		t.Error("Expected search_available_modules tool")
	}

	if !strings.Contains(prompt, `"inspect_module"`) {
		t.Error("Expected inspect_module tool")
	}
}

// TestDiscoveryEngine tests the discovery engine
func TestDiscoveryEngine(t *testing.T) {
	vfs := NewVirtualFS()
	vfs.RegisterServer("google-drive", &mockMCPClient{
		info: ServerInfo{
			Name:        "google-drive",
			Description: "Google Drive integration",
		},
		tools: []ToolDefinition{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        "get_document",
					Description: "Get document",
					Parameters:  map[string]interface{}{"type": "object"},
				},
			},
		},
	})

	engine := NewDiscoveryEngine(vfs)

	// Test search
	results := engine.SearchModules("")
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if len(results) > 0 {
		if results[0].Name != "google-drive" {
			t.Errorf("Expected 'google-drive', got '%s'", results[0].Name)
		}
		if results[0].ToolCount != 1 {
			t.Errorf("Expected tool count 1, got %d", results[0].ToolCount)
		}
	}

	// Test inspect
	doc, err := engine.InspectModule("google-drive", "overview")
	if err != nil {
		t.Errorf("InspectModule failed: %v", err)
	}

	if !strings.Contains(doc, "get_document") {
		t.Error("Expected documentation to contain tool name")
	}
}

// TestCalculateDiscoveryCost tests token cost calculation
func TestCalculateDiscoveryCost(t *testing.T) {
	vfs := NewVirtualFS()
	engine := NewDiscoveryEngine(vfs)

	cost := engine.CalculateDiscoveryCost("progressive", 10, 5)

	if cost["tokens"] == 0 {
		t.Error("Expected non-zero token cost")
	}

	savings, ok := cost["savings_vs_traditional"].(float64)
	if !ok {
		t.Error("Expected savings percentage")
	}

	if savings < 90 {
		t.Errorf("Expected >90%% savings, got %.1f%%", savings)
	}
}

// TestResultManager tests the result manager
func TestResultManager(t *testing.T) {
	rm := NewResultManager("/tmp/test_workspace", 100)

	// Test storing a small result
	smallData := map[string]string{"key": "value"}
	stored := rm.StoreResult(smallData)

	if stored.ID == "" {
		t.Error("Expected non-empty result ID")
	}

	// Test retrieving result
	retrieved, ok := rm.GetResult(stored.ID)
	if !ok {
		t.Error("Expected to retrieve stored result")
	}

	if retrieved == nil {
		t.Error("Expected non-nil retrieved data")
	}

	// Test Python representation
	repr := rm.GetPythonRepresentation(stored.ID)
	if repr == "" {
		t.Error("Expected non-empty Python representation")
	}

	// Test large result storage
	largeData := make([]map[string]interface{}, 1000)
	for i := range largeData {
		largeData[i] = map[string]interface{}{"id": i, "data": "test"}
	}

	largeStored := rm.StoreResult(largeData)
	if largeStored.Size < 100 {
		t.Error("Expected large result to have high token estimate")
	}

	// Python representation should show reference
	largeRepr := rm.GetPythonRepresentation(largeStored.ID)
	if !strings.Contains(largeRepr, "ResultRef") {
		t.Error("Expected large result to be represented as ResultRef")
	}
}

// TestPersistentState tests the persistent state
func TestPersistentState(t *testing.T) {
	state := NewPersistentState("test_session", "/tmp/test_state")

	// Test setting and getting variables
	state.SetVariable("x", 42)
	state.SetVariable("y", "hello")

	val, ok := state.GetVariable("x")
	if !ok {
		t.Error("Expected to find variable 'x'")
	}

	if val != 42 {
		t.Errorf("Expected 42, got %v", val)
	}

	// Test imports
	state.AddImport("servers.google_drive")
	state.AddImport("servers.salesforce")

	if !state.HasImport("servers.google_drive") {
		t.Error("Expected to find google_drive import")
	}

	imports := state.ListImports()
	if len(imports) != 2 {
		t.Errorf("Expected 2 imports, got %d", len(imports))
	}

	// Test serialization
	serialized := state.Serialize()
	if !strings.Contains(serialized, "x = 42") {
		t.Error("Expected serialization to contain x = 42")
	}

	if !strings.Contains(serialized, "servers.google_drive") {
		t.Error("Expected serialization to contain imports")
	}

	// Test update
	newResults := map[string]interface{}{
		"z": []string{"a", "b", "c"},
	}
	newImports := []string{"servers.notion"}

	err := state.Update(newResults, newImports)
	if err != nil {
		t.Errorf("Update failed: %v", err)
	}

	// Check new variable
	z, ok := state.GetVariable("z")
	if !ok {
		t.Error("Expected to find new variable 'z'")
	}

	if len(z.([]string)) != 3 {
		t.Error("Expected z to have 3 items")
	}

	// Check new import was added
	if !state.HasImport("servers.notion") {
		t.Error("Expected to find new import")
	}
}

// TestExecutorCreation tests executor initialization
func TestExecutorCreation(t *testing.T) {
	vfs := NewVirtualFS()
	vfs.RegisterServer("test-server", &mockMCPClient{})

	config := &ExecutorConfig{
		MaxTurns:          10,
		WorkspaceDir:      "/tmp/test_workspace",
		ResultThreshold:   500,
		EnablePersistence: false,
	}

	executor := NewExecutor(vfs, config)

	if executor == nil {
		t.Fatal("Expected non-nil executor")
	}

	if executor.maxTurns != 10 {
		t.Errorf("Expected maxTurns 10, got %d", executor.maxTurns)
	}

	// Test state loading
	err := executor.LoadOrCreateState("test_session")
	if err != nil {
		t.Errorf("Failed to load/create state: %v", err)
	}

	if executor.state == nil {
		t.Error("Expected state to be loaded")
	}

	if executor.state.SessionID != "test_session" {
		t.Errorf("Expected session ID 'test_session', got '%s'", executor.state.SessionID)
	}
}

// mockMCPClient is a mock implementation for testing
type mockMCPClient struct {
	tools []ToolDefinition
	info  ServerInfo
}

func (m *mockMCPClient) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	return m.tools, nil
}

func (m *mockMCPClient) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (interface{}, error) {
	return map[string]string{"result": "success"}, nil
}

func (m *mockMCPClient) GetServerInfo(ctx context.Context) (ServerInfo, error) {
	return m.info, nil
}
