# Implementation Plan: Task Priority Aging (#331)

## Overview

Add priority aging to prevent low-priority tasks from being starved. Tasks that wait too long get automatic priority boost, making them effectively higher priority without changing their base configuration.

## Implementation Steps

### Phase 1: Data Model (Priority, no dependencies)

**1.1 Add PriorityAgingConfig to project.go**
- File: `internal/project/project.go`
- Add `PriorityAgingConfig` struct with `Enabled`, `Threshold`, `BoostBy`, `MaxBoost` fields
- Add `PriorityAging` field to `Config` struct
- Add `PriorityAging` field to `Todo` struct (`*bool` pointer for tri-state)

**1.2 Add EffectivePriority method**
- File: `internal/project/project.go`
- Add `EffectivePriority(config PriorityAgingConfig, waitTime time.Duration) int` method on `Todo`
- Logic:
  - If PriorityAging is explicitly false, return base Priority
  - If global config Enabled is false and PriorityAging is nil, return base Priority
  - Calculate boost: min((waitTime / Threshold), MaxBoost) * BoostBy
  - Return max(0, Priority - boost)

### Phase 2: Config Loading (depends on Phase 1)

**2.1 Ensure config loads priority_aging**
- File: `internal/config/config.go`
- Verify PriorityAgingConfig is loaded from `.anvil/config.yaml`
- Add default values: threshold=30m, boost_by=2, max_boost=4, enabled=false (opt-in)

### Phase 3: Daemon Integration (depends on Phase 1)

**3.1 Track wait time in pending tasks**
- File: `internal/daemon/daemon.go`
- Add `queuedAt time.Time` field to the pending task state (in the map that holds pending tasks)
- Set `queuedAt` when task is added to pending queue
- Update to use wait time in priority calculation

**3.2 Update priority sorting**
- File: `internal/daemon/daemon.go`
- Modify the priority comparison at line ~2234 to use `EffectivePriority()`
- Need to pass the global config's PriorityAgingConfig and the task's wait time

**3.3 Make config accessible to daemon**
- Ensure daemon can access the project's PriorityAgingConfig
- May need to pass config to sorting function or store in daemon struct

### Phase 4: CLI Visibility (depends on Phase 1)

**4.1 Check for existing queue command**
- Look for `anvil task queue` command implementation
- If exists, modify to show effective priority
- If not, this may be a new command to create

**4.2 Add effective priority display**
- Show columns: TASK, WAITED, PRIORITY, EFFECTIVE
- Mark aged tasks with "(aged)" suffix in EFFECTIVE column

## Dependencies

- Phase 2 depends on Phase 1 (needs PriorityAgingConfig struct)
- Phase 3 depends on Phase 1 (needs EffectivePriority method)
- Phase 4 depends on Phase 1 (needs EffectivePriority method)
- Phase 2, 3, 4 are independent after Phase 1

## Testing Strategy

- Manual testing:
  - Create tasks with different priorities (p1, p5)
  - Let system run with pending tasks
  - Verify effective priority is shown correctly
  - Test per-task disable with `priority_aging: false`
- Unit tests for EffectivePriority method:
  - Test no aging when disabled
  - Test aging when enabled
  - Test max_boost cap
  - Test p0 floor

## Files to Modify

1. `internal/project/project.go` - Data model + EffectivePriority method
2. `internal/config/config.go` - Ensure config loading
3. `internal/daemon/daemon.go` - Sorting logic + wait time tracking
4. CLI command file (TBD - find existing queue command or create new)
