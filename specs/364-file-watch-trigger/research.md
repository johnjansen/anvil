# Research: File Watcher Trigger for Tasks

**Feature**: 364-file-watch-trigger
**Date**: 2026-03-07

## R1: Debounce Strategy for File Events

**Decision**: Timer-reset debounce — each new event resets a per-task timer. When the timer fires (no new events within the debounce window), the task is triggered with all accumulated events.

**Rationale**: This is the standard debounce pattern for file watchers. It handles bulk operations (e.g., `git checkout` modifying many files) by waiting for activity to settle. A simple `time.AfterFunc` with reset on each event is efficient and idiomatic in Go.

**Alternatives considered**:
- Fixed window (trigger every N seconds regardless): Doesn't handle bursty writes well; may trigger mid-operation.
- Leading-edge debounce (trigger immediately, ignore subsequent): Misses files from bulk operations.
- Throttle (trigger at most once per interval): Similar to fixed window, still may trigger mid-operation.

## R2: Glob Pattern Matching with fsnotify

**Decision**: Watch the parent directory and filter events by matching the file path against the configured glob pattern using `filepath.Match` or `path.Match`. For recursive globs (`**`), watch subdirectories recursively.

**Rationale**: `fsnotify` watches directories, not glob patterns. The standard approach is to watch the directory and filter in the event handler. `filepath.Match` handles standard globs (`*.json`, `data-*`). For `**` recursive patterns, we need to walk the directory tree and add watchers for each subdirectory, plus watch for new subdirectory creation.

**Alternatives considered**:
- Watching individual files: Doesn't work for new file creation (the file doesn't exist yet to watch).
- Using a polling approach: Defeats the purpose of native OS notifications.

## R3: Event Type Mapping

**Decision**: Map user-facing event types to fsnotify operations:
- `create` → `fsnotify.Create`
- `modify` → `fsnotify.Write`
- `delete` → `fsnotify.Remove`
- `rename` → `fsnotify.Rename` (optional, included for completeness)

Default when no events specified: `[create, modify, delete]` (all events).

**Rationale**: These map cleanly to fsnotify's event types. Using user-friendly names (`create`/`modify`/`delete`) rather than OS-level names (`Write`/`Remove`) makes the YAML configuration intuitive.

**Alternatives considered**:
- Exposing fsnotify event names directly: Less user-friendly.
- Only supporting create: Too limiting for config-reload use cases.

## R4: Passing File Change Information to Tasks

**Decision**: Use environment variables, consistent with the existing `ANVIL_FS_EVENT` and `ANVIL_FS_PATH` pattern in `fs.go`:
- `ANVIL_FS_EVENTS`: JSON array of `{"path": "...", "event": "..."}` objects for the debounced batch.
- `ANVIL_FS_EVENT`: Event type of the last change (backward compatible).
- `ANVIL_FS_PATH`: Path of the last changed file (backward compatible).
- `ANVIL_FS_EVENT_COUNT`: Number of files changed in the batch.

**Rationale**: Environment variables are already used by the existing FSWatcher. Adding a JSON array for the full batch enables scripts to process all changed files while maintaining backward compatibility with the existing single-event variables.

**Alternatives considered**:
- Writing a temp file with the list: Adds cleanup complexity.
- Passing as command-line arguments: May exceed argument length limits for large batches.

## R5: Handling Non-Existent Watch Directories

**Decision**: Log a warning and start a polling check (every 30 seconds) for the directory to appear. Once it exists, create the fsnotify watcher. Also watch the parent directory for creation events if the parent exists.

**Rationale**: Users may configure tasks before the data directory exists (e.g., before first deployment). Silently failing would be confusing; erroring out would prevent daemon startup for valid configurations.

**Alternatives considered**:
- Error on startup: Too strict; prevents valid use cases.
- Ignore silently: User won't know why tasks aren't triggering.

## R6: Integration with Trigger Framework (#363)

**Decision**: The `file_watch` trigger will be implemented as an extension to the existing `SubscriptionConfig` system (type: `fs`), not as a new trigger type under the `TaskTrigger` framework. This is consistent with how the existing `FSWatcher` already works.

**Rationale**: The existing `SubscriptionConfig` already has `type: "fs"` with `fs_path`. Adding `events`, `debounce`, and `glob` fields to `SubscriptionConfig` is the minimal change. If #363 introduces a new trigger registration mechanism, the `file_watch` type can be migrated later.

**Alternatives considered**:
- Wait for #363 to land first: Delays this feature unnecessarily; the existing subscription system works.
- Create a parallel trigger type: Would duplicate the FSWatcher infrastructure.
