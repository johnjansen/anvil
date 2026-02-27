# Tasks: Fix Raw Mode Line Break Output

**Input**: Design documents from `/specs/001-fix-raw-mode-linebreaks/`
**Prerequisites**: plan.md, spec.md, research.md, quickstart.md

**Tests**: Not explicitly requested. Existing tests must continue to pass.

**Organization**: Tasks grouped by user story. Both stories are P1 but US1 (the fix) must complete before US2 (regression verification).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No setup needed — existing project, existing build system. Skip.

---

## Phase 2: Foundational

**Purpose**: No foundational work needed — all infrastructure exists. Skip.

---

## Phase 3: User Story 1 - Daemon foreground output displays correctly (Priority: P1) MVP

**Goal**: Fix `dlog.println()` to produce `\r\n` when raw terminal mode is active, so all daemon log lines start at column 0.

**Independent Test**: Run `anvil watch` in foreground with a recurring task. Every log line begins at column 0 with no horizontal offset.

### Implementation for User Story 1

- [x] T001 [US1] Add `rawMode bool` field to `daemonLogger` struct and add exported `SetRawMode(enabled bool)` method in `internal/daemon/logger.go`
- [x] T002 [US1] Modify `println()` method to write `\r\n` line ending when `rawMode` is true in `internal/daemon/logger.go`
- [x] T003 [US1] Call `daemon.SetRawMode(true)` after `term.MakeRaw()` and `daemon.SetRawMode(false)` in the restore defer in `cmd/anvil/main.go`

**Checkpoint**: Foreground daemon output should now display correctly with each line starting at column 0.

---

## Phase 4: User Story 2 - Non-raw-mode output remains unaffected (Priority: P1)

**Goal**: Verify that background/daemonized mode output is unaffected by the change.

**Independent Test**: Run existing tests; verify daemon log file contains standard `\n` (no spurious `\r`).

### Implementation for User Story 2

- [x] T004 [US2] Run `go test ./internal/daemon/...` to verify existing logger tests pass with no modifications in `internal/daemon/`
- [x] T005 [US2] Run `go test ./...` to verify full test suite passes

**Checkpoint**: All existing tests pass. No `\r` characters injected in non-raw-mode output.

---

## Phase 5: Polish & Cross-Cutting Concerns

- [x] T006 Run quickstart.md validation checklist

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 3 (US1)**: No dependencies — can start immediately
- **Phase 4 (US2)**: Depends on Phase 3 (T001-T003 must be complete before testing)
- **Phase 5 (Polish)**: Depends on Phase 4

### Within User Story 1

- T001 → T002 (field must exist before println uses it)
- T002 → T003 (method must work before caller invokes it)

### Parallel Opportunities

- T001 and T003 touch different files and could theoretically be parallel, but T003 depends on the exported `SetRawMode` function created in T001

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete T001-T003 (the actual fix)
2. **STOP and VALIDATE**: Run tests (T004-T005)
3. Verify with manual foreground test

### Summary

| Metric                | Value |
|-----------------------|-------|
| Total tasks           | 6     |
| US1 tasks             | 3     |
| US2 tasks             | 2     |
| Polish tasks          | 1     |
| Parallel opportunities| 0 (sequential chain) |
| Files modified        | 2     |

---

## Notes

- This is a minimal 2-file bug fix — tasks are intentionally few
- The core change is ~10 lines of Go code across 2 files
- No new files created, no architectural changes
- Commit after T003 (the complete fix) as a single logical change
