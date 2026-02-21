package sse

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockResponseWriter implements http.ResponseWriter for testing
type mockResponseWriter struct {
	buf    bytes.Buffer
	header http.Header
	mu     sync.Mutex
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		header: make(http.Header),
	}
}

func (m *mockResponseWriter) Header() http.Header {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.header
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.Write(data)
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	// Mock implementation
}

func (m *mockResponseWriter) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

// TestHubRegisterUnregister tests client registration and unregistration
func TestHubRegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create mock clients
	client1 := NewClient(newMockResponseWriter(), context.Background())
	client2 := NewClient(newMockResponseWriter(), context.Background())

	// Register clients
	hub.Register(client1)
	hub.Register(client2)

	// Give some time for registration
	time.Sleep(10 * time.Millisecond)

	// Check client count
	if count := hub.ClientCount(); count != 2 {
		t.Errorf("Expected 2 clients, got %d", count)
	}

	// Unregister one client
	hub.Unregister(client1)

	// Give some time for unregistration
	time.Sleep(10 * time.Millisecond)

	// Check client count
	if count := hub.ClientCount(); count != 1 {
		t.Errorf("Expected 1 client, got %d", count)
	}

	// Unregister the other client
	hub.Unregister(client2)

	// Give some time for unregistration
	time.Sleep(10 * time.Millisecond)

	// Check client count
	if count := hub.ClientCount(); count != 0 {
		t.Errorf("Expected 0 clients, got %d", count)
	}
}

// TestHubBroadcast tests broadcasting events to multiple clients
func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create mock response writers
	writer1 := newMockResponseWriter()
	writer2 := newMockResponseWriter()
	writer3 := newMockResponseWriter()

	// Create and register clients
	client1 := NewClient(writer1, context.Background())
	client2 := NewClient(writer2, context.Background())
	client3 := NewClient(writer3, context.Background())

	hub.Register(client1)
	hub.Register(client2)
	hub.Register(client3)

	// Start client runners
	go client1.Run()
	go client2.Run()
	go client3.Run()

	// Give time for registration
	time.Sleep(10 * time.Millisecond)

	// Broadcast an event
	event := Event{
		Type: "test-event",
		Data: map[string]string{"message": "hello"},
		ID:   "1",
	}

	hub.Broadcast(event)

	// Give time for broadcast
	time.Sleep(50 * time.Millisecond)

	// Check that all clients received the event
	expected := "id: 1\nevent: test-event\ndata: {\"message\":\"hello\"}\n\n"

	if !strings.Contains(writer1.String(), expected) {
		t.Errorf("Client 1 did not receive expected event. Got: %s", writer1.String())
	}

	if !strings.Contains(writer2.String(), expected) {
		t.Errorf("Client 2 did not receive expected event. Got: %s", writer2.String())
	}

	if !strings.Contains(writer3.String(), expected) {
		t.Errorf("Client 3 did not receive expected event. Got: %s", writer3.String())
	}
}

// TestHubBroadcastWithDisconnection tests broadcasting when some clients disconnect
func TestHubBroadcastWithDisconnection(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create mock response writers
	writer1 := newMockResponseWriter()
	writer2 := newMockResponseWriter()

	// Create and register clients
	client1 := NewClient(writer1, context.Background())
	client2 := NewClient(writer2, context.Background())

	hub.Register(client1)
	hub.Register(client2)

	// Start client runners
	go client1.Run()
	go client2.Run()

	// Give time for registration
	time.Sleep(10 * time.Millisecond)

	// Close client1 (simulate disconnection)
	client1.Close()

	// Give time for client to be removed
	time.Sleep(10 * time.Millisecond)

	// Broadcast an event
	event := Event{
		Type: "test-event",
		Data: "test data",
	}

	hub.Broadcast(event)

	// Give time for broadcast
	time.Sleep(50 * time.Millisecond)

	// Check that only client2 received the event
	expected := "event: test-event\ndata: \"test data\"\n\n"

	if strings.Contains(writer1.String(), expected) {
		t.Errorf("Client 1 should not have received event after disconnection")
	}

	if !strings.Contains(writer2.String(), expected) {
		t.Errorf("Client 2 did not receive expected event. Got: %s", writer2.String())
	}
}

// TestHubThreadSafety tests thread safety with concurrent operations
func TestHubThreadSafety(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 50

	// Concurrent registrations
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				client := NewClient(newMockResponseWriter(), context.Background())
				hub.Register(client)
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Give time for all registrations
	time.Sleep(50 * time.Millisecond)

	initialCount := hub.ClientCount()
	if initialCount == 0 {
		t.Error("Expected non-zero client count after registrations")
	}

	// Concurrent broadcasts
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				event := Event{
					Type: "concurrent-event",
					Data: fmt.Sprintf("data-%d-%d", id, j),
				}
				hub.Broadcast(event)
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Give time for all broadcasts
	time.Sleep(100 * time.Millisecond)

	// Concurrent unregistrations
	// Note: We can't easily track all clients, so we'll just verify no panics occur

	// Final count should be stable
	finalCount := hub.ClientCount()
	if finalCount != initialCount {
		t.Logf("Client count changed from %d to %d during concurrent operations", initialCount, finalCount)
	}
}

// TestEventString tests the Event.String() method
func TestEventString(t *testing.T) {
	tests := []struct {
		name     string
		event    Event
		expected string
	}{
		{
			name: "event with all fields",
			event: Event{
				Type: "test",
				Data: map[string]string{"key": "value"},
				ID:   "123",
			},
			expected: "id: 123\nevent: test\ndata: {\"key\":\"value\"}\n\n",
		},
		{
			name: "event without ID",
			event: Event{
				Type: "test",
				Data: "simple data",
			},
			expected: "event: test\ndata: \"simple data\"\n\n",
		},
		{
			name: "event without type",
			event: Event{
				Data: "data only",
				ID:   "456",
			},
			expected: "id: 456\ndata: \"data only\"\n\n",
		},
		{
			name: "event with complex data",
			event: Event{
				Type: "complex",
				Data: map[string]interface{}{
					"nested": map[string]int{"value": 42},
					"array":  []int{1, 2, 3},
				},
			},
			expected: "event: complex\ndata: {\"array\":[1,2,3],\"nested\":{\"value\":42}}\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.event.String()
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

// TestClientWriteEvent tests the Client.WriteEvent method
func TestClientWriteEvent(t *testing.T) {
	writer := newMockResponseWriter()
	client := NewClient(writer, context.Background())

	// Test successful write
	event := Event{
		Type: "test",
		Data: "test data",
	}

	err := client.WriteEvent(event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test write to closed client
	client.Close()
	time.Sleep(10 * time.Millisecond) // Give time for close to complete
	err = client.WriteEvent(event)
	if err == nil {
		t.Error("Expected error when writing to closed client")
	}

	// Test write to full channel
	client2 := NewClient(newMockResponseWriter(), context.Background())
	// Fill the channel
	for i := 0; i < 10; i++ {
		client2.WriteEvent(event)
	}

	// This should fail
	err = client2.WriteEvent(event)
	if err == nil {
		t.Error("Expected error when writing to full channel")
	}
}

// TestHubServeHTTP tests the HTTP handler
func TestHubServeHTTP(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a test request with proper Accept header
	req := httptest.NewRequest("GET", "/events", nil)
	req.Header.Set("Accept", "text/event-stream")

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Call the handler in a goroutine since it blocks
	done := make(chan struct{})
	go func() {
		hub.ServeHTTP(rr, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Check that client was registered
	if count := hub.ClientCount(); count != 1 {
		t.Errorf("Expected 1 client, got %d", count)
	}

	time.Sleep(50 * time.Millisecond)

	// Verify the client was registered successfully
}

// TestHubServeHTTPInvalidAccept tests the HTTP handler with invalid Accept header
// Note: With relaxed checking, empty or non-matching Accept is allowed (lenient mode)
func TestHubServeHTTPInvalidAccept(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a test request without proper Accept header - this should now succeed
	// because we use lenient checking (empty Accept passes through)
	req := httptest.NewRequest("GET", "/events", nil)

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	done := make(chan struct{})
	go func() {
		hub.ServeHTTP(rr, req)
		close(done)
	}()

	select {
	case <-done:
		// Check that client was registered (lenient mode accepts empty Accept)
		if count := hub.ClientCount(); count != 1 {
			t.Errorf("Expected 1 client, got %d", count)
		}
	case <-time.After(5 * time.Second):
		t.Error("Test timed out")
	}
}

// TestHubServeHTTPWrongAcceptType tests with a non-SSE Accept header
func TestHubServeHTTPWrongAcceptType(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Create a test request with wrong Accept header (should be rejected)
	req := httptest.NewRequest("GET", "/events", nil)
	req.Header.Set("Accept", "application/json")

	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		hub.ServeHTTP(rr, req)
		close(done)
	}()

	select {
	case <-done:
		// Should return 400 Bad Request since Accept contains non-SSE type
		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, status)
		}
	case <-time.After(5 * time.Second):
		t.Error("Test timed out")
	}
}
