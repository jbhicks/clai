package ui

import (
	"clai/internal/logger"
	"encoding/json"
	"net"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const SocketPath = "/tmp/clai.sock"

var debugListener net.Listener

type DebugCommand struct {
	Command string                 `json:"command"`
	Args    map[string]interface{} `json:"args,omitempty"`
}

type DebugResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

type DebugServerMsg struct {
	Conn net.Conn
	Cmd  DebugCommand
}

func StartDebugServer(p *tea.Program) error {
	os.Remove(SocketPath)

	listener, err := net.Listen("unix", SocketPath)
	if err != nil {
		return err
	}

	debugListener = listener
	logger.Info("[DEBUG] Server listening on %s", SocketPath)

	go func() {
		logger.Info("[DEBUG] Debug server goroutine started")
		for {
			logger.Debug("[DEBUG] Waiting for connection...")
			conn, err := listener.Accept()
			if err != nil {
				logger.Info("[DEBUG] Accept error (listener closed): %v", err)
				return
			}
			logger.Info("[DEBUG] Accepted connection from client")

			go handleConnection(conn, p)
		}
	}()

	return nil
}

func StopDebugServer() {
	if debugListener != nil {
		debugListener.Close()
		os.Remove(SocketPath)
	}
}

func handleConnection(conn net.Conn, p *tea.Program) {
	logger.Info("[DEBUG] Handling new connection")
	decoder := json.NewDecoder(conn)
	var cmd DebugCommand

	if err := decoder.Decode(&cmd); err != nil {
		logger.Info("[DEBUG] Decode error: %v", err)
		sendResponse(conn, DebugResponse{
			Success: false,
			Error:   err.Error(),
		})
		conn.Close()
		return
	}

	logger.Info("[DEBUG] Received command: %s", cmd.Command)

	p.Send(DebugServerMsg{
		Conn: conn,
		Cmd:  cmd,
	})
}

func sendResponse(conn net.Conn, resp DebugResponse) {
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(resp); err != nil {
		logger.Debug("[DEBUG] Encode error: %v", err)
	}
}

func SendDebugResponse(conn net.Conn, resp DebugResponse) {
	sendResponse(conn, resp)
}
