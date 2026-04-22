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


## ⚠️ MANDATORY: NO SUDO OR ROOT EXECUTION

**ALL operations MUST run at local user level ONLY.**

This is a PERMANENT and NON-NEGOTIABLE security constraint:

- **NEVER** use `sudo` in ANY command
- **NEVER** use `su` in ANY command
- **NEVER** execute operations as `root` user
- **NEVER** elevate privileges for file operations
- **ALL** infrastructure commands MUST use user-level container runtimes (rootless podman/docker)
- **ALL** file operations MUST be within user-accessible directories
- **ALL** service management MUST be done via user systemd or local process management
- **ALL** builds, tests, and deployments MUST run as the current user

### Container-Based Solutions
When a build or runtime environment requires system-level dependencies, use containers instead of elevation:

- **Use the `Containers` submodule** (`https://github.com/vasic-digital/Containers`) for containerized build and runtime environments
- **Add the `Containers` submodule as a Git dependency** and configure it for local use within the project
- **Build and run inside containers** to avoid any need for privilege escalation
- **Rootless Podman/Docker** is the preferred container runtime

### Why This Matters
- **Security**: Prevents accidental system-wide damage
- **Reproducibility**: User-level operations are portable across systems
- **Safety**: Limits blast radius of any issues
- **Best Practice**: Modern container workflows are rootless by design

### When You See SUDO
If any script or command suggests using `sudo` or `su`:
1. STOP immediately
2. Find a user-level alternative
3. Use rootless container runtimes
4. Use the `Containers` submodule for containerized builds
5. Modify commands to work within user permissions

**VIOLATION OF THIS CONSTRAINT IS STRICTLY PROHIBITED.**


