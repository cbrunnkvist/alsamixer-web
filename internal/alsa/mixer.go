// Package alsa provides an abstraction layer for ALSA mixer operations.
//go:build linux

package alsa

import (
	"fmt"
	"sync"

	"github.com/gen2brain/alsa"
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
	return &Mixer{
		open: true,
	}
}

// ListCards enumerates all available sound cards
func (m *Mixer) ListCards() ([]Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.open {
		return nil, fmt.Errorf("mixer is closed")
	}

	var cards []Card
	cardIndex := uint(0)

	for {
		handle, err := alsa.Open("hw", cardIndex, 0, 0)
		if err != nil {
			// No more cards available
			break
		}
		defer handle.Close()

		cardName, err := handle.CardName()
		if err != nil {
			cardName = fmt.Sprintf("Card %d", cardIndex)
		}

		cards = append(cards, Card{
			ID:   cardIndex,
			Name: cardName,
		})

		cardIndex++
		if cardIndex > 100 {
			// Safety limit to prevent infinite loops
			break
		}
	}

	if len(cards) == 0 {
		return nil, fmt.Errorf("no sound cards found")
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

	handle, err := alsa.Open("hw", card, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open card %d: %w", card, err)
	}
	defer handle.Close()

	var controls []Control
	elem, err := handle.FirstElem()
	if err != nil {
		return nil, fmt.Errorf("failed to get first control: %w", err)
	}

	for {
		if elem == nil {
			break
		}

		info, err := elem.Info()
		if err != nil {
			// Skip controls we can't get info for
			elem, _ = elem.Next()
			continue
		}

		control := Control{
			Name:  info.Name,
			Count: info.Count,
		}

		// Determine control type and value range
		switch info.Type {
		case alsa.ElemTypeInteger:
			control.Type = "integer"
			if value, err := elem.Value(); err == nil && len(value) > 0 {
				if intValue, ok := value[0].(int64); ok {
					control.Min = 0
					control.Max = 100 // Default range for percentage
					// Try to get actual min/max if available
					if min, max, err := elem.GetRange(); err == nil {
						control.Min = min
						control.Max = max
					}
					// Check if this is a mute control
					if info.Name == "Mute" || info.Name == "Capture Switch" {
						control.IsMuted = intValue == 0
					}
				}
			}
		case alsa.ElemTypeBoolean:
			control.Type = "boolean"
			if value, err := elem.Value(); err == nil && len(value) > 0 {
				if boolValue, ok := value[0].(bool); ok {
					control.IsMuted = boolValue
				}
			}
		default:
			control.Type = "unknown"
		}

		controls = append(controls, control)

		elem, err = elem.Next()
		if err != nil {
			break
		}
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

	handle, err := alsa.Open("hw", card, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open card %d: %w", card, err)
	}
	defer handle.Close()

	elem, err := handle.FindElem(control)
	if err != nil {
		return nil, fmt.Errorf("control '%s' not found on card %d: %w", control, card, err)
	}

	values, err := elem.Value()
	if err != nil {
		return nil, fmt.Errorf("failed to get volume for control '%s': %w", control, err)
	}

	// Convert to percentage (0-100)
	var volumes []int
	for _, v := range values {
		if intValue, ok := v.(int64); ok {
			// Try to get the actual range
			if min, max, err := elem.GetRange(); err == nil && max > min {
				// Convert to percentage
				percentage := int((intValue - min) * 100 / (max - min))
				if percentage < 0 {
					percentage = 0
				} else if percentage > 100 {
					percentage = 100
				}
				volumes = append(volumes, percentage)
			} else {
				// Fallback: assume 0-100 range
				if intValue < 0 {
					intValue = 0
				} else if intValue > 100 {
					intValue = 100
				}
				volumes = append(volumes, int(intValue))
			}
		}
	}

	return volumes, nil
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

	handle, err := alsa.Open("hw", card, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to open card %d: %w", card, err)
	}
	defer handle.Close()

	elem, err := handle.FindElem(control)
	if err != nil {
		return fmt.Errorf("control '%s' not found on card %d: %w", control, card, err)
	}

	// Get the actual range
	min, max, err := elem.GetRange()
	if err != nil {
		min = 0
		max = 100
	}

	// Convert percentage to ALSA values
	alsaValues := make([]interface{}, len(values))
	for i, v := range values {
		if v < 0 {
			v = 0
		} else if v > 100 {
			v = 100
		}
		// Convert percentage to ALSA range
		alsaValues[i] = min + (int64(v) * (max - min) / 100)
	}

	if err := elem.SetValue(alsaValues...); err != nil {
		return fmt.Errorf("failed to set volume for control '%s': %w", control, err)
	}

	return nil
}

// GetMute retrieves the mute state for a control
func (m *Mixer) GetMute(card uint, control string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.open {
		return false, fmt.Errorf("mixer is closed")
	}

	handle, err := alsa.Open("hw", card, 0, 0)
	if err != nil {
		return false, fmt.Errorf("failed to open card %d: %w", card, err)
	}
	defer handle.Close()

	// Try to find a mute control first
	muteControl := control + " Switch"
	elem, err := handle.FindElem(muteControl)
	if err != nil {
		// Try alternative mute control names
		muteControls := []string{control + " Switch", "Capture Switch", "Master Switch", "Mute"}
		for _, mc := range muteControls {
			if elem, err = handle.FindElem(mc); err == nil {
				break
			}
		}
		if err != nil {
			// No mute control found, assume not muted
			return false, nil
		}
	}

	values, err := elem.Value()
	if err != nil {
		return false, fmt.Errorf("failed to get mute state for control '%s': %w", control, err)
	}

	if len(values) == 0 {
		return false, fmt.Errorf("no mute state available for control '%s'", control)
	}

	// Check the first channel's mute state
	switch v := values[0].(type) {
	case bool:
		return v, nil
	case int64:
		return v == 0, nil // 0 typically means muted in ALSA
	default:
		return false, fmt.Errorf("unexpected mute value type for control '%s'", control)
	}
}

// SetMute sets the mute state for a control
func (m *Mixer) SetMute(card uint, control string, muted bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.open {
		return fmt.Errorf("mixer is closed")
	}

	handle, err := alsa.Open("hw", card, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to open card %d: %w", card, err)
	}
	defer handle.Close()

	// Try to find a mute control first
	muteControl := control + " Switch"
	elem, err := handle.FindElem(muteControl)
	if err != nil {
		// Try alternative mute control names
		muteControls := []string{control + " Switch", "Capture Switch", "Master Switch", "Mute"}
		for _, mc := range muteControls {
			if elem, err = handle.FindElem(mc); err == nil {
				break
			}
		}
		if err != nil {
			return fmt.Errorf("no mute control found for '%s' on card %d", control, card)
		}
	}

	// Get current values to determine the type
	values, err := elem.Value()
	if err != nil {
		return fmt.Errorf("failed to get current mute values for control '%s': %w", control, err)
	}

	if len(values) == 0 {
		return fmt.Errorf("no mute control channels found for '%s'", control)
	}

	// Set mute state for all channels
	newValues := make([]interface{}, len(values))
	for i := range values {
		switch values[i].(type) {
		case bool:
			newValues[i] = muted
		case int64:
			if muted {
				newValues[i] = int64(0) // 0 typically means muted
			} else {
				newValues[i] = int64(1) // 1 typically means unmuted
			}
		default:
			return fmt.Errorf("unsupported mute control type for '%s'", control)
		}
	}

	if err := elem.SetValue(newValues...); err != nil {
		return fmt.Errorf("failed to set mute state for control '%s': %w", control, err)
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
