package server

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"github.com/user/alsamixer-web/internal/alsa"
	"github.com/user/alsamixer-web/internal/config"
	"github.com/user/alsamixer-web/internal/sse"
)

// VolumeController defines the minimal mixer operations needed by the HTTP handlers.
type VolumeController interface {
	SetVolume(card uint, control string, values []int) error
}

// Server handles HTTP requests and integrates SSE hub, config, mixer, and static file serving.
type Server struct {
	config *config.Config
	hub    *sse.Hub
	mux    *http.ServeMux
	server *http.Server
	tmpl   *template.Template
	mixer  VolumeController
}

type Theme string

const (
	ThemeTerminal Theme = "terminal"
	ThemeModern   Theme = "modern"
	ThemeMuji     Theme = "muji"
	ThemeMobile   Theme = "mobile"
	ThemeCreative Theme = "creative"
)

const defaultTheme = ThemeTerminal

var allowedThemes = map[Theme]struct{}{
	ThemeTerminal: {},
	ThemeModern:   {},
	ThemeMuji:     {},
	ThemeMobile:   {},
	ThemeCreative: {},
}

func mustParseTemplates() *template.Template {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to determine server source path")
	}

	rootDir := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	templatesDir := filepath.Join(rootDir, "web", "templates")

	return template.Must(template.ParseFiles(
		filepath.Join(templatesDir, "base.html"),
		filepath.Join(templatesDir, "index.html"),
		filepath.Join(templatesDir, "controls.html"),
	))
}

func normalizeTheme(raw string) Theme {
	if raw == "" {
		return defaultTheme
	}

	t := Theme(raw)
	if _, ok := allowedThemes[t]; !ok {
		return defaultTheme
	}

	return t
}

// NewServer creates a new HTTP server instance.
func NewServer(cfg *config.Config, hub *sse.Hub) *Server {
	s := &Server{
		config: cfg,
		hub:    hub,
		mux:    http.NewServeMux(),
		mixer:  alsa.NewMixer(),
	}

	s.tmpl = mustParseTemplates()

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
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		requestedTheme := r.URL.Query().Get("theme")
		theme := normalizeTheme(requestedTheme)

		data := struct {
			Theme string
		}{
			Theme: string(theme),
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.tmpl.ExecuteTemplate(w, "base", data); err != nil {
			log.Printf("failed to render index template: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	})

	// SSE endpoint
	s.mux.Handle("/events", s.hub)

	// Static file server
	staticFS := http.FileServer(http.Dir("web/static"))
	s.mux.Handle("/static/", http.StripPrefix("/static/", staticFS))

	// Control endpoints
	s.mux.HandleFunc("POST /control/volume", s.VolumeHandler)
	s.mux.HandleFunc("POST /control/mute", s.MuteHandler)
	s.mux.HandleFunc("POST /control/capture", s.CaptureHandler)
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
