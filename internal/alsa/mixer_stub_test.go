//go:build !linux

package alsa

import "testing"

func TestStubMixerUnsupported(t *testing.T) {
	m := NewMixer()
	if m == nil {
		t.Fatal("NewMixer() returned nil")
	}
	if m.IsOpen() {
		t.Fatal("stub mixer should not report open")
	}

	if _, err := m.ListCards(); err == nil {
		t.Fatal("expected ListCards() to error on non-linux")
	}
	if _, err := m.ListControls(0); err == nil {
		t.Fatal("expected ListControls() to error on non-linux")
	}
	if _, err := m.GetVolume(0, "Master"); err == nil {
		t.Fatal("expected GetVolume() to error on non-linux")
	}
	if err := m.SetVolume(0, "Master", []int{50}); err == nil {
		t.Fatal("expected SetVolume() to error on non-linux")
	}
	if _, err := m.GetMute(0, "Master"); err == nil {
		t.Fatal("expected GetMute() to error on non-linux")
	}
	if err := m.SetMute(0, "Master", true); err == nil {
		t.Fatal("expected SetMute() to error on non-linux")
	}

	if err := m.Close(); err != nil {
		t.Fatalf("Close() should be no-op on stub, got %v", err)
	}
}
