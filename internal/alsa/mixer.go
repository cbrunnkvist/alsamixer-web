// Package alsa provides an abstraction layer for ALSA mixer operations.
//go:build linux

package alsa

import (
	"fmt"
	"log"
	"sync"

	alsalib "github.com/gen2brain/alsa"
)

// Card represents an ALSA sound card
type Card struct {
	ID   uint   // Card index
	Name string // Card name
}

// Control represents an ALSA mixer control
type Control struct {
	Name    string // Control name
	Type    string // Control type (e.g., "integer", "boolean")
	Min     int64  // Minimum value
	Max     int64  // Maximum value
	Count   int    // Number of channels
	IsMuted bool   // Mute state (if applicable)
}

// Mixer provides an abstraction layer for ALSA mixer operations
type Mixer struct {
	mu   sync.Mutex
	open bool
}

// NewMixer creates a new ALSA mixer instance
func NewMixer() *Mixer {
	if _, err := alsalib.EnumerateCards(); err != nil {
		log.Printf("WARNING: ALSA enumeration failed: %v", err)
	}

	return &Mixer{open: true}
}

// ListCards enumerates all available sound cards
func (m *Mixer) ListCards() ([]Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.open {
		return nil, fmt.Errorf("mixer is closed")
	}

	soundCards, err := alsalib.EnumerateCards()
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate cards: %w", err)
	}

	if len(soundCards) == 0 {
		return nil, fmt.Errorf("no sound cards found")
	}

	cards := make([]Card, 0, len(soundCards))
	for _, c := range soundCards {
		cards = append(cards, Card{ID: uint(c.ID), Name: c.Name})
	}

	return cards, nil
}

// ListControls enumerates all mixer controls for a given card
func (m *Mixer) ListControls(card uint) ([]Control, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.open {
		return nil, fmt.Errorf("mixer is closed")
	}

	mixer, err := alsalib.MixerOpen(card)
	if err != nil {
		return nil, fmt.Errorf("failed to open mixer for card %d: %w", card, err)
	}
	defer mixer.Close()

	var controls []Control
	for i := 0; i < mixer.NumCtls(); i++ {
		ctl, err := mixer.CtlByIndex(uint(i))
		if err != nil {
			continue
		}

		ctrl := Control{Name: ctl.Name(), Count: int(ctl.NumValues())}

		switch ctl.Type() {
		case alsalib.SNDRV_CTL_ELEM_TYPE_INTEGER:
			ctrl.Type = "integer"
			min, _ := ctl.RangeMin()
			max, _ := ctl.RangeMax()
			ctrl.Min = int64(min)
			ctrl.Max = int64(max)
		case alsalib.SNDRV_CTL_ELEM_TYPE_BOOLEAN:
			ctrl.Type = "boolean"
		default:
			continue
		}

		controls = append(controls, ctrl)
	}

	if len(controls) == 0 {
		return nil, fmt.Errorf("no controls found for card %d", card)
	}

	return controls, nil
}

// GetVolume retrieves the current volume levels for a control
func (m *Mixer) GetVolume(card uint, control string) ([]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.open {
		return nil, fmt.Errorf("mixer is closed")
	}

	mixer, err := alsalib.MixerOpen(card)
	if err != nil {
		return nil, fmt.Errorf("failed to open mixer: %w", err)
	}
	defer mixer.Close()

	ctl, err := mixer.CtlByName(control)
	if err != nil {
		return nil, fmt.Errorf("control '%s' not found: %w", control, err)
	}

	min, _ := ctl.RangeMin()
	max, _ := ctl.RangeMax()
	val, err := ctl.Value(0)
	if err != nil {
		return nil, err
	}

	percent := int((val - min) * 100 / (max - min))
	return []int{percent}, nil
}

// SetVolume sets the volume levels for a control
func (m *Mixer) SetVolume(card uint, control string, values []int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.open {
		return fmt.Errorf("mixer is closed")
	}

	if len(values) == 0 {
		return fmt.Errorf("no volume values provided")
	}

	mixer, err := alsalib.MixerOpen(card)
	if err != nil {
		return fmt.Errorf("failed to open mixer: %w", err)
	}
	defer mixer.Close()

	ctl, err := mixer.CtlByName(control)
	if err != nil {
		return err
	}

	min, _ := ctl.RangeMin()
	max, _ := ctl.RangeMax()
	v := values[0]
	raw := min + (v*(max-min))/100
	return ctl.SetValue(0, raw)
}

// GetMute retrieves the mute state for a control
func (m *Mixer) GetMute(card uint, control string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.open {
		return false, fmt.Errorf("mixer is closed")
	}

	mixer, err := alsalib.MixerOpen(card)
	if err != nil {
		return false, err
	}
	defer mixer.Close()

	ctl, err := mixer.CtlByName(control)
	if err != nil {
		return false, fmt.Errorf("control not found: %s", control)
	}

	if ctl.Type() != alsalib.SNDRV_CTL_ELEM_TYPE_BOOLEAN {
		return false, fmt.Errorf("control '%s' is not boolean (type: %v)", control, ctl.Type())
	}

	val, err := ctl.Value(0)
	if err != nil {
		return false, err
	}

	return val != 0, nil
}

// SetMute sets the mute state for a control
func (m *Mixer) SetMute(card uint, control string, muted bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.open {
		return fmt.Errorf("mixer is closed")
	}

	mixer, err := alsalib.MixerOpen(card)
	if err != nil {
		return err
	}
	defer mixer.Close()

	ctl, err := mixer.CtlByName(control)
	if err != nil {
		return fmt.Errorf("control not found: %s", control)
	}

	if ctl.Type() != alsalib.SNDRV_CTL_ELEM_TYPE_BOOLEAN {
		return fmt.Errorf("control '%s' is not boolean (type: %v)", control, ctl.Type())
	}

	val := 0
	if muted {
		val = 1
	}
	return ctl.SetValue(0, val)
}

// Close cleans up resources and marks the mixer as closed
func (m *Mixer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.open {
		return fmt.Errorf("mixer already closed")
	}

	m.open = false
	return nil
}

// IsOpen returns whether the mixer is open and ready for operations
func (m *Mixer) IsOpen() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.open
}
