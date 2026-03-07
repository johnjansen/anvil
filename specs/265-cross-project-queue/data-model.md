# Data Model: Cross-Project Dependency Status in Task Queue

## Extended Entities

### TaskQueueInfo (extended)

Existing struct in `internal/daemon/daemon.go`. Add new fields for cross-project dependency information.

| Field | Type | Description |
|-------|------|-------------|
| Project | string | Project path (existing) |
| Name | string | Task name (existing) |
| Priority | int | Task priority (existing) |
| Schedule | string | Cron schedule (existing) |
| Status | string | running/pending/skipped (existing) |
| SkipReason | string | Why task was skipped (existing) |
| Boost | int | Priority boost from aging (existing) |
| CascadeCount | int | Cascade failure count (existing) |
| CascadeFrom | string | Cascade failure source (existing) |
| **CrossDeps** | **[]CrossDepStatus** | **NEW: Cross-project dependency statuses** |

### CrossDepStatus (new)

New struct representing the status of a single cross-project dependency.

| Field | Type | Description |
|-------|------|-------------|
| Project | string | Name of the external project |
| Task | string | Task name in the external project |
| Status | string | "success", "failed", "no runs", "unknown project", "task not found" |
| Blocking | bool | True if this dependency is preventing task execution |
| LastRun | string | Timestamp of the last run (empty if no runs) |

## State Transitions

Cross-project dependency status values:

```
unknown project  →  (project added to watched)  →  task not found
                                                  →  no runs  →  success
                                                              →  failed
```

- `success`: Last run record shows successful completion. Not blocking.
- `failed`: Last run record shows failure. Blocking.
- `no runs`: Task exists but has never been executed. Blocking.
- `unknown project`: Project not found in `~/.anvil/watched/`. Blocking.
- `task not found`: Project exists but task file not found. Blocking.

## Relationships

- A `TaskQueueInfo` has zero or more `CrossDepStatus` entries (via `CrossDeps` field)
- `CrossDepStatus` is derived from `project.Dependency` + `project.RunRecord`
- Resolution uses `project.ParseDependency()` → `project.ResolveDependencyRunRecord()`
