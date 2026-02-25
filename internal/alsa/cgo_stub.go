//go:build !linux || !cgo

package alsa

import "fmt"

// getControlNamesInOrder returns an error when CGo/Linux is not available.
// The main logic in ListControls falls back to library order when this fails.
func (m *Mixer) getControlNamesInOrder(card uint) ([]string, error) {
	return nil, fmt.Errorf("cgo/libasound not available")
}
