# Lesson 2: Filters and Handlers

## Objectives

- Filter events by extension, type, and glob pattern
- Compose filters with `And`, `Or`, and `Not`
- Build handler chains for sequential event processing

## Concepts

### The Filter Interface

```go
type Filter interface {
    Match(event watcher.Event) bool
}
```

### Built-in Filters

- **ExtensionFilter** -- matches file extensions (normalized to lowercase with leading dot)
- **TypeFilter** -- matches event types (Create, Write, Remove, etc.)
- **GlobFilter** -- matches paths against a glob pattern (both base name and full path)

### Combinators

```go
filter.And(f1, f2)  // all must match
filter.Or(f1, f2)   // at least one must match
filter.Not(f)       // inverts the match
```

### The Handler Interface

```go
type Handler interface {
    Handle(event watcher.Event) error
}
```

`HandlerFunc` adapts any `func(watcher.Event) error` to the `Handler` interface.

### Handler Chain

`Chain` runs handlers sequentially. The first error stops processing.

## Code Walkthrough

### Extension filter

```go
f := filter.NewExtensionFilter("go", "rs", "py")
// Matches: main.go, lib.rs, script.py
// Skips:   README.md, data.json
```

Extensions are normalized: `"go"` becomes `".go"`.

### Type filter

```go
f := filter.NewTypeFilter(watcher.Create, watcher.Write)
// Matches Create and Write events, skips Remove/Rename/Chmod
```

### Composed filter

```go
f := filter.And(
    filter.NewTypeFilter(watcher.Create, watcher.Write),
    filter.Or(
        filter.NewExtensionFilter("go"),
        filter.NewGlobFilter("Makefile"),
    ),
    filter.Not(filter.NewGlobFilter("*_test.go")),
)
// Matches: new/modified .go files and Makefiles, excluding test files
```

### Handler chain

```go
chain := handler.NewChain(
    handler.HandlerFunc(logHandler),
    handler.HandlerFunc(validateHandler),
    handler.HandlerFunc(processHandler),
)

for event := range w.Events() {
    if f.Match(event) {
        if err := chain.Handle(event); err != nil {
            log.Printf("chain error: %v", err)
        }
    }
}
```

## Practice Exercise

1. Create a filter that matches only Create events for `.go` files excluding `*_test.go`. Test with various event types and filenames.
2. Build a handler chain with three handlers: a counter, a logger, and a validator that rejects files over 1MB. Verify the chain stops on the validator error.
3. Compose a complex filter using And, Or, and Not: match video files (.mp4, .mkv) or image files (.jpg, .png) that are Create or Write events, excluding anything in a `.cache` directory. Test with 10 different events.
