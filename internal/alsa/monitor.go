package alsa

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/user/alsamixer-web/internal/sse"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

// Hub interface for broadcasting events
type Hub interface {
	ClientCount() int
	Broadcast(event sse.Event)
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
func NewMonitor(mixer *Mixer, hub Hub, monitorFile string) *Monitor {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("failed to create file watcher: %v", err)
	}

	paths := []string{}
	if monitorFile != "" {
		paths = append(paths, monitorFile)
	}

	monitor := &Monitor{
		mixer:       mixer,
		hub:         hub,
		stopCh:      make(chan struct{}),
		watcher:     watcher,
		configPaths: paths,
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

	log.Printf("ALSA monitor loop started, hub client count: %d", m.hub.ClientCount())

	// Create ticker for 100ms intervals
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	log.Printf("ALSA monitor: ticker created, starting select loop")

	// Always poll ALSA to detect external changes, regardless of client count.
	// This ensures changes made via amixer or other tools are detected.
	// The client count check was causing the monitor to skip polling when
	// ClientCount() returned 0 even when clients were connected.

	tickCount := 0
	log.Println("ALSA monitor: about to enter select loop")
	for {
		select {
		case <-ticker.C:
			log.Println("ALSA monitor: tick fired!")
			tickCount++

			clients := m.hub.ClientCount()

			// Log every 50 ticks (5 seconds) for debugging
			if tickCount%50 == 0 {
				log.Printf("ALSA monitor: tick %d, clients=%d", tickCount, clients)
			}

			// Always poll ALSA state to detect external changes
			// This is the key fix - don't skip polling based on client count

			// Get current state and compare with last state
			log.Println("ALSA monitor: calling getCurrentState...")
			currentState := m.getCurrentState()
			log.Printf("ALSA monitor: getCurrentState returned %v", currentState)
			log.Printf("ALSA monitor: about to check nil")
			if currentState == nil {
				log.Printf("ALSA monitor: getCurrentState returned nil - retrying in 1 second")
				time.Sleep(1 * time.Second)
				currentState = m.getCurrentState()
				log.Printf("ALSA monitor: retry getCurrentState returned %v", currentState)
				if currentState == nil {
					log.Printf("ALSA monitor: getCurrentState still nil, skipping tick")
					continue
				}
			}

			m.mu.Lock()
			log.Printf("ALSA monitor: got lock, checking state change")
			lastState := m.lastState
			changed := m.hasStateChanged(currentState, lastState)
			log.Printf("ALSA monitor: hasStateChanged returned %v", changed)
			if changed {
				log.Printf("ALSA monitor: state changed, broadcasting to %d clients", clients)
				m.lastState = currentState
				m.mu.Unlock()
				log.Printf("ALSA monitor: about to call broadcastStateChange")
				m.broadcastStateChange(currentState)
				log.Printf("ALSA monitor: broadcastStateChange returned")
			} else {
				m.mu.Unlock()
				log.Printf("ALSA monitor: state unchanged")
			}

		case <-m.stopCh:
			log.Printf("ALSA monitor: stop signal received")
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
					m.hub.Broadcast(sse.Event{Type: "config-change", Data: map[string]interface{}{
						"path": event.Name,
					}})
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
func (m *Monitor) hasStateChanged(current, last *StateSnapshot) bool {
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
	m.hub.Broadcast(sse.Event{Type: "mixer-update", Data: map[string]interface{}{
		"state":     snapshot,
		"timestamp": time.Now().Unix(),
	}})
}
