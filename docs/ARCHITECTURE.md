# Watcher Architecture

## Purpose

`digital.vasic.watcher` is a standalone Go module for filesystem change monitoring. It
wraps `fsnotify/fsnotify` with recursive directory watching, configurable ignore patterns,
event debouncing, composable filters, and handler chains. The module is designed to be
embedded by applications that need to react to file creation, modification, and deletion.

## Package Overview

| Package | Responsibility |
|---------|---------------|
| `pkg/watcher` | Core `Watcher` interface and fsnotify-backed implementation with recursive watching, ignore patterns, and built-in debouncing |
| `pkg/debounce` | Standalone event debouncer that coalesces rapid events for the same path using generation-counted timers |
| `pkg/filter` | Composable `Filter` interface with `ExtensionFilter`, `TypeFilter`, `GlobFilter`, and boolean combinators (`And`, `Or`, `Not`) |
| `pkg/handler` | `Handler` interface, `HandlerFunc` adapter, and `Chain` for sequential event processing |

## Design Patterns

| Package | Pattern | Rationale |
|---------|---------|-----------|
| `pkg/watcher` | **Adapter** | `fsWatcher` adapts the `fsnotify.Watcher` API into the module's `Watcher` interface, converting event types and adding recursive watching |
| `pkg/watcher` | **Observer** | Events are delivered via a channel that any number of goroutines can consume |
| `pkg/watcher` | **Template Method** | The `loop()` goroutine defines the event processing skeleton: receive, ignore-check, convert, debounce, send |
| `pkg/debounce` | **Debounce / Coalescing** | Generation-counted timers ensure only the latest event for a given path is forwarded after the quiet period |
| `pkg/filter` | **Strategy** | `Filter` interface allows swappable matching logic; each implementation encapsulates a different criterion |
| `pkg/filter` | **Composite (boolean algebra)** | `And()`, `Or()`, `Not()` combinators build complex filter trees from simple filters |
| `pkg/handler` | **Chain of Responsibility** | `Chain` processes events through an ordered list of handlers; the first error halts the chain |
| `pkg/handler` | **Adapter** | `HandlerFunc` converts a plain function into a `Handler` interface implementation |

## Dependency Diagram

```
  +-------------------------------------------+
  |              Application                   |
  |                                            |
  |   Filter.Match()   Handler.Handle()        |
  |       |                  |                  |
  +-------+------------------+-----------------+
          |                  |
     +----+-----+      +----+-----+
     |  filter  |      | handler  |
     +----------+      +----+-----+
          |                  |
          |    +----------+  |
          +--->| watcher  |<-+   (filter and handler consume watcher.Event)
               +----+-----+
                    |
               +----+-----+
               | debounce  |   (standalone; also consumes watcher.Event)
               +----------+

  External dependency:
    watcher --> fsnotify/fsnotify
```

## Key Interfaces

```go
// pkg/watcher -- core filesystem watcher:
type Watcher interface {
    Watch(ctx context.Context, paths ...string) error
    Events() <-chan Event
    Errors() <-chan error
    Close() error
}

// pkg/filter -- determines whether an event should be processed:
type Filter interface {
    Match(event watcher.Event) bool
}

// pkg/handler -- processes filesystem events:
type Handler interface {
    Handle(event watcher.Event) error
}
```

### Event Types

```go
const (
    Create EventType = iota
    Write
    Remove
    Rename
    Chmod
)
```

### Filter Combinators

```go
filter.And(filters ...Filter) Filter   // all must match
filter.Or(filters ...Filter) Filter    // at least one must match
filter.Not(filter Filter) Filter       // negation
```

## Usage Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "digital.vasic.watcher/pkg/debounce"
    "digital.vasic.watcher/pkg/filter"
    "digital.vasic.watcher/pkg/handler"
    "digital.vasic.watcher/pkg/watcher"
    "time"
)

func main() {
    // 1. Create a recursive watcher with ignore patterns.
    cfg := &watcher.Config{
        Recursive:      true,
        DebounceDelay:  200 * time.Millisecond,
        BufferSize:     256,
        IgnorePatterns: []string{".git", "*.tmp", "*.swp"},
    }

    w, err := watcher.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer w.Close()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := w.Watch(ctx, "/media/library"); err != nil {
        log.Fatal(err)
    }

    // 2. Build a filter: only Create/Write events for video files.
    f := filter.And(
        filter.NewTypeFilter(watcher.Create, watcher.Write),
        filter.NewExtensionFilter("mp4", "mkv", "avi"),
    )

    // 3. Build a handler chain.
    chain := handler.NewChain(
        handler.HandlerFunc(func(e watcher.Event) error {
            fmt.Printf("[%s] %s %s\n", e.Timestamp.Format(time.Kitchen), e.Type, e.Path)
            return nil
        }),
        handler.HandlerFunc(func(e watcher.Event) error {
            // Trigger media detection, database update, etc.
            return nil
        }),
    )

    // 4. Consume events.
    for event := range w.Events() {
        if f.Match(event) {
            if err := chain.Handle(event); err != nil {
                log.Printf("handler error: %v", err)
            }
        }
    }
}
```

The standalone `debounce.Debouncer` can also be used independently when the
built-in watcher debouncing is not sufficient:

```go
d := debounce.New(500*time.Millisecond, 100)
defer d.Close()

// Feed events from any source.
d.Add(event)

// Read coalesced events.
for e := range d.Events() {
    fmt.Println(e.Path)
}
```
