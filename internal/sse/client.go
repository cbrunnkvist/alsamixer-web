package sse

import (
	"fmt"
	"net/http"
	"sync"
)

// Client represents an SSE client connection.
type Client struct {
	writer  http.ResponseWriter
	eventCh chan Event
	done    chan struct{}
	closed  bool
	mu      sync.Mutex
}

// NewClient creates a new SSE client.
func NewClient(w http.ResponseWriter) *Client {
	return &Client{
		writer:  w,
		eventCh: make(chan Event, 10), // Buffered channel to prevent blocking
		done:    make(chan struct{}),
	}
}

// WriteEvent sends an SSE formatted event to the client.
func (c *Client) WriteEvent(event Event) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()

	if closed {
		return fmt.Errorf("client disconnected")
	}

	select {
	case c.eventCh <- event:
		return nil
	default:
		return fmt.Errorf("client event channel full")
	}
}

// Close signals the client to stop receiving events.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		close(c.done)
		c.closed = true
	}
}

// Run starts the client's event writer goroutine.
func (c *Client) Run() {
	// Set SSE headers
	c.writer.Header().Set("Content-Type", "text/event-stream")
	c.writer.Header().Set("Cache-Control", "no-cache")
	c.writer.Header().Set("Connection", "keep-alive")
	c.writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Flush headers immediately
	if flusher, ok := c.writer.(http.Flusher); ok {
		flusher.Flush()
	}

	// Write events as they arrive
	for {
		select {
		case event, ok := <-c.eventCh:
			if !ok {
				return
			}

			// Write the event
			if _, err := fmt.Fprint(c.writer, event.String()); err != nil {
				c.Close()
				return
			}

			// Flush after each event
			if flusher, ok := c.writer.(http.Flusher); ok {
				flusher.Flush()
			}

		case <-c.done:
			return
		}
	}
}
