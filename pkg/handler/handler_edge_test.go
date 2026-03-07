package handler

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.watcher/pkg/watcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandler_PanicRecovery verifies that a handler that panics propagates
// the panic to the caller (the handler package does not include built-in
// panic recovery — callers are responsible for wrapping handlers if
// recovery is needed).
func TestHandler_PanicRecovery(t *testing.T) {
	panicHandler := HandlerFunc(func(e watcher.Event) error {
		panic("handler exploded")
	})

	safeHandler := HandlerFunc(func(e watcher.Event) error {
		return nil
	})

	chain := NewChain(panicHandler, safeHandler)
	event := watcher.Event{
		Type:      watcher.Create,
		Path:      "/tmp/panic.txt",
		Timestamp: time.Now(),
	}

	// The chain does not recover panics, so we verify the panic occurs
	// and can be caught by the caller.
	assert.Panics(t, func() {
		_ = chain.Handle(event)
	}, "handler panic should propagate to caller")

	// Demonstrate that a caller can wrap with recovery.
	var recovered bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				recovered = true
			}
		}()
		_ = chain.Handle(event)
	}()
	assert.True(t, recovered, "caller should be able to recover from handler panic")
}

// TestHandler_SlowHandler verifies that a slow handler does not prevent
// subsequent events from being dispatched (the chain is synchronous, so
// each call blocks until complete).
func TestHandler_SlowHandler(t *testing.T) {
	var callCount int32
	slowDuration := 50 * time.Millisecond

	slowHandler := HandlerFunc(func(e watcher.Event) error {
		time.Sleep(slowDuration)
		atomic.AddInt32(&callCount, 1)
		return nil
	})

	chain := NewChain(slowHandler)

	numEvents := 5
	start := time.Now()

	for i := 0; i < numEvents; i++ {
		event := watcher.Event{
			Type: watcher.Write,
			Path: fmt.Sprintf("/tmp/slow_%d.txt", i),
		}
		err := chain.Handle(event)
		assert.NoError(t, err)
	}

	elapsed := time.Since(start)
	assert.Equal(t, int32(numEvents), atomic.LoadInt32(&callCount))
	// Since handlers are synchronous, total time should be at least
	// numEvents * slowDuration.
	assert.GreaterOrEqual(t, elapsed, time.Duration(numEvents)*slowDuration,
		"synchronous chain should block for each slow handler")
}

// TestHandler_ConcurrentHandlerCalls verifies that the same chain can be
// called concurrently from multiple goroutines. The chain itself is
// stateless, so this should be safe as long as individual handlers are
// goroutine-safe.
func TestHandler_ConcurrentHandlerCalls(t *testing.T) {
	var mu sync.Mutex
	var events []watcher.Event

	h := HandlerFunc(func(e watcher.Event) error {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
		return nil
	})

	chain := NewChain(h)

	numGoroutines := 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			event := watcher.Event{
				Type:      watcher.Write,
				Path:      fmt.Sprintf("/tmp/concurrent_%d.txt", id),
				Timestamp: time.Now(),
			}
			err := chain.Handle(event)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, events, numGoroutines,
		"all concurrent events should be handled")
}

// TestHandler_ChainErrorPropagation verifies that errors from handlers
// in the chain are properly propagated and subsequent handlers are skipped.
func TestHandler_ChainErrorPropagation(t *testing.T) {
	var callOrder []string

	h1 := HandlerFunc(func(e watcher.Event) error {
		callOrder = append(callOrder, "h1")
		return nil
	})

	expectedErr := errors.New("h2 failure")
	h2 := HandlerFunc(func(e watcher.Event) error {
		callOrder = append(callOrder, "h2")
		return expectedErr
	})

	h3 := HandlerFunc(func(e watcher.Event) error {
		callOrder = append(callOrder, "h3")
		return nil
	})

	h4 := HandlerFunc(func(e watcher.Event) error {
		callOrder = append(callOrder, "h4")
		return nil
	})

	chain := NewChain(h1, h2, h3, h4)

	err := chain.Handle(watcher.Event{Type: watcher.Create, Path: "/tmp/err.txt"})
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, []string{"h1", "h2"}, callOrder,
		"h3 and h4 should not be called after h2 errors")
}

// TestHandler_HandlerFuncNilReturn verifies that a HandlerFunc that returns
// nil works correctly.
func TestHandler_HandlerFuncNilReturn(t *testing.T) {
	h := HandlerFunc(func(e watcher.Event) error {
		return nil
	})

	err := h.Handle(watcher.Event{})
	assert.NoError(t, err)
}

// TestHandler_ChainWithSingleErrorHandler verifies a chain of one handler
// that returns an error.
func TestHandler_ChainWithSingleErrorHandler(t *testing.T) {
	expectedErr := errors.New("solo error")
	h := HandlerFunc(func(e watcher.Event) error {
		return expectedErr
	})

	chain := NewChain(h)
	assert.Equal(t, 1, chain.Len())

	err := chain.Handle(watcher.Event{})
	assert.ErrorIs(t, err, expectedErr)
}

// TestHandler_ChainHandlesAllEventTypes verifies the chain processes all
// event types correctly.
func TestHandler_ChainHandlesAllEventTypes(t *testing.T) {
	var received []watcher.EventType

	h := HandlerFunc(func(e watcher.Event) error {
		received = append(received, e.Type)
		return nil
	})

	chain := NewChain(h)

	allTypes := []watcher.EventType{
		watcher.Create,
		watcher.Write,
		watcher.Remove,
		watcher.Rename,
		watcher.Chmod,
	}

	for _, et := range allTypes {
		err := chain.Handle(watcher.Event{Type: et, Path: "/tmp/test"})
		require.NoError(t, err)
	}

	assert.Equal(t, allTypes, received)
}

// TestHandler_ChainLen verifies the Len() method on chains of various sizes.
func TestHandler_ChainLen(t *testing.T) {
	noop := HandlerFunc(func(e watcher.Event) error { return nil })

	tests := []struct {
		name     string
		handlers []Handler
		expected int
	}{
		{"zero", nil, 0},
		{"one", []Handler{noop}, 1},
		{"three", []Handler{noop, noop, noop}, 3},
		{"ten", []Handler{noop, noop, noop, noop, noop, noop, noop, noop, noop, noop}, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := NewChain(tt.handlers...)
			assert.Equal(t, tt.expected, chain.Len())
		})
	}
}

// TestHandler_ZeroValueEvent verifies that a handler chain works correctly
// when passed a zero-value Event.
func TestHandler_ZeroValueEvent(t *testing.T) {
	var received watcher.Event
	h := HandlerFunc(func(e watcher.Event) error {
		received = e
		return nil
	})

	chain := NewChain(h)
	err := chain.Handle(watcher.Event{})
	assert.NoError(t, err)
	assert.Equal(t, watcher.Event{}, received)
}

// TestHandler_WrappedPanicRecoveryHandler demonstrates how to build a
// panic-recovering handler wrapper on top of the existing handler interface.
func TestHandler_WrappedPanicRecoveryHandler(t *testing.T) {
	panicHandler := HandlerFunc(func(e watcher.Event) error {
		panic("boom")
	})

	// Wrap with recovery.
	safeHandler := HandlerFunc(func(e watcher.Event) error {
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("recovered panic: %v", r)
				}
			}()
			err = panicHandler.Handle(e)
		}()
		return err
	})

	chain := NewChain(safeHandler)
	err := chain.Handle(watcher.Event{Type: watcher.Create, Path: "/tmp/safe.txt"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "recovered panic: boom")
}
