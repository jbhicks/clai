package ui

import (
	"clai/internal/logger"
	"encoding/json"
	"net"
	"os"
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

func StartDebugServer(debugChan chan DebugServerMsg) error {
	logger.Info("[DEBUG] Attempting to start debug server on %s", SocketPath)
	if _, err := os.Stat(SocketPath); err == nil {
		logger.Info("[DEBUG] Removing existing socket file")
		os.Remove(SocketPath)
	}

	listener, err := net.Listen("unix", SocketPath)
	if err != nil {
		logger.Error("[DEBUG] Failed to listen on unix socket: %v", err)
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

			go handleConnection(conn, debugChan)
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

func handleConnection(conn net.Conn, debugChan chan DebugServerMsg) {
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

	debugChan <- DebugServerMsg{
		Conn: conn,
		Cmd:  cmd,
	}
}

func sendResponse(conn net.Conn, resp DebugResponse) {
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(resp); err != nil {
		logger.Debug("[DEBUG] Encode error: %v", err)
	}
}

func SendDebugResponse(conn net.Conn, resp DebugResponse) {
	logger.Info("[DEBUG] Sending response: success=%v, error=%s", resp.Success, resp.Error)
	sendResponse(conn, resp)
}
