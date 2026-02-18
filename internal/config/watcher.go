package config

import (
	"os"
	"sync"
	"time"
)

// Watcher polls for changes to a set of files at a configurable interval.
// It uses modification times (mtime) to detect changes and debounces
// rapid successive changes into a single callback invocation.
type Watcher struct {
	files             []string
	interval          time.Duration
	callback          func()
	lastModTimes      map[string]time.Time
	stop              chan struct{}
	mu                sync.Mutex
	lastGlobalTrigger time.Time
	started           bool
}

// NewWatcher creates a new Watcher for the provided files with the given interval.
// If interval is zero or negative, a default of 15 seconds is used.
func NewWatcher(files []string, interval time.Duration) *Watcher {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	w := &Watcher{
		files:        files,
		interval:     interval,
		lastModTimes: make(map[string]time.Time),
		stop:         nil,
	}
	// initialize last known modification times (best effort; missing files are zero time)
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil {
			w.lastModTimes[f] = fi.ModTime()
		} else {
			w.lastModTimes[f] = time.Time{}
		}
	}
	return w
}

// OnChange registers a callback to be invoked when a watched file changes.
func (w *Watcher) OnChange(callback func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callback = callback
}

// Start begins watching the configured files in a background goroutine.
func (w *Watcher) Start() {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return
	}
	w.stop = make(chan struct{})
	w.started = true
	w.mu.Unlock()
	go w.run()
}

// Stop terminates the watcher goroutine.
func (w *Watcher) Stop() {
	w.mu.Lock()
	if w.stop != nil {
		close(w.stop)
		w.stop = nil
		w.started = false
	}
	w.mu.Unlock()
}

// run is the polling loop that checks for file changes at the configured interval.
func (w *Watcher) run() {
	// Use a local ticker; this allows Stop to close the goroutine cooperatively.
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.mu.Lock()
	stopCh := w.stop
	w.mu.Unlock()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			w.checkChanges()
		}
	}
}

// checkChanges inspects all watched files for modification and triggers the
// callback once per debounced burst.
func (w *Watcher) checkChanges() {
	w.mu.Lock()
	// Track if any file changed in this cycle.
	changed := false
	now := time.Now()
	// Examine each file's current modification time against the last seen value.
	for _, path := range w.files {
		if info, err := os.Stat(path); err == nil {
			cur := info.ModTime()
			last, ok := w.lastModTimes[path]
			if !ok {
				w.lastModTimes[path] = cur
				continue
			}
			if cur.After(last) {
				w.lastModTimes[path] = cur
				changed = true
			}
		}
		// If the file doesn't exist, we gracefully skip.
	}

	var cb func()
	if changed {
		// Debounce across bursts: only trigger if enough time has passed since the last trigger.
		if w.lastGlobalTrigger.IsZero() || now.Sub(w.lastGlobalTrigger) >= w.interval {
			w.lastGlobalTrigger = now
			cb = w.callback
		}
	}
	w.mu.Unlock()

	if cb != nil {
		// Call callback outside the lock to avoid potential deadlocks.
		cb()
	}
}
