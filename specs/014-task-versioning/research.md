# Research: Task Diff and Versioning

**Date**: 2026-02-27
**Feature**: 014-task-versioning

## Decision 1: Version Storage Format

**Decision**: Store each version as a JSON file containing full task file content plus metadata.

**Rationale**: Follows the established `RunRecord` pattern in `project.go` (line 704, `WriteRunRecord()`). Full snapshots are simpler than delta-based storage and allow direct content comparison. JSON format is consistent with all other .anvil/ data files.

**Alternatives considered**:
- Git-based versioning: Rejected -- adds git dependency for non-git projects
- Delta-based storage: Rejected -- complexity not justified for task files (typically <1KB)
- SQLite database: Rejected -- inconsistent with existing file-based storage pattern

## Decision 2: Change Detection Mechanism

**Decision**: Add SHA256 hash tracking to the daemon, computed on each tick when loading todos.

**Rationale**: The daemon already calls `proj.LoadTodos()` on every tick (daemon.go line 2271). Computing a hash of each task file content during this load is cheap and reliable. No existing hash tracking exists in the Daemon struct (confirmed by research).

**Alternatives considered**:
- File modification time (mtime): Rejected -- unreliable across filesystems, can be reset by tools
- Filesystem watcher (fsnotify): Rejected -- adds external dependency, daemon already polls
- Manual-only versioning: Rejected -- spec requires automatic versioning (FR-009)

## Decision 3: Version Numbering

**Decision**: Sequential integers starting at 1, stored as `v1.json`, `v2.json`, etc.

**Rationale**: Simple, human-readable, and easy to determine next version by counting files in the directory. Matches the spec assumption and user-facing format (`v1`, `v2`, `v3`).

**Alternatives considered**:
- Timestamp-based names: Rejected -- harder to reference in CLI commands
- UUID-based names: Rejected -- not human-friendly

## Decision 4: Author Detection

**Decision**: Try `git config user.name` first, fall back to `os/user` `Current().Username`.

**Rationale**: Most anvil projects are git-tracked, so git config is the natural source. System username is a reliable fallback. No existing author detection in the codebase.

**Alternatives considered**:
- Environment variable only: Rejected -- less reliable than git config
- Config file setting: Rejected -- over-engineering for initial implementation

## Decision 5: Diff Algorithm

**Decision**: Use Go's standard line-by-line diff with unified diff format output.

**Rationale**: Task files are plain text (markdown with frontmatter). A simple line-by-line comparison produces clear, familiar output. No external diff library needed -- implement minimal unified diff in Go.

**Alternatives considered**:
- External diff command: Rejected -- adds OS dependency, less portable
- Structured frontmatter diff: Rejected -- over-engineering, unified diff covers all content

## Decision 6: CLI Command Structure

**Decision**: Add `diff`, `restore`, `blame` as new `taskCmd()` subcommands. Add `--versions` flag to existing `history` subcommand.

**Rationale**: Follows existing `taskCmd()` dispatcher pattern (main.go line 1847). The `--versions` flag on history avoids creating a separate `versions` subcommand since version history is conceptually related to task history.

**Alternatives considered**:
- Separate `versions` subcommand: Rejected -- `--versions` flag is more discoverable alongside run history
- Nested `task version diff` commands: Rejected -- too deep, `task diff` is cleaner

## Decision 7: Daemon Integration Point

**Decision**: Add versioning check in `tick()` after `LoadTodos()` returns, before task dispatch.

**Rationale**: `LoadTodos()` already reads all task files. By hashing their content and comparing against a stored map, we detect changes with minimal overhead. Versioning before dispatch ensures the snapshot is created before the task runs.

**Alternatives considered**:
- Separate versioning goroutine: Rejected -- adds concurrency complexity for no benefit
- Post-dispatch versioning: Rejected -- could miss the pre-execution state
