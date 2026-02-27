# Tasks: Task Fan-in/Fan-out Patterns for Parallel Execution (#323)

## Phase 1: Core Spawn Infrastructure

### Task 1.1: Add Spawn field to Todo struct
- **File:** `internal/project/project.go`
- **Description:** Add `Spawn []string` field to Todo struct with yaml tag. Add documentation comment explaining it's a list of task names to spawn in parallel.
- **Dependencies:** None
- **Estimate:** 0.25h

### Task 1.2: Add ParentID and ChildRunIDs to RunRecord
- **File:** `internal/project/project.go`
- **Description:** Add `ParentID string` and `ChildRunIDs []string` fields to RunRecord struct with JSON tags. Add documentation.
- **Dependencies:** 1.1
- **Estimate:** 0.25h

### Task 1.3: Create spawn helper functions
- **File:** `internal/project/spawn.go` (NEW)
- **Description:** Create helper functions:
  - `GetTaskTree(projectPath, taskID string) (parent RunRecord, children []RunRecord, error)` - get parent and children
  - `GetChildTasks(projectPath, spawnNames []string) ([]*Todo, error)` - resolve spawn names to Todo objects
  - `CreateChildRun(parent *Todo, child *Todo) (*RunRecord, error)` - create child run record
- **Dependencies:** 1.1, 1.2
- **Estimate:** 1h

### Task 1.4: Create task tree command
- **File:** `cmd/anvil/task_tree.go` (NEW)
- **Description:** Implement `taskTreeCmd(args []string)` that:
  - Takes task name as argument
  - `--all` flag to show all depth levels (default 3, max 5)
  - `--json` flag for machine-readable output
  - Calls GetTaskTree and prints tree structure
- **Dependencies:** 1.3
- **Estimate:** 1.5h

### Task 1.5: Register tree subcommand
- **File:** `cmd/anvil/main.go`
- **Description:** Add `case "tree"` to taskCmd switch, import task_tree package
- **Dependencies:** 1.4
- **Estimate:** 0.1h

## Phase 2: Scheduler Integration

### Task 2.1: Detect spawn field in scheduler
- **File:** `internal/daemon/scheduler.go`
- **Description:** Modify task dispatch logic to detect Todo.Spawn field. If present, queue children for parallel execution instead of running parent immediately.
- **Dependencies:** 1.1
- **Estimate:** 1h

### Task 2.2: Implement parallel child spawning
- **File:** `internal/daemon/scheduler.go`
- **Description:** When spawn detected:
  - Look up each child task name
  - Create new RunRecord for each child with ParentID set
  - Add child RunIDs to parent's ChildRunIDs
  - Dispatch all children in parallel using existing runner pool
- **Dependencies:** 2.1
- **Estimate:** 1.5h

### Task 2.3: Implement parent-wait-for-children
- **File:** `internal/daemon/scheduler.go`
- **Description:** Parent task blocks (doesn't complete) until all children in ChildRunIDs complete. Status reflects worst child status. Handle continue_on_error vs fail-fast.
- **Dependencies:** 2.2
- **Estimate:** 1h

### Task 2.4: Add cancellation propagation
- **File:** `internal/daemon/scheduler.go` or `daemon.go`
- **Description:** When parent is cancelled, propagate cancel signal to all active child RunRecords. Children check for cancellation before next work chunk.
- **Dependencies:** 2.3
- **Estimate:** 1h

## Phase 3: Dynamic Spawn

### Task 3.1: Add output monitoring for spawn directive
- **File:** `internal/daemon/runner.go` (or where stdout is captured)
- **Description:** Monitor task stdout/stderr for `##anvil:spawn <task-name> [--args]` pattern. Use regex to extract task name and arguments.
- **Dependencies:** Phase 2
- **Estimate:** 1h

### Task 3.2: Parse and create dynamic tasks
- **File:** `internal/daemon/runner.go`
- **Description:** On spawn directive match:
  - Parse task name and arguments
  - Look up task by name (or create new if doesn't exist - optional, start with existing tasks only)
  - Create child run instance
  - Queue for execution after parent completes
- **Dependencies:** 3.1
- **Estimate:** 1h

### Task 3.3: Handle multiple spawn directives
- **File:** `internal/daemon/runner.go`
- **Description:** Continue monitoring output until task completes. Collect all spawn directives. Spawn all child tasks in parallel after parent completes.
- **Dependencies:** 3.2
- **Estimate:** 0.5h

## Phase 4: Visibility & Polish

### Task 4.1: Update anvil ps for parent/child visibility
- **File:** `cmd/anvil/ps.go`
- **Description:**
  - Child tasks show "child-of: <parent-name>" in INFO column
  - Parent tasks show "parent-of: N tasks (X running, Y done, Z failed)"
- **Dependencies:** 1.2
- **Estimate:** 1h

### Task 4.2: Add persistence rebuild on startup
- **File:** `internal/daemon/daemon.go`
- **Description:** On daemon startup, scan recent RunRecords to rebuild parent-child relationships. Detect and mark orphaned children (parent no longer exists).
- **Dependencies:** 1.2
- **Estimate:** 0.5h

### Task 4.3: Add circular spawn detection
- **File:** `internal/daemon/scheduler.go`
- **Description:** Track task lineage IDs (parent → child → grandchild). Prevent spawning if task already in lineage (max 5 levels). Return error if circular.
- **Dependencies:** Phase 2
- **Estimate:** 0.5h

### Task 4.4: Add tests for spawn logic
- **File:** `internal/project/spawn_test.go` (NEW)
- **Description:** Add table-driven tests for:
  - Spawn field parsing
  - Task tree resolution
  - Parent-child relationship building
  - Circular detection
- **Dependencies:** 1.3
- **Estimate:** 1h

### Task 4.5: Manual testing
- **Description:** Create test tasks with spawn, run them, verify children spawn and parent waits. Test dynamic spawn. Verify tree command output.
- **Dependencies:** Phase 1-4
- **Estimate:** 1h

## Summary

| Phase | Tasks | Estimated Time |
|-------|-------|---------------|
| 1. Core Spawn Infrastructure | 5 | ~4.1h |
| 2. Scheduler Integration | 4 | ~4.5h |
| 3. Dynamic Spawn | 3 | ~2.5h |
| 4. Visibility & Polish | 5 | ~4h |
| **Total** | **17** | **~15.1h** |
