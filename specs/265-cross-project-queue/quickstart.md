# Quickstart: Cross-Project Dependency Status in Task Queue

## Overview

This feature extends `anvil task queue` to show cross-project dependency status. Three changes are needed:

1. **New struct** `CrossDepStatus` in `internal/daemon/daemon.go`
2. **Extend** `TaskQueueInfo` with a `CrossDeps` field
3. **Extend** `handleQueue` to resolve cross-project dependencies using existing `ResolveDependencyRunRecord`
4. **Extend** `taskQueueCmd` in `cmd/anvil/task_queue.go` to display cross-project info and support `--all`

## Key Files

| File | Change |
|------|--------|
| `internal/daemon/daemon.go` | Add `CrossDepStatus` struct, extend `TaskQueueInfo`, update `handleQueue` |
| `cmd/anvil/task_queue.go` | Add `--all` flag parsing, add CROSS-DEPS column, render cross-project entries |

## Key Functions to Use (already exist)

- `project.ParseDependency(dep string) Dependency` — parses `project:task` format
- `project.ResolveDependencyRunRecord(projectPath, dep string) (RunRecord, error)` — resolves to last run record
- `project.ResolveWatchedProjectPath(projectName string) (string, error)` — finds project path from watched dir

## Build & Test

```bash
go build ./cmd/anvil/
go test ./internal/daemon/ ./cmd/anvil/
```
