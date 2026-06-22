# digital.vasic.watcher

Filesystem change monitoring for Go with event debouncing, filtering, and handler chains.

> **Anti-bluff guarantee (CONST-035 / Article XI §11.9):** every test and
> Challenge in this submodule ships positive runtime evidence captured at
> execution. No "test PASS = feature works" without backing artefacts.
> Round-289 deep-doc + Challenge enrichment audit confirms this README
> matches the production surface verbatim (see `docs/test-coverage.md`).

## Features

- Cross-platform filesystem watching via [fsnotify](https://github.com/fsnotify/fsnotify)
- Recursive directory monitoring with automatic subdirectory tracking
- Configurable event debouncing to coalesce rapid changes
- Composable event filters (by extension, event type, glob pattern)
- Handler chains for building event processing pipelines
- Ignore patterns for excluding files and directories
- Context-aware watching with clean shutdown
- Locale-aware translation seam (`pkg/i18n`, CONST-046 compliant)

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

### Locale-aware messaging (CONST-046)

```go
import "digital.vasic.watcher/pkg/i18n"

// Production-safe default: keys returned verbatim, no project coupling.
t := i18n.NoopTranslator{}
label := t.T("watcher_event_create", map[string]any{"path": "/tmp/foo"})
// Consuming projects inject a real translator backed by bundles/active.<locale>.yaml.
```

## Packages

| Package        | Description                                                         |
|----------------|---------------------------------------------------------------------|
| `pkg/watcher`  | Core `Watcher` interface and fsnotify implementation                |
| `pkg/handler`  | Event handler interface, function adapter, and chain                |
| `pkg/filter`   | Event filters with `And`, `Or`, `Not` combinators                   |
| `pkg/debounce` | Standalone event debouncer with generation-counted timers           |
| `pkg/i18n`     | Locale-aware translator seam (`Translator`, `NoopTranslator`)       |

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

- Go 1.25.0+
- Linux, macOS, or Windows (via fsnotify)

## Testing & Challenges

```bash
# Unit + edge tests (race-checked)
go test -race -count=1 ./...

# Real Watcher exerciser (round-289 runner) — 5-locale bilingual evidence
go run ./challenges/runner -tmp ./.tmp-watch -events 8

# Paired-mutation describe challenge
bash challenges/watcher_describe_challenge.sh           # normal exit 0
WATCHER_DESCRIBE_MUTATE=1 bash challenges/watcher_describe_challenge.sh  # exit 99
```

See `docs/test-coverage.md` for the symbol→test ledger (every exported
symbol → which test exercises it → which Challenge captures runtime
evidence).

## Governance

This submodule is owned by `vasic-digital`. Every change cascades to the
parent project per CONST-047 + CONST-051(A). Governance
inheritance pointer: `CLAUDE.md`, `AGENTS.md`, `CONSTITUTION.md` start
with the inheritance preamble per CONST-059. The `pkg/i18n` seam keeps
the submodule project-not-aware per CONST-051(B).
