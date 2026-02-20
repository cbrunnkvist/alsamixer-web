package server

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/user/alsamixer-web/internal/alsa"
	"github.com/user/alsamixer-web/internal/config"
	"github.com/user/alsamixer-web/internal/sse"
	"github.com/user/alsamixer-web/web"
)

// Server handles HTTP requests and integrates SSE hub, config, mixer, and static file serving.
type Server struct {
	config  *config.Config
	hub     *sse.Hub
	mux     *http.ServeMux
	server  *http.Server
	tmpl    *template.Template
	mixer   *alsa.Mixer
	monitor *alsa.Monitor
}

type Theme string

const (
	ThemeTerminal     Theme = "terminal"
	ThemeModern       Theme = "modern"
	ThemeMuji         Theme = "muji"
	ThemeMobile       Theme = "mobile"
	ThemeCreative     Theme = "creative"
	ThemeLinuxConsole Theme = "linux-console"
)

const defaultTheme = ThemeTerminal

var allowedThemes = map[Theme]struct{}{
	ThemeTerminal:     {},
	ThemeModern:       {},
	ThemeMuji:         {},
	ThemeMobile:       {},
	ThemeCreative:     {},
	ThemeLinuxConsole: {},
}

type pageData struct {
	Theme string
	Cards []cardView
}

type cardView struct {
	ID          uint
	Name        string
	Description string
	Controls    []controlView
}

type controlView struct {
	ID               string
	CardID           uint
	Name             string
	Description      string
	HasVolume        bool
	HasMute          bool
	HasCapture       bool
	VolumeMin        int
	VolumeMax        int
	VolumeNow        int
	VolumeText       string
	VolumeAriaLabel  string
	MuteAriaLabel    string
	CaptureAriaLabel string
	Muted            bool
	CaptureActive    bool
	View             string
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func controlID(cardID uint, controlName string) string {
	name := strings.ToLower(controlName)
	name = nonAlphaNum.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "control"
	}
	return fmt.Sprintf("%d-%s", cardID, name)
}

func controlViewType(controlName string) string {
	name := strings.ToLower(controlName)
	if strings.Contains(name, "capture") ||
		strings.Contains(name, "mic") ||
		strings.Contains(name, "boost") ||
		strings.Contains(name, "pre-amp") ||
		strings.Contains(name, "input") ||
		strings.Contains(name, "digital") {
		return "capture"
	}
	return "playback"
}

func (s *Server) loadCards() []cardView {
	if s.mixer == nil || !s.mixer.IsOpen() {
		return nil
	}

	cards, err := s.mixer.ListCards()
	if err != nil {
		log.Printf("failed to list cards: %v", err)
		return nil
	}

	result := make([]cardView, 0, len(cards))
	for _, card := range cards {
		cv := cardView{ID: card.ID, Name: card.Name}

		controls, err := s.mixer.ListControls(card.ID)
		if err != nil {
			log.Printf("failed to list controls for card %d: %v", card.ID, err)
			result = append(result, cv)
			continue
		}

		for _, ctrl := range controls {
			if ctrl.Type != "integer" {
				continue
			}

			view := controlViewType(ctrl.Name)

			volumes, err := s.mixer.GetVolume(card.ID, ctrl.Name)
			volumeNow := 0
			if err == nil && len(volumes) > 0 {
				volumeNow = volumes[0]
			}

			muted, muteErr := s.mixer.GetMute(card.ID, ctrl.Name)
			hasMute := muteErr == nil

			cv.Controls = append(cv.Controls, controlView{
				ID:               controlID(card.ID, ctrl.Name),
				CardID:           card.ID,
				Name:             ctrl.Name,
				HasVolume:        true,
				HasMute:          hasMute,
				HasCapture:       false,
				VolumeMin:        0,
				VolumeMax:        100,
				VolumeNow:        volumeNow,
				VolumeText:       fmt.Sprintf("%d%%", volumeNow),
				VolumeAriaLabel:  fmt.Sprintf("%s volume", ctrl.Name),
				MuteAriaLabel:    fmt.Sprintf("%s mute", ctrl.Name),
				CaptureAriaLabel: fmt.Sprintf("%s capture", ctrl.Name),
				Muted:            muted,
				CaptureActive:    false,
				View:             view,
			})
		}

		result = append(result, cv)
	}

	return result
}

func mustParseTemplates() *template.Template {
	// Use embed.TemplateFS() to get the embedded filesystem
	return template.Must(template.ParseFS(web.TemplateFS(), "base.html", "index.html", "controls.html"))
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

	if s.mixer == nil {
		log.Printf("ALSA mixer unavailable; continuing without monitor")
	} else if !s.mixer.IsOpen() {
		log.Printf("ALSA mixer not open; continuing without monitor")
	} else {
		s.monitor = alsa.NewMonitor(s.mixer, s.hub, cfg.MonitorFile)
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

// Hub returns the SSE hub for use by the monitor
func (s *Server) Hub() *sse.Hub {
	return s.hub
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

		data := pageData{Theme: string(theme), Cards: s.loadCards()}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := s.tmpl.ExecuteTemplate(w, "base", data); err != nil {
			log.Printf("failed to render index template: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	})

	// SSE endpoint
	s.mux.Handle("/events", s.hub)

	// Static file server (embedded)
	staticFS := http.FileServer(http.FS(web.StaticFS()))
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
	if s.monitor != nil {
		s.monitor.Start()
	}
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	log.Println("Shutting down server...")
	if s.monitor != nil {
		s.monitor.Stop()
	}
	return s.server.Shutdown(ctx)
}
