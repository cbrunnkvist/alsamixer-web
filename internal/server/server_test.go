package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/user/alsamixer-web/internal/config"
	"github.com/user/alsamixer-web/internal/sse"
)

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Port:     0, // Use port 0 to let system assign a random port
		BindAddr: "127.0.0.1",
	}
	hub := sse.NewHub()

	srv := NewServer(cfg, hub)

	if srv == nil {
		t.Fatal("NewServer returned nil")
	}

	if srv.config != cfg {
		t.Error("Server config mismatch")
	}

	if srv.hub != hub {
		t.Error("Server hub mismatch")
	}

	if srv.mux == nil {
		t.Error("Server mux is nil")
	}

	if srv.server == nil {
		t.Error("HTTP server is nil")
	}
}

func TestServerRoutes(t *testing.T) {
	cfg := &config.Config{
		Port:     0,
		BindAddr: "127.0.0.1",
	}
	hub := sse.NewHub()
	srv := NewServer(cfg, hub)

	// Create a test server
	ts := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: srv.mux,
	}

	// Start listener to get actual port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go ts.Serve(listener)
	defer ts.Close()

	baseURL := "http://" + listener.Addr().String()

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET / returns alsamixer-web",
			method:         "GET",
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "alsamixer-web",
		},
		{
			name:           "GET /nonexistent returns 404",
			method:         "GET",
			path:           "/nonexistent",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "POST /control/volume returns 501",
			method:         "POST",
			path:           "/control/volume",
			expectedStatus: http.StatusNotImplemented,
			expectedBody:   "Not Implemented",
		},
		{
			name:           "POST /control/mute returns 501",
			method:         "POST",
			path:           "/control/mute",
			expectedStatus: http.StatusNotImplemented,
			expectedBody:   "Not Implemented",
		},
		{
			name:           "OPTIONS request returns 200",
			method:         "OPTIONS",
			path:           "/",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, baseURL+tt.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if tt.expectedBody != "" {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("Failed to read body: %v", err)
				}
				if string(body) != tt.expectedBody {
					t.Errorf("Expected body %q, got %q", tt.expectedBody, string(body))
				}
			}
		})
	}
}

func TestServerCORSMiddleware(t *testing.T) {
	cfg := &config.Config{
		Port:     0,
		BindAddr: "127.0.0.1",
	}
	hub := sse.NewHub()
	srv := NewServer(cfg, hub)

	// Create a test server using the full handler chain with middleware
	ts := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: srv.server.Handler,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	go ts.Serve(listener)
	defer ts.Close()

	baseURL := "http://" + listener.Addr().String()

	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check CORS headers
	if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin: *, got %s", origin)
	}

	if methods := resp.Header.Get("Access-Control-Allow-Methods"); methods == "" {
		t.Error("Expected Access-Control-Allow-Methods header to be set")
	}
}

func TestServerStartAndStop(t *testing.T) {
	cfg := &config.Config{
		Port:     0,
		BindAddr: "127.0.0.1",
	}
	hub := sse.NewHub()
	srv := NewServer(cfg, hub)

	// Create a listener to get a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	// Update server address to use the actual listener address
	srv.server.Addr = listener.Addr().String()

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.server.Serve(listener)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Make a request to verify server is running
	baseURL := "http://" + listener.Addr().String()
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("Server did not start: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = srv.Stop(ctx)
	if err != nil {
		t.Errorf("Stop returned error: %v", err)
	}

	// Wait for server to stop
	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Server error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Server did not stop in time")
	}

	// Verify server is no longer accepting connections
	_, err = http.Get(baseURL + "/")
	if err == nil {
		t.Error("Server should not be accepting connections after stop")
	}
}
