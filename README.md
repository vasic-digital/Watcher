# digital.vasic.watcher

Filesystem change monitoring for Go with event debouncing, filtering, and handler chains.

## Features

- Cross-platform filesystem watching via [fsnotify](https://github.com/fsnotify/fsnotify)
- Recursive directory monitoring with automatic subdirectory tracking
- Configurable event debouncing to coalesce rapid changes
- Composable event filters (by extension, event type, glob pattern)
- Handler chains for building event processing pipelines
- Ignore patterns for excluding files and directories
- Context-aware watching with clean shutdown

## Installation

```bash
go get digital.vasic.watcher
```

## Usage

### Basic Watcher

```go
package main

import (
    "context"
    "fmt"
    "log"

    "digital.vasic.watcher/pkg/watcher"
)

func main() {
    cfg := watcher.DefaultConfig()
    cfg.IgnorePatterns = []string{"*.tmp", ".*"}

    w, err := watcher.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer w.Close()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := w.Watch(ctx, "/path/to/watch"); err != nil {
        log.Fatal(err)
    }

    for ev := range w.Events() {
        fmt.Printf("%s: %s\n", ev.Type, ev.Path)
    }
}
```

### Filtered Handling

```go
import (
    "digital.vasic.watcher/pkg/filter"
    "digital.vasic.watcher/pkg/handler"
    "digital.vasic.watcher/pkg/watcher"
)

// Only process Go and Rust source file creations
f := filter.And(
    filter.NewExtensionFilter("go", "rs"),
    filter.NewTypeFilter(watcher.Create, watcher.Write),
    filter.Not(filter.NewGlobFilter("*_test.*")),
)

chain := handler.NewChain(
    handler.HandlerFunc(func(e watcher.Event) error {
        log.Printf("Source changed: %s", e.Path)
        return nil
    }),
)

for ev := range w.Events() {
    if f.Match(ev) {
        chain.Handle(ev)
    }
}
```

### Standalone Debouncer

```go
import (
    "time"
    "digital.vasic.watcher/pkg/debounce"
)

d := debounce.New(200*time.Millisecond, 100)
defer d.Close()

// Feed events from any source
d.Add(event)

// Read coalesced events
for ev := range d.Events() {
    process(ev)
}
```

## Packages

| Package | Description |
|---|---|
| `pkg/watcher` | Core `Watcher` interface and fsnotify implementation |
| `pkg/handler` | Event handler interface, function adapter, and chain |
| `pkg/filter` | Event filters with `And`, `Or`, `Not` combinators |
| `pkg/debounce` | Standalone event debouncer with generation-counted timers |

## Configuration

```go
cfg := &watcher.Config{
    Recursive:      true,                    // Watch subdirectories
    DebounceDelay:  100 * time.Millisecond,  // Coalesce rapid events
    BufferSize:     100,                     // Event channel buffer
    IgnorePatterns: []string{"*.tmp", ".*"}, // Glob patterns to skip
}
```

## Requirements

- Go 1.24.0+
- Linux, macOS, or Windows (via fsnotify)
