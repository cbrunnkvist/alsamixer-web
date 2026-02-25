//go:build !linux

package alsa

import "fmt"

// Card represents an ALSA sound card (stub implementation for non-Linux platforms).
type Card struct {
	ID   uint
	Name string
}

// Control represents an ALSA mixer control (stub implementation for non-Linux platforms).
type Control struct {
	Name    string
	Type    string
	Min     int64
	Max     int64
	Step    int64
	Count   int
	IsMuted bool
}

// Mixer is a no-op stub used on platforms where ALSA is not available.
type Mixer struct{}

// NewMixer creates a stub mixer.
func NewMixer() *Mixer { return &Mixer{} }

// ListCards returns an error indicating ALSA is unavailable.
func (m *Mixer) ListCards() ([]Card, error) {
	return nil, fmt.Errorf("alsa mixer is not supported on this platform")
}

// ListControls returns an error indicating ALSA is unavailable.
func (m *Mixer) ListControls(card uint) ([]Control, error) {
	return nil, fmt.Errorf("alsa mixer is not supported on this platform")
}

// GetVolume returns an error indicating ALSA is unavailable.
func (m *Mixer) GetVolume(card uint, control string) ([]int, error) {
	return nil, fmt.Errorf("alsa mixer is not supported on this platform")
}

// SetVolume returns an error indicating ALSA is unavailable.
func (m *Mixer) SetVolume(card uint, control string, values []int) error {
	return fmt.Errorf("alsa mixer is not supported on this platform")
}

// GetMute returns an error indicating ALSA is unavailable.
func (m *Mixer) GetMute(card uint, control string) (bool, error) {
	return false, fmt.Errorf("alsa mixer is not supported on this platform")
}

// SetMute returns an error indicating ALSA is unavailable.
func (m *Mixer) SetMute(card uint, control string, muted bool) error {
	return fmt.Errorf("alsa mixer is not supported on this platform")
}

// Close is a no-op for the stub mixer.
func (m *Mixer) Close() error { return nil }

// IsOpen always reports false for the stub mixer.
func (m *Mixer) IsOpen() bool { return false }

// HasPlaybackVolume returns false with an error for stub.
func (m *Mixer) HasPlaybackVolume(card uint, control string) (bool, error) {
	return false, fmt.Errorf("alsa mixer is not supported on this platform")
}

// HasPlaybackSwitch returns false with an error for stub.
func (m *Mixer) HasPlaybackSwitch(card uint, control string) (bool, error) {
	return false, fmt.Errorf("alsa mixer is not supported on this platform")
}

// HasCaptureVolume returns false with an error for stub.
func (m *Mixer) HasCaptureVolume(card uint, control string) (bool, error) {
	return false, fmt.Errorf("alsa mixer is not supported on this platform")
}

// HasCaptureSwitch returns false with an error for stub.
func (m *Mixer) HasCaptureSwitch(card uint, control string) (bool, error) {
	return false, fmt.Errorf("alsa mixer is not supported on this platform")
}
