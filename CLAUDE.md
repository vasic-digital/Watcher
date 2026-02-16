# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`digital.vasic.watcher` is a standalone Go module for filesystem change monitoring. It provides event debouncing, filtering by type/extension/glob pattern, and composable handler chains. The module wraps fsnotify/fsnotify for cross-platform filesystem notifications.

## Commands

```bash
# Run all tests
go test ./... -count=1

# Run tests with verbose output
go test -v ./... -count=1

# Run a specific package's tests
go test -v ./pkg/watcher/ -count=1
go test -v ./pkg/handler/ -count=1
go test -v ./pkg/filter/ -count=1
go test -v ./pkg/debounce/ -count=1

# Build all packages
go build ./...

# Tidy dependencies
go mod tidy
```

## Architecture

The module is organized into four packages under `pkg/`:

| Package | Purpose |
|---|---|
| `pkg/watcher` | Core `Watcher` interface and fsnotify-backed implementation. Recursive directory watching, ignore patterns, built-in debouncing. |
| `pkg/handler` | `Handler` interface, `HandlerFunc` adapter, and `Chain` for sequential event processing pipelines. |
| `pkg/filter` | Composable `Filter` interface with `ExtensionFilter`, `TypeFilter`, `GlobFilter`, plus `And`, `Or`, `Not` combinators. |
| `pkg/debounce` | Standalone `Debouncer` that coalesces rapid events for the same path using generation-counted timers. |

### Data Flow

```
filesystem -> fsnotify -> Watcher (ignore + debounce) -> Events channel
                                                             |
                                              Filter.Match() -> Handler.Handle()
```

## Conventions

- All public types have doc comments.
- Tests use `t.TempDir()` for filesystem tests, no cleanup needed.
- Debouncing uses generation counters to prevent stale timer callbacks (pattern from Catalogizer's `SMBChangeWatcher`).
- Filters are composable via `And()`, `Or()`, `Not()` combinators.
- The `Watcher` interface is context-aware; cancelling the context stops watching.
- `Close()` is idempotent on all types via `sync.Once`.

## Constraints

- Minimum Go version: 1.24.0
- Primary dependency: `github.com/fsnotify/fsnotify` v1.8.0
- Test dependency: `github.com/stretchr/testify` v1.9.0
- No external logging framework; the module is designed for embedding.
