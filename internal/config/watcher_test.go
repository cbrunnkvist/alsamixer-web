package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWatcherDetectsChanges(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(f1, []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher([]string{f1, f2}, 50*time.Millisecond)
	var mu sync.Mutex
	changes := 0
	w.OnChange(func() {
		mu.Lock()
		changes++
		mu.Unlock()
	})
	w.Start()
	defer w.Stop()

	time.Sleep(20 * time.Millisecond)

	if err := os.WriteFile(f1, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(1 * time.Second)
	for {
		mu.Lock()
		c := changes
		mu.Unlock()
		if c > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for change callback")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWatcherDebounce(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(f, []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}

	w := NewWatcher([]string{f}, 200*time.Millisecond)
	var mu sync.Mutex
	changes := 0
	w.OnChange(func() {
		mu.Lock()
		changes++
		mu.Unlock()
	})
	w.Start()
	defer w.Stop()

	t1 := time.Now().Add(50 * time.Millisecond)
	if err := os.Chtimes(f, t1, t1); err != nil {
		t.Fatal(err)
	}
	t2 := t1.Add(50 * time.Millisecond)
	if err := os.Chtimes(f, t2, t2); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	c := changes
	mu.Unlock()
	if c != 1 {
		t.Fatalf("expected 1 debounced change, got %d", c)
	}
}
