package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/user/alsamixer-web/internal/alsa"
	"github.com/user/alsamixer-web/internal/sse"
)

// mixer defines the subset of the ALSA mixer interface used by
// the HTTP control handlers. It is defined here so tests can
// swap in a fake implementation without requiring real ALSA
// hardware.
type mixer interface {
	GetMute(card uint, control string) (bool, error)
	SetMute(card uint, control string, muted bool) error
	SetVolume(card uint, control string, values []int) error
}

// newMixer constructs a real ALSA mixer. Tests may override this
// variable with a stub implementation.
var newMixer = func() mixer {
	return alsa.NewMixer()
}

// MuteHandler handles POST /control/mute requests from HTMX
// toggle buttons. It toggles the mute state of a control and
// broadcasts an SSE event so all connected clients can update.
func (s *Server) MuteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	cardStr := r.Form.Get("card")
	control := r.Form.Get("control")
	if cardStr == "" || control == "" {
		http.Error(w, "missing card or control", http.StatusBadRequest)
		return
	}

	cardValue, err := strconv.ParseUint(cardStr, 10, 0)
	if err != nil {
		http.Error(w, "invalid card", http.StatusBadRequest)
		return
	}
	cardID := uint(cardValue)

	// Current state as reported by the client; used only for
	// diagnostics and future reconciliation.
	clientMuted := false
	if v := r.Form.Get("muted"); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			clientMuted = parsed
		}
	}

	m := newMixer()
	if m == nil {
		http.Error(w, "mixer unavailable", http.StatusInternalServerError)
		return
	}
	if closer, ok := m.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	currentMuted, err := m.GetMute(cardID, control)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get mute state: %v", err), http.StatusInternalServerError)
		return
	}

	newMuted := !currentMuted
	if err := m.SetMute(cardID, control, newMuted); err != nil {
		http.Error(w, fmt.Sprintf("failed to set mute state: %v", err), http.StatusInternalServerError)
		return
	}

	// Broadcast SSE event so other clients stay in sync.
	if s.hub != nil {
		ctrl := s.getControlView(cardID, control)
		if ctrl != nil {
			html, err := s.renderControlHTML(*ctrl)
			if err != nil {
				log.Printf("failed to render control HTML: %v", err)
			} else {
				payload := fmt.Sprintf(`<article id="control-%d-%s" hx-swap-oob="outerHTML">%s</article>`, cardID, control, html)
				go s.hub.Broadcast(sse.Event{
					Type:   "control-update",
					Data:   payload,
					IsHTML: true,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"card":           cardID,
		"control":        control,
		"muted":          newMuted,
		"previous_muted": currentMuted,
		"client_muted":   clientMuted,
	})
}

// VolumeHandler handles POST /control/volume requests from HTMX
// volume sliders. It sets the volume for a control and broadcasts
// an SSE event so all connected clients can update.
func (s *Server) VolumeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	cardStr := r.Form.Get("card")
	control := r.Form.Get("control")
	volumeStr := r.Form.Get("volume")

	if cardStr == "" || control == "" || volumeStr == "" {
		http.Error(w, "missing card, control, or volume", http.StatusBadRequest)
		return
	}

	cardValue, err := strconv.ParseUint(cardStr, 10, 0)
	if err != nil {
		http.Error(w, "invalid card", http.StatusBadRequest)
		return
	}
	cardID := uint(cardValue)

	volume, err := strconv.Atoi(volumeStr)
	if err != nil {
		http.Error(w, "invalid volume", http.StatusBadRequest)
		return
	}

	// Clamp volume to 0-100 range
	if volume < 0 {
		volume = 0
	} else if volume > 100 {
		volume = 100
	}

	m := newMixer()
	if m == nil {
		http.Error(w, "mixer unavailable", http.StatusInternalServerError)
		return
	}
	if closer, ok := m.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	if err := m.SetVolume(cardID, control, []int{volume}); err != nil {
		http.Error(w, fmt.Sprintf("failed to set volume: %v", err), http.StatusInternalServerError)
		return
	}

	// Broadcast SSE event so other clients stay in sync.
	if s.hub != nil {
		ctrl := s.getControlView(cardID, control)
		if ctrl != nil {
			html, err := s.renderControlHTML(*ctrl)
			if err != nil {
				log.Printf("failed to render control HTML: %v", err)
			} else {
				payload := fmt.Sprintf(`<article id="control-%d-%s" hx-swap-oob="outerHTML">%s</article>`, cardID, control, html)
				go s.hub.Broadcast(sse.Event{
					Type:   "control-update",
					Data:   payload,
					IsHTML: true,
				})
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// CaptureHandler handles POST /control/capture requests from HTMX
// toggle buttons. Capture "active" is treated as the inverse of
// the underlying mute state.
func (s *Server) CaptureHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	cardStr := r.Form.Get("card")
	control := r.Form.Get("control")
	if cardStr == "" || control == "" {
		http.Error(w, "missing card or control", http.StatusBadRequest)
		return
	}

	cardValue, err := strconv.ParseUint(cardStr, 10, 0)
	if err != nil {
		http.Error(w, "invalid card", http.StatusBadRequest)
		return
	}
	cardID := uint(cardValue)

	clientActive := false
	if v := r.Form.Get("active"); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			clientActive = parsed
		}
	}

	m := newMixer()
	if m == nil {
		http.Error(w, "mixer unavailable", http.StatusInternalServerError)
		return
	}
	if closer, ok := m.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	// Capture "active" is modelled as not muted.
	currentMuted, err := m.GetMute(cardID, control)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get capture state: %v", err), http.StatusInternalServerError)
		return
	}
	currentActive := !currentMuted

	newActive := !currentActive
	newMuted := !newActive

	if err := m.SetMute(cardID, control, newMuted); err != nil {
		http.Error(w, fmt.Sprintf("failed to set capture state: %v", err), http.StatusInternalServerError)
		return
	}

	if s.hub != nil {
		ctrl := s.getControlView(cardID, control)
		if ctrl != nil {
			html, err := s.renderControlHTML(*ctrl)
			if err != nil {
				log.Printf("failed to render control HTML: %v", err)
			} else {
				payload := fmt.Sprintf(`<article id="control-%d-%s" hx-swap-oob="outerHTML">%s</article>`, cardID, control, html)
				go s.hub.Broadcast(sse.Event{
					Type:   "control-update",
					Data:   payload,
					IsHTML: true,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"card":            cardID,
		"control":         control,
		"active":          newActive,
		"previous_active": currentActive,
		"client_active":   clientActive,
	})
}
