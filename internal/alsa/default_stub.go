//go:build !linux

package alsa

// GetDefaultCard returns -1 on non-Linux platforms (no ALSA).
func GetDefaultCard() int {
	return -1
}

// ResolveDefaultCard returns 0 on non-Linux platforms.
func ResolveDefaultCard(cards []Card, defaultCard int) uint {
	if len(cards) > 0 {
		return cards[0].ID
	}
	return 0
}
