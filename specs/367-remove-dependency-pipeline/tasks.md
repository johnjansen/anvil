# Tasks: Remove Task Dependency Pipeline

**Input**: Design documents from `/specs/367-remove-dependency-pipeline/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Not requested — no test tasks included.

**Organization**: Tasks are grouped by user story. Since this is a removal feature, all stories are tightly coupled (removing code from shared files), so sequential execution is recommended.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No setup needed — this is a removal-only feature on an existing codebase.

*(No tasks)*

---

## Phase 2: Foundational (Delete standalone dependency files)

**Purpose**: Remove files that are entirely dependency-specific. This unblocks the modification tasks by eliminating the source of dependency types.

- [ ] T001 [P] Delete dependency types, parsing, resolution, validation, and cycle detection in `internal/project/dependencies.go`
- [ ] T002 [P] Delete all dependency tests in `internal/project/dependencies_test.go`
- [ ] T003 [P] Delete pipeline CLI command and visualization in `cmd/anvil/task_pipeline.go`

**Checkpoint**: Standalone dependency files deleted. Code will not compile yet — references in other files still exist.

---

## Phase 3: User Story 1 - Clean removal of depends_on frontmatter (Priority: P1) 🎯 MVP

**Goal**: Remove `DependsOn`, `DependencyPolicy`, and `DependencyPolicyConfig` from the `Todo` struct, frontmatter parsing, and task file generation so that `depends_on` fields in task files are silently ignored.

**Independent Test**: Create a task file with `depends_on: foo` in frontmatter, load it with the daemon, verify it loads without errors and runs on its own cron schedule.

### Implementation for User Story 1

- [ ] T004 [US1] Remove `DependencyPolicyConfig` type, `DependsOn []string` field, and `DependencyPolicy` field from the `Todo` struct in `internal/project/project.go`
- [ ] T005 [US1] Remove `DependsOn` and `DependencyPolicy` fields from the YAML frontmatter parsing struct in `internal/project/project.go`
- [ ] T006 [US1] Remove `depends_on` and `dependency_policy` from task file generation/writing logic in `internal/project/project.go`
- [ ] T007 [US1] Remove `depFailInfo` type and `checkDependenciesMet()` function from `internal/daemon/daemon.go`
- [ ] T008 [US1] Remove dependency collection (`CollectDependencyResults` call) from task execution in `internal/daemon/daemon.go`
- [ ] T009 [US1] Remove cross-project dependency validation and cycle detection from daemon startup in `internal/daemon/daemon.go`
- [ ] T010 [US1] Remove dependency checking from task dispatch logic in `internal/daemon/daemon.go`
- [ ] T011 [US1] Remove `--depends-on` CLI flag and dependency validation from `cmd/anvil/task_create.go`

**Checkpoint**: Core dependency logic fully removed. `go build ./...` should succeed. Tasks with stale `depends_on` fields load without errors.

---

## Phase 4: User Story 2 - Removal of pipeline CLI command (Priority: P1)

**Goal**: Remove the `pipeline` subcommand from CLI routing and help text.

**Independent Test**: Run `anvil task pipeline` and confirm unknown command error. Run `anvil help` and confirm no pipeline references.

### Implementation for User Story 2

- [ ] T012 [US2] Remove `pipeline` subcommand routing from `cmd/anvil/task_router.go`
- [ ] T013 [US2] Remove `pipeline` from help text in `cmd/anvil/main.go`

**Checkpoint**: Pipeline command fully removed from CLI.

---

## Phase 5: User Story 3 - Removal of dependency display from CLI output (Priority: P1)

**Goal**: Remove dependency-related display from task list, dry-run, and results commands.

**Independent Test**: Run `anvil task list --json`, `anvil task dry-run`, and `anvil task results` — verify no dependency fields or sections appear.

### Implementation for User Story 3

- [ ] T014 [P] [US3] Remove dependency results display from `cmd/anvil/task_results.go`
- [ ] T015 [P] [US3] Remove dependency display from JSON and text output in `cmd/anvil/task_list.go`
- [ ] T016 [P] [US3] Remove dependency section from dry-run output in `cmd/anvil/dryrun.go`

**Checkpoint**: All CLI output is free of dependency references.

---

## Phase 6: User Story 4 - Documentation cleanup (Priority: P2)

**Goal**: Remove all documentation references to `depends_on`, pipelines, and cross-project dependencies.

**Independent Test**: `grep -r "depends_on\|pipeline\|ParseDependency\|cross-project" tools/skills/ CLAUDE.md` returns no results.

### Implementation for User Story 4

- [ ] T017 [P] [US4] Remove `--depends-on` flag docs and `pipeline` command docs from `tools/skills/anvil/SKILL.md`
- [ ] T018 [P] [US4] Remove 263-cross-project-pipeline and 265-cross-project-queue technology references from `CLAUDE.md`

**Checkpoint**: Documentation fully cleaned.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final verification and cleanup.

- [ ] T019 Verify `go build ./...` succeeds with no compilation errors
- [ ] T020 Verify `go test ./...` passes with no test failures
- [ ] T021 Run quickstart.md validation steps to confirm all removal criteria met

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies — delete standalone files first
- **User Story 1 (Phase 3)**: Depends on Phase 2 (needs dependency types deleted first to avoid confusion, though not strictly required)
- **User Story 2 (Phase 4)**: Depends on Phase 2 (T003 deletes the pipeline file)
- **User Story 3 (Phase 5)**: Independent — modifies different files. Can run in parallel with US1/US2
- **User Story 4 (Phase 6)**: Independent — modifies documentation only. Can run in parallel with all stories
- **Polish (Phase 7)**: Depends on all phases complete

### Parallel Opportunities

- T001, T002, T003 can all run in parallel (different files)
- T014, T015, T016 can all run in parallel (different files)
- T017, T018 can run in parallel (different files)
- US3 and US4 can run in parallel with US1/US2

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Delete standalone files (T001-T003)
2. Complete Phase 3: Remove core dependency logic (T004-T011)
3. **STOP and VALIDATE**: `go build ./...` should succeed
4. Continue with remaining stories

### Incremental Delivery

1. Delete standalone files → Foundation ready
2. Remove core dependency logic → MVP (tasks work without dependencies)
3. Remove pipeline CLI → Clean CLI surface
4. Remove display references → Clean output
5. Clean documentation → Complete removal

---

## Notes

- This is a pure removal — no new code is written
- The main risk is missing a reference that causes a compilation error — `go build` after each phase catches this
- T007-T010 all modify `internal/daemon/daemon.go` — must be done sequentially within that file
- T004-T006 all modify `internal/project/project.go` — must be done sequentially within that file
- YAML frontmatter parsing silently ignores unknown fields, so stale `depends_on` in task files is safe
