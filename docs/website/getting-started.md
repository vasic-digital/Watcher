# Getting Started

## Installation

```bash
go get digital.vasic.watcher
```

## Watch a Directory

Create a watcher and start monitoring filesystem changes:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "digital.vasic.watcher/pkg/watcher"
)

func main() {
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

    for event := range w.Events() {
        fmt.Printf("[%s] %s: %s\n",
            event.Timestamp.Format(time.Kitchen),
            event.Type, event.Path)
    }
}
```

## Filter Events

Use composable filters to process only events you care about:

```go
package main

import (
    "digital.vasic.watcher/pkg/filter"
    "digital.vasic.watcher/pkg/watcher"
)

func main() {
    // Only Create and Write events for video files
    f := filter.And(
        filter.NewTypeFilter(watcher.Create, watcher.Write),
        filter.NewExtensionFilter("mp4", "mkv", "avi"),
    )

    for event := range w.Events() {
        if f.Match(event) {
            processVideoFile(event.Path)
        }
    }
}
```

## Handle Events with a Chain

Build a handler pipeline for sequential event processing:

```go
package main

import (
    "fmt"
    "log"

    "digital.vasic.watcher/pkg/handler"
    "digital.vasic.watcher/pkg/watcher"
)

func main() {
    chain := handler.NewChain(
        handler.HandlerFunc(func(e watcher.Event) error {
            fmt.Printf("Event: %s %s\n", e.Type, e.Path)
            return nil
        }),
        handler.HandlerFunc(func(e watcher.Event) error {
            // Trigger media detection, database update, etc.
            return nil
        }),
    )

    for event := range w.Events() {
        if err := chain.Handle(event); err != nil {
            log.Printf("Handler error: %v", err)
        }
    }
}
```

## Use the Standalone Debouncer

The debouncer can be used independently of the watcher:

```go
import "digital.vasic.watcher/pkg/debounce"

d := debounce.New(500*time.Millisecond, 100)
defer d.Close()

// Feed events from any source
d.Add(event)

// Read coalesced events
for e := range d.Events() {
    fmt.Println(e.Path)
}
```
