# Research: Filesystem Subscription for Task Triggers

**Feature**: Filesystem Subscription for Task Triggers
**Date**: 2026-03-01

## Research Summary

This feature builds on the task subscription framework defined in spec 016-task-subscriptions. The filesystem subscription allows tasks to be triggered by file system events.

## Key Decisions

### Decision: File Watching Library

**Chosen**: `github.com/fsnotify/fsnotify` (v1.7.0+)

**Rationale**:
- Cross-platform support (Linux, macOS, Windows)
- Uses OS-native notifications (inotify on Linux, FSEvents on macOS, ReadDirectoryChangesW on Windows)
- Well-maintained and widely used
- Simple API

**Alternatives considered**:
- `gopkg.in/fsnotify.v1`: Older version, less maintained
- `github.com/howeyc/fsnotify`: Less active, fewer features
- Custom implementation using `notify` package: More complex, no benefit

### Decision: Pattern Matching

**Chosen**: Standard library `path/filepath.Match` and `filepath.Glob`

**Rationale**:
- No additional dependencies
- Matches Go's standard path patterns
- Sufficient for glob patterns like `*.json`, `data/*.txt`, `**/*.log`

**Alternatives considered**:
- `github.com/gobwas/glob`: More powerful patterns but adds dependency
- Custom regex: Overkill for simple glob patterns

### Decision: Event Types

**Chosen**: Create, Modify, Delete as distinct event types

**Rationale**:
- Matches common filesystem event types across platforms
- Simple to understand and configure
- Already defined in spec 016-task-subscriptions

### Decision: Configuration Storage

**Chosen**: JSON files in `.anvil/subscriptions/` (following existing patterns)

**Rationale**:
- Consistent with `.anvil/alerts/` and `.anvil/circuits/` structure
- Simple to manage without external database
- Human-readable for debugging

## Implementation Approach

1. Add `subscription.fs` field to task frontmatter (in project.go)
2. Create `internal/subscription/fs/` package for filesystem watcher
3. Integrate with daemon's subscription manager
4. Pass file event data via environment variables (following webhook pattern)

## Integration with Existing Subscription Framework

Per spec 016-task-subscriptions:
- `subscription.type: fs` in task frontmatter
- `subscription.path: ./data/*.json` for glob pattern
- `subscription.events: [create, modify, delete]` for event filtering
- File event data available via `ANVIL_FS_PATH`, `ANVIL_FS_EVENT`, `ANVIL_FS_TIMESTAMP`

## Notes

- No additional dependencies beyond fsnotify needed
- Follows existing patterns for alerts and circuits in .anvil directory
- Implementation should integrate with the daemon like other subscription types
