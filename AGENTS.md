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


### ⚠️⚠️⚠️ ABSOLUTELY MANDATORY: ZERO UNFINISHED WORK POLICY

NO unfinished work, TODOs, or known issues may remain in the codebase. EVER.

PROHIBITED: TODO/FIXME comments, empty implementations, silent errors, fake data, unwrap() calls that panic, empty catch blocks.

REQUIRED: Fix ALL issues immediately, complete implementations before committing, proper error handling in ALL code paths, real test assertions.

Quality Principle: If it is not finished, it does not ship. If it ships, it is finished.
