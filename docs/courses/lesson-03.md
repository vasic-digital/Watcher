# Lesson 3: Standalone Debouncing

## Objectives

- Use the `Debouncer` independently of the watcher
- Understand generation counters and stale timer prevention
- Integrate debouncing into custom event systems

## Concepts

### The Debouncer

`debounce.Debouncer` coalesces rapid events for the same path. When multiple events arrive within the delay window, only the last one is forwarded to the output channel.

### Generation Counters

Each time `Add` is called for the same path, the generation counter increments and the timer resets. When a timer fires, it checks whether its generation matches the current generation. If a newer event has arrived since the timer was set, the generation will not match and the callback is discarded.

This prevents a common bug where an old timer fires and delivers a stale event.

## Code Walkthrough

### Creating a debouncer

```go
d := debounce.New(200*time.Millisecond, 50)
defer d.Close()
```

Parameters: delay duration and output channel buffer size.

### Adding events

```go
d.Add(watcher.Event{
    Type:      watcher.Write,
    Path:      "/data/config.yaml",
    Timestamp: time.Now(),
})
```

If a timer for `/data/config.yaml` is already pending, it is stopped and replaced with a new one. The generation counter increments.

### Consuming debounced events

```go
for event := range d.Events() {
    fmt.Printf("Debounced: %s %s\n", event.Type, event.Path)
}
```

The channel closes when `Close()` is called.

### How generation counting works

```
Add("file.go") -> gen=1, timer started
Add("file.go") -> gen=2, timer 1 stopped, timer 2 started
Add("file.go") -> gen=3, timer 2 stopped, timer 3 started
                   ... 200ms passes ...
Timer 3 fires  -> gen==3? yes -> emit event, clean up
```

Without generation counting, timer 1 and timer 2 would also fire, delivering duplicate events.

### Closing

```go
d.Close() // stops all pending timers, closes output channel
```

After `Close`, `Add` calls are no-ops. The `closed` flag is checked under the mutex.

## Summary

The standalone `Debouncer` is useful anywhere rapid events need coalescing -- file watchers, UI input, sensor data. Generation counters ensure correctness by preventing stale timer callbacks from delivering outdated events.
