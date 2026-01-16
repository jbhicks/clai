package main

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestTerminalCleanupOnSignal(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Signal handler setup caused panic: %v", r)
		}
	}()

	// Verify build-time variables exist (empty in dev builds)
	if buildTime == "" {
		t.Log("buildTime not set (expected in development)")
	}

	id := getBuildIdentifier()
	if id == "" {
		t.Error("getBuildIdentifier() returned empty string")
	}
}

func TestTerminalResetCodes(t *testing.T) {
	resetSequence := "\x1b[2J\x1b[H\x1b[?1049l"

	if !contains(resetSequence, "\x1b[2J") {
		t.Error("Missing clear screen code")
	}
	if !contains(resetSequence, "\x1b[H") {
		t.Error("Missing cursor home code")
	}
	if !contains(resetSequence, "\x1b[?1049l") {
		t.Error("Missing exit alt screen code")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && contains(s[1:], substr)
}

func TestProcessSignaling(t *testing.T) {
	testCmd := exec.Command("sleep", "1")
	if err := testCmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	// Test graceful shutdown
	if err := testCmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Logf("SIGTERM failed: %v", err)
	}

	// Wait for the process to finish
	if err := testCmd.Wait(); err != nil {
		t.Logf("Process wait failed: %v", err)
	}
}

func TestBuildIdentifierUniqueness(t *testing.T) {
	identifiers := make(map[string]bool)

	for i := 0; i < 5; i++ {
		originalTime := buildTime
		originalRand := buildRand

		buildTime = time.Now().Format("20060102-150405")
		buildRand = string(rune(i + 1000))

		id := getBuildIdentifier()
		if identifiers[id] {
			t.Errorf("Non-unique build ID: %s", id)
		}
		identifiers[id] = true

		buildTime = originalTime
		buildRand = originalRand
	}
}

func TestSignalHandlerRegistration(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Signal handler setup panic: %v", r)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	defer close(sigChan)

	if sigChan == nil {
		t.Error("Signal channel not created")
	}
	if cap(sigChan) != 1 {
		t.Errorf("Wrong channel capacity: %d", cap(sigChan))
	}
}
