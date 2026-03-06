# Course: Filesystem Watching in Go

Learn to monitor filesystem changes, filter events, and build processing pipelines using `digital.vasic.watcher`.

## Lessons

1. **The Watcher** -- The `Watcher` interface, fsnotify integration, recursive watching, ignore patterns, and event debouncing.
2. **Filters and Handlers** -- Composable `Filter` with `And`/`Or`/`Not`, `Handler` interface, `HandlerFunc` adapter, and `Chain` pipelines.
3. **Standalone Debouncing** -- The `Debouncer` type, generation counters, and integrating debouncing into custom event systems.
