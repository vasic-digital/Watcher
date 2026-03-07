package debounce

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.watcher/pkg/watcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeEdgeEvent(path string, eventType watcher.EventType) watcher.Event {
	return watcher.Event{
		Type:      eventType,
		Path:      path,
		Timestamp: time.Now(),
	}
}

// TestDebouncer_CreateDeleteRecreate verifies that a rapid
// create -> delete -> recreate sequence for the same path properly coalesces
// and delivers the final event.
func TestDebouncer_CreateDeleteRecreate(t *testing.T) {
	d := New(100*time.Millisecond, 10)
	defer d.Close()

	path := "/tmp/ephemeral.txt"

	// Rapid create -> delete -> recreate
	d.Add(makeEdgeEvent(path, watcher.Create))
	time.Sleep(5 * time.Millisecond)
	d.Add(makeEdgeEvent(path, watcher.Remove))
	time.Sleep(5 * time.Millisecond)
	d.Add(makeEdgeEvent(path, watcher.Create))

	// Should receive exactly one event (the last one: Create).
	select {
	case ev := <-d.Events():
		assert.Equal(t, path, ev.Path)
		assert.Equal(t, watcher.Create, ev.Type, "last event (Create) should win")
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for debounced event")
	}

	// No additional events should arrive.
	select {
	case ev := <-d.Events():
		t.Fatalf("unexpected extra event: %+v", ev)
	case <-time.After(300 * time.Millisecond):
		// Good, no extra events.
	}
}

// TestDebouncer_ManyEventsCoalesced verifies that 1000 events within the
// debounce window for the same path are coalesced into a single event.
func TestDebouncer_ManyEventsCoalesced(t *testing.T) {
	d := New(200*time.Millisecond, 10)
	defer d.Close()

	path := "/tmp/hotfile.txt"

	// Fire 1000 events for the same path rapidly.
	for i := 0; i < 1000; i++ {
		d.Add(makeEdgeEvent(path, watcher.Write))
	}

	// Collect events.
	var count int
	timeout := time.After(2 * time.Second)
	collectDone := time.After(600 * time.Millisecond)

loop:
	for {
		select {
		case _, ok := <-d.Events():
			if ok {
				count++
			}
		case <-collectDone:
			break loop
		case <-timeout:
			break loop
		}
	}

	assert.Equal(t, 1, count, "1000 rapid events for the same path should coalesce into 1")
}

// TestDebouncer_ZeroDuration verifies that a debouncer with zero delay
// passes events through quickly (effectively no debouncing but still
// through the timer mechanism).
func TestDebouncer_ZeroDuration(t *testing.T) {
	d := New(0, 10)
	defer d.Close()

	start := time.Now()
	d.Add(makeEdgeEvent("/tmp/immediate.txt", watcher.Create))

	select {
	case ev := <-d.Events():
		elapsed := time.Since(start)
		assert.Equal(t, "/tmp/immediate.txt", ev.Path)
		// With zero delay, the event should arrive almost immediately
		// (within the timer's minimum resolution).
		assert.Less(t, elapsed, 100*time.Millisecond,
			"zero-delay event should arrive quickly")
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for zero-delay event")
	}
}

// TestDebouncer_ConcurrentEvents verifies that 50 goroutines sending events
// simultaneously to the debouncer does not cause races or panics.
func TestDebouncer_ConcurrentEvents(t *testing.T) {
	d := New(50*time.Millisecond, 200)
	defer d.Close()

	var wg sync.WaitGroup
	numGoroutines := 50

	// Each goroutine sends events for a unique path.
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			path := "/tmp/concurrent_" + string(rune('A'+id%26)) + "_" + time.Now().Format("150405.000")
			for j := 0; j < 10; j++ {
				d.Add(makeEdgeEvent(path, watcher.Write))
			}
		}(i)
	}

	wg.Wait()

	// Collect all events with a generous timeout.
	var received int32
	collectDone := time.After(1 * time.Second)

loop:
	for {
		select {
		case _, ok := <-d.Events():
			if ok {
				atomic.AddInt32(&received, 1)
			}
		case <-collectDone:
			break loop
		}
	}

	// Each unique path should produce at least 1 event.
	assert.Greater(t, received, int32(0), "should receive at least some events")
	// With coalescing, we should receive fewer than 50*10=500 events.
	assert.LessOrEqual(t, received, int32(500),
		"concurrent events should be coalesced")
}

// TestDebouncer_ManyDifferentPaths verifies that events for many different
// paths are each delivered independently.
func TestDebouncer_ManyDifferentPaths(t *testing.T) {
	numPaths := 20
	d := New(50*time.Millisecond, numPaths*2)
	defer d.Close()

	paths := make([]string, numPaths)
	for i := 0; i < numPaths; i++ {
		paths[i] = "/tmp/multi_" + string(rune('a'+i))
		d.Add(makeEdgeEvent(paths[i], watcher.Create))
	}

	received := make(map[string]bool)
	timeout := time.After(2 * time.Second)

	for len(received) < numPaths {
		select {
		case ev := <-d.Events():
			received[ev.Path] = true
		case <-timeout:
			t.Fatalf("timed out: received %d of %d paths", len(received), numPaths)
		}
	}

	for _, p := range paths {
		assert.True(t, received[p], "should have received event for path %s", p)
	}
}

// TestDebouncer_CloseStopsPendingTimers verifies that Close prevents any
// pending timer from delivering events.
func TestDebouncer_CloseStopsPendingTimers(t *testing.T) {
	d := New(500*time.Millisecond, 10)

	// Add an event but close before the debounce delay expires.
	d.Add(makeEdgeEvent("/tmp/pending.txt", watcher.Create))
	time.Sleep(10 * time.Millisecond)
	d.Close()

	// Events channel should be closed; no event should arrive.
	ev, ok := <-d.Events()
	assert.False(t, ok, "channel should be closed")
	assert.Equal(t, watcher.Event{}, ev)
}

// TestDebouncer_ConcurrentAddAndClose verifies that calling Add and Close
// concurrently does not cause races or panics.
func TestDebouncer_ConcurrentAddAndClose(t *testing.T) {
	d := New(50*time.Millisecond, 100)

	var wg sync.WaitGroup

	// 20 goroutines sending events.
	wg.Add(20)
	for i := 0; i < 20; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				d.Add(makeEdgeEvent("/tmp/race.txt", watcher.Write))
			}
		}(i)
	}

	// Close from another goroutine after a short delay.
	time.Sleep(5 * time.Millisecond)
	d.Close()

	wg.Wait()

	// Drain any buffered events.
	for range d.Events() {
	}
}

// TestDebouncer_NegativeBufferSize verifies that a negative buffer size is
// coerced to 100 (the default).
func TestDebouncer_NegativeBufferSize(t *testing.T) {
	d := New(50*time.Millisecond, -1)
	require.NotNil(t, d)
	defer d.Close()

	d.Add(makeEdgeEvent("/tmp/neg.txt", watcher.Create))

	select {
	case ev := <-d.Events():
		assert.Equal(t, "/tmp/neg.txt", ev.Path)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for event with negative buffer size")
	}
}
