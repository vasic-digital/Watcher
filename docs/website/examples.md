# Examples

## Media Library Watcher

Watch a media library for new video files and process them:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "digital.vasic.watcher/pkg/filter"
    "digital.vasic.watcher/pkg/handler"
    "digital.vasic.watcher/pkg/watcher"
)

func main() {
    w, err := watcher.New(&watcher.Config{
        Recursive:      true,
        DebounceDelay:  500 * time.Millisecond,
        BufferSize:     512,
        IgnorePatterns: []string{".git", "*.part", "*.tmp", "Thumbs.db", ".DS_Store"},
    })
    if err != nil {
        log.Fatal(err)
    }
    defer w.Close()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    w.Watch(ctx, "/media/movies", "/media/tv", "/media/music")

    // Filter: new or modified video/audio files
    mediaFilter := filter.And(
        filter.NewTypeFilter(watcher.Create, watcher.Write),
        filter.Or(
            filter.NewExtensionFilter("mp4", "mkv", "avi", "mov"),
            filter.NewExtensionFilter("flac", "mp3", "wav", "aac"),
        ),
    )

    chain := handler.NewChain(
        handler.HandlerFunc(func(e watcher.Event) error {
            fmt.Printf("New media: %s\n", e.Path)
            return nil
        }),
        handler.HandlerFunc(func(e watcher.Event) error {
            // Trigger media detection pipeline
            return detectAndCatalog(e.Path)
        }),
    )

    for event := range w.Events() {
        if mediaFilter.Match(event) {
            if err := chain.Handle(event); err != nil {
                log.Printf("Processing error: %v", err)
            }
        }
    }
}
```

## Complex Filter Composition

Build sophisticated filters using boolean combinators:

```go
package main

import (
    "digital.vasic.watcher/pkg/filter"
    "digital.vasic.watcher/pkg/watcher"
)

func main() {
    // Source code changes: .go or .ts files, excluding test files
    sourceFilter := filter.And(
        filter.NewTypeFilter(watcher.Create, watcher.Write),
        filter.Or(
            filter.NewExtensionFilter("go"),
            filter.NewExtensionFilter("ts", "tsx"),
        ),
        filter.Not(filter.NewGlobFilter("*_test.go")),
        filter.Not(filter.NewGlobFilter("*.spec.ts")),
    )

    // Config changes: JSON or YAML in specific directories
    configFilter := filter.And(
        filter.NewTypeFilter(watcher.Write),
        filter.Or(
            filter.NewExtensionFilter("json", "yaml", "yml"),
        ),
        filter.NewGlobFilter("config/*"),
    )

    // Combined: either source or config changes
    anyChange := filter.Or(sourceFilter, configFilter)

    for event := range w.Events() {
        if anyChange.Match(event) {
            triggerRebuild(event)
        }
    }
}
```

## Debouncing Rapid File Saves

Use the standalone debouncer to handle editors that save files with multiple rapid writes:

```go
package main

import (
    "fmt"
    "time"

    "digital.vasic.watcher/pkg/debounce"
    "digital.vasic.watcher/pkg/watcher"
)

func main() {
    // 300ms quiet period before forwarding an event
    d := debounce.New(300*time.Millisecond, 256)
    defer d.Close()

    // Feed watcher events into the debouncer
    go func() {
        for event := range w.Events() {
            d.Add(event)
        }
    }()

    // Only the last event per path within the quiet period is emitted
    for event := range d.Events() {
        fmt.Printf("Debounced: %s %s\n", event.Type, event.Path)
    }
}
```
