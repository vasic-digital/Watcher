package debounce

import (
	"testing"
	"time"

	"digital.vasic.watcher/pkg/watcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeEvent(path string, eventType watcher.EventType) watcher.Event {
	return watcher.Event{
		Type:      eventType,
		Path:      path,
		Timestamp: time.Now(),
	}
}

func TestNewDebouncer(t *testing.T) {
	d := New(100*time.Millisecond, 10)
	require.NotNil(t, d)
	defer d.Close()

	assert.NotNil(t, d.Events())
}

func TestNewDebouncerDefaultBuffer(t *testing.T) {
	d := New(100*time.Millisecond, 0)
	require.NotNil(t, d)
	defer d.Close()
}

func TestDebouncerSingleEvent(t *testing.T) {
	d := New(50*time.Millisecond, 10)
	defer d.Close()

	event := makeEvent("/tmp/test.txt", watcher.Create)
	d.Add(event)

	select {
	case received := <-d.Events():
		assert.Equal(t, event.Path, received.Path)
		assert.Equal(t, event.Type, received.Type)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for debounced event")
	}
}

func TestDebouncerCoalescing(t *testing.T) {
	d := New(100*time.Millisecond, 10)
	defer d.Close()

	// Add multiple events for the same path rapidly.
	for i := 0; i < 5; i++ {
		d.Add(makeEvent("/tmp/test.txt", watcher.Write))
		time.Sleep(10 * time.Millisecond)
	}

	// Should receive exactly one event after the debounce delay.
	var count int
	timeout := time.After(1 * time.Second)
	collectDone := time.After(400 * time.Millisecond)

loop:
	for {
		select {
		case <-d.Events():
			count++
		case <-collectDone:
			break loop
		case <-timeout:
			break loop
		}
	}

	assert.Equal(t, 1, count, "should coalesce rapid events into one")
}

func TestDebouncerDifferentPaths(t *testing.T) {
	d := New(50*time.Millisecond, 10)
	defer d.Close()

	// Add events for different paths.
	d.Add(makeEvent("/tmp/a.txt", watcher.Create))
	d.Add(makeEvent("/tmp/b.txt", watcher.Create))
	d.Add(makeEvent("/tmp/c.txt", watcher.Create))

	// Should receive one event per path.
	received := make(map[string]bool)
	timeout := time.After(1 * time.Second)

	for len(received) < 3 {
		select {
		case ev := <-d.Events():
			received[ev.Path] = true
		case <-timeout:
			t.Fatalf("timed out: received only %d of 3 events", len(received))
		}
	}

	assert.True(t, received["/tmp/a.txt"])
	assert.True(t, received["/tmp/b.txt"])
	assert.True(t, received["/tmp/c.txt"])
}

func TestDebouncerLastEventWins(t *testing.T) {
	d := New(100*time.Millisecond, 10)
	defer d.Close()

	// Add events with different types for the same path.
	d.Add(makeEvent("/tmp/test.txt", watcher.Create))
	time.Sleep(10 * time.Millisecond)
	d.Add(makeEvent("/tmp/test.txt", watcher.Write))
	time.Sleep(10 * time.Millisecond)

	lastEvent := makeEvent("/tmp/test.txt", watcher.Remove)
	d.Add(lastEvent)

	// The last event should be the one delivered.
	select {
	case received := <-d.Events():
		assert.Equal(t, watcher.Remove, received.Type, "last event type should win")
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for debounced event")
	}
}

func TestDebouncerClose(t *testing.T) {
	d := New(50*time.Millisecond, 10)

	d.Add(makeEvent("/tmp/test.txt", watcher.Create))
	d.Close()

	// Events channel should be closed.
	_, ok := <-d.Events()
	assert.False(t, ok, "events channel should be closed after Close")
}

func TestDebouncerDoubleClose(t *testing.T) {
	d := New(50*time.Millisecond, 10)
	d.Close()
	// Should not panic.
	d.Close()
}

func TestDebouncerAddAfterClose(t *testing.T) {
	d := New(50*time.Millisecond, 10)
	d.Close()

	// Should not panic.
	d.Add(makeEvent("/tmp/test.txt", watcher.Create))
}

func TestDebouncerDelayTiming(t *testing.T) {
	delay := 100 * time.Millisecond
	d := New(delay, 10)
	defer d.Close()

	start := time.Now()
	d.Add(makeEvent("/tmp/test.txt", watcher.Create))

	select {
	case <-d.Events():
		elapsed := time.Since(start)
		assert.GreaterOrEqual(t, elapsed, delay, "event should not arrive before delay")
		assert.Less(t, elapsed, delay+100*time.Millisecond, "event should arrive soon after delay")
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for debounced event")
	}
}
