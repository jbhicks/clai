package mcpbridge

import (
	"context"
	"strings"
	"testing"
)

// TestInternalToolAdapter tests wrapping existing tools
func TestInternalToolAdapter(t *testing.T) {
	// Create a tool
	tool := ToolDefinition{
		Type: "function",
		Function: ToolFunction{
			Name:        "echo",
			Description: "Echo back input",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []string{"message"},
			},
		},
	}

	// Create executor that just echoes
	executor := func(toolCall ToolCall) (string, error) {
		msg := toolCall.Arguments["message"].(string)
		return msg, nil
	}

	// Create adapter
	adapter := NewInternalToolAdapter(
		"echo-server",
		"Echo server for testing",
		[]ToolDefinition{tool},
		executor,
	)

	// Test ListTools
	ctx := context.Background()
	tools, err := adapter.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}

	if tools[0].Function.Name != "echo" {
		t.Errorf("Expected tool 'echo', got '%s'", tools[0].Function.Name)
	}

	// Test CallTool
	result, err := adapter.CallTool(ctx, "echo", map[string]interface{}{
		"message": "hello world",
	})

	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	resultMap, ok := result.(map[string]string)
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	if resultMap["output"] != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", resultMap["output"])
	}
}

// TestCreateDefaultToolAdapter tests the default tool adapter
func TestCreateDefaultToolAdapter(t *testing.T) {
	adapter := CreateDefaultToolAdapter()

	if adapter == nil {
		t.Fatal("Expected non-nil adapter")
	}

	if adapter.name != "clai-tools" {
		t.Errorf("Expected name 'clai-tools', got '%s'", adapter.name)
	}

	ctx := context.Background()
	tools, err := adapter.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	// Should have execute_code, execute_bash, execute_python, execute_javascript
	if len(tools) != 4 {
		t.Errorf("Expected 4 tools, got %d", len(tools))
	}

	// Check for specific tools
	hasCode := false
	hasBash := false
	for _, tool := range tools {
		if tool.Function.Name == "execute_code" {
			hasCode = true
		}
		if tool.Function.Name == "execute_bash" {
			hasBash = true
		}
	}

	if !hasCode {
		t.Error("Expected execute_code tool")
	}
	if !hasBash {
		t.Error("Expected execute_bash tool")
	}
}

// TestMCPServerRegistry tests the server registry
func TestMCPServerRegistry(t *testing.T) {
	registry := NewMCPServerRegistry()

	// Create mock clients
	client1 := &mockMCPClient{}
	client2 := &mockMCPClient{}

	// Register servers
	registry.Register("server1", client1)
	registry.Register("server2", client2)

	// Test List
	servers := registry.List()
	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}

	// Test Get
	retrieved, ok := registry.Get("server1")
	if !ok {
		t.Error("Expected to find server1")
	}
	if retrieved != client1 {
		t.Error("Expected retrieved client to match")
	}

	// Test Unregister
	registry.Unregister("server1")

	_, ok = registry.Get("server1")
	if ok {
		t.Error("Expected server1 to be unregistered")
	}

	// Should only have server2 now
	servers = registry.List()
	if len(servers) != 1 {
		t.Errorf("Expected 1 server after unregister, got %d", len(servers))
	}
}

// TestExternalMCPClientCreation tests external client setup
func TestExternalMCPClientCreation(t *testing.T) {
	// Test stdio client
	stdioClient := NewExternalMCPClient(
		"test-stdio",
		"npx",
		[]string{"-y", "@test/server"},
		[]string{"KEY=value"},
		"",
	)

	if stdioClient.name != "test-stdio" {
		t.Errorf("Expected name 'test-stdio', got '%s'", stdioClient.name)
	}

	if stdioClient.transport != "stdio" {
		t.Errorf("Expected transport 'stdio', got '%s'", stdioClient.transport)
	}

	if len(stdioClient.args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(stdioClient.args))
	}

	// Test SSE client
	sseClient := NewExternalMCPClient(
		"test-sse",
		"",
		nil,
		nil,
		"http://localhost:3000/sse",
	)

	if sseClient.transport != "sse" {
		t.Errorf("Expected transport 'sse', got '%s'", sseClient.transport)
	}

	if sseClient.url != "http://localhost:3000/sse" {
		t.Errorf("Expected URL, got '%s'", sseClient.url)
	}
}

// TestToolIntegrationScenario tests a complete tool integration scenario
func TestToolIntegrationScenario(t *testing.T) {
	// Setup
	vfs := NewVirtualFS()

	// Add internal tools
	internalAdapter := CreateDefaultToolAdapter()
	vfs.RegisterServer("clai-tools", internalAdapter)

	// Add mock external server
	externalClient := &mockMCPClient{
		tools: []ToolDefinition{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        "mock_external_tool",
					Description: "A mock external tool",
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
	}
	vfs.RegisterServer("external-server", externalClient)

	// Verify setup
	servers := vfs.ListServers()
	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}

	// Generate modules for both servers
	internalModule, err := vfs.GenerateModule("clai-tools")
	if err != nil {
		t.Fatalf("Failed to generate internal module: %v", err)
	}

	if !strings.Contains(internalModule.PythonCode, "def execute_code") {
		t.Error("Expected internal module to contain execute_code")
	}

	externalModule, err := vfs.GenerateModule("external-server")
	if err != nil {
		t.Fatalf("Failed to generate external module: %v", err)
	}

	if !strings.Contains(externalModule.PythonCode, "def mock_external_tool") {
		t.Error("Expected external module to contain mock_external_tool")
	}

	// Create executor
	config := &ExecutorConfig{
		MaxTurns:          10,
		WorkspaceDir:      "/tmp/test_workspace",
		ResultThreshold:   500,
		EnablePersistence: false,
	}

	executor := NewExecutor(vfs, config)

	// Verify executor has access to both servers
	if executor.vfs == nil {
		t.Error("Expected vfs to be set")
	}

	// Verify discovery engine
	discovery := NewDiscoveryEngine(vfs)
	modules := discovery.SearchModules("")
	if len(modules) != 2 {
		t.Errorf("Expected discovery to find 2 modules, got %d", len(modules))
	}
}
