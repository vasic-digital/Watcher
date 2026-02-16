// Package watcher provides filesystem change monitoring with
// event debouncing, filtering, and handler chains.
package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// EventType represents the type of filesystem event.
type EventType int

const (
	Create EventType = iota
	Write
	Remove
	Rename
	Chmod
)

func (e EventType) String() string {
	switch e {
	case Create:
		return "create"
	case Write:
		return "write"
	case Remove:
		return "remove"
	case Rename:
		return "rename"
	case Chmod:
		return "chmod"
	default:
		return "unknown"
	}
}

// Event represents a filesystem change event.
type Event struct {
	Type      EventType
	Path      string
	OldPath   string // For rename events
	Timestamp time.Time
}

// Watcher watches filesystem paths for changes.
type Watcher interface {
	Watch(ctx context.Context, paths ...string) error
	Events() <-chan Event
	Errors() <-chan error
	Close() error
}

// Config holds watcher configuration.
type Config struct {
	Recursive      bool
	DebounceDelay  time.Duration
	BufferSize     int
	IgnorePatterns []string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Recursive:     true,
		DebounceDelay: 100 * time.Millisecond,
		BufferSize:    100,
	}
}

// debounceEntry tracks a pending debounce timer with a generation counter
// to prevent stale callbacks from firing.
type debounceEntry struct {
	timer      *time.Timer
	generation uint64
}

// fsWatcher is the fsnotify-backed implementation of Watcher.
type fsWatcher struct {
	cfg       *Config
	inner     *fsnotify.Watcher
	events    chan Event
	errors    chan error
	stopCh    chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
	mu        sync.Mutex
	watching  bool
}

// New creates a new filesystem watcher backed by fsnotify.
func New(cfg *Config) (Watcher, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	inner, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	bufSize := cfg.BufferSize
	if bufSize <= 0 {
		bufSize = 100
	}

	w := &fsWatcher{
		cfg:    cfg,
		inner:  inner,
		events: make(chan Event, bufSize),
		errors: make(chan error, bufSize),
		stopCh: make(chan struct{}),
	}

	return w, nil
}

// Watch starts watching the given paths for filesystem changes. It blocks until
// the context is cancelled or Close is called. If cfg.Recursive is true,
// subdirectories are added automatically.
func (w *fsWatcher) Watch(ctx context.Context, paths ...string) error {
	w.mu.Lock()
	if w.watching {
		w.mu.Unlock()
		return nil
	}
	w.watching = true
	w.mu.Unlock()

	for _, p := range paths {
		if w.cfg.Recursive {
			if err := w.addRecursive(p); err != nil {
				return err
			}
		} else {
			if err := w.inner.Add(p); err != nil {
				return err
			}
		}
	}

	// Start the event forwarding loop.
	w.wg.Add(1)
	go w.loop(ctx)

	return nil
}

// Events returns the channel on which filesystem events are delivered.
func (w *fsWatcher) Events() <-chan Event {
	return w.events
}

// Errors returns the channel on which watcher errors are delivered.
func (w *fsWatcher) Errors() <-chan error {
	return w.errors
}

// Close stops the watcher and releases resources.
func (w *fsWatcher) Close() error {
	var err error
	w.closeOnce.Do(func() {
		close(w.stopCh)
		err = w.inner.Close()
		w.wg.Wait()
		close(w.events)
		close(w.errors)
	})
	return err
}

// loop reads from the underlying fsnotify watcher and forwards events.
func (w *fsWatcher) loop(ctx context.Context) {
	defer w.wg.Done()

	// debounce timers keyed by path
	debounceMap := make(map[string]*debounceEntry)
	var debounceMu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case ev, ok := <-w.inner.Events:
			if !ok {
				return
			}

			if w.shouldIgnore(ev.Name) {
				continue
			}

			event := w.convertEvent(ev)

			// If a new directory is created and we are recursive, add it.
			if w.cfg.Recursive && event.Type == Create {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					_ = w.addRecursive(ev.Name)
				}
			}

			if w.cfg.DebounceDelay > 0 {
				w.debounce(&debounceMu, debounceMap, event)
			} else {
				w.sendEvent(event)
			}

		case err, ok := <-w.inner.Errors:
			if !ok {
				return
			}
			select {
			case w.errors <- err:
			default:
			}
		}
	}
}

// debounce coalesces rapid events for the same path.
func (w *fsWatcher) debounce(mu *sync.Mutex, m map[string]*debounceEntry, event Event) {
	mu.Lock()
	defer mu.Unlock()

	key := event.Path

	var gen uint64
	if existing, ok := m[key]; ok {
		existing.timer.Stop()
		gen = existing.generation + 1
	} else {
		gen = 1
	}

	currentGen := gen
	timer := time.AfterFunc(w.cfg.DebounceDelay, func() {
		mu.Lock()
		if e, ok := m[key]; ok && e.generation == currentGen {
			delete(m, key)
		}
		mu.Unlock()
		w.sendEvent(event)
	})

	m[key] = &debounceEntry{
		timer:      timer,
		generation: currentGen,
	}
}

// sendEvent sends an event to the events channel without blocking.
func (w *fsWatcher) sendEvent(event Event) {
	select {
	case w.events <- event:
	default:
	}
}

// convertEvent converts an fsnotify.Event to our Event type.
func (w *fsWatcher) convertEvent(ev fsnotify.Event) Event {
	var t EventType
	switch {
	case ev.Op&fsnotify.Create == fsnotify.Create:
		t = Create
	case ev.Op&fsnotify.Write == fsnotify.Write:
		t = Write
	case ev.Op&fsnotify.Remove == fsnotify.Remove:
		t = Remove
	case ev.Op&fsnotify.Rename == fsnotify.Rename:
		t = Rename
	case ev.Op&fsnotify.Chmod == fsnotify.Chmod:
		t = Chmod
	}

	return Event{
		Type:      t,
		Path:      ev.Name,
		Timestamp: time.Now(),
	}
}

// addRecursive walks the given path and adds all directories to the watcher.
func (w *fsWatcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if info.IsDir() {
			if w.shouldIgnore(path) {
				return filepath.SkipDir
			}
			return w.inner.Add(path)
		}
		return nil
	})
}

// shouldIgnore returns true if the path matches any ignore pattern.
func (w *fsWatcher) shouldIgnore(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range w.cfg.IgnorePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		// Also try matching against the full path.
		if strings.Contains(pattern, string(filepath.Separator)) || strings.Contains(pattern, "/") {
			if matched, _ := filepath.Match(pattern, path); matched {
				return true
			}
		}
	}
	return false
}
