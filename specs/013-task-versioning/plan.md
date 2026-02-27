# Implementation Plan: Task Versioning

## Technical Context

- **Language**: Go 1.24.6 (stdlib only: encoding/json, os/exec, time, path/filepath)
- **Architecture**: New `internal/versioning` package, integrated into project/todo save operations
- **Storage**: JSON files in `.anvil/versions/<task-id>/`
- **Dependencies**: Uses existing `project.LoadTodos()`, `project.WriteTodo()` functions

## File Structure

```text
internal/versioning/types.go       — TaskVersion, VersionIndex types
internal/versioning/store.go       — Read/write versions to .anvil/versions/<task-id>/
internal/versioning/differ.go     — Generate unified diffs between versions
internal/versioning/git.go         — Git blame integration
internal/project/project.go       — Add versioning hooks in AddTodo/UpdateTodo
cmd/anvil/main.go                 — Add task history --versions, task diff, task restore, task blame commands
```

## Implementation Approach

### Phase 1: Core Types and Storage (Setup)
1. Define TaskVersion, VersionIndex structs
2. Implement VersionStore with CreateVersion, GetVersion, ListVersions, GetLatestVersion
3. Add versioning package with auto-versioning on task file changes

### Phase 2: Version History Command (US1, US2 - P1)
4. Add `--versions` flag to existing `taskHistoryCmd`
5. Implement version list display with version, date, author columns
6. Handle edge case: task with no versions yet

### Phase 3: Diff Command (US3 - P1)
7. Create new `taskDiffCmd` for comparing versions
8. Implement unified diff generation between any two versions
9. Default to comparing v1 and latest if no versions specified

### Phase 4: Restore Command (US4 - P1)
10. Create new `taskRestoreCmd` for reverting versions
11. Implement restore that overwrites current task file
12. Auto-create new version recording the restore

### Phase 5: Git Blame (US5 - P2)
13. Create new `taskBlameCmd`
14. Implement git blame via `os/exec` (git blame <file>)
15. Handle case when not in git repository

## Dependencies
- internal/project/project.go (LoadTodos, WriteTodo, GetTodoPath)
- internal/config (no changes needed)
- No external dependencies

## Constitution Check
- All stdlib, no new dependencies ✓
- Backward compatible (versioning is transparent to existing workflows) ✓
- Works offline, no network dependencies ✓
- Version history persists in project directory ✓
