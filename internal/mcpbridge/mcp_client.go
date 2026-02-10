package mcpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

// InternalToolAdapter wraps existing clai tools as an MCP server.
//
// This allows the existing tool system (execute_code, execute_bash, etc.)
// to be exposed through the MCP Code Bridge virtual filesystem.
//
// Example:
//
//	// Agent writes Python:
//	import servers.clai_tools as tools
//	result = tools.execute_bash("ls -la")
//
//	// Gets translated to:
//	ToolCall{Name: "execute_bash", Arguments: {"command": "ls -la"}}
//
//	// And executed via existing tools.ExecuteTool()
type InternalToolAdapter struct {
	tools        []ToolDefinition
	toolExecutor func(toolCall ToolCall) (string, error)
	name         string
	description  string
}

// NewInternalToolAdapter creates an adapter for existing tools
//
// toolExecutor is a function that executes a tool call (e.g., tools.ExecuteTool)
func NewInternalToolAdapter(
	name string,
	description string,
	tools []ToolDefinition,
	toolExecutor func(toolCall ToolCall) (string, error),
) *InternalToolAdapter {
	return &InternalToolAdapter{
		name:         name,
		description:  description,
		tools:        tools,
		toolExecutor: toolExecutor,
	}
}

// ListTools returns all available tools
func (a *InternalToolAdapter) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	return a.tools, nil
}

// CallTool executes a specific tool
func (a *InternalToolAdapter) CallTool(
	ctx context.Context,
	toolName string,
	arguments map[string]interface{},
) (interface{}, error) {
	// Find the tool definition
	var toolDef *ToolDefinition
	for _, tool := range a.tools {
		if tool.Function.Name == toolName {
			toolDef = &tool
			break
		}
	}

	if toolDef == nil {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	// Create tool call
	toolCall := ToolCall{
		Name:      toolName,
		Arguments: arguments,
	}

	// Execute via existing tool system
	result, err := a.toolExecutor(toolCall)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	// Try to parse result as JSON, otherwise return as string
	var parsedResult interface{}
	if err := json.Unmarshal([]byte(result), &parsedResult); err == nil {
		return parsedResult, nil
	}

	return map[string]string{"output": result}, nil
}

// GetServerInfo returns metadata about this tool server
func (a *InternalToolAdapter) GetServerInfo(ctx context.Context) (ServerInfo, error) {
	return ServerInfo{
		Name:        a.name,
		Description: a.description,
		Version:     "1.0.0",
		Categories:  []string{"internal", "core"},
	}, nil
}

// CreateDefaultToolAdapter creates an adapter with clai's default tools
//
// This exposes execute_code, execute_bash, execute_python, execute_javascript
func CreateDefaultToolAdapter() *InternalToolAdapter {
	tools := []ToolDefinition{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "execute_code",
				Description: "Execute code in Python, Bash, or JavaScript. General-purpose tool for file operations, shell commands, data processing.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"language": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"python", "bash", "javascript"},
							"description": "Programming language",
						},
						"code": map[string]interface{}{
							"type":        "string",
							"description": "Code to execute",
						},
						"purpose": map[string]interface{}{
							"type":        "string",
							"description": "What this code does",
						},
					},
					"required": []string{"language", "code"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "execute_bash",
				Description: "Execute bash/shell commands for system operations and file manipulation.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Bash command to execute",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "execute_python",
				Description: "Execute Python code for scripting, data processing, and computations.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{
							"type":        "string",
							"description": "Python code to execute",
						},
					},
					"required": []string{"code"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "execute_javascript",
				Description: "Execute JavaScript/Node.js code for JavaScript operations.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{
							"type":        "string",
							"description": "JavaScript code to execute",
						},
					},
					"required": []string{"code"},
				},
			},
		},
	}

	return NewInternalToolAdapter(
		"clai-tools",
		"CLAI internal tools for code execution and system operations",
		tools,
		defaultToolExecutor,
	)
}

// defaultToolExecutor executes tools using the default tool system
// This is a placeholder - in production it would call tools.ExecuteTool
func defaultToolExecutor(toolCall ToolCall) (string, error) {
	// This would call the actual tools.ExecuteTool from internal/tools
	// For now, return a mock result
	return fmt.Sprintf("Executed %s with args: %v", toolCall.Name, toolCall.Arguments), nil
}

// ExternalMCPClient connects to external MCP servers via stdio or SSE
type ExternalMCPClient struct {
	name      string
	command   string   // Command to start MCP server (for stdio)
	args      []string // Arguments for command
	env       []string // Environment variables
	url       string   // URL for SSE connection
	transport string   // "stdio" or "sse"

	conn      *MCPConnection
	mu        sync.RWMutex
	connected bool
}

// MCPConnection represents an active MCP connection
type MCPConnection struct {
	stdin  *os.File
	stdout *os.File
	stderr *os.File
	cmd    *exec.Cmd
	mu     sync.Mutex
}

// NewExternalMCPClient creates a client for an external MCP server
//
// Example stdio server:
//
//	client := NewExternalMCPClient("google-drive", "npx", []string{"-y", "@modelcontextprotocol/server-google-drive"}, nil, "")
//
// Example SSE server:
//
//	client := NewExternalMCPClient("slack", "", nil, nil, "http://localhost:3000/sse")
func NewExternalMCPClient(
	name string,
	command string,
	args []string,
	env []string,
	url string,
) *ExternalMCPClient {
	transport := "stdio"
	if url != "" {
		transport = "sse"
	}

	return &ExternalMCPClient{
		name:      name,
		command:   command,
		args:      args,
		env:       env,
		url:       url,
		transport: transport,
	}
}

// Connect establishes connection to the MCP server
func (c *ExternalMCPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil // Already connected
	}

	switch c.transport {
	case "stdio":
		return c.connectStdio(ctx)
	case "sse":
		return c.connectSSE(ctx)
	default:
		return fmt.Errorf("unknown transport: %s", c.transport)
	}
}

// connectStdio establishes stdio connection to MCP server
func (c *ExternalMCPClient) connectStdio(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.command, c.args...)

	if len(c.env) > 0 {
		cmd.Env = append(os.Environ(), c.env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	c.conn = &MCPConnection{
		stdin:  stdin.(*os.File),
		stdout: stdout.(*os.File),
		stderr: stderr.(*os.File),
		cmd:    cmd,
	}

	c.connected = true
	return nil
}

// connectSSE establishes SSE connection to MCP server
func (c *ExternalMCPClient) connectSSE(ctx context.Context) error {
	// TODO: Implement SSE connection
	// This would use HTTP client with SSE parsing
	return fmt.Errorf("SSE transport not yet implemented")
}

// Disconnect closes the MCP connection
func (c *ExternalMCPClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return nil
	}

	if c.conn.cmd != nil {
		c.conn.cmd.Process.Kill()
		c.conn.cmd.Wait()
	}

	c.connected = false
	c.conn = nil
	return nil
}

// ListTools returns available tools from the external MCP server
func (c *ExternalMCPClient) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	// Send JSON-RPC request for tools/list
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	response, err := c.sendRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// Parse response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	toolsData, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("tools not found in response")
	}

	tools := make([]ToolDefinition, len(toolsData))
	for i, toolData := range toolsData {
		toolMap, ok := toolData.(map[string]interface{})
		if !ok {
			continue
		}

		// Convert to ToolDefinition
		tool := convertMCPToolToDefinition(toolMap)
		tools[i] = tool
	}

	return tools, nil
}

// CallTool executes a tool on the external MCP server
func (c *ExternalMCPClient) CallTool(
	ctx context.Context,
	toolName string,
	arguments map[string]interface{},
) (interface{}, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	// Send JSON-RPC request for tools/call
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	response, err := c.sendRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}

	// Parse response
	result, ok := response["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	// Extract result content
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	// Return first content item
	if contentMap, ok := content[0].(map[string]interface{}); ok {
		if text, ok := contentMap["text"].(string); ok {
			// Try to parse as JSON
			var parsed interface{}
			if err := json.Unmarshal([]byte(text), &parsed); err == nil {
				return parsed, nil
			}
			return text, nil
		}
	}

	return content, nil
}

// GetServerInfo returns server metadata
func (c *ExternalMCPClient) GetServerInfo(ctx context.Context) (ServerInfo, error) {
	return ServerInfo{
		Name:        c.name,
		Description: fmt.Sprintf("External MCP server (%s)", c.transport),
		Version:     "1.0.0",
	}, nil
}

// ensureConnected makes sure the client is connected
func (c *ExternalMCPClient) ensureConnected(ctx context.Context) error {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return c.Connect(ctx)
	}
	return nil
}

// sendRequest sends a JSON-RPC request and returns the response
func (c *ExternalMCPClient) sendRequest(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Marshal request
	requestData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request (for stdio)
	if c.transport == "stdio" {
		c.conn.mu.Lock()
		defer c.conn.mu.Unlock()

		// Write request
		if _, err := c.conn.stdin.Write(append(requestData, '\n')); err != nil {
			return nil, fmt.Errorf("failed to send request: %w", err)
		}

		// Read response (with timeout)
		responseCh := make(chan []byte, 1)
		errCh := make(chan error, 1)

		go func() {
			buf := make([]byte, 65536)
			n, err := c.conn.stdout.Read(buf)
			if err != nil {
				errCh <- err
				return
			}
			responseCh <- buf[:n]
		}()

		select {
		case responseData := <-responseCh:
			// Parse response
			var response map[string]interface{}
			if err := json.Unmarshal(responseData, &response); err != nil {
				return nil, fmt.Errorf("failed to parse response: %w", err)
			}
			return response, nil

		case err := <-errCh:
			return nil, fmt.Errorf("failed to read response: %w", err)

		case <-time.After(30 * time.Second):
			return nil, fmt.Errorf("request timeout")

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("transport %s not supported", c.transport)
}

// convertMCPToolToDefinition converts MCP tool format to our ToolDefinition
func convertMCPToolToDefinition(mcpTool map[string]interface{}) ToolDefinition {
	name, _ := mcpTool["name"].(string)
	description, _ := mcpTool["description"].(string)

	inputSchema, _ := mcpTool["inputSchema"].(map[string]interface{})
	if inputSchema == nil {
		inputSchema = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	return ToolDefinition{
		Type: "function",
		Function: ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  inputSchema,
		},
	}
}

// MCPServerRegistry manages multiple MCP server connections
type MCPServerRegistry struct {
	servers map[string]MCPClientInterface
	mu      sync.RWMutex
}

// NewMCPServerRegistry creates a new registry
func NewMCPServerRegistry() *MCPServerRegistry {
	return &MCPServerRegistry{
		servers: make(map[string]MCPClientInterface),
	}
}

// Register adds an MCP server to the registry
func (r *MCPServerRegistry) Register(name string, client MCPClientInterface) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers[name] = client
}

// Unregister removes an MCP server
func (r *MCPServerRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.servers, name)
}

// Get retrieves an MCP server by name
func (r *MCPServerRegistry) Get(name string) (MCPClientInterface, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	client, ok := r.servers[name]
	return client, ok
}

// List returns all registered server names
func (r *MCPServerRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.servers))
	for name := range r.servers {
		names = append(names, name)
	}
	return names
}

// ConnectAll connects to all registered servers
func (r *MCPServerRegistry) ConnectAll(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, client := range r.servers {
		if externalClient, ok := client.(*ExternalMCPClient); ok {
			if err := externalClient.Connect(ctx); err != nil {
				return fmt.Errorf("failed to connect to %s: %w", name, err)
			}
		}
	}

	return nil
}

// DisconnectAll disconnects from all servers
func (r *MCPServerRegistry) DisconnectAll() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var errs []error
	for name, client := range r.servers {
		if externalClient, ok := client.(*ExternalMCPClient); ok {
			if err := externalClient.Disconnect(); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("disconnect errors: %v", errs)
	}
	return nil
}
