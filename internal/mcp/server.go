package mcp

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/jbhicks/clai/internal/debug"
)

// JSON-RPC 2.0 request/response structures
type JSONRPCRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

type JSONRPCResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP protocol structures
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]interface{} `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema,omitempty"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type Server struct {
	socketPath string
}

func NewServer(socketPath string) *Server {
	return &Server{
		socketPath: socketPath,
	}
}

func (s *Server) HandleRequest(requestJSON string) string {
	var request JSONRPCRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		log.Printf("Failed to parse JSON-RPC request: %v", err)
		response := s.createErrorResponse(nil, -32700, "Parse error")
		responseJSON, _ := json.Marshal(response)
		return string(responseJSON)
	}

	response := s.processRequest(request)
	responseJSON, err := json.Marshal(response)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		errorResp := s.createErrorResponse(request.ID, -32603, "Internal error")
		errorRespJSON, _ := json.Marshal(errorResp)
		return string(errorRespJSON)
	}

	return string(responseJSON)
}

func (s *Server) processRequest(request JSONRPCRequest) JSONRPCResponse {
	switch request.Method {
	case "initialize":
		return s.handleInitialize(request.ID)
	case "tools/list":
		return s.handleListTools(request.ID)
	case "tools/call":
		return s.handleCallTool(request.ID, request.Params)
	default:
		return s.createErrorResponse(request.ID, -32601, fmt.Sprintf("Method not found: %s", request.Method))
	}
}

func (s *Server) handleInitialize(id interface{}) JSONRPCResponse {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		ServerInfo: ServerInfo{
			Name:    "clai-debug",
			Version: "1.0.0",
		},
	}

	return JSONRPCResponse{
		Jsonrpc: "2.0",
		Result:  result,
		ID:      id,
	}
}

func (s *Server) handleListTools(id interface{}) JSONRPCResponse {
	tools := []Tool{
		{
			Name:        "clai-debug_ping",
			Description: "Test connectivity to CLAI debug server",
		},
		{
			Name:        "clai-debug_inspect",
			Description: "Capture current TUI state and viewport via debug MCP",
		},
		{
			Name:        "clai-debug_inspect_styles",
			Description: "Return structured UI pane size/layout data",
		},
		{
			Name:        "clai-debug_get_history",
			Description: "Return recent chat/log message history",
		},
		{
			Name:        "clai-debug_send_key",
			Description: "Send a keystroke to the TUI",
		},
		{
			Name:        "clai-debug_type_text",
			Description: "Type text into the input field",
		},
		{
			Name:        "clai-debug_send_message",
			Description: "Send a message to the conversation",
		},
		{
			Name:        "clai-debug_switch_pane",
			Description: "Switch between chat and log panes",
		},
	}

	result := ListToolsResult{
		Tools: tools,
	}

	return JSONRPCResponse{
		Jsonrpc: "2.0",
		Result:  result,
		ID:      id,
	}
}

func (s *Server) handleCallTool(id interface{}, params interface{}) JSONRPCResponse {
	// Parse params
	var callParams CallToolParams
	paramsBytes, err := json.Marshal(params)
	if err != nil {
		return s.createErrorResponse(id, -32602, "Invalid params")
	}

	if err := json.Unmarshal(paramsBytes, &callParams); err != nil {
		return s.createErrorResponse(id, -32602, "Invalid params")
	}

	// Call debug server
	debugResp, err := s.callDebugServer(callParams.Name, callParams.Arguments)
	if err != nil {
		return s.createErrorResponse(id, -32603, fmt.Sprintf("Internal error: %v", err))
	}

	if !debugResp.Success {
		return s.createErrorResponse(id, -32603, fmt.Sprintf("Debug command failed: %s", debugResp.Error))
	}

	return JSONRPCResponse{
		Jsonrpc: "2.0",
		Result:  debugResp.Data,
		ID:      id,
	}
}

func (s *Server) callDebugServer(toolName string, args map[string]interface{}) (*debug.ClientResponse, error) {
	// Map MCP tool names to debug server commands
	command := s.mapToolToCommand(toolName)
	if command == "" {
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}

	return debug.SendCommand(command, args)
}

func (s *Server) mapToolToCommand(toolName string) string {
	mapping := map[string]string{
		"clai-debug_ping":           "ping",
		"clai-debug_inspect":        "inspect",
		"clai-debug_inspect_styles": "inspect_styles",
		"clai-debug_get_history":    "get_history",
		"clai-debug_send_key":       "send_key",
		"clai-debug_type_text":      "type_text",
		"clai-debug_send_message":   "send_message",
		"clai-debug_switch_pane":    "switch_pane",
	}

	return mapping[toolName]
}

func (s *Server) createErrorResponse(id interface{}, code int, message string) JSONRPCResponse {
	return JSONRPCResponse{
		Jsonrpc: "2.0",
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
}
