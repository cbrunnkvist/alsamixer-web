package sse

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const heartbeatInterval = 25 * time.Second

// Client represents an SSE client connection.
type Client struct {
	writer  http.ResponseWriter
	ctx     context.Context
	cancel  context.CancelFunc
	eventCh chan Event
	done    chan struct{}
	closed  bool
	mu      sync.Mutex
}

// NewClient creates a new SSE client.
func NewClient(w http.ResponseWriter, ctx context.Context) *Client {
	ctx, cancel := context.WithCancel(ctx)
	return &Client{
		writer:  w,
		ctx:     ctx,
		cancel:  cancel,
		eventCh: make(chan Event, 10),
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
		c.cancel()
		close(c.done)
		c.closed = true
	}
}

// Run starts the client's event writer goroutine.
func (c *Client) Run() {
	log.Printf("SSE Client.Run() started")
	// Set SSE headers
	c.writer.Header().Set("Content-Type", "text/event-stream")
	c.writer.Header().Set("Cache-Control", "no-cache")
	c.writer.Header().Set("Connection", "keep-alive")
	c.writer.Header().Set("Access-Control-Allow-Origin", "*")

	// Flush headers immediately
	if flusher, ok := c.writer.(http.Flusher); ok {
		log.Printf("SSE Client.Run() flushing headers")
		flusher.Flush()
	} else {
		log.Printf("SSE Client.Run() WARNING: ResponseWriter is not a Flusher")
	}

	log.Printf("SSE Client.Run() entering event loop")

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.ctx.Done():
			log.Printf("SSE Client.Run() context cancelled")
			return
		case <-c.done:
			log.Printf("SSE Client.Run() done signal received")
			return
		case <-heartbeat.C:
			// Send heartbeat comment to keep connection alive
			if _, err := fmt.Fprint(c.writer, ": heartbeat\n\n"); err != nil {
				log.Printf("SSE Client.Run() heartbeat failed: %v", err)
				c.Close()
				return
			}
			if flusher, ok := c.writer.(http.Flusher); ok {
				flusher.Flush()
			}
		case event, ok := <-c.eventCh:
			if !ok {
				return
			}

			// Write the event
			if _, err := fmt.Fprint(c.writer, event.String()); err != nil {
				log.Printf("SSE Client.Run() write failed: %v", err)
				c.Close()
				return
			}

			// Flush after each event
			if flusher, ok := c.writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}
