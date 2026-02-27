# Tasks: Task Execution Sandbox

**Input**: Design documents from `/specs/006-prompt-sandbox/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: No explicit test-first requirement in spec. Unit tests included where valuable.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Wire up the `prompt` command group in the CLI

- [x] T001 Add `promptCmd` function and `case "prompt"` dispatch in `cmd/anvil/main.go` that routes to `promptSandboxCmd` for the `sandbox` subcommand
- [x] T002 Add `prompt sandbox` to the help text in `usageCmd` in `cmd/anvil/main.go`

---

## Phase 2: Foundational

**Purpose**: Core sandbox execution logic that all user stories depend on

- [x] T003 Define `SandboxResult` struct in `cmd/anvil/main.go` with fields: Label, Response, InputTokens, OutputTokens, EstimatedCost, Duration, Error, RunnerIndex
- [x] T004 Implement `runSandbox` helper function in `cmd/anvil/main.go` that: loads the project and task by name, creates a temp log directory, calls `runner.Run()` with the task content (no hooks, no run record), parses token usage from stderr via `runner.ParseTokenUsage()`, calculates cost using config rates, cleans up temp files, returns a `SandboxResult`
- [x] T005 Implement `printSandboxResult` helper in `cmd/anvil/main.go` for human-readable text output (response, tokens, cost, duration) matching the contract format
- [x] T006 Implement `printSandboxResultJSON` helper in `cmd/anvil/main.go` for JSON output matching the contract format
- [x] T007 Write unit test `TestSandboxResultJSON` in `cmd/anvil/main_test.go` (or inline test) to verify JSON output format

**Checkpoint**: Foundation ready — `runSandbox` can execute a task prompt and return structured results

---

## Phase 3: User Story 1 - Run Prompt in Sandbox (Priority: P1) 🎯 MVP

**Goal**: Users can run `anvil prompt sandbox <task>` to execute a prompt with zero side effects and see response + stats

**Independent Test**: Run `anvil prompt sandbox <task>` on a real task, verify LLM responds, no run record written, no hooks fired

### Implementation for User Story 1

- [x] T008 [US1] Implement `promptSandboxCmd` in `cmd/anvil/main.go` that parses flags (`--json`, `--compare`, `--watch`), validates task name argument, calls `runSandbox`, and outputs results via `printSandboxResult` or `printSandboxResultJSON`
- [x] T009 [US1] Add error handling in `promptSandboxCmd` for: task not found, empty content, runner failure, project not initialized
- [x] T010 [US1] Verify sandbox does not write run records — `runSandbox` must not call `project.WriteRunRecord` and must use a temp directory for logs that gets cleaned up

**Checkpoint**: `anvil prompt sandbox my-task` works end-to-end with text and JSON output, zero side effects

---

## Phase 4: User Story 2 - Compare Prompt Variations (Priority: P2)

**Goal**: Users can run `anvil prompt sandbox <task> --compare v1.md v2.md` to test multiple prompts and see results for each

**Independent Test**: Create two variation files, run with `--compare`, verify both execute and results show for each

### Implementation for User Story 2

- [x] T011 [US2] Add `--compare` flag parsing in `promptSandboxCmd` in `cmd/anvil/main.go` that collects file paths after the flag
- [x] T012 [US2] Implement variation loading: read each compare file, validate it exists, extract its content as replacement prompt
- [x] T013 [US2] Implement comparison execution loop in `promptSandboxCmd`: run default task content first, then each variation file's content, collecting SandboxResult for each
- [x] T014 [US2] Implement `printComparisonSummary` in `cmd/anvil/main.go` that displays a summary table (variation, tokens in, tokens out, cost, duration) after all individual results
- [x] T015 [US2] Support `--json` with `--compare`: output a JSON array of SandboxResult objects

**Checkpoint**: `anvil prompt sandbox my-task --compare v1.md v2.md` runs all variations and shows comparison summary

---

## Phase 5: User Story 3 - Watch Mode (Priority: P3)

**Goal**: Users can run `anvil prompt sandbox <task> --watch` for automatic re-execution on file changes

**Independent Test**: Start watch mode, edit the task file, verify sandbox re-runs automatically

### Implementation for User Story 3

- [x] T016 [US3] Add `--watch` flag handling in `promptSandboxCmd` that enters a watch loop instead of single execution
- [x] T017 [US3] Implement `watchAndRun` function in `cmd/anvil/main.go` that: polls the task file's mtime every 1 second, detects changes, debounces (500ms minimum gap), re-loads the task, calls `runSandbox`, prints results, handles Ctrl+C for clean exit via signal handling
- [x] T018 [US3] Handle watch mode errors gracefully: if frontmatter parse fails after edit, print error and continue watching; if runner fails, print error and continue watching

**Checkpoint**: `anvil prompt sandbox my-task --watch` detects file changes and re-runs automatically

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final build verification and cleanup

- [x] T019 Verify `go build ./...` passes with all changes
- [x] T020 Verify `go test ./...` passes with all changes
- [x] T021 Run `anvil prompt sandbox` with `--help` equivalent (no args) and verify usage text is clear

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (needs `promptSandboxCmd` stub to exist)
- **US1 (Phase 3)**: Depends on Phase 2 (`runSandbox` and output helpers)
- **US2 (Phase 4)**: Depends on Phase 3 (extends `promptSandboxCmd` with `--compare`)
- **US3 (Phase 5)**: Depends on Phase 3 (extends `promptSandboxCmd` with `--watch`)
- **Polish (Phase 6)**: Depends on all desired user stories

### User Story Dependencies

- **US1 (P1)**: Can start after Foundational — no dependencies on other stories
- **US2 (P2)**: Depends on US1 being complete (extends the same command function)
- **US3 (P3)**: Depends on US1 being complete (extends the same command function); independent of US2

### Parallel Opportunities

- T005 and T006 can run in parallel (text vs JSON output helpers)
- T011 and T016 are independent flag additions (but share same function, so sequential is safer)
- US2 and US3 are independent of each other (but both modify `promptSandboxCmd`)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T002)
2. Complete Phase 2: Foundational (T003-T007)
3. Complete Phase 3: User Story 1 (T008-T010)
4. **STOP and VALIDATE**: `anvil prompt sandbox my-task` works, no side effects
5. Can ship as-is with core value delivered

### Incremental Delivery

1. Setup + Foundational → Core sandbox infrastructure
2. US1 → Basic sandbox works → MVP!
3. US2 → Comparison mode added
4. US3 → Watch mode added
5. Polish → Build verified, help text clean

---

## Notes

- All new code goes in `cmd/anvil/main.go` following existing command patterns
- `runSandbox` calls `runner.Run()` directly — does NOT go through daemon
- Temp log directory must be cleaned up after each sandbox run
- Cost calculation reuses the same rates as daemon (`config.InputTokenRate`, `OutputTokenRate`)
- Session ID for sandbox runs uses `sandbox-` prefix to distinguish if any trace remains
