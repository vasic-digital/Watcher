// Package debounce provides event debouncing to coalesce rapid filesystem changes.
package debounce

import (
	"sync"
	"time"

	"digital.vasic.watcher/pkg/watcher"
)

// entry tracks a pending debounce timer with a generation counter to
// prevent stale timer callbacks from firing.
type entry struct {
	timer      *time.Timer
	generation uint64
}

// Debouncer coalesces rapid filesystem events for the same path. When
// multiple events arrive for the same path within the configured delay,
// only the last event is forwarded to the output channel.
type Debouncer struct {
	delay   time.Duration
	timers  map[string]*entry
	mu      sync.Mutex
	output  chan watcher.Event
	closed  bool
}

// New creates a new Debouncer with the given delay and output buffer size.
func New(delay time.Duration, bufferSize int) *Debouncer {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &Debouncer{
		delay:  delay,
		timers: make(map[string]*entry),
		output: make(chan watcher.Event, bufferSize),
	}
}

// Add submits an event to the debouncer. If an event for the same path
// is already pending, the timer is reset and the newer event replaces it.
func (d *Debouncer) Add(event watcher.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}

	key := event.Path

	var gen uint64
	if existing, ok := d.timers[key]; ok {
		existing.timer.Stop()
		gen = existing.generation + 1
	} else {
		gen = 1
	}

	currentGen := gen
	timer := time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		// Only clean up if this is still the current generation.
		if e, ok := d.timers[key]; ok && e.generation == currentGen {
			delete(d.timers, key)
		}
		closed := d.closed
		d.mu.Unlock()

		if !closed {
			select {
			case d.output <- event:
			default:
			}
		}
	})

	d.timers[key] = &entry{
		timer:      timer,
		generation: currentGen,
	}
}

// Events returns the channel on which debounced events are delivered.
func (d *Debouncer) Events() <-chan watcher.Event {
	return d.output
}

// Close stops all pending timers and closes the output channel.
// After Close returns, Add calls are no-ops.
func (d *Debouncer) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}
	d.closed = true

	for key, e := range d.timers {
		e.timer.Stop()
		delete(d.timers, key)
	}

	close(d.output)
}
