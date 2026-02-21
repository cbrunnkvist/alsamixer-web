package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/user/alsamixer-web/internal/config"
	"github.com/user/alsamixer-web/internal/sse"
)

type fakeMixer struct {
	card    uint
	control string
	values  []int
	called  bool
	err     error
}

func (f *fakeMixer) GetMute(card uint, control string) (bool, error) {
	return false, nil
}

func (f *fakeMixer) SetMute(card uint, control string, muted bool) error {
	return nil
}

func (f *fakeMixer) SetVolume(card uint, control string, values []int) error {
	f.card = card
	f.control = control
	if values != nil {
		f.values = append([]int(nil), values...)
	} else {
		f.values = nil
	}
	f.called = true
	return f.err
}

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
			name:           "GET / returns HTML shell",
			method:         "GET",
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedBody:   "<!doctype html>",
		},
		{
			name:           "GET /nonexistent returns 404",
			method:         "GET",
			path:           "/nonexistent",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "POST /control/volume without form data returns 400",
			method:         "POST",
			path:           "/control/volume",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "POST /control/mute without form data returns 400",
			method:         "POST",
			path:           "/control/mute",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GET / with valid theme query applies requested theme",
			method:         "GET",
			path:           "/?theme=modern",
			expectedStatus: http.StatusOK,
			expectedBody:   "/static/themes/modern.css",
		},
		{
			name:           "GET / with invalid theme falls back to default",
			method:         "GET",
			path:           "/?theme=unknown",
			expectedStatus: http.StatusOK,
			expectedBody:   "/static/themes/linux-console.css",
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
				bodyStr := string(body)
				if !strings.Contains(bodyStr, tt.expectedBody) {
					t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, bodyStr)
				}
			}
		})
	}
}

func TestVolumeHandler_Success(t *testing.T) {
	cfg := &config.Config{
		Port:     0,
		BindAddr: "127.0.0.1",
	}
	hub := sse.NewHub()
	srv := NewServer(cfg, hub)

	fm := &fakeMixer{}
	origNewMixer := newMixer
	newMixer = func() mixer {
		return fm
	}
	defer func() {
		newMixer = origNewMixer
	}()

	form := url.Values{}
	form.Set("card", "0")
	form.Set("control", "Master")
	form.Set("volume", "75")

	req := httptest.NewRequest(http.MethodPost, "/control/volume", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp := httptest.NewRecorder()
	srv.VolumeHandler(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, resp.Code)
	}

	if !fm.called {
		t.Fatalf("expected mixer.SetVolume to be called")
	}

	if fm.card != 0 {
		t.Errorf("expected card 0, got %d", fm.card)
	}

	if fm.control != "Master" {
		t.Errorf("expected control 'Master', got %q", fm.control)
	}

	if len(fm.values) != 1 || fm.values[0] != 75 {
		t.Errorf("expected values [75], got %v", fm.values)
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
