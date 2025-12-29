package benchmark

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestBroadcastServerUpdate(t *testing.T) {
	server := &Server{
		sseClients:   make(map[chan string]bool),
		modelManager: NewModelManager(),
	}

	// Create mock SSE clients
	client1 := make(chan string, 10)
	client2 := make(chan string, 10)

	server.sseClients[client1] = true
	server.sseClients[client2] = true

	// Broadcast update
	server.broadcastServerUpdate()

	// Verify both clients received the message
	select {
	case msg := <-client1:
		if msg != "refresh-servers" {
			t.Errorf("Client 1 received %q, want %q", msg, "refresh-servers")
		}
	case <-time.After(1 * time.Second):
		t.Error("Client 1 did not receive message")
	}

	select {
	case msg := <-client2:
		if msg != "refresh-servers" {
			t.Errorf("Client 2 received %q, want %q", msg, "refresh-servers")
		}
	case <-time.After(1 * time.Second):
		t.Error("Client 2 did not receive message")
	}
}

func TestBroadcastServerUpdate_NoClients(t *testing.T) {
	server := &Server{
		sseClients:   make(map[chan string]bool),
		modelManager: NewModelManager(),
	}

	// Should not panic when no clients connected
	server.broadcastServerUpdate()
}

func TestBroadcastServerUpdate_FullBuffer(t *testing.T) {
	server := &Server{
		sseClients:   make(map[chan string]bool),
		modelManager: NewModelManager(),
	}

	// Create client with no buffer (will block immediately)
	client := make(chan string)
	server.sseClients[client] = true

	// Broadcast should not block even if client buffer is full
	done := make(chan bool)
	go func() {
		server.broadcastServerUpdate()
		done <- true
	}()

	select {
	case <-done:
		// Success - broadcast didn't block
	case <-time.After(1 * time.Second):
		t.Error("broadcastServerUpdate() blocked on full client buffer")
	}
}

func TestSSEConcurrentBroadcast(t *testing.T) {
	server := &Server{
		sseClients:   make(map[chan string]bool),
		modelManager: NewModelManager(),
	}

	// Add multiple clients
	numClients := 10
	clients := make([]chan string, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = make(chan string, 10)
		server.sseClients[clients[i]] = true
	}

	// Broadcast multiple times concurrently
	numBroadcasts := 5
	var wg sync.WaitGroup
	for i := 0; i < numBroadcasts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			server.broadcastServerUpdate()
		}()
	}

	wg.Wait()

	// Verify all clients received all messages
	for i, client := range clients {
		received := 0
		timeout := time.After(2 * time.Second)

		for received < numBroadcasts {
			select {
			case msg := <-client:
				if msg != "refresh-servers" {
					t.Errorf("Client %d received unexpected message: %q", i, msg)
				}
				received++
			case <-timeout:
				t.Errorf("Client %d received %d messages, want %d", i, received, numBroadcasts)
				return
			}
		}
	}
}

func TestSSEClientConnection(t *testing.T) {
	server := &Server{
		sseClients:   make(map[chan string]bool),
		modelManager: NewModelManager(),
	}

	// Create test HTTP server
	handler := http.HandlerFunc(server.handleServerEvents)
	testServer := httptest.NewServer(handler)
	defer testServer.Close()

	// Use a context we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to SSE endpoint in goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)

		req, err := http.NewRequest("GET", testServer.URL, nil)
		if err != nil {
			t.Errorf("Failed to create request: %v", err)
			return
		}
		req.Header.Set("Accept", "text/event-stream")
		req = req.WithContext(ctx)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil && ctx.Err() == nil {
			// Error is only a problem if context wasn't canceled
			t.Errorf("Failed to connect: %v", err)
			return
		}
		if resp != nil {
			defer resp.Body.Close()
			// Try to read a bit to keep connection alive
			buf := make([]byte, 1024)
			resp.Body.Read(buf)
		}
	}()

	// Give connection time to establish
	time.Sleep(200 * time.Millisecond)

	// Verify client was registered
	server.sseClientsMu.Lock()
	clientCount := len(server.sseClients)
	server.sseClientsMu.Unlock()

	if clientCount != 1 {
		t.Errorf("Expected 1 SSE client, got %d", clientCount)
	}

	// Cancel context to close connection
	cancel()

	// Wait for goroutine to finish
	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for connection to close")
	}

	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)

	server.sseClientsMu.Lock()
	clientCount = len(server.sseClients)
	server.sseClientsMu.Unlock()

	if clientCount != 0 {
		t.Errorf("Expected 0 SSE clients after disconnect, got %d", clientCount)
	}
}
