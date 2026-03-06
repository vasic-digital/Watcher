# Course: Filesystem Change Monitoring in Go

## Module Overview

This course covers the `digital.vasic.watcher` module, which wraps `fsnotify` with recursive directory watching, configurable ignore patterns, event debouncing, composable filters with boolean combinators, and handler chains. You will learn to build reactive file monitoring pipelines.

## Prerequisites

- Intermediate Go knowledge (goroutines, channels, filesystem operations)
- Basic understanding of filesystem events (create, write, remove, rename)
- Go 1.24+ installed

## Lessons

| # | Title | Duration |
|---|-------|----------|
| 1 | The Watcher -- Recursive Watching and Debouncing | 40 min |
| 2 | Filters and Handlers -- Composable Event Pipelines | 40 min |
| 3 | Standalone Debouncing -- Generation Counters | 35 min |

## Source Files

- `pkg/watcher/` -- Core Watcher interface, fsnotify adapter, recursive watching
- `pkg/debounce/` -- Standalone debouncer with generation-counted timers
- `pkg/filter/` -- Composable Filter interface with And/Or/Not combinators
- `pkg/handler/` -- Handler interface, HandlerFunc adapter, Chain
