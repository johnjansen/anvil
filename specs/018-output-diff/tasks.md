# Tasks: Task Output Diffing

**Input**: Design documents from `/specs/018-output-diff/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/cli.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Create the new output diff file with types and scaffolding

- [x] T001 Create cmd/anvil/outputdiff.go with DiffLine, DiffHunk, RunMeta, and OutputDiffResult structs per data-model.md
- [x] T002 Add findRunByPrefix helper function in cmd/anvil/outputdiff.go that searches a []RunRecord slice for a run whose RunID starts with a given prefix, returning error if no match or ambiguous

---

## Phase 2: Foundational (Routing)

**Purpose**: Wire up the diff command routing so output diff mode is accessible

- [x] T003 Modify taskDiffCmd in cmd/anvil/main.go to detect output-diff mode: when called with 1 arg (task name only) or when --run1/--run2 flags are present, call taskOutputDiffCmd(args) instead of the existing version-diff logic. Preserve existing version-diff behavior for 2+ non-flag args.

**Checkpoint**: `anvil task diff my-task` now routes to the new output diff handler

---

## Phase 3: User Story 1 - Compare Last Two Runs (Priority: P1) MVP

**Goal**: Users can run `anvil task diff <task>` to see unified diff of last two run outputs

**Independent Test**: Run a task twice with different outputs, then `anvil task diff <task>` shows the unified diff

### Implementation for User Story 1

- [x] T004 [US1] Implement taskOutputDiffCmd function in cmd/anvil/outputdiff.go: parse task name from args, call findTodo to resolve task, call project.ReadAllRunRecords to load runs, validate at least 2 runs exist, extract OutputSummary from the two most recent runs
- [x] T005 [US1] Implement printOutputDiff function in cmd/anvil/outputdiff.go: generate unified diff headers showing "--- Run <id> (<timestamp>) <STATUS>" and "+++ Run <id> (<timestamp>) <STATUS>", call project.UnifiedDiff with the two OutputSummary strings, print the result or "Outputs are identical" message
- [x] T006 [US1] Add error handling in taskOutputDiffCmd for edge cases: task not found, no runs, only one run, empty OutputSummary (treat as empty string)

**Checkpoint**: `anvil task diff my-task` compares last two runs with unified diff output and proper error messages

---

## Phase 4: User Story 2 - Compare Specific Runs (Priority: P2)

**Goal**: Users can specify --run1 and --run2 to compare any two specific runs

**Independent Test**: Create multiple runs, use --run1 and --run2 with partial IDs to compare specific runs

### Implementation for User Story 2

- [x] T007 [US2] Add --run1, --run2, --context, and --ignore-whitespace flag parsing to taskOutputDiffCmd in cmd/anvil/outputdiff.go
- [x] T008 [US2] Implement run selection logic in taskOutputDiffCmd: when --run1 and --run2 are provided, use findRunByPrefix to locate specific runs; when only one is provided, use it as run1 and default run2 to the most recent run; error on invalid/ambiguous prefixes
- [x] T009 [US2] Implement --context N support: pass context line count to a modified diff call. Implement --ignore-whitespace: pre-process OutputSummary lines by trimming whitespace before passing to UnifiedDiff

**Checkpoint**: `anvil task diff my-task --run1 abc --run2 def` works with prefix matching, context lines, and whitespace ignoring

---

## Phase 5: User Story 3 - Programmatic JSON Output (Priority: P3)

**Goal**: Users can get structured JSON output for scripting and automation

**Independent Test**: Run `anvil task diff my-task --json` and verify output is valid JSON with expected fields

### Implementation for User Story 3

- [x] T010 [US3] Add --json flag parsing to taskOutputDiffCmd in cmd/anvil/outputdiff.go
- [x] T011 [US3] Implement parseDiffToHunks function in cmd/anvil/outputdiff.go: parse the unified diff string output from project.UnifiedDiff into []DiffHunk structs by splitting on @@ hunk headers and categorizing each line as context/added/removed
- [x] T012 [US3] Implement printOutputDiffJSON function in cmd/anvil/outputdiff.go: build OutputDiffResult with RunMeta for both runs, Identical flag, and parsed hunks, then marshal to JSON and print to stdout

**Checkpoint**: `anvil task diff my-task --json` outputs valid JSON with run metadata and diff hunks

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Help text and final integration

- [x] T013 Add diff output help text to the task help section in cmd/anvil/main.go showing usage for output diff mode alongside existing version diff
- [x] T014 Verify go build ./cmd/anvil/ compiles cleanly and go vet ./... passes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies - create new file with types
- **Phase 2 (Foundational)**: Depends on Phase 1 - routing change references new function
- **Phase 3 (US1)**: Depends on Phase 2 - implements the routed function
- **Phase 4 (US2)**: Depends on Phase 3 - extends the existing function with flags
- **Phase 5 (US3)**: Depends on Phase 3 - adds JSON output path
- **Phase 6 (Polish)**: Depends on all user stories

### User Story Dependencies

- **US1 (P1)**: Foundation only - no story dependencies
- **US2 (P2)**: Extends US1's function with flag handling
- **US3 (P3)**: Can start after US1 (needs the base function), parallel with US2

### Parallel Opportunities

- T001 and T002 can run in parallel (same file but independent sections)
- US2 (T007-T009) and US3 (T010-T012) can run in parallel after US1 completes

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Create outputdiff.go with types
2. Complete Phase 2: Wire routing in main.go
3. Complete Phase 3: Implement last-two-runs comparison
4. **STOP and VALIDATE**: `anvil task diff <task>` works with unified diff output

### Full Delivery

5. Phase 4: Add --run1/--run2, --context, --ignore-whitespace
6. Phase 5: Add --json output
7. Phase 6: Help text, build verification

---

## Notes

- Reuses existing `project.UnifiedDiff()` from internal/project/diff.go
- Reuses existing `project.ReadAllRunRecords()` from internal/project/project.go
- Reuses existing `findTodo()` pattern from other task subcommands
- All new code goes in cmd/anvil/outputdiff.go (single new file)
- Only main.go modification is the routing change in taskDiffCmd
