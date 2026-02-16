# AGENTS.md

Agent guidelines for working with the `digital.vasic.watcher` module.

## Repository Structure

```
/
  go.mod                          Module definition
  pkg/
    watcher/                      Core watcher interface and implementation
      watcher.go
      watcher_test.go
    handler/                      Event handler chain
      handler.go
      handler_test.go
    filter/                       Event filtering (extension, type, glob, combinators)
      filter.go
      filter_test.go
    debounce/                     Event debouncing
      debounce.go
      debounce_test.go
```

## Development Workflow

1. Always run `go mod tidy` after changing dependencies.
2. Run `go build ./...` to verify compilation.
3. Run `go test ./... -count=1` to run all tests without caching.
4. Test files live alongside source files as `*_test.go`.

## Code Patterns

- **Watcher interface**: `Watch(ctx, paths...)` starts watching; `Events()` and `Errors()` return read-only channels; `Close()` releases resources.
- **Handler interface**: Single `Handle(Event) error` method. Use `HandlerFunc` for inline handlers. Chain handlers with `NewChain(...)`.
- **Filter interface**: Single `Match(Event) bool` method. Compose with `And()`, `Or()`, `Not()`.
- **Debouncer**: `New(delay, bufSize)` creates one. `Add(event)` submits events. `Events()` returns the output channel. `Close()` stops everything.
- **Concurrency**: All types are safe for concurrent use. Debouncer and Watcher use `sync.Mutex` internally.
- **Resource cleanup**: Always call `Close()` on Watcher and Debouncer. Both are idempotent.

## Testing Guidelines

- Use `t.TempDir()` for temporary directories in filesystem tests.
- Use short debounce delays (50-100ms) in tests to keep them fast.
- Use `time.After` with generous timeouts (1-2s) to avoid flaky tests.
- The `watcher_test.go` tests exercise real filesystem operations; they may be sensitive to CI environment timing.
