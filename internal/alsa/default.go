//go:build linux

package alsa

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

// GetDefaultCard determines the default ALSA card index.
// Priority: ALSA_CARD env > ~/.asoundrc > /etc/asound.conf > first hardware card
// Returns the card index, or -1 if no preference is configured.
func GetDefaultCard() int {
	// 1. Check ALSA_CARD environment variable (highest priority)
	if cardStr := os.Getenv("ALSA_CARD"); cardStr != "" {
		if card, err := strconv.Atoi(cardStr); err == nil && card >= 0 {
			return card
		}
	}

	// 2. Check ~/.asoundrc (user config)
	home := os.Getenv("HOME")
	if home != "" {
		if card := parseAsoundrc(home + "/.asoundrc"); card >= 0 {
			return card
		}
	}

	// 3. Check /etc/asound.conf (system config)
	if card := parseAsoundrc("/etc/asound.conf"); card >= 0 {
		return card
	}

	// 4. No preference configured
	return -1
}

// parseAsoundrc parses an ALSA config file for default card settings.
// Returns the card index, or -1 if not found.
func parseAsoundrc(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1
	}

	content := string(data)

	// Match patterns like:
	//   defaults.pcm.card 1
	//   defaults.ctl.card 1
	//   defaults.pcm.card "PCH"
	re := regexp.MustCompile(`(?m)^\s*defaults\.(?:pcm|ctl)\.card\s+(\S+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		// Try parsing as integer
		if card, err := strconv.Atoi(matches[1]); err == nil && card >= 0 {
			return card
		}
		// Could be a card name like "PCH" - we'd need to resolve it
		// For now, skip named references
	}

	return -1
}

// ResolveDefaultCard returns the actual card ID to use.
// If defaultCard is -1 (no preference), returns the first non-Loopback card,
// or falls back to card 0 if no better option exists.
func ResolveDefaultCard(cards []Card, defaultCard int) uint {
	// If explicit default is configured, use it
	if defaultCard >= 0 {
		for _, c := range cards {
			if int(c.ID) == defaultCard {
				return c.ID
			}
		}
	}

	// No preference: prefer first non-Loopback, non-software card
	for _, c := range cards {
		name := strings.ToLower(c.Name)
		// Skip loopback and software cards
		if !strings.Contains(name, "loopback") &&
			!strings.Contains(name, "null") &&
			!strings.Contains(name, "dummy") {
			return c.ID
		}
	}

	// Fallback: return first card if available
	if len(cards) > 0 {
		return cards[0].ID
	}

	return 0
}
