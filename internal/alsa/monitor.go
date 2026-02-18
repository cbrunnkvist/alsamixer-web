package alsa

import (
	"log"
	"sync"
	"time"
)

// Hub interface for broadcasting events
type Hub interface {
	ClientCount() int
	Broadcast(event interface{})
}

// Monitor watches for ALSA mixer state changes and broadcasts them via SSE
type Monitor struct {
	mixer     *Mixer
	hub       Hub
	ticker    *time.Ticker
	stopCh    chan struct{}
	wg        sync.WaitGroup
	lastState *StateSnapshot
	mu        sync.Mutex
}

// StateSnapshot represents a snapshot of ALSA mixer state for comparison
type StateSnapshot struct {
	Cards map[uint]CardState
}

// CardState represents the state of a single card's controls
type CardState struct {
	Controls map[string]ControlState
}

// ControlState represents the state of a single control
type ControlState struct {
	Volume []int
	Mute   bool
}

// NewMonitor creates a new ALSA monitor instance
func NewMonitor(mixer *Mixer, hub Hub) *Monitor {
	return &Monitor{
		mixer:  mixer,
		hub:    hub,
		stopCh: make(chan struct{}),
	}
}

// Start begins monitoring ALSA state changes
func (m *Monitor) Start() {
	m.wg.Add(1)
	go m.monitorLoop()
	log.Println("ALSA monitor started")
}

// Stop halts the monitoring loop
func (m *Monitor) Stop() {
	close(m.stopCh)
	m.wg.Wait()
	log.Println("ALSA monitor stopped")
}

// monitorLoop is the main polling loop that checks for ALSA state changes
func (m *Monitor) monitorLoop() {
	defer m.wg.Done()

	// Create ticker for 100ms intervals
	m.ticker = time.NewTicker(100 * time.Millisecond)
	defer m.ticker.Stop()

	for {
		select {
		case <-m.ticker.C:
			// Only poll if clients are connected
			if m.hub.ClientCount() == 0 {
				continue
			}

			// Get current state and compare with last state
			currentState := m.getCurrentState()
			if currentState == nil {
				continue
			}

			m.mu.Lock()
			if m.hasStateChanged(currentState) {
				m.lastState = currentState
				m.mu.Unlock()

				// Broadcast the change
				m.broadcastStateChange(currentState)
			} else {
				m.mu.Unlock()
			}

		case <-m.stopCh:
			return
		}
	}
}

// getCurrentState captures the current ALSA mixer state
func (m *Monitor) getCurrentState() *StateSnapshot {
	cards, err := m.mixer.ListCards()
	if err != nil {
		log.Printf("Failed to list cards: %v", err)
		return nil
	}

	snapshot := &StateSnapshot{
		Cards: make(map[uint]CardState),
	}

	for _, card := range cards {
		controls, err := m.mixer.ListControls(card.ID)
		if err != nil {
			log.Printf("Failed to list controls for card %d: %v", card.ID, err)
			continue
		}

		cardState := CardState{
			Controls: make(map[string]ControlState),
		}

		for _, control := range controls {
			// Skip controls that aren't volume-related
			if control.Type != "integer" && control.Type != "boolean" {
				continue
			}

			controlState := ControlState{}

			// Get volume if it's an integer control
			if control.Type == "integer" {
				volume, err := m.mixer.GetVolume(card.ID, control.Name)
				if err != nil {
					log.Printf("Failed to get volume for %s on card %d: %v", control.Name, card.ID, err)
					continue
				}
				controlState.Volume = volume
			}

			// Get mute state
			mute, err := m.mixer.GetMute(card.ID, control.Name)
			if err != nil {
				// Not all controls have mute, that's okay
				mute = false
			}
			controlState.Mute = mute

			cardState.Controls[control.Name] = controlState
		}

		snapshot.Cards[card.ID] = cardState
	}

	return snapshot
}

// hasStateChanged compares the current state with the last state
func (m *Monitor) hasStateChanged(current *StateSnapshot) bool {
	m.mu.Lock()
	last := m.lastState
	m.mu.Unlock()

	if last == nil {
		return true // First time capturing state
	}

	// Compare card counts
	if len(current.Cards) != len(last.Cards) {
		return true
	}

	// Compare each card's controls
	for cardID, currentCard := range current.Cards {
		lastCard, exists := last.Cards[cardID]
		if !exists {
			return true
		}

		// Compare control counts
		if len(currentCard.Controls) != len(lastCard.Controls) {
			return true
		}

		// Compare each control's state
		for controlName, currentControl := range currentCard.Controls {
			lastControl, exists := lastCard.Controls[controlName]
			if !exists {
				return true
			}

			// Compare volume arrays
			if len(currentControl.Volume) != len(lastControl.Volume) {
				return true
			}
			for i, v := range currentControl.Volume {
				if i >= len(lastControl.Volume) || v != lastControl.Volume[i] {
					return true
				}
			}

			// Compare mute state
			if currentControl.Mute != lastControl.Mute {
				return true
			}
		}
	}

	return false
}

// broadcastStateChange sends the state change event to all connected clients
func (m *Monitor) broadcastStateChange(snapshot *StateSnapshot) {
	event := map[string]interface{}{
		"type":      "mixer-update",
		"state":     snapshot,
		"timestamp": time.Now().Unix(),
	}

	m.hub.Broadcast(event)
}
