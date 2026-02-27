# Tasks: Task Priority Aging (#331)

## Task Breakdown

### Phase 1: Data Model

- [ ] **1.1** Add `PriorityAgingConfig` struct to `internal/project/project.go`
  - Fields: `Enabled bool`, `Threshold time.Duration`, `BoostBy int`, `MaxBoost int`
- [ ] **1.2** Add `PriorityAging` field to `Config` struct
- [ ] **1.3** Add `PriorityAging *bool` field to `Todo` struct (tri-state: nil=inherit, false=disabled, true=enabled)
- [ ] **1.4** Add `EffectivePriority(config PriorityAgingConfig, waitTime time.Duration) int` method on Todo
  - Returns base priority minus boost (clamped to 0)
  - Handles all precedence cases (per-task disabled > global disabled)

### Phase 2: Config Loading

- [ ] **2.1** Verify `PriorityAgingConfig` loads from `.anvil/config.yaml` in `internal/config/config.go`
- [ ] **2.2** Set defaults: enabled=false, threshold=30m, boost_by=2, max_boost=4
- [ ] **2.3** Add validation (threshold > 0, boost_by >= 1, max_boost >= boost_by)

### Phase 3: Daemon Integration

- [ ] **3.1** Add `queuedAt time.Time` field to pending task state in daemon
- [ ] **3.2** Set `queuedAt` when task is added to pending queue
- [ ] **3.3** Pass config to daemon's priority sorting logic
- [ ] **3.4** Modify priority comparison at line ~2234 to use `EffectivePriority()`
- [ ] **3.5** Ensure daemon can access project's PriorityAgingConfig (may need to pass on startup)

### Phase 4: CLI Visibility

- [ ] **4.1** Add `Waited` and `EffectivePriority` fields to QueueItem type (if not present)
- [ ] **4.2** Modify `taskQueueCmd` in `cmd/anvil/main.go`:
  - Change header from "PRIORITY" to show both base and effective
  - Add "WAITED" column showing time in queue (e.g., "45m", "1m")
  - Add "(aged)" suffix when effective != base
  - Update column widths as needed

### Phase 5: Testing & Edge Cases

- [ ] **5.1** Write unit tests for `EffectivePriority` method
  - No aging when disabled globally
  - No aging when disabled per-task
  - Aging applied correctly with various wait times
  - Max boost cap enforced
  - Priority floor at 0
- [ ] **5.2** Manual testing:
  - Create tasks with p1 and p5 priorities
  - Let system accumulate pending tasks
  - Verify queue shows correct effective priorities
  - Test per-task disable with `priority_aging: false`

## Implementation Order

1. Data model (Phase 1) - no dependencies
2. Config loading (Phase 2) - depends on Phase 1
3. Daemon integration (Phase 3) - depends on Phase 1
4. CLI visibility (Phase 4) - depends on Phase 1
5. Testing (Phase 5) - can start after Phase 1, finishes after Phase 4

## Notes

- The issue specification shows p5 becoming p3, which means boost REDUCES the priority number (lower = higher priority). This is consistent with existing priority behavior (p0 = highest).
- Wait time should be calculated from when the task became due (not when it was created)
- If daemon restarts, wait times reset (expected - tasks go back to fresh state)
