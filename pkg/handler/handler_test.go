package handler

import (
	"errors"
	"testing"
	"time"

	"digital.vasic.watcher/pkg/watcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerFunc(t *testing.T) {
	var called bool
	var receivedEvent watcher.Event

	fn := HandlerFunc(func(e watcher.Event) error {
		called = true
		receivedEvent = e
		return nil
	})

	event := watcher.Event{
		Type:      watcher.Create,
		Path:      "/tmp/test.txt",
		Timestamp: time.Now(),
	}

	err := fn.Handle(event)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, event.Path, receivedEvent.Path)
	assert.Equal(t, event.Type, receivedEvent.Type)
}

func TestHandlerFuncError(t *testing.T) {
	expectedErr := errors.New("handler error")

	fn := HandlerFunc(func(e watcher.Event) error {
		return expectedErr
	})

	err := fn.Handle(watcher.Event{})
	assert.ErrorIs(t, err, expectedErr)
}

func TestChainEmpty(t *testing.T) {
	chain := NewChain()
	require.NotNil(t, chain)
	assert.Equal(t, 0, chain.Len())

	err := chain.Handle(watcher.Event{})
	assert.NoError(t, err)
}

func TestChainSingleHandler(t *testing.T) {
	var events []watcher.Event

	h := HandlerFunc(func(e watcher.Event) error {
		events = append(events, e)
		return nil
	})

	chain := NewChain(h)
	assert.Equal(t, 1, chain.Len())

	event := watcher.Event{
		Type: watcher.Write,
		Path: "/tmp/file.txt",
	}

	err := chain.Handle(event)
	assert.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, event.Path, events[0].Path)
}

func TestChainMultipleHandlers(t *testing.T) {
	var order []int

	h1 := HandlerFunc(func(e watcher.Event) error {
		order = append(order, 1)
		return nil
	})

	h2 := HandlerFunc(func(e watcher.Event) error {
		order = append(order, 2)
		return nil
	})

	h3 := HandlerFunc(func(e watcher.Event) error {
		order = append(order, 3)
		return nil
	})

	chain := NewChain(h1, h2, h3)
	assert.Equal(t, 3, chain.Len())

	err := chain.Handle(watcher.Event{})
	assert.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, order)
}

func TestChainStopsOnError(t *testing.T) {
	var order []int
	expectedErr := errors.New("stop here")

	h1 := HandlerFunc(func(e watcher.Event) error {
		order = append(order, 1)
		return nil
	})

	h2 := HandlerFunc(func(e watcher.Event) error {
		order = append(order, 2)
		return expectedErr
	})

	h3 := HandlerFunc(func(e watcher.Event) error {
		order = append(order, 3)
		return nil
	})

	chain := NewChain(h1, h2, h3)

	err := chain.Handle(watcher.Event{})
	assert.ErrorIs(t, err, expectedErr)
	assert.Equal(t, []int{1, 2}, order, "handler 3 should not have been called")
}

func TestChainWithDifferentEventTypes(t *testing.T) {
	var types []watcher.EventType

	h := HandlerFunc(func(e watcher.Event) error {
		types = append(types, e.Type)
		return nil
	})

	chain := NewChain(h)

	events := []watcher.Event{
		{Type: watcher.Create, Path: "/a"},
		{Type: watcher.Write, Path: "/b"},
		{Type: watcher.Remove, Path: "/c"},
		{Type: watcher.Rename, Path: "/d"},
	}

	for _, ev := range events {
		err := chain.Handle(ev)
		assert.NoError(t, err)
	}

	expected := []watcher.EventType{watcher.Create, watcher.Write, watcher.Remove, watcher.Rename}
	assert.Equal(t, expected, types)
}
