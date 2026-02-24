// Package alsa provides an abstraction layer for ALSA mixer operations.
//go:build linux

package alsa

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
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
	Min     int64  // Minimum raw value
	Max     int64  // Maximum raw value
	Step    int64  // Step size for percentage calculation
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
			// Calculate step: percentage value per ALSA unit
			if max > min {
				ctrl.Step = int64(100 / (max - min))
			}
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

// GetVolume retrieves the current volume levels for a control.
// Returns a slice of percentage values, one per channel.
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
	if max == min {
		return nil, fmt.Errorf("control '%s' has invalid range (min equals max)", control)
	}

	numChannels := int(ctl.NumValues())
	values := make([]int, numChannels)

	// Read all channel values using Array
	rawValues := make([]int32, numChannels)
	if err := ctl.Array(&rawValues); err != nil {
		// Fall back to reading each channel individually
		for i := 0; i < numChannels; i++ {
			val, err := ctl.Value(uint(i))
			if err != nil {
				return nil, fmt.Errorf("failed to get channel %d value: %w", i, err)
			}
			values[i] = int((val - min) * 100 / (max - min))
		}
		return values, nil
	}

	for i := 0; i < numChannels; i++ {
		values[i] = int((int(rawValues[i]) - min) * 100 / (max - min))
	}

	return values, nil
}

// SetVolume sets the volume levels for a control.
// When a single value is provided, it is applied to ALL channels (matching alsamixer behavior).
// When multiple values are provided, each value is applied to its corresponding channel.
func (m *Mixer) SetVolume(card uint, control string, values []int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.open {
		return fmt.Errorf("mixer is closed")
	}

	if len(values) == 0 {
		return fmt.Errorf("no volume values provided")
	}

	// Convert control name from UI format (e.g., "Speaker Playback Volume") to
	// ALSA format (e.g., "Speaker")
	alsaControl := control
	// Remove " Playback Volume" or " Volume" suffix if present
	if strings.HasSuffix(alsaControl, " Playback Volume") {
		alsaControl = strings.TrimSuffix(alsaControl, " Playback Volume")
	} else if strings.HasSuffix(alsaControl, " Volume") {
		alsaControl = strings.TrimSuffix(alsaControl, " Volume")
	}

	// Use amixer command-line tool which correctly sets all channels
	// IMPORTANT: Always use % suffix for percentage-based values
	// amixer interprets values differently:
	//   - value < 100 without %: treated as raw value
	//   - value with % suffix: treated as percentage
	//   - 100 without %: treated as 100% (special case)
	// Since UI works in percentages, always add % suffix for consistency
	cmd := exec.Command("amixer", "-c", fmt.Sprintf("%d", card), "sset", alsaControl)
	if len(values) == 1 {
		// Single value: set both/all channels to the same percentage
		cmd.Args = append(cmd.Args, fmt.Sprintf("%d%%", values[0]))
	} else {
		// Multiple values: set each channel as percentage
		var channelVals []string
		for _, v := range values {
			channelVals = append(channelVals, fmt.Sprintf("%d%%", v))
		}
		cmd.Args = append(cmd.Args, strings.Join(channelVals, ","))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("SetVolume: amixer failed for '%s': %v output: %s", alsaControl, err, string(output))
		// Fall back to library method
		return m.setVolumeLibrary(card, control, values)
	}

	return nil
}

// setVolumeLibrary is the fallback volume setter using the alsa library
func (m *Mixer) setVolumeLibrary(card uint, control string, values []int) error {
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
	if max == min {
		return fmt.Errorf("control '%s' has invalid range (min equals max)", control)
	}

	numChannels := int(ctl.NumValues())

	// Set each channel individually
	if len(values) == 1 {
		raw := min + (values[0]*(max-min))/100
		for i := 0; i < numChannels; i++ {
			if err := ctl.SetValue(uint(i), raw); err != nil {
				return fmt.Errorf("failed to set channel %d: %w", i, err)
			}
		}
	} else {
		for i := 0; i < numChannels && i < len(values); i++ {
			raw := min + (values[i]*(max-min))/100
			if err := ctl.SetValue(uint(i), raw); err != nil {
				return fmt.Errorf("failed to set channel %d: %w", i, err)
			}
		}
	}

	return nil
}

// GetMute retrieves the mute state for a control.
// Returns true if ALL channels are muted, false otherwise.
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

	numChannels := int(ctl.NumValues())

	// Check if ALL channels are muted
	for i := 0; i < numChannels; i++ {
		val, err := ctl.Value(uint(i))
		if err != nil {
			return false, fmt.Errorf("failed to get channel %d value: %w", i, err)
		}
		if val != 0 {
			// At least one channel is not muted
			return false, nil
		}
	}

	return true, nil
}

// SetMute sets the mute state for a control on ALL channels.
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

	val := 1
	if muted {
		val = 0
	}

	numChannels := int(ctl.NumValues())
	for i := 0; i < numChannels; i++ {
		if err := ctl.SetValue(uint(i), val); err != nil {
			return fmt.Errorf("failed to set channel %d mute: %w", i, err)
		}
	}

	return nil
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
