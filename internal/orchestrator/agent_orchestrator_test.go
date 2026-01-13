package orchestrator

import (
	"clai/internal/logger"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	logger.Init(os.Stderr)
	os.Exit(m.Run())
}

func TestAgentManager_Basic(t *testing.T) {
	manager := NewAgentManager(".")

	if manager == nil {
		t.Fatal("AgentManager should not be nil")
	}

	agents := manager.ListAgents()
	if len(agents) != 0 {
		t.Errorf("Expected 0 agents initially, got %d", len(agents))
	}
}

func TestAgentCommunicationBus(t *testing.T) {
	bus := NewAgentCommunicationBus()

	bus.Broadcast("Test message", "agent-1")

	err := bus.Send("agent-2", "Direct message", "agent-1")
	if err != nil {
		t.Errorf("Failed to send message: %v", err)
	}

	messages := bus.GetMessages("agent-2", time.Now().Add(-1*time.Hour))
	found := false
	for _, msg := range messages {
		if msg.Content == "Direct message" && msg.To == "agent-2" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find direct message to agent-2")
	}

	broadcastMsgs := bus.GetMessages("", time.Now().Add(-1*time.Hour))
	found = false
	for _, msg := range broadcastMsgs {
		if msg.Content == "Test message" && msg.To == "" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find broadcast message")
	}
}
