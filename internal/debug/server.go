package debug

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const ServerSocketPath = "/tmp/clai.sock"

type ServerResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

type Command struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

type Server struct {
	listener net.Listener
	program  *tea.Program
	model    tea.Model
	mutex    sync.RWMutex
	running  bool
}

func (s *Server) GetModel() tea.Model {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.model
}

func (s *Server) SetModel(model tea.Model) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.model = model
}

func NewServer(program *tea.Program) *Server {
	return &Server{
		program: program,
		running: false,
	}
}
func (s *Server) Start() error {
	if _, err := os.Stat(ServerSocketPath); err == nil {
		os.Remove(ServerSocketPath)
	}

	listener, err := net.Listen("unix", ServerSocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", ServerSocketPath, err)
	}

	s.listener = listener
	s.running = true
	os.Chmod(ServerSocketPath, 0777)

	log.Printf("[DEBUG] Server listening on %s", ServerSocketPath)

	go s.handleConnections()
	return nil
}

func (s *Server) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
}

func (s *Server) handleConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				log.Printf("Error accepting connection: %v", err)
			}
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		request := scanner.Text()
		response := s.handleRequest(request)

		responseJSON, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response: %v", err)
			continue
		}

		responseJSON = append(responseJSON, '\n')
		if _, err := conn.Write(responseJSON); err != nil {
			log.Printf("Error writing response: %v", err)
			break
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from connection: %v", err)
	}
}

func (s *Server) handleRequest(requestJSON string) *ServerResponse {
	var cmd Command
	if err := json.Unmarshal([]byte(requestJSON), &cmd); err != nil {
		return &ServerResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid JSON: %v", err),
		}
	}

	switch cmd.Command {
	case "ping":
		return s.handlePing()
	case "inspect":
		return s.handleInspect(cmd.Args)
	case "inspect_styles":
		return s.handleInspectStyles(cmd.Args)
	case "get_history":
		return s.handleGetHistory(cmd.Args)
	case "send_key":
		return s.handleSendKey(cmd.Args)
	case "type_text":
		return s.handleTypeText(cmd.Args)
	case "send_message":
		return s.handleSendMessage(cmd.Args)
	case "switch_pane":
		return s.handleSwitchPane(cmd.Args)
	default:
		return &ServerResponse{
			Success: false,
			Error:   fmt.Sprintf("Unknown command: %s", cmd.Command),
		}
	}
}

func (s *Server) handlePing() *ServerResponse {
	return &ServerResponse{
		Success: true,
		Data:    map[string]interface{}{"message": "pong"},
	}
}

func (s *Server) handleInspect(args map[string]interface{}) *ServerResponse {
	model := s.GetModel()
	data := map[string]interface{}{
		"timestamp":      time.Now().Format(time.RFC3339),
		"server_version": "1.0.0",
	}

	modelValue := reflect.ValueOf(model)

	if method := modelValue.MethodByName("getWidth"); method.IsValid() {
		if result := method.Call(nil); len(result) > 0 {
			data["width"] = result[0].Interface()
		}
	}

	if method := modelValue.MethodByName("getHeight"); method.IsValid() {
		if result := method.Call(nil); len(result) > 0 {
			data["height"] = result[0].Interface()
		}
	}

	if method := modelValue.MethodByName("getLogs"); method.IsValid() {
		if result := method.Call(nil); len(result) > 0 {
			logs := result[0].Interface()
			data["logs"] = logs
			if logsSlice, ok := logs.([]string); ok {
				data["logs_count"] = len(logsSlice)
			}
		}
	}

	if method := modelValue.MethodByName("getViewportContent"); method.IsValid() {
		if result := method.Call(nil); len(result) > 0 {
			data["viewport_content"] = result[0].Interface()
		}
	} else {
		data["viewport_content"] = "Debug server connected and ready"
		data["type_assertion_failed"] = true
		data["model_type"] = fmt.Sprintf("%T", model)
	}

	return &ServerResponse{
		Success: true,
		Data:    data,
	}
}

func (s *Server) handleInspectStyles(args map[string]interface{}) *ServerResponse {
	data := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"layout": map[string]interface{}{
			"header_height":  1,
			"footer_height":  1,
			"content_height": s.getModelHeight() - 2,
		},
	}

	return &ServerResponse{
		Success: true,
		Data:    data,
	}
}

func (s *Server) handleGetHistory(args map[string]interface{}) *ServerResponse {
	data := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": "Debug server started",
				"time":    time.Now().Format(time.RFC3339),
			},
		},
		"count": 1,
	}

	return &ServerResponse{
		Success: true,
		Data:    data,
	}
}

func (s *Server) handleSendKey(args map[string]interface{}) *ServerResponse {
	key, ok := args["key"].(string)
	if !ok {
		return &ServerResponse{
			Success: false,
			Error:   "Missing 'key' parameter",
		}
	}

	if s.program != nil {
		s.program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	}

	return &ServerResponse{
		Success: true,
		Data:    map[string]interface{}{"key_sent": key},
	}
}

func (s *Server) handleTypeText(args map[string]interface{}) *ServerResponse {
	text, ok := args["text"].(string)
	if !ok {
		return &ServerResponse{
			Success: false,
			Error:   "Missing 'text' parameter",
		}
	}

	if s.program != nil {
		for _, r := range text {
			s.program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}

	return &ServerResponse{
		Success: true,
		Data:    map[string]interface{}{"text_typed": text},
	}
}

func (s *Server) handleSendMessage(args map[string]interface{}) *ServerResponse {
	role, roleOk := args["role"].(string)
	content, contentOk := args["content"].(string)

	if !roleOk || !contentOk {
		return &ServerResponse{
			Success: false,
			Error:   "Missing 'role' or 'content' parameter",
		}
	}

	data := map[string]interface{}{
		"role_sent":     role,
		"content_sent":  content,
		"message_count": 1,
	}

	return &ServerResponse{
		Success: true,
		Data:    data,
	}
}

func (s *Server) handleSwitchPane(args map[string]interface{}) *ServerResponse {
	if s.program != nil {
		s.program.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	}

	return &ServerResponse{
		Success: true,
		Data:    map[string]interface{}{"action": "pane_switched"},
	}
}

func (s *Server) getModelHeight() int {
	model := s.GetModel()
	modelValue := reflect.ValueOf(model)
	if method := modelValue.MethodByName("getHeight"); method.IsValid() {
		if result := method.Call(nil); len(result) > 0 {
			if height, ok := result[0].Interface().(int); ok {
				return height
			}
		}
	}
	return 24
}
