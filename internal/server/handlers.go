package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/user/alsamixer-web/internal/alsa"
	"github.com/user/alsamixer-web/internal/sse"
)

func (s *Server) resolveVolumeControlName(cardID uint, baseName string) string {
	controls, err := s.mixer.ListControls(cardID)
	if err != nil {
		return baseName + " Playback Volume"
	}
	for _, ctrl := range controls {
		bn := extractBaseName(ctrl.Name)
		if bn == baseName && strings.Contains(ctrl.Name, "Volume") {
			return ctrl.Name
		}
	}
	return baseName + " Playback Volume"
}

func (s *Server) resolveSwitchControlName(cardID uint, baseName string) string {
	volName := s.resolveVolumeControlName(cardID, baseName)
	return strings.Replace(volName, " Volume", " Switch", 1)
}

func (s *Server) CardControlVolumeHandler(w http.ResponseWriter, r *http.Request) {
	cardIDStr := r.PathValue("cardId")
	controlBaseName := r.PathValue("controlName")

	unescapedName, err := url.PathUnescape(controlBaseName)
	if err != nil {
		http.Error(w, "invalid control name", http.StatusBadRequest)
		return
	}
	controlBaseName = unescapedName

	cardID, err := strconv.ParseUint(cardIDStr, 10, 0)
	if err != nil {
		http.Error(w, "invalid card id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	volumeStr := r.Form.Get("value")
	if volumeStr == "" {
		volumeStr = r.Form.Get("volume")
	}
	if volumeStr == "" {
		http.Error(w, "missing volume value", http.StatusBadRequest)
		return
	}

	volume, err := strconv.Atoi(volumeStr)
	if err != nil {
		http.Error(w, "invalid volume", http.StatusBadRequest)
		return
	}

	if volume < 0 {
		volume = 0
	} else if volume > 100 {
		volume = 100
	}

	controlName := s.resolveVolumeControlName(uint(cardID), controlBaseName)

	log.Printf("[POST /card/%d/control/%s/volume] volume=%d (resolved: %s)", cardID, controlBaseName, volume, controlName)

	m := newMixer()
	if m == nil {
		http.Error(w, "mixer unavailable", http.StatusInternalServerError)
		return
	}
	if closer, ok := m.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	// Check if control exists before trying to set it
	controls, err := m.ListControls(uint(cardID))
	if err == nil {
		found := false
		for _, ctrl := range controls {
			if ctrl.Name == controlName {
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "control not found", http.StatusBadRequest)
			return
		}
	}

	if err := m.SetVolume(uint(cardID), controlName, []int{volume}); err != nil {
		http.Error(w, fmt.Sprintf("failed to set volume: %v", err), http.StatusInternalServerError)
		return
	}

	if s.hub != nil {
		ctrl := s.getControlView(uint(cardID), controlName)
		if ctrl != nil {
			log.Printf("[SSE broadcast] %s", compactEventData(ctrl))
			go s.hub.Broadcast(sse.Event{
				Type: "mixer-update",
				Data: map[string]interface{}{
					"state": map[string]interface{}{
						fmt.Sprintf("%d", cardID): map[string]interface{}{
							controlName: map[string]interface{}{
								"Volume": []int{volume},
								"Mute":   ctrl.Muted,
							},
						},
					},
					"source":  "handler",
					"control": controlName,
				},
			})
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) CardControlMuteHandler(w http.ResponseWriter, r *http.Request) {
	cardIDStr := r.PathValue("cardId")
	controlBaseName := r.PathValue("controlName")

	unescapedName, err := url.PathUnescape(controlBaseName)
	if err != nil {
		http.Error(w, "invalid control name", http.StatusBadRequest)
		return
	}
	controlBaseName = unescapedName

	cardID, err := strconv.ParseUint(cardIDStr, 10, 0)
	if err != nil {
		http.Error(w, "invalid card id", http.StatusBadRequest)
		return
	}

	m := newMixer()
	if m == nil {
		http.Error(w, "mixer unavailable", http.StatusInternalServerError)
		return
	}
	if closer, ok := m.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	switchControl := s.resolveSwitchControlName(uint(cardID), controlBaseName)
	volumeControl := s.resolveVolumeControlName(uint(cardID), controlBaseName)

	currentMuted, err := m.GetMute(uint(cardID), switchControl)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get mute state: %v", err), http.StatusInternalServerError)
		return
	}

	newMuted := !currentMuted

	if err := m.SetMute(uint(cardID), switchControl, newMuted); err != nil {
		http.Error(w, fmt.Sprintf("failed to set mute state: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[POST /card/%d/control/%s/mute] muted=%v (resolved: %s)", cardID, controlBaseName, newMuted, switchControl)

	if s.hub != nil {
		ctrl := s.getControlView(uint(cardID), volumeControl)
		if ctrl != nil {
			log.Printf("[SSE broadcast] %s", compactEventData(ctrl))
			go s.hub.Broadcast(sse.Event{
				Type: "mixer-update",
				Data: map[string]interface{}{
					"state": map[string]interface{}{
						fmt.Sprintf("%d", cardID): map[string]interface{}{
							volumeControl: map[string]interface{}{
								"Volume": []int{ctrl.VolumeNow},
								"Mute":   newMuted,
							},
						},
					},
					"source":  "handler",
					"control": volumeControl,
				},
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"card":    cardID,
		"control": controlBaseName,
		"muted":   newMuted,
	})
}

func (s *Server) CardControlCaptureHandler(w http.ResponseWriter, r *http.Request) {
	cardIDStr := r.PathValue("cardId")
	controlBaseName := r.PathValue("controlName")

	unescapedName, err := url.PathUnescape(controlBaseName)
	if err != nil {
		http.Error(w, "invalid control name", http.StatusBadRequest)
		return
	}
	controlBaseName = unescapedName

	cardID, err := strconv.ParseUint(cardIDStr, 10, 0)
	if err != nil {
		http.Error(w, "invalid card id", http.StatusBadRequest)
		return
	}

	m := newMixer()
	if m == nil {
		http.Error(w, "mixer unavailable", http.StatusInternalServerError)
		return
	}
	if closer, ok := m.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	switchControl := s.resolveSwitchControlName(uint(cardID), controlBaseName)
	volumeControl := s.resolveVolumeControlName(uint(cardID), controlBaseName)

	currentMuted, err := m.GetMute(uint(cardID), switchControl)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get capture state: %v", err), http.StatusInternalServerError)
		return
	}
	currentActive := !currentMuted
	newActive := !currentActive
	newMuted := !newActive

	if err := m.SetMute(uint(cardID), switchControl, newMuted); err != nil {
		http.Error(w, fmt.Sprintf("failed to set capture state: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[POST /card/%d/control/%s/capture] active=%v (resolved: %s)", cardID, controlBaseName, newActive, switchControl)

	if s.hub != nil {
		ctrl := s.getControlView(uint(cardID), volumeControl)
		if ctrl != nil {
			log.Printf("[SSE broadcast] %s", compactEventData(ctrl))
			go s.hub.Broadcast(sse.Event{
				Type: "mixer-update",
				Data: map[string]interface{}{
					"state": map[string]interface{}{
						fmt.Sprintf("%d", cardID): map[string]interface{}{
							volumeControl: map[string]interface{}{
								"Volume": []int{ctrl.VolumeNow},
								"Mute":   newMuted,
							},
						},
					},
					"source":  "handler",
					"control": volumeControl,
				},
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"card":    cardID,
		"control": controlBaseName,
		"active":  newActive,
	})
}

// compactEventData creates a compact JSON representation of an SSE broadcast for logging
func compactEventData(ctrl *controlView) string {
	if ctrl == nil {
		return "{}"
	}
	// Create a minimal representation
	type CompactControl struct {
		C string `json:"c"` // control name
		V int    `json:"v"` // volume now
		M bool   `json:"m"` // muted
	}
	data := CompactControl{
		C: ctrl.Name,
		V: ctrl.VolumeNow,
		M: ctrl.Muted,
	}
	b, _ := json.Marshal(data)
	return string(b)
}

// mixer defines the subset of the ALSA mixer interface used by
// the HTTP control handlers. It is defined here so tests can
// swap in a fake implementation without requiring real ALSA
// hardware.
type mixer interface {
	GetMute(card uint, control string) (bool, error)
	SetMute(card uint, control string, muted bool) error
	SetVolume(card uint, control string, values []int) error
	ListControls(card uint) ([]alsa.Control, error)
}

// newMixer constructs a real ALSA mixer. Tests may override this
// variable with a stub implementation.
var newMixer = func() mixer {
	return alsa.NewMixer()
}

// MuteHandler handles POST /control/mute requests from HTMX
// toggle buttons. It toggles the mute state of a control and
// broadcasts an SSE event so all connected clients can update.
// MuteHandler handles POST /control/mute requests from HTMX toggle buttons.
// It toggles the mute state of a control and broadcasts an SSE event.
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

	// Log the request body
	log.Printf("[POST /control/mute] card=%s control=%s", cardStr, control)

	cardValue, err := strconv.ParseUint(cardStr, 10, 0)
	if err != nil {
		http.Error(w, "invalid card", http.StatusBadRequest)
		return
	}
	cardID := uint(cardValue)

	// Current state as reported by the client; used only for diagnostics
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

	// Use the corresponding switch control for mute
	switchControl := strings.Replace(control, " Volume", " Switch", 1)
	currentMuted, err := m.GetMute(cardID, switchControl)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get mute state: %v", err), http.StatusInternalServerError)
		return
	}

	newMuted := !currentMuted
	if err := m.SetMute(cardID, switchControl, newMuted); err != nil {
		http.Error(w, fmt.Sprintf("failed to set mute state: %v", err), http.StatusInternalServerError)
		return
	}

	// Broadcast SSE event so other clients stay in sync.
	if s.hub != nil {
		ctrl := s.getControlView(cardID, control)
		if ctrl != nil {
			// Log the SSE broadcast (compact JSON)
			log.Printf("[SSE broadcast] %s", compactEventData(ctrl))
			// Broadcast mixer-update style event for JS-only clients
			go s.hub.Broadcast(sse.Event{
				Type: "mixer-update",
				Data: map[string]interface{}{
					"state": map[string]interface{}{
						fmt.Sprintf("%d", cardID): map[string]interface{}{
							control: map[string]interface{}{
								"Volume": []int{ctrl.VolumeNow},
								"Mute":   newMuted,
							},
						},
					},
					"source":  "handler",
					"control": control,
				},
			})
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

	// Log the request body
	log.Printf("[POST /control/volume] card=%s control=%s volume=%s", cardStr, control, volumeStr)

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

	// Validate control exists before trying to set it
	controls, err := m.ListControls(cardID)
	if err == nil {
		found := false
		for _, ctrl := range controls {
			if ctrl.Name == control {
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "control not found", http.StatusBadRequest)
			return
		}
	}

	if err := m.SetVolume(cardID, control, []int{volume}); err != nil {
		http.Error(w, fmt.Sprintf("failed to set volume: %v", err), http.StatusInternalServerError)
		return
	}

	// Broadcast SSE event so other clients stay in sync.
	if s.hub != nil {
		ctrl := s.getControlView(cardID, control)
		if ctrl != nil {
			// Log the SSE broadcast (compact JSON)
			log.Printf("[SSE broadcast] %s", compactEventData(ctrl))
			// Broadcast mixer-update style event for JS-only clients
			// Include timestamp so client knows this is fresh from handler (not monitor)
			go s.hub.Broadcast(sse.Event{
				Type: "mixer-update",
				Data: map[string]interface{}{
					"state": map[string]interface{}{
						fmt.Sprintf("%d", cardID): map[string]interface{}{
							control: map[string]interface{}{
								"Volume": []int{volume},
								"Mute":   ctrl.Muted,
							},
						},
					},
					"source":  "handler",
					"control": control,
				},
			})
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

	// Log the request body
	log.Printf("[POST /control/capture] card=%s control=%s", cardStr, control)

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
	// Use the corresponding switch control
	switchControl := strings.Replace(control, " Volume", " Switch", 1)
	currentMuted, err := m.GetMute(cardID, switchControl)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get capture state: %v", err), http.StatusInternalServerError)
		return
	}
	currentActive := !currentMuted

	newActive := !currentActive
	newMuted := !newActive

	if err := m.SetMute(cardID, switchControl, newMuted); err != nil {
		http.Error(w, fmt.Sprintf("failed to set capture state: %v", err), http.StatusInternalServerError)
		return
	}

	if s.hub != nil {
		ctrl := s.getControlView(cardID, control)
		if ctrl != nil {
			// Log the SSE broadcast (compact JSON)
			log.Printf("[SSE broadcast] %s", compactEventData(ctrl))
			// Broadcast mixer-update style event for JS-only clients
			go s.hub.Broadcast(sse.Event{
				Type: "mixer-update",
				Data: map[string]interface{}{
					"state": map[string]interface{}{
						fmt.Sprintf("%d", cardID): map[string]interface{}{
							control: map[string]interface{}{
								"Volume": []int{ctrl.VolumeNow},
								"Mute":   newMuted, // Capture is inverse of mute
							},
						},
					},
					"source":  "handler",
					"control": control,
				},
			})
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
