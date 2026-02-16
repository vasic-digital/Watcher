// Package handler provides event handler chains for filesystem watchers.
package handler

import "digital.vasic.watcher/pkg/watcher"

// Handler processes filesystem events.
type Handler interface {
	Handle(event watcher.Event) error
}

// HandlerFunc is a function adapter for Handler.
type HandlerFunc func(watcher.Event) error

// Handle calls the underlying function.
func (f HandlerFunc) Handle(e watcher.Event) error { return f(e) }

// Chain chains multiple handlers so that events are processed sequentially
// through each handler. If any handler returns an error, the chain stops
// and the error is returned.
type Chain struct {
	handlers []Handler
}

// NewChain creates a new handler chain from the given handlers.
func NewChain(handlers ...Handler) *Chain {
	return &Chain{handlers: handlers}
}

// Handle processes the event through every handler in the chain.
// Handlers are called in order. The first non-nil error stops
// the chain and is returned.
func (c *Chain) Handle(e watcher.Event) error {
	for _, h := range c.handlers {
		if err := h.Handle(e); err != nil {
			return err
		}
	}
	return nil
}

// Len returns the number of handlers in the chain.
func (c *Chain) Len() int {
	return len(c.handlers)
}
