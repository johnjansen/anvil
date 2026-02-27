# Tasks: Task Dry-Run Impact Analysis

**Input**: Design documents from `/specs/010-dry-run-impact-analysis/`
**Prerequisites**: plan.md, spec.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)

---

## Phase 1: Core Infrastructure (Shared by All Stories)

**Purpose**: Data structures and helper functions needed by all user stories

- [ ] T001 [P] [US1-US5] Add ImpactAnalysis, ConflictInfo, CostEstimate, AlternativeSchedule structs to `cmd/anvil/main.go`
- [ ] T002 [P] [US1] Extract conflict detection into `checkConflicts(schedule string) []ConflictInfo` function
- [ ] T003 [P] [US2] Implement `estimateCost(content string, schedule string) CostEstimate` function
- [ ] T004 [P] [US3] Implement `suggestAlternatives(schedule string, conflicts []ConflictInfo) []AlternativeSchedule` function

**Checkpoint**: Core helper functions ready - can proceed to CLI integration

---

## Phase 2: CLI Integration (User Stories 1 & 4)

**Purpose**: Add --dry-run flag and integrate impact analysis into anvil add

- [ ] T005 [US1] Add `--dry-run` / `-n` flag to `taskCreateCmd` in `cmd/anvil/main.go`
- [ ] T006 [US1] Implement `printImpactAnalysis(impact ImpactAnalysis)` function with formatted output
- [ ] T007 [US4] Add interactive confirmation prompt "Add anyway? [y/N]" when conflicts exist
- [ ] T008 [US1] Wire dry-run flow: detect conflicts → calculate cost → print analysis → prompt (if needed) → proceed or exit

**Checkpoint**: --dry-run flag fully functional

---

## Phase 3: Edge Cases & Polish

**Purpose**: Handle edge cases and non-interactive mode

- [ ] T009 [US1] Handle non-interactive mode: exit 1 if conflicts, exit 0 otherwise
- [ ] T010 [P] [US1] Handle empty content: show error before impact analysis
- [ ] T011 [P] [US1] Handle invalid schedule: show parse error before impact analysis
- [ ] T012 [US1] Handle many conflicts: truncate to 10, show "and X more"
- [ ] T013 [US2] Handle one-shot task: show per-run cost instead of monthly

**Checkpoint**: All edge cases handled

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Core)**: No dependencies - can start immediately
- **Phase 2 (CLI)**: Depends on Phase 1 completion - BLOCKS user story delivery
- **Phase 3 (Polish)**: Depends on Phase 2 completion - finalizes feature

### Within Each Phase

- Phase 1 tasks marked [P] can run in parallel
- Phase 3 tasks marked [P] can run in parallel

### Parallel Opportunities

- T001-T004 can be developed in parallel (different functions)
- T010-T011 can be developed in parallel (different edge cases)
