# Lesson 1: The Watcher

## Objectives

- Create and configure an fsnotify-backed watcher
- Understand recursive watching and ignore patterns
- Use built-in event debouncing

## Concepts

### The Watcher Interface

```go
type Watcher interface {
    Watch(ctx context.Context, paths ...string) error
    Events() <-chan Event
    Errors() <-chan error
    Close() error
}
```

### Event Types

Five event types map directly to filesystem operations: `Create`, `Write`, `Remove`, `Rename`, `Chmod`.

```go
type Event struct {
    Type      EventType
    Path      string
    OldPath   string    // for rename events
    Timestamp time.Time
}
```

### Configuration

```go
cfg := &watcher.Config{
    Recursive:      true,                // watch subdirectories
    DebounceDelay:  100 * time.Millisecond, // coalesce rapid events
    BufferSize:     100,                 // channel capacity
    IgnorePatterns: []string{".git", "*.swp"},
}
```

## Code Walkthrough

### Creating and starting

```go
w, err := watcher.New(cfg)
if err != nil {
    log.Fatal(err)
}
defer w.Close()

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

err = w.Watch(ctx, "/data/projects")
```

`Watch` walks the root directory, adds all subdirectories (if recursive), and starts the event loop goroutine.

### Recursive directory addition

When a new directory is created inside a watched path, the watcher detects the `Create` event, checks if it is a directory, and calls `addRecursive` to add it and its children.

### Ignore patterns

Patterns use `filepath.Match` syntax:

```go
IgnorePatterns: []string{
    ".git",          // matches .git directory
    "*.tmp",         // matches any .tmp file
    "node_modules",  // matches node_modules directory
}
```

Matching is tried against both the base name and the full path.

### Consuming events and errors

```go
go func() {
    for err := range w.Errors() {
        log.Println("watcher error:", err)
    }
}()

for event := range w.Events() {
    fmt.Printf("%s: %s\n", event.Type, event.Path)
}
```

### Debouncing

When `DebounceDelay > 0`, the watcher coalesces rapid events for the same path. Only the last event within the delay window is forwarded. The generation counter prevents stale timer callbacks from firing.

## Practice Exercise

1. Create a watcher with recursive=true and a temporary directory. Create a file inside, then create a subdirectory with a file inside it. Verify events are received for both levels.
2. Configure ignore patterns for `*.tmp` and `.git`. Create matching and non-matching files. Verify only non-matching files produce events.
3. Set `DebounceDelay=200ms`. Write to the same file 10 times rapidly. Verify only one event is received for that file (the last one).
