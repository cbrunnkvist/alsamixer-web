package alsa

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Hub interface for broadcasting events
type Hub interface {
	ClientCount() int
	Broadcast(event interface{})
}

// Monitor watches for ALSA mixer state changes and broadcasts them via SSE
type Monitor struct {
	mixer       *Mixer
	hub         Hub
	ticker      *time.Ticker
	stopCh      chan struct{}
	wg          sync.WaitGroup
	lastState   *StateSnapshot
	mu          sync.Mutex
	watcher     *fsnotify.Watcher
	configPaths []string
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
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("failed to create file watcher: %v", err)
	}

	monitor := &Monitor{
		mixer:   mixer,
		hub:     hub,
		stopCh:  make(chan struct{}),
		watcher: watcher,
		configPaths: []string{
			filepath.Join(os.Getenv("HOME"), ".asoundrc"),
			"/etc/asound.conf",
			// Add more paths if needed, e.g., for /etc/asound.conf.d/
		},
	}

	// Add config files to watcher
	for _, path := range monitor.configPaths {
		if _, err := os.Stat(path); err == nil {
			if err := monitor.watcher.Add(path); err != nil {
				log.Printf("failed to watch %s: %v", path, err)
			}
		} else if os.IsNotExist(err) {
			log.Printf("config file not found: %s, skipping watch", path)
		} else {
			log.Printf("error stating config file %s: %v", path, err)
		}
	}

	return monitor
}

// Start begins monitoring ALSA state changes
func (m *Monitor) Start() {
	m.wg.Add(1)
	go m.monitorLoop()
	m.wg.Add(1)
	go m.configWatcherLoop()
	log.Println("ALSA monitor started")
}

// Stop halts the monitoring loop
func (m *Monitor) Stop() {
	close(m.stopCh)
	m.watcher.Close()
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

// configWatcherLoop watches ALSA config files for changes and broadcasts an SSE event
func (m *Monitor) configWatcherLoop() {
	defer m.wg.Done()

	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("ALSA config file changed: %s", event.Name)
				if m.hub != nil {
					m.hub.Broadcast(map[string]interface{}{
						"type": "config-change",
						"path": event.Name,
					})
				}
			}
		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("error watching config files: %v", err)
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
