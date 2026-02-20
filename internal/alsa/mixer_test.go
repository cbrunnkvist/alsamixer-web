//go:build linux

package alsa

import (
	"testing"
)

// TestNewMixer tests creating a new mixer instance
func TestNewMixer(t *testing.T) {
	mixer := NewMixer()
	if mixer == nil {
		t.Fatal("NewMixer() returned nil")
	}
	if !mixer.IsOpen() {
		t.Error("New mixer should be open")
	}
}

// TestMixerClose tests closing the mixer
func TestMixerClose(t *testing.T) {
	mixer := NewMixer()

	err := mixer.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if mixer.IsOpen() {
		t.Error("Mixer should be closed after Close()")
	}

	// Closing again should return an error
	err = mixer.Close()
	if err == nil {
		t.Error("Expected error when closing already closed mixer")
	}
}

// TestListCards tests listing sound cards
func TestListCards(t *testing.T) {
	mixer := NewMixer()
	defer mixer.Close()

	cards, err := mixer.ListCards()
	if err != nil {
		// Skip test if no ALSA hardware is available
		if err.Error() == "no sound cards found" {
			t.Skip("No ALSA sound cards found, skipping test")
		}
		t.Fatalf("ListCards() error = %v", err)
	}

	if len(cards) == 0 {
		t.Error("Expected at least one sound card")
	}

	// Verify card structure
	for _, card := range cards {
		if card.Name == "" {
			t.Error("Card name should not be empty")
		}
		t.Logf("Found card: ID=%d, Name=%s", card.ID, card.Name)
	}
}

// TestListControls tests listing mixer controls
func TestListControls(t *testing.T) {
	mixer := NewMixer()
	defer mixer.Close()

	// First get available cards
	cards, err := mixer.ListCards()
	if err != nil {
		t.Skipf("No cards available, skipping test: %v", err)
	}

	if len(cards) == 0 {
		t.Skip("No cards found, skipping test")
	}

	// Test with first card
	controls, err := mixer.ListControls(cards[0].ID)
	if err != nil {
		t.Fatalf("ListControls() error = %v", err)
	}

	if len(controls) == 0 {
		t.Error("Expected at least one control")
	}

	// Verify control structure
	for _, control := range controls {
		if control.Name == "" {
			t.Error("Control name should not be empty")
		}
		if control.Type == "" {
			t.Error("Control type should not be empty")
		}
		t.Logf("Found control: Name=%s, Type=%s, Min=%d, Max=%d, Count=%d, IsMuted=%v",
			control.Name, control.Type, control.Min, control.Max, control.Count, control.IsMuted)
	}
}

// TestGetVolume tests getting volume levels
func TestGetVolume(t *testing.T) {
	mixer := NewMixer()
	defer mixer.Close()

	// First get available cards and controls
	cards, err := mixer.ListCards()
	if err != nil {
		t.Skipf("No cards available, skipping test: %v", err)
	}

	if len(cards) == 0 {
		t.Skip("No cards found, skipping test")
	}

	controls, err := mixer.ListControls(cards[0].ID)
	if err != nil {
		t.Skipf("No controls available, skipping test: %v", err)
	}

	if len(controls) == 0 {
		t.Skip("No controls found, skipping test")
	}

	// Try to find a volume control
	var volumeControl string
	for _, ctrl := range controls {
		if ctrl.Type == "integer" && (ctrl.Name == "Master" || ctrl.Name == "PCM") {
			volumeControl = ctrl.Name
			break
		}
	}

	if volumeControl == "" {
		t.Skip("No suitable volume control found, skipping test")
	}

	volumes, err := mixer.GetVolume(cards[0].ID, volumeControl)
	if err != nil {
		t.Fatalf("GetVolume() error = %v", err)
	}

	if len(volumes) == 0 {
		t.Error("Expected at least one volume value")
	}

	// Verify volume values are in valid range (0-100)
	for i, vol := range volumes {
		if vol < 0 || vol > 100 {
			t.Errorf("Volume[%d] = %d, expected 0-100", i, vol)
		}
		t.Logf("Volume[%d] = %d%%", i, vol)
	}
}

// TestSetVolume tests setting volume levels
func TestSetVolume(t *testing.T) {
	mixer := NewMixer()
	defer mixer.Close()

	// First get available cards and controls
	cards, err := mixer.ListCards()
	if err != nil {
		t.Skipf("No cards available, skipping test: %v", err)
	}

	if len(cards) == 0 {
		t.Skip("No cards found, skipping test")
	}

	controls, err := mixer.ListControls(cards[0].ID)
	if err != nil {
		t.Skipf("No controls available, skipping test: %v", err)
	}

	if len(controls) == 0 {
		t.Skip("No controls found, skipping test")
	}

	// Try to find a volume control
	var volumeControl string
	for _, ctrl := range controls {
		if ctrl.Type == "integer" && (ctrl.Name == "Master" || ctrl.Name == "PCM") {
			volumeControl = ctrl.Name
			break
		}
	}

	if volumeControl == "" {
		t.Skip("No suitable volume control found, skipping test")
	}

	// Get current volume
	originalVolumes, err := mixer.GetVolume(cards[0].ID, volumeControl)
	if err != nil {
		t.Skipf("Cannot get current volume, skipping test: %v", err)
	}

	// Set test volume (50% for all channels)
	testVolumes := make([]int, len(originalVolumes))
	for i := range testVolumes {
		testVolumes[i] = 50
	}

	err = mixer.SetVolume(cards[0].ID, volumeControl, testVolumes)
	if err != nil {
		t.Fatalf("SetVolume() error = %v", err)
	}

	// Verify the volume was set
	newVolumes, err := mixer.GetVolume(cards[0].ID, volumeControl)
	if err != nil {
		t.Fatalf("Failed to get volume after setting: %v", err)
	}

	if len(newVolumes) != len(testVolumes) {
		t.Errorf("Volume channel count mismatch: got %d, expected %d", len(newVolumes), len(testVolumes))
	}

	// Restore original volume
	defer func() {
		if err := mixer.SetVolume(cards[0].ID, volumeControl, originalVolumes); err != nil {
			t.Logf("Warning: failed to restore original volume: %v", err)
		}
	}()

	// Verify volume was approximately set (allowing for rounding errors)
	for i, vol := range newVolumes {
		if vol < 45 || vol > 55 {
			t.Errorf("Volume[%d] = %d%%, expected approximately 50%%", i, vol)
		}
	}
}

// TestGetMute tests getting mute state
func TestGetMute(t *testing.T) {
	mixer := NewMixer()
	defer mixer.Close()

	// First get available cards and controls
	cards, err := mixer.ListCards()
	if err != nil {
		t.Skipf("No cards available, skipping test: %v", err)
	}

	if len(cards) == 0 {
		t.Skip("No cards found, skipping test")
	}

	controls, err := mixer.ListControls(cards[0].ID)
	if err != nil {
		t.Skipf("No controls available, skipping test: %v", err)
	}

	if len(controls) == 0 {
		t.Skip("No controls found, skipping test")
	}

	// Try to find a mute control
	var muteControl string
	for _, ctrl := range controls {
		if ctrl.Type == "boolean" || ctrl.Name == "Mute" || ctrl.Name == "Capture Switch" {
			muteControl = ctrl.Name
			break
		}
	}

	if muteControl == "" {
		t.Skip("No mute control found, skipping test")
	}

	muted, err := mixer.GetMute(cards[0].ID, muteControl)
	if err != nil {
		// Some controls might not support mute
		t.Skipf("GetMute() not supported for this control, skipping test: %v", err)
	}

	t.Logf("Mute state for '%s': %v", muteControl, muted)
}

// TestSetMute tests setting mute state
func TestSetMute(t *testing.T) {
	mixer := NewMixer()
	defer mixer.Close()

	// First get available cards and controls
	cards, err := mixer.ListCards()
	if err != nil {
		t.Skipf("No cards available, skipping test: %v", err)
	}

	if len(cards) == 0 {
		t.Skip("No cards found, skipping test")
	}

	controls, err := mixer.ListControls(cards[0].ID)
	if err != nil {
		t.Skipf("No controls available, skipping test: %v", err)
	}

	if len(controls) == 0 {
		t.Skip("No controls found, skipping test")
	}

	// Try to find a volume control that might have a mute switch
	var testControl string
	for _, ctrl := range controls {
		if ctrl.Name == "Master" || ctrl.Name == "PCM" {
			testControl = ctrl.Name
			break
		}
	}

	if testControl == "" {
		t.Skip("No suitable control found for mute test")
	}

	// Get original mute state
	originalMuted, err := mixer.GetMute(cards[0].ID, testControl)
	if err != nil {
		t.Skipf("Cannot get mute state, skipping test: %v", err)
	}

	// Set mute to true
	err = mixer.SetMute(cards[0].ID, testControl, true)
	if err != nil {
		t.Skipf("SetMute() not supported for this control, skipping test: %v", err)
	}

	// Verify mute was set
	muted, err := mixer.GetMute(cards[0].ID, testControl)
	if err != nil {
		t.Fatalf("Failed to get mute state after setting: %v", err)
	}

	if !muted {
		t.Error("Expected muted state to be true")
	}

	// Restore original mute state
	defer func() {
		if err := mixer.SetMute(cards[0].ID, testControl, originalMuted); err != nil {
			t.Logf("Warning: failed to restore original mute state: %v", err)
		}
	}()

	// Test unmuting
	err = mixer.SetMute(cards[0].ID, testControl, false)
	if err != nil {
		t.Fatalf("Failed to unmute: %v", err)
	}

	muted, err = mixer.GetMute(cards[0].ID, testControl)
	if err != nil {
		t.Fatalf("Failed to get mute state after unmuting: %v", err)
	}

	if muted {
		t.Error("Expected muted state to be false after unmuting")
	}
}

// TestErrorHandling tests error handling for various scenarios
func TestErrorHandling(t *testing.T) {
	mixer := NewMixer()
	defer mixer.Close()

	// Test operations on closed mixer
	mixer.Close()

	_, err := mixer.ListCards()
	if err == nil || err.Error() != "mixer is closed" {
		t.Error("Expected 'mixer is closed' error from ListCards()")
	}

	_, err = mixer.ListControls(0)
	if err == nil || err.Error() != "mixer is closed" {
		t.Error("Expected 'mixer is closed' error from ListControls()")
	}

	_, err = mixer.GetVolume(0, "test")
	if err == nil || err.Error() != "mixer is closed" {
		t.Error("Expected 'mixer is closed' error from GetVolume()")
	}

	err = mixer.SetVolume(0, "test", []int{50})
	if err == nil || err.Error() != "mixer is closed" {
		t.Error("Expected 'mixer is closed' error from SetVolume()")
	}

	_, err = mixer.GetMute(0, "test")
	if err == nil || err.Error() != "mixer is closed" {
		t.Error("Expected 'mixer is closed' error from GetMute()")
	}

	err = mixer.SetMute(0, "test", true)
	if err == nil || err.Error() != "mixer is closed" {
		t.Error("Expected 'mixer is closed' error from SetMute()")
	}
}

// TestSetVolumeValidation tests input validation for SetVolume
func TestSetVolumeValidation(t *testing.T) {
	mixer := NewMixer()
	defer mixer.Close()

	// Test with empty values slice
	err := mixer.SetVolume(0, "test", []int{})
	if err == nil || err.Error() != "no volume values provided" {
		t.Error("Expected 'no volume values provided' error")
	}

	// Test with invalid card (should fail to open)
	_, err = mixer.GetVolume(999, "test")
	if err == nil {
		t.Error("Expected error for invalid card")
	}
}
