package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/user/alsamixer-web/internal/config"
	"github.com/user/alsamixer-web/internal/sse"
)

// Server handles HTTP requests and integrates SSE hub, config, and static file serving.
type Server struct {
	config *config.Config
	hub    *sse.Hub
	mux    *http.ServeMux
	server *http.Server
}

// NewServer creates a new HTTP server instance.
func NewServer(cfg *config.Config, hub *sse.Hub) *Server {
	s := &Server{
		config: cfg,
		hub:    hub,
		mux:    http.NewServeMux(),
	}

	s.setupRoutes()

	addr := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.loggingMiddleware(s.corsMiddleware(s.mux)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() {
	// Root handler - placeholder
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("alsamixer-web"))
	})

	// SSE endpoint
	s.mux.Handle("/events", s.hub)

	// Static file server
	staticFS := http.FileServer(http.Dir("web/static"))
	s.mux.Handle("/static/", http.StripPrefix("/static/", staticFS))

	// Control endpoints - placeholders returning 501 Not Implemented
	s.mux.HandleFunc("POST /control/volume", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("Not Implemented"))
	})

	s.mux.HandleFunc("POST /control/mute", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("Not Implemented"))
	})
}

// loggingMiddleware logs all HTTP requests.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap ResponseWriter to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		log.Printf("[%s] %s %s %d %v",
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
			wrapped.statusCode,
			duration,
		)
	})
}

// corsMiddleware adds CORS headers to allow all origins.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Start begins the HTTP server.
func (s *Server) Start() error {
	log.Printf("Starting server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	log.Println("Shutting down server...")
	return s.server.Shutdown(ctx)
}
