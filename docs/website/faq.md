# FAQ

## Does the watcher handle newly created subdirectories?

Yes. When `Recursive` is enabled in the config, the watcher automatically adds watches for newly created subdirectories. This means if a new folder is created inside a watched directory, its contents will also be monitored without manual intervention.

## How does debouncing work?

The debouncer uses generation-counted timers. When an event arrives for a path, a timer is started (or reset if one already exists). The generation counter increments with each reset. When the timer fires, it checks if the generation matches -- if a newer event arrived, the old timer is discarded. Only the latest event per path is forwarded after the quiet period elapses.

## What event types are supported?

Five event types: `Create`, `Write`, `Remove`, `Rename`, and `Chmod`. These map directly to the fsnotify event types. Use `filter.NewTypeFilter()` to select which types to process.

## How do I ignore specific files or directories?

Set `IgnorePatterns` in the watcher config. Patterns are matched against the file/directory name (not the full path). Examples: `.git` ignores the `.git` directory, `*.tmp` ignores all `.tmp` files, `Thumbs.db` ignores that specific file. Matching uses `filepath.Match` glob syntax.

## Is Close() idempotent?

Yes. All types in the module (`Watcher`, `Debouncer`) use `sync.Once` to ensure `Close()` is safe to call multiple times. The second and subsequent calls are no-ops.
