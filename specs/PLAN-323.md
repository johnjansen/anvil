# Implementation Plan: Task Fan-in/Fan-out Patterns for Parallel Execution

**Branch**: `323-task-fan-out` | **Date**: 2026-02-28 | **Spec**: [SPEC-323.md](SPEC-323.md)
**Input**: Issue #323: "Add task fan-in/fan-out patterns for parallel execution"

## Summary

Add task fan-out capability to spawn parallel child tasks from a parent task, plus dynamic spawning via `##anvil:spawn` output directives. Includes task tree visibility and fan-out status in process list. Fan-in (multiple `depends_on`) already exists.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `internal/project` (Todo, RunRecord), `internal/daemon` (task scheduling), standard library
**Storage**: Add ParentID/ChildIDs to RunRecord, persist to `.anvil/runs/<task-id>/`
**Testing**: `go test ./...` with table-driven tests
**Target Platform**: macOS, Linux (CLI tool + daemon)
**Project Type**: CLI tool with background daemon
**Performance Goals**: Child spawning should add <10ms overhead per child
**Constraints**: Must persist across daemon restarts
**Scale/Scope**: Per-project task management (typically 1-50 tasks, each may spawn 1-10 children)

## Constitution Check

*GATE: Must pass before implementation. Re-check after research.*

- Tests included for all new logic: YES
- Backward compatible (no changes to existing behavior): YES
- Follows existing patterns (Task struct, RunRecord, daemon scheduling): YES

**Post-Research re-check**: Design follows established patterns. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/323-task-fan-out/
├── SPEC-323.md            # Feature specification (this file parent)
└── PLAN-323.md            # This file
```

### Source Code (repository root)

```text
cmd/anvil/
├── main.go               # Add taskTreeCmd, integrate into task subcommand
└── task_tree.go          # NEW: Tree command implementation

internal/project/
├── project.go            # Add Spawn field to Todo, ParentID/Children to RunRecord
└── spawn.go              # NEW: Spawn logic and data structures

internal/daemon/
├── daemon.go             # Add parent-child tracking, cancellation propagation
├── scheduler.go          # Modify to handle spawn and wait-for-children logic
└── state.go              # Add TaskTree state tracking
```

**Structure Decision**: New `spawn.go` isolates spawn logic. Tree command in `cmd/anvil/task_tree.go`. Daemon modifications integrate parent-child lifecycle management.

## Technical Design

### 1. Data Structures

**SpawnConfig (new in Todo)**
```go
type Todo struct {
    // ... existing fields ...
    Spawn []string `yaml:"spawn"` // list of task names to spawn in parallel
}
```

**TaskSpawn (new)**
```go
type TaskSpawn struct {
    ParentID  string   // ID of parent task
    ChildIDs  []string // IDs of spawned child tasks
    CreatedAt time.Time
}
```

**Updated RunRecord**
```go
type RunRecord struct {
    // ... existing fields ...
    ParentID     string    // ID of parent task if this is a spawned child
    ChildRunIDs  []string // IDs of spawned child runs if this is a parent
}
```

### 2. Spawn Mechanism

**Static Spawn (frontmatter)**
1. When task with `spawn:` field starts, scheduler looks up each named task
2. Creates new run instances for each child task (unique RunID)
3. Runs all children in parallel using existing concurrency model
4. Parent waits (blocks) until all children complete
5. Parent status reflects worst child status (fail-fast unless continue_on_error)

**Dynamic Spawn (output directive)**
1. Task stdout/stderr is monitored for `##anvil:spawn` pattern
2. On match, parse task name and arguments
3. Create new task instance dynamically
4. Continue monitoring for more spawn directives until task completes
5. All dynamically spawned tasks run in parallel after parent completes

### 3. Cancellation Propagation

1. Parent cancellation signal triggers cancel on all active child RunRecords
2. Each child runner checks for cancellation before next work chunk
3. Children that complete after parent cancellation are marked "orphaned" in status

### 4. Tree Command

```
anvil task tree <parent-task> [--all] [--json]
```

- Default: Show only direct children
- `--all`: Show full depth (up to 5 levels)
- `--json`: Machine-readable output

Output format:
```
my-task (parent, running)
├── process-repo-a (child, running)
├── process-repo-b (child, success)
└── process-repo-c (child, pending)
```

### 5. Visibility in Process List

`anvil ps` output adds:
- Child tasks show: `child-of: <parent-name>` in INFO column
- Parent tasks show: `parent-of: N tasks (X running, Y done, Z failed)`

### 6. Persistence

- RunRecords with ParentID/ChildRunIDs written to `.anvil/runs/<task-id>/`
- Daemon rebuilds TaskTree on startup by scanning recent RunRecords
- Orphan detection: Children whose parent RunRecord no longer exists

## Implementation Phases

### Phase 1: Core Spawn Infrastructure

1. Add `Spawn` field to Todo struct in `internal/project/project.go`
2. Add `ParentID` and `ChildRunIDs` to RunRecord
3. Create `internal/project/spawn.go` with spawn helper functions
4. Create `cmd/anvil/task_tree.go` with tree command
5. Add tree subcommand to task command tree in main.go

### Phase 2: Scheduler Integration

1. Modify daemon scheduler to detect `spawn:` field
2. Implement parallel child spawning logic
3. Implement parent-wait-for-children blocking
4. Add cancellation propagation to child tasks

### Phase 3: Dynamic Spawn

1. Add output monitoring for `##anvil:spawn` pattern
2. Parse spawn directives from task output
3. Create dynamic task instances from directives
4. Handle multiple spawn directives in single run

### Phase 4: Visibility & Polish

1. Update `anvil ps` to show parent/child relationships
2. Add persistence rebuild on daemon startup
3. Add tests for all new functionality
4. Edge case handling (child with same name, deep nesting, etc.)

## Dependencies & Risks

**Dependencies**: None (uses existing daemon and storage)
**Risks**:
- Cancellation propagation timing: Children may take time to respond to cancellation. Acceptable — track as "terminating" state.
- Deep nesting performance: Limit to 5 levels to prevent runaway spawning. Enforced in scheduler.
- Circular spawn detection: Prevent task A spawning B which spawns A. Track lineage IDs.

## Acceptance Criteria Mapping

| AC | Implementation |
|----|----------------|
| Tasks can spawn parallel children via `spawn` frontmatter | Todo.Spawn field + scheduler parallel dispatch |
| Children run in parallel, parent waits for all | Parent blocks on ChildRunIDs completion |
| `##anvil:spawn` triggers dynamic task spawning | Output monitor in task runner |
| `anvil task tree` shows parent/child relationships | taskTreeCmd implementation |
| Fan-out status visible in `anvil ps` | Parent/child columns in ps output |

## Success Metrics

- Parent task with 3 children completes only after all children finish
- Dynamic spawn creates correct task instances from output
- Tree command accurately shows hierarchy
- `anvil ps` displays child count and status
- Cancellation propagates within 5 seconds
