package ui

import (
	"encoding/json"
	"log"
	"net"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const SocketPath = "/tmp/clai.sock"

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

	log.Printf("[DEBUG] Server listening on %s", SocketPath)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("[DEBUG] Accept error: %v", err)
				continue
			}

			go handleConnection(conn, p)
		}
	}()

	return nil
}

func handleConnection(conn net.Conn, p *tea.Program) {
	decoder := json.NewDecoder(conn)
	var cmd DebugCommand

	if err := decoder.Decode(&cmd); err != nil {
		log.Printf("[DEBUG] Decode error: %v", err)
		sendResponse(conn, DebugResponse{
			Success: false,
			Error:   err.Error(),
		})
		conn.Close()
		return
	}

	log.Printf("[DEBUG] Received command: %s", cmd.Command)

	p.Send(DebugServerMsg{
		Conn: conn,
		Cmd:  cmd,
	})
}

func sendResponse(conn net.Conn, resp DebugResponse) {
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(resp); err != nil {
		log.Printf("[DEBUG] Encode error: %v", err)
	}
}

func SendDebugResponse(conn net.Conn, resp DebugResponse) {
	sendResponse(conn, resp)
}
