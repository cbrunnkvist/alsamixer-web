package server

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strconv"
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

const defaultTheme = ThemeLinuxConsole

var allowedThemes = map[Theme]struct{}{
	ThemeTerminal:     {},
	ThemeModern:       {},
	ThemeMuji:         {},
	ThemeMobile:       {},
	ThemeCreative:     {},
	ThemeLinuxConsole: {},
}

type pageData struct {
	Theme        string
	Cards        []cardView
	SelectedCard uint
	DefaultCard  uint
	AllCards     []alsa.Card
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
	VolumeStep       int
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

// shouldSkipControl returns true if the control should be hidden from the UI.
// This matches alsamixer's filtering logic to show only user-relevant controls.
func shouldSkipControl(controlName, view string) bool {
	name := strings.ToLower(controlName)

	// Skip internal/low-level ALSA controls that users shouldn't manipulate
	skipPatterns := []string{
		"pcm",         // Low-level PCM controls
		"rate",        // Sample rate controls
		"clock",       // Clock source
		"iec958",      // Raw digital I/O (shown as S/PDIF in alsamixer, but only enum type)
		"channel map", // Channel mapping
		"routing",     // Signal routing
		"mux",         // Multiplexer
		"loopback",    // Loopback controls
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(name, pattern) {
			// Exception: Some controls like "Pre-amp" should be shown
			if strings.Contains(name, "pre-amp") {
				return false
			}
			return true
		}
	}

	return false
}

func (s *Server) loadCards() []cardView {
	return s.loadCardsForFilter(-1)
}

func (s *Server) loadCardsForFilter(selectedCardID int) []cardView {
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
		if selectedCardID >= 0 && int(card.ID) != selectedCardID {
			continue
		}

		cv := cardView{ID: card.ID, Name: card.Name}

		controls, err := s.mixer.ListControls(card.ID)
		if err != nil {
			log.Printf("failed to list controls for card %d: %v", card.ID, err)
			result = append(result, cv)
			continue
		}

		for _, ctrl := range controls {
			// Determine view type based on control name
			view := controlViewType(ctrl.Name)

			// Only show controls that have volume (integer type with range)
			if ctrl.Type != "integer" {
				continue
			}

			// Additional filtering: skip internal ALSA controls that aren't user-relevant
			// This matches alsamixer's behavior of filtering out low-level PCM controls
			if shouldSkipControl(ctrl.Name, view) {
				continue
			}

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
				VolumeStep:       int(ctrl.Step),
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

func (s *Server) renderControlHTML(ctrl controlView) (string, error) {
	var buf strings.Builder
	if err := s.tmpl.ExecuteTemplate(&buf, "control", ctrl); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (s *Server) getControlView(cardID uint, controlName string) *controlView {
	controls, err := s.mixer.ListControls(cardID)
	if err != nil {
		return nil
	}

	for _, ctrl := range controls {
		if ctrl.Name != controlName {
			continue
		}

		volumes, err := s.mixer.GetVolume(cardID, controlName)
		volumeNow := 0
		if err == nil && len(volumes) > 0 {
			volumeNow = volumes[0]
		}

		muted, muteErr := s.mixer.GetMute(cardID, controlName)
		hasMute := muteErr == nil

		view := controlViewType(ctrl.Name)

		return &controlView{
			ID:               controlID(cardID, ctrl.Name),
			CardID:           cardID,
			Name:             ctrl.Name,
			HasVolume:        ctrl.Type == "integer",
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
		}
	}

	return nil
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
		WriteTimeout: 0, // No write timeout - needed for SSE connections
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

		allCards, _ := s.mixer.ListCards()
		configuredDefault := alsa.GetDefaultCard()
		resolvedDefault := alsa.ResolveDefaultCard(allCards, configuredDefault)

		cardParam := r.URL.Query().Get("card")
		var selectedCardID uint
		if cardParam == "" || cardParam == "default" {
			selectedCardID = resolvedDefault
		} else if cardNum, err := strconv.ParseUint(cardParam, 10, 0); err == nil {
			selectedCardID = uint(cardNum)
			found := false
			for _, c := range allCards {
				if c.ID == selectedCardID {
					found = true
					break
				}
			}
			if !found {
				selectedCardID = resolvedDefault
			}
		} else {
			selectedCardID = resolvedDefault
		}

		data := pageData{
			Theme:        string(theme),
			Cards:        s.loadCardsForFilter(int(selectedCardID)),
			SelectedCard: selectedCardID,
			DefaultCard:  resolvedDefault,
			AllCards:     allCards,
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

	// Static file server (embedded)
	staticFS := http.FileServer(http.FS(web.StaticFS()))
	s.mux.Handle("/static/", http.StripPrefix("/static/", staticFS))

	// Control endpoints
	s.mux.HandleFunc("POST /control/volume", s.VolumeHandler)
	s.mux.HandleFunc("POST /control/mute", s.MuteHandler)
	s.mux.HandleFunc("POST /control/capture", s.CaptureHandler)

	// Debug endpoint
	s.mux.HandleFunc("GET /debug/controls", s.DebugControlsHandler)
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

func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *responseWriter) Hijack() (interface{}, interface{}, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
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

// DebugControlsHandler returns debug info about ALSA controls
func (s *Server) DebugControlsHandler(w http.ResponseWriter, r *http.Request) {
	if s.mixer == nil || !s.mixer.IsOpen() {
		http.Error(w, "mixer not available", http.StatusServiceUnavailable)
		return
	}

	cards, err := s.mixer.ListCards()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to list cards: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Cards found: %d\n", len(cards))
	for _, card := range cards {
		fmt.Fprintf(w, "\nCard %d: %s\n", card.ID, card.Name)
		controls, err := s.mixer.ListControls(card.ID)
		if err != nil {
			fmt.Fprintf(w, "  Error listing controls: %v\n", err)
			continue
		}
		fmt.Fprintf(w, "  Controls: %d\n", len(controls))
		for _, ctrl := range controls {
			fmt.Fprintf(w, "    - %s (type: %s)\n", ctrl.Name, ctrl.Type)
		}
	}
}
