# Architecture -- Watcher

## Purpose

Filesystem change monitoring for Go with cross-platform support via fsnotify. Provides recursive directory watching, configurable event debouncing, composable event filters (extension, type, glob with And/Or/Not combinators), and handler chains for building event processing pipelines.

## Structure

```
pkg/
  watcher/    Core Watcher interface and fsnotify-backed implementation with recursive watching, ignore patterns, and built-in debouncing
  handler/    Handler interface, HandlerFunc adapter, and Chain for sequential event processing pipelines
  filter/     Composable Filter interface: ExtensionFilter, TypeFilter, GlobFilter, plus And, Or, Not combinators
  debounce/   Standalone Debouncer that coalesces rapid events for the same path using generation-counted timers
```

## Key Components

- **`watcher.Watcher`** -- Interface: Watch(ctx, path), Events() channel, Close(). Context-aware with clean shutdown
- **`watcher.Config`** -- Recursive (bool), DebounceDelay (duration), BufferSize (int), IgnorePatterns (glob strings)
- **`handler.Handler`** -- Interface: Handle(Event) error. HandlerFunc adapter and Chain for sequential processing
- **`filter.Filter`** -- Interface: Match(Event) bool. Composable with And, Or, Not combinators
- **`filter.ExtensionFilter`** / **`filter.TypeFilter`** / **`filter.GlobFilter`** -- Built-in filter implementations
- **`debounce.Debouncer`** -- Standalone debouncer with generation-counted timers to prevent stale callbacks

## Data Flow

```
filesystem events -> fsnotify -> Watcher (ignore patterns + debounce)
    |
    Events() channel -> filter.Match(event)?
        |                     |
        (match)          (no match, skip)
        |
    handler.Chain.Handle(event) -> handler1 -> handler2 -> ...
```

## Dependencies

- `github.com/fsnotify/fsnotify` -- Cross-platform filesystem notifications
- `github.com/stretchr/testify` -- Test assertions

## Testing Strategy

Tests use `t.TempDir()` for filesystem tests (automatic cleanup). Debouncing uses generation counters to prevent stale timer callbacks. Tests cover recursive directory watching, ignore pattern matching, filter composition with combinators, handler chain execution, debounce coalescing, and context cancellation.
