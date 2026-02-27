# Tasks: Dry-Run Impact Analysis

**Input**: Design documents from `/specs/016-dryrun-impact/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: No tests explicitly requested in spec. Test tasks omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Create the new impact.go file and establish the data structures

- [ ] T001 Create cmd/anvil/impact.go with ImpactReport, Conflict, and Suggestion structs per data-model.md
- [ ] T002 Add --json flag parsing to taskCreateCmd dry-run flow in cmd/anvil/main.go (alongside existing --dry-run/-n flag)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Extract existing overlap detection into reusable functions

- [ ] T003 Extract overlap detection logic from cmd/anvil/main.go (lines 2165-2226) into a reusable function computeConflicts(schedule string, todos []project.Todo) []Conflict in cmd/anvil/impact.go
- [ ] T004 Refactor the original overlap detection in taskCreateCmd (main.go) to call the new computeConflicts function instead of inline code

**Checkpoint**: Existing overlap detection works identically through the refactored function

---

## Phase 3: User Story 1 - Schedule Conflict Detection (Priority: P1) MVP

**Goal**: anvil add --dry-run shows scheduling conflicts with existing tasks

**Independent Test**: Run anvil add -s "0 9 * * *" "Test" --dry-run against a project with existing 09:00 tasks and verify conflicts are listed

### Implementation for User Story 1

- [ ] T005 [US1] Implement analyzeImpact(schedule string, todos []project.Todo) ImpactReport function in cmd/anvil/impact.go that validates schedule, computes conflicts, and populates ImpactReport
- [ ] T006 [US1] Implement printImpactReport(report ImpactReport) function in cmd/anvil/impact.go for human-readable output per contracts/cli.md format
- [ ] T007 [US1] Update the --dry-run block in taskCreateCmd (cmd/anvil/main.go) to load todos, call analyzeImpact, and call printImpactReport instead of the current minimal validation

**Checkpoint**: anvil add --dry-run shows schedule validation + conflict list

---

## Phase 4: User Story 2 - Worker Load Estimate (Priority: P2)

**Goal**: anvil add --dry-run shows peak concurrency at proposed time slots

**Independent Test**: Run anvil add --dry-run with a schedule that overlaps multiple tasks and verify peak concurrency count appears

### Implementation for User Story 2

- [ ] T008 [US2] Implement computePeakConcurrency(schedule string, todos []project.Todo) (peakCount int, peakTime time.Time) in cmd/anvil/impact.go that enumerates next-24h firing times for all active tasks and finds the busiest time slot
- [ ] T009 [US2] Integrate peak concurrency results into analyzeImpact and printImpactReport in cmd/anvil/impact.go

**Checkpoint**: anvil add --dry-run shows conflicts + peak concurrency

---

## Phase 5: User Story 3 - Alternative Schedule Suggestions (Priority: P3)

**Goal**: When conflicts exist, suggest 2-3 alternative schedules with fewer conflicts

**Independent Test**: Run anvil add --dry-run with a conflicting schedule and verify alternative suggestions appear

### Implementation for User Story 3

- [ ] T010 [US3] Implement suggestAlternatives(schedule string, todos []project.Todo, currentConflicts int) []Suggestion in cmd/anvil/impact.go that shifts the proposed schedule by +/- 1-3 hours and returns alternatives with fewer conflicts
- [ ] T011 [US3] Integrate suggestions into analyzeImpact and printImpactReport in cmd/anvil/impact.go

**Checkpoint**: Full impact analysis with conflicts + concurrency + suggestions

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: JSON output and edge case handling

- [ ] T012 Implement printImpactJSON(report ImpactReport) function in cmd/anvil/impact.go for JSON output per contracts/cli.md format
- [ ] T013 Wire --json flag in taskCreateCmd (cmd/anvil/main.go) to call printImpactJSON instead of printImpactReport when --json and --dry-run are both set
- [ ] T014 Handle edge cases in analyzeImpact: no existing tasks, one-shot tasks (no schedule), invalid cron syntax — per spec.md edge cases section
- [ ] T015 Update help text in addCmd (cmd/anvil/main.go) to document the enhanced --dry-run and --json flags

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001 must exist before T003)
- **User Story 1 (Phase 3)**: Depends on Phase 2 (T003, T004)
- **User Story 2 (Phase 4)**: Depends on Phase 3 (T005 analyzeImpact must exist)
- **User Story 3 (Phase 5)**: Depends on Phase 3 (T005 analyzeImpact must exist)
- **Polish (Phase 6)**: Depends on Phase 3 (T005, T006 must exist for JSON variant)

### User Story Dependencies

- **User Story 1 (P1)**: Core — no other story dependencies
- **User Story 2 (P2)**: Extends US1's analyzeImpact function
- **User Story 3 (P3)**: Extends US1's analyzeImpact function, independent of US2

### Parallel Opportunities

- T001 and T002 can run in parallel (different files)
- US2 (Phase 4) and US3 (Phase 5) can theoretically run in parallel (independent features extending same function), but sequential is simpler since both modify impact.go
- T012 and T015 can run in parallel (different files)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T004)
3. Complete Phase 3: User Story 1 (T005-T007)
4. **STOP and VALIDATE**: Test `anvil add -s "0 9 * * *" "Test" --dry-run` shows conflicts
5. Continue with remaining stories

### Incremental Delivery

1. Setup + Foundational → Refactored overlap detection
2. User Story 1 → Conflicts shown in dry-run (MVP!)
3. User Story 2 → Peak concurrency added
4. User Story 3 → Alternative suggestions added
5. Polish → JSON output + edge cases + help text

---

## Notes

- All implementation is in cmd/anvil/ package (impact.go + main.go modifications)
- No new dependencies needed — uses existing internal/cron and internal/project
- Total tasks: 15
- US1: 3 tasks, US2: 2 tasks, US3: 2 tasks, Setup: 2 tasks, Foundation: 2 tasks, Polish: 4 tasks
