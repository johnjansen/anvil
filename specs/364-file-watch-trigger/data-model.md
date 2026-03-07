# Data Model: File Watcher Trigger for Tasks

**Feature**: 364-file-watch-trigger
**Date**: 2026-03-07

## Entities

### SubscriptionConfig (extended)

Extends the existing `SubscriptionConfig` in `internal/project/project.go`.

**New fields**:

| Field      | Type       | YAML Key    | Description                                                        | Default              |
| ---------- | ---------- | ----------- | ------------------------------------------------------------------ | -------------------- |
| FsEvents   | `[]string` | `fs_events` | Event types to watch: `create`, `modify`, `delete`, `rename`       | `[create, modify, delete]` |
| FsDebounce | `string`   | `fs_debounce` | Duration string for debounce window (e.g., `5s`, `500ms`)        | `1s`                 |
| FsGlob     | `string`   | `fs_glob`   | Glob pattern to filter file events (e.g., `*.json`, `**/*.yaml`)  | `*` (all files)      |
| FsRecursive | `bool`    | `fs_recursive` | Whether to watch subdirectories recursively                     | `false`              |

**Existing fields used**:

| Field    | Type     | YAML Key  | Description                         |
| -------- | -------- | --------- | ----------------------------------- |
| Type     | `string` | `type`    | Must be `"fs"` for file watching    |
| FsPath   | `string` | `fs_path` | Directory path to watch             |

### FileEvent (new, internal)

Represents a single file change event collected during a debounce window.

| Field     | Type        | JSON Key    | Description                              |
| --------- | ----------- | ----------- | ---------------------------------------- |
| Path      | `string`    | `path`      | Absolute path of the changed file        |
| Event     | `string`    | `event`     | Event type: `create`, `modify`, `delete`, `rename` |
| Timestamp | `time.Time` | `timestamp` | When the event was detected              |

### debouncer (new, internal)

Runtime component within `FSWatcher` that manages the debounce timer for a task.

| Field       | Type           | Description                                       |
| ----------- | -------------- | ------------------------------------------------- |
| timer       | `*time.Timer`  | Debounce timer, reset on each new event           |
| duration    | `time.Duration` | Configured debounce window                       |
| events      | `[]FileEvent`  | Accumulated events during current debounce window |
| mu          | `sync.Mutex`   | Protects events slice and timer                   |
| onFlush     | `func([]FileEvent)` | Callback invoked when debounce timer fires   |

## State Transitions

### Watcher Lifecycle

```
[Not Watching] ---(daemon starts with fs task)---> [Watching]
[Watching] ---(file event matches glob + event filter)---> [Debouncing]
[Debouncing] ---(new matching event)---> [Debouncing] (timer reset)
[Debouncing] ---(timer fires)---> [Triggering] ---> [Watching]
[Watching] ---(task removed / daemon stops)---> [Not Watching]
```

### Directory Missing State

```
[Not Watching] ---(fs_path doesn't exist)---> [Waiting for Directory]
[Waiting for Directory] ---(directory created)---> [Watching]
[Waiting for Directory] ---(daemon stops)---> [Not Watching]
```

## Relationships

- `SubscriptionConfig` is embedded in `Todo` (existing relationship, no change).
- `FSWatcher` holds a map of `watcher` instances keyed by task ID (existing relationship).
- Each `watcher` gains a `debouncer` for managing the debounce window.
- `FileEvent` objects are accumulated in the `debouncer` and passed to the task as environment variables on flush.

## Validation Rules

- `fs_path` must be a non-empty string.
- `fs_events` values must be one of: `create`, `modify`, `delete`, `rename`.
- `fs_debounce` must be a valid Go duration string (e.g., `1s`, `500ms`, `2m`). Minimum: `100ms`.
- `fs_glob` must be a valid glob pattern per `filepath.Match` rules.
- If `fs_recursive` is true, `fs_path` must be a directory (not a file path).
