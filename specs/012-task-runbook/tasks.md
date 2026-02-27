# Tasks: Task Runbook Linking

**Input**: Design documents from `/specs/012-task-runbook/`
**Prerequisites**: plan.md, spec.md

## Phase 1: Core Data Model & Parsing

**Purpose**: Add runbook field to task configuration and ensure it parses correctly

- [ ] T001 [P] [US1] Add `Runbook string` field to `Todo` struct in `internal/project/project.go`
- [ ] T002 [P] [US1] Add `Runbook string` field to `TaskDefaults` struct in `internal/project/project.go` (for project-level defaults)

---

## Phase 2: CLI Integration - Core

**Purpose**: Add runbook display to existing task commands

- [ ] T003 [US1] Modify `taskGetCmd` in `cmd/anvil/main.go` to display runbook content when present
- [ ] T004 [P] [US1] Add runbook display helper function for rendering markdown in terminal

---

## Phase 3: CLI Integration - New Command

**Purpose**: Add dedicated runbook command

- [ ] T005 [US3] Add `runbook` subcommand case to task command switch in `cmd/anvil/main.go` (around line 1900)
- [ ] T006 [US3] Implement `taskRunbookCmd` function to display runbook content
- [ ] T007 [US3] Handle URL vs inline runbook - detect URL and display appropriately
- [ ] T008 [US3] Add `--open` flag to open URL runbooks in browser (using `open` command on macOS)

---

## Phase 4: Auto-Suggest on Failure

**Purpose**: Display runbook when task fails

- [ ] T009 [US4] Identify where task failure output is generated (likely in daemon or CLI result handling)
- [ ] T010 [US4] Modify failure output to include runbook information when defined

---

## Phase 5: Edge Cases & Polish

**Purpose**: Handle edge cases and improve user experience

- [ ] T011 [P] Handle task without runbook gracefully (show helpful message)
- [ ] T012 [P] Validate URL format for URL runbooks
- [ ] T013 Add documentation for runbook feature in docs/

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1**: No dependencies - can start immediately
- **Phase 2**: Depends on Phase 1 (need Runbook field to exist)
- **Phase 3**: Depends on Phase 1 (runbook field parsing)
- **Phase 4**: Depends on Phases 1-3 (need all infrastructure in place)
- **Phase 5**: Depends on earlier phases, can be done in parallel with Phase 4

### Parallel Opportunities

- T001 and T002 can run in parallel (different structs)
- T003 and T004 can run in parallel (same file but different sections)
- T011 and T012 can run in parallel (different edge cases)

### Within Each Phase

- Data model changes before CLI integration
- Inline runbook support before URL support
- Basic display before auto-suggest on failure

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- This feature is relatively straightforward - all user stories can be implemented in order
- No external dependencies needed - Go standard library sufficient for markdown rendering and URL opening
