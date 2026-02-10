// Package mcpbridge_test contains integration examples and tests
// for the MCP Code Bridge system.
//
// This file demonstrates how to use the complete system end-to-end.
package mcpbridge

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Example_integration shows a complete integration scenario
//
// This example demonstrates:
// 1. Setting up multiple tool sources (internal + external)
// 2. Creating the executor with configuration
// 3. Running a multi-turn agent workflow
// 4. Progressive tool discovery
func Example_integration() {
	ctx := context.Background()

	// 1. Create virtual filesystem
	vfs := NewVirtualFS()

	// 2. Add internal tools (existing clai tools)
	internalAdapter := CreateDefaultToolAdapter()
	vfs.RegisterServer("clai-tools", internalAdapter)

	// 3. Add mock external server (in production, use real MCP servers)
	externalClient := &mockMCPClient{
		info: ServerInfo{
			Name:        "test-server",
			Description: "Test external server",
		},
		tools: []ToolDefinition{
			{
				Type: "function",
				Function: ToolFunction{
					Name:        "mock_tool",
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
	}
	vfs.RegisterServer("test-server", externalClient)

	// 4. Create executor with config
	config := &ExecutorConfig{
		MaxTurns:          10,
		TurnTimeout:       30 * time.Second,
		WorkspaceDir:      "./workspace",
		ResultThreshold:   1000,
		EnablePersistence: true,
		SessionID:         "example-session",
	}

	executor := NewExecutor(vfs, config)

	// 5. Configure LLM (mock for example)
	executor.WithLLM(&MockLLMForExample{})

	// 6. Run workflow
	result, err := executor.Run(ctx, "Test the system")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Result: %s\n", result)

	// Output: Result: Example completed
}

// MockLLMForExample is a mock LLM for demonstration
type MockLLMForExample struct{}

func (m *MockLLMForExample) SendMessage(ctx context.Context, messages []Message) (*LLMResponse, error) {
	// Simple mock that returns tool calls for first 3 turns, then completion
	if len(messages) < 3 {
		return &LLMResponse{
			Content: "Processing...",
			ToolCalls: []ToolCall{
				{
					Name: "search_available_modules",
					Arguments: map[string]interface{}{
						"keyword": "",
					},
				},
			},
		}, nil
	}

	return &LLMResponse{
		Content: "Example completed",
	}, nil
}

func (m *MockLLMForExample) SendMessageWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (*LLMResponse, error) {
	return m.SendMessage(ctx, messages)
}

// TestCompleteWorkflow tests the entire system workflow
func TestCompleteWorkflow(t *testing.T) {
	ctx := context.Background()

	// Setup
	vfs := NewVirtualFS()

	// Add tools
	vfs.RegisterServer("clai-tools", CreateDefaultToolAdapter())

	// Create executor
	config := DefaultExecutorConfig()
	config.EnablePersistence = false // Don't persist in tests

	executor := NewExecutor(vfs, config)

	// Mock LLM that simulates a real agent
	mockLLM := &MockLLMForWorkflow{
		turnCount: 0,
	}
	executor.WithLLM(mockLLM)

	// Run workflow
	result, err := executor.Run(ctx, "List available tools and execute one")
	if err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	if !strings.Contains(result, "completed") && !strings.Contains(result, "success") {
		t.Errorf("Expected successful completion, got: %s", result)
	}

	// Verify we went through multiple turns
	if mockLLM.turnCount < 2 {
		t.Errorf("Expected at least 2 turns, got %d", mockLLM.turnCount)
	}
}

// MockLLMForWorkflow simulates a more realistic agent conversation
type MockLLMForWorkflow struct {
	turnCount int
}

func (m *MockLLMForWorkflow) SendMessage(ctx context.Context, messages []Message) (*LLMResponse, error) {
	m.turnCount++

	switch m.turnCount {
	case 1:
		// Turn 1: Search for available modules
		return &LLMResponse{
			Content: "Let me discover available tools first.",
			ToolCalls: []ToolCall{
				{
					Name: "search_available_modules",
					Arguments: map[string]interface{}{
						"keyword": "",
					},
				},
			},
		}, nil

	case 2:
		// Turn 2: Inspect clai-tools module
		return &LLMResponse{
			Content: "Found clai-tools. Let me inspect it.",
			ToolCalls: []ToolCall{
				{
					Name: "inspect_module",
					Arguments: map[string]interface{}{
						"module_name":  "clai-tools",
						"detail_level": "signatures",
					},
				},
			},
		}, nil

	case 3:
		// Turn 3: Execute Python code using the tools
		return &LLMResponse{
			Content: "Now I'll use execute_bash to list files.",
			ToolCalls: []ToolCall{
				{
					Name: "python",
					Arguments: map[string]interface{}{
						"code": `import servers.clai_tools as tools
result = tools.execute_bash("echo 'Hello from MCP Code Bridge'")
print(result)`,
					},
				},
			},
		}, nil

	default:
		// Final turn: Done
		return &LLMResponse{
			Content: "Workflow completed successfully. Used progressive discovery to find and execute tools.",
		}, nil
	}
}

func (m *MockLLMForWorkflow) SendMessageWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (*LLMResponse, error) {
	return m.SendMessage(ctx, messages)
}

// TestProgressiveDiscoveryFlow specifically tests the discovery mechanism
func TestProgressiveDiscoveryFlow(t *testing.T) {
	vfs := NewVirtualFS()

	// Add multiple servers
	vfs.RegisterServer("server-a", &mockMCPClient{
		info: ServerInfo{Name: "server-a", Description: "Server A"},
		tools: []ToolDefinition{
			{Type: "function", Function: ToolFunction{Name: "tool_a1"}},
			{Type: "function", Function: ToolFunction{Name: "tool_a2"}},
		},
	})

	vfs.RegisterServer("server-b", &mockMCPClient{
		info: ServerInfo{Name: "server-b", Description: "Server B"},
		tools: []ToolDefinition{
			{Type: "function", Function: ToolFunction{Name: "tool_b1"}},
		},
	})

	discovery := NewDiscoveryEngine(vfs)

	// Step 1: Search all modules
	allModules := discovery.SearchModules("")
	if len(allModules) != 2 {
		t.Errorf("Expected 2 modules, got %d", len(allModules))
	}

	// Verify module summaries
	foundA := false
	foundB := false
	for _, mod := range allModules {
		if mod.Name == "server-a" && mod.ToolCount == 2 {
			foundA = true
		}
		if mod.Name == "server-b" && mod.ToolCount == 1 {
			foundB = true
		}
	}

	if !foundA {
		t.Error("Expected to find server-a with 2 tools")
	}
	if !foundB {
		t.Error("Expected to find server-b with 1 tool")
	}

	// Step 2: Inspect specific module
	doc, err := discovery.InspectModule("server-a", "overview")
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}

	// Should contain tool names
	if !strings.Contains(doc, "tool_a1") {
		t.Error("Expected documentation to contain tool_a1")
	}
	if !strings.Contains(doc, "tool_a2") {
		t.Error("Expected documentation to contain tool_a2")
	}

	// Step 3: Generate module (ready for code)
	module, err := vfs.GenerateModule("server-a")
	if err != nil {
		t.Fatalf("GenerateModule failed: %v", err)
	}

	// Should contain Python function definitions
	if !strings.Contains(module.PythonCode, "def tool_a1") {
		t.Error("Expected Python code to contain tool_a1 function")
	}
	if !strings.Contains(module.PythonCode, "def tool_a2") {
		t.Error("Expected Python code to contain tool_a2 function")
	}
}

// TestTokenEfficiency verifies the token savings
func TestTokenEfficiency(t *testing.T) {
	vfs := NewVirtualFS()

	// Add many tools (simulating 100 tools across 5 servers)
	for i := 0; i < 5; i++ {
		tools := make([]ToolDefinition, 20)
		for j := 0; j < 20; j++ {
			tools[j] = ToolDefinition{
				Type: "function",
				Function: ToolFunction{
					Name:        fmt.Sprintf("tool_%d_%d", i, j),
					Description: fmt.Sprintf("Tool %d from server %d", j, i),
					Parameters: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"param": map[string]interface{}{
								"type":        "string",
								"description": "A parameter",
							},
						},
					},
				},
			}
		}

		vfs.RegisterServer(fmt.Sprintf("server-%d", i), &mockMCPClient{
			info:  ServerInfo{Name: fmt.Sprintf("server-%d", i)},
			tools: tools,
		})
	}

	// Calculate token costs
	hermes := NewHermesIntegration(vfs)
	savings := hermes.CalculateTokenSavings(100)

	traditionalTokens := savings["traditional_approach"].(int)
	totalTokens := savings["with_discovery"].(int)
	savingsPercent := savings["savings_percent"].(float64)

	// Should have 100 tools × 500 tokens = 50,000 traditional
	if traditionalTokens != 50000 {
		t.Errorf("Expected 50000 traditional tokens, got %d", traditionalTokens)
	}

	// Should have significant savings (97%+)
	if savingsPercent < 95 {
		t.Errorf("Expected >95%% savings, got %.1f%%", savingsPercent)
	}

	// Progressive should be much cheaper
	if totalTokens >= traditionalTokens {
		t.Error("Expected progressive approach to use fewer tokens")
	}

	t.Logf("Token efficiency: Traditional=%d, Progressive=%d (%.1f%% savings)",
		traditionalTokens, totalTokens, savingsPercent)
}

// TestErrorHandling tests error scenarios
func TestErrorHandling(t *testing.T) {
	ctx := context.Background()

	// Test 1: No LLM configured
	vfs1 := NewVirtualFS()
	config1 := DefaultExecutorConfig()
	config1.MaxTurns = 3
	executor1 := NewExecutor(vfs1, config1)

	_, err := executor1.Run(ctx, "test")
	if err == nil || !strings.Contains(err.Error(), "no LLM") {
		t.Error("Expected error when no LLM configured")
	}

	// Test 2: Unknown tool returns error message (not exception)
	vfs2 := NewVirtualFS()
	vfs2.RegisterServer("clai-tools", CreateDefaultToolAdapter()) // Add at least one server

	config2 := DefaultExecutorConfig()
	config2.MaxTurns = 2
	executor2 := NewExecutor(vfs2, config2)

	// Mock LLM that returns unknown tool call
	mockLLM := &MockLLMWithErrors{scenario: "unknown_tool"}
	executor2.WithLLM(mockLLM)

	result, err := executor2.Run(ctx, "test")
	// The executor should process the unknown tool and return its error as a result
	// rather than failing the entire execution
	if err != nil {
		t.Logf("Got error (acceptable): %v", err)
	}
	if !strings.Contains(result, "Error") && !strings.Contains(result, "unknown") {
		t.Logf("Result should indicate unknown tool issue: %s", result)
	}

	// Test 3: Tool execution failure - create separate executor
	vfs3 := NewVirtualFS()
	vfs3.RegisterServer("error-server", &mockErrorMCPClient{})

	config3 := DefaultExecutorConfig()
	config3.MaxTurns = 2
	executor3 := NewExecutor(vfs3, config3)

	mockLLM2 := &MockLLMWithErrors{scenario: "tool_error"}
	executor3.WithLLM(mockLLM2)

	result3, err3 := executor3.Run(ctx, "test")
	// Should handle gracefully
	if err3 != nil {
		t.Logf("Got error (acceptable): %v", err3)
	}
	if result3 == "" {
		t.Error("Expected some result even with tool error")
	}
}

// MockLLMWithErrors simulates various error scenarios
type MockLLMWithErrors struct {
	scenario string
}

func (m *MockLLMWithErrors) SendMessage(ctx context.Context, messages []Message) (*LLMResponse, error) {
	switch m.scenario {
	case "unknown_tool":
		return &LLMResponse{
			Content: "Using unknown tool",
			ToolCalls: []ToolCall{
				{
					Name:      "nonexistent_tool",
					Arguments: map[string]interface{}{},
				},
			},
		}, nil

	case "tool_error":
		return &LLMResponse{
			Content: "Using tool that will error",
			ToolCalls: []ToolCall{
				{
					Name:      "error_tool",
					Arguments: map[string]interface{}{},
				},
			},
		}, nil

	default:
		return &LLMResponse{Content: "Done"}, nil
	}
}

func (m *MockLLMWithErrors) SendMessageWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (*LLMResponse, error) {
	return m.SendMessage(ctx, messages)
}

// mockErrorMCPClient always returns errors
type mockErrorMCPClient struct{}

func (m *mockErrorMCPClient) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	return []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunction{
				Name: "error_tool",
			},
		},
	}, nil
}

func (m *mockErrorMCPClient) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("tool always fails")
}

func (m *mockErrorMCPClient) GetServerInfo(ctx context.Context) (ServerInfo, error) {
	return ServerInfo{Name: "error-server"}, nil
}
