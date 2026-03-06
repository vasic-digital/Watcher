# Watcher Module

`digital.vasic.watcher` is a standalone Go module for filesystem change monitoring. It wraps `fsnotify/fsnotify` with recursive directory watching, configurable ignore patterns, event debouncing, composable filters, and handler chains.

## Key Features

- **Recursive watching** -- Automatically watches subdirectories created after initial setup
- **Ignore patterns** -- Configurable patterns to skip directories and files (e.g., `.git`, `*.tmp`)
- **Event debouncing** -- Coalesces rapid events for the same path using generation-counted timers
- **Composable filters** -- `ExtensionFilter`, `TypeFilter`, `GlobFilter` with boolean combinators (`And`, `Or`, `Not`)
- **Handler chains** -- Sequential event processing pipelines with error propagation
- **Context-aware** -- Cancelling the context stops watching and cleans up resources

## Package Overview

| Package | Purpose |
|---------|---------|
| `pkg/watcher` | Core `Watcher` interface and fsnotify-backed implementation |
| `pkg/debounce` | Standalone event debouncer with generation-counted timers |
| `pkg/filter` | Composable `Filter` interface with extension, type, glob, and boolean combinators |
| `pkg/handler` | `Handler` interface, `HandlerFunc` adapter, and `Chain` for event processing |

## Installation

```bash
go get digital.vasic.watcher
```

Requires Go 1.24 or later.

## Dependencies

| Dependency | Purpose |
|-----------|---------|
| `github.com/fsnotify/fsnotify` | Cross-platform filesystem notifications |
