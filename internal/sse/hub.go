package sse

import (
	"net/http"
	"sync"
)

// Hub manages SSE client connections and broadcasts events.
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan Event
	stop       chan struct{}
	mu         sync.Mutex
}

// NewHub creates a new SSE hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Event),
		stop:       make(chan struct{}),
	}
}

// Register adds a new SSE client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes an SSE client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast sends an event to all connected clients.
func (h *Hub) Broadcast(event Event) {
	h.broadcast <- event
}

// Run starts the hub's main goroutine handling register/unregister/broadcast channels.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				if err := client.WriteEvent(event); err != nil {
					// Client disconnected or channel full, remove it
					delete(h.clients, client)
					client.Close()
				}
			}
			h.mu.Unlock()

		case <-h.stop:
			// Close all clients before stopping
			h.mu.Lock()
			for client := range h.clients {
				client.Close()
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return
		}
	}
}

// Stop signals the hub to stop running.
func (h *Hub) Stop() {
	close(h.stop)
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// ServeHTTP handles HTTP requests and registers new clients.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if client supports SSE
	if r.Header.Get("Accept") != "text/event-stream" {
		http.Error(w, "Expected Accept: text/event-stream", http.StatusBadRequest)
		return
	}

	// Create and register new client
	client := NewClient(w)
	h.Register(client)
	defer h.Unregister(client)

	// Start client writer
	client.Run()
}
