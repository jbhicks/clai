package debug

import (
	"encoding/json"
	"fmt"
	"net"
)

const ClientSocketPath = "/tmp/clai.sock"

type ClientResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

func SendCommand(command string, args map[string]interface{}) (*ClientResponse, error) {
	conn, err := net.Dial("unix", "/tmp/clai.sock")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", "/tmp/clai.sock", err)
	}
	defer conn.Close()

	cmd := map[string]interface{}{
		"command": command,
	}
	if args != nil {
		cmd["args"] = args
	}

	cmdJSON, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	cmdJSON = append(cmdJSON, '\n')
	if _, err := conn.Write(cmdJSON); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	buf := make([]byte, 1024*1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp ClientResponse
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}
