package alsa

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/user/alsamixer-web/internal/sse"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

type Hub interface {
	ClientCount() int
	Broadcast(event sse.Event)
}

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

type StateSnapshot struct {
	Cards map[uint]CardState
}

type CardState struct {
	Controls map[string]ControlState
}

type ControlState struct {
	Volume []int
	Mute   bool
}

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

func (m *Monitor) Start() {
	m.wg.Add(1)
	go m.monitorLoop()
	m.wg.Add(1)
	go m.configWatcherLoop()
	log.Println("ALSA monitor started")
}

func (m *Monitor) Stop() {
	close(m.stopCh)
	m.watcher.Close()
	m.wg.Wait()
	log.Println("ALSA monitor stopped")
}

func (m *Monitor) monitorLoop() {
	defer m.wg.Done()

	log.Printf("ALSA monitor loop started")

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentState := m.getCurrentState()
			if currentState == nil {
				continue
			}

			m.mu.Lock()
			lastState := m.lastState
			changed, delta := m.computeDelta(currentState, lastState)
			if changed {
				clients := m.hub.ClientCount()
				log.Printf("ALSA state changed, broadcasting delta to %d clients", clients)
				m.lastState = currentState
				m.mu.Unlock()
				m.broadcastDelta(delta)
			} else {
				m.mu.Unlock()
			}

		case <-m.stopCh:
			log.Printf("ALSA monitor: stop signal received")
			return
		}
	}
}

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
			if control.Type != "integer" && control.Type != "boolean" {
				continue
			}

			controlState := ControlState{}

			if control.Type == "integer" {
				volume, err := m.mixer.GetVolume(card.ID, control.Name)
				if err != nil {
					log.Printf("Failed to get volume for %s on card %d: %v", control.Name, card.ID, err)
					continue
				}
				controlState.Volume = volume
			}

			switchControlName := strings.Replace(control.Name, " Volume", " Switch", 1)
			mute, err := m.mixer.GetMute(card.ID, switchControlName)
			if err != nil {
				mute = false
			}
			controlState.Mute = mute

			cardState.Controls[control.Name] = controlState
		}

		snapshot.Cards[card.ID] = cardState
	}

	return snapshot
}

// computeDelta compares current and last state, returning only what changed
func (m *Monitor) computeDelta(current, last *StateSnapshot) (bool, *StateSnapshot) {
	if last == nil {
		return true, current
	}

	delta := &StateSnapshot{
		Cards: make(map[uint]CardState),
	}
	hasChanges := false

	for cardID, currentCard := range current.Cards {
		lastCard, exists := last.Cards[cardID]
		if !exists {
			delta.Cards[cardID] = currentCard
			hasChanges = true
			continue
		}

		cardDelta := CardState{
			Controls: make(map[string]ControlState),
		}
		cardHasChanges := false

		for controlName, currentControl := range currentCard.Controls {
			lastControl, exists := lastCard.Controls[controlName]
			if !exists {
				cardDelta.Controls[controlName] = currentControl
				cardHasChanges = true
				continue
			}

			volumeChanged := len(currentControl.Volume) != len(lastControl.Volume)
			if !volumeChanged {
				for i, v := range currentControl.Volume {
					if i >= len(lastControl.Volume) || v != lastControl.Volume[i] {
						volumeChanged = true
						break
					}
				}
			}

			muteChanged := currentControl.Mute != lastControl.Mute

			if volumeChanged || muteChanged {
				cardDelta.Controls[controlName] = currentControl
				cardHasChanges = true
			}
		}

		if cardHasChanges {
			delta.Cards[cardID] = cardDelta
			hasChanges = true
		}
	}

	if !hasChanges {
		return false, nil
	}

	return true, delta
}

func (m *Monitor) broadcastDelta(delta *StateSnapshot) {
	m.hub.Broadcast(sse.Event{Type: "mixer-update", Data: map[string]interface{}{
		"state":     delta,
		"source":    "monitor",
		"timestamp": time.Now().Unix(),
	}})
}
