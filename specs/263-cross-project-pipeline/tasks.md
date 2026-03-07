# Tasks: Cross-Project Pipeline Visualization

**Input**: Design documents from `/specs/263-cross-project-pipeline/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/cli.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No new files needed. This feature modifies existing code in `cmd/anvil/task_pipeline.go`.

- [ ] T001 Read and understand existing pipeline code in cmd/anvil/task_pipeline.go and cross-project dependency infrastructure in internal/project/dependencies.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Extend the pipeline graph builder to support cross-project awareness. MUST be complete before any user story rendering changes.

- [ ] T002 Add `projectName` field to `pipelineTaskInfo` struct in cmd/anvil/task_pipeline.go and populate it from each project's directory basename when loading tasks in `buildPipelineGraph`
- [ ] T003 Modify `buildPipelineGraph` in cmd/anvil/task_pipeline.go to use `project.ParseDependency` when processing `dependsOn` entries, resolving cross-project references to qualified `projectName:taskName` keys in the tasks map when multiple projects are loaded
- [ ] T004 Update dependency validation and children map building in `buildPipelineGraph` in cmd/anvil/task_pipeline.go to handle qualified keys for cross-project deps while keeping plain keys for single-project mode (backward compatibility)

**Checkpoint**: Graph builder now distinguishes local vs cross-project dependencies. Single-project output unchanged.

---

## Phase 3: User Story 1 - View Cross-Project Dependencies in Pipeline (Priority: P1) + User Story 2 - Visual Distinction (Priority: P1)

**Goal**: `anvil task pipeline --all` shows cross-project deps with project headers and visual distinction between local and cross-project edges.

**Independent Test**: Create two projects with cross-project deps, run `anvil task pipeline --all`, verify project headers appear and cross-project deps show with `[project]` prefix.

### Implementation

- [ ] T005 [US1] Update `pipelineASCII` in cmd/anvil/task_pipeline.go to group output by project with `=== project-name ===` headers when `--all` is active and multiple projects are loaded
- [ ] T006 [US2] Update `printTree` within `pipelineASCII` in cmd/anvil/task_pipeline.go to render cross-project dependency nodes with `[source-project] task-name` bracket prefix by checking if a dependency's `projectName` differs from the current tree's project
- [ ] T007 [US1] Ensure single-project mode (no `--all`) produces identical output to current behavior in cmd/anvil/task_pipeline.go by only activating project headers and qualified keys when `len(projects) > 1`
- [ ] T008 [US1] Add warning output to stderr for unresolvable cross-project dependencies (watched project not found or task not found) in cmd/anvil/task_pipeline.go

**Checkpoint**: ASCII output with `--all` shows project-grouped pipelines with cross-project edges visually distinguished.

---

## Phase 4: User Story 3 - Cross-Project Deps in DOT Output (Priority: P2)

**Goal**: `anvil task pipeline --dot --all` generates DOT output with subgraph clusters per project and dashed edges for cross-project dependencies.

**Independent Test**: Generate DOT output with `--dot --all` on multi-project setup, verify `subgraph cluster_` blocks and `[style=dashed]` on cross-project edges.

### Implementation

- [ ] T009 [US3] Update `pipelineDOT` in cmd/anvil/task_pipeline.go to wrap tasks in `subgraph cluster_<project>` blocks with `label="<project>"` and `style=rounded` when `--all` is active
- [ ] T010 [US3] Update edge rendering in `pipelineDOT` in cmd/anvil/task_pipeline.go to use qualified node IDs (`project:task`) with display labels showing only task name, and add `[style=dashed]` attribute to cross-project edges
- [ ] T011 [US3] Ensure single-project DOT output (no `--all`) remains identical to current behavior in cmd/anvil/task_pipeline.go

**Checkpoint**: DOT output with `--all` produces valid GraphViz with clustered subgraphs and dashed cross-project edges.

---

## Phase 5: User Story 4 - Verbose Mode with Cross-Project Context (Priority: P3)

**Goal**: `anvil task pipeline --all --verbose` shows schedule and last-run status alongside project labels for cross-project tasks.

**Independent Test**: Run `--all --verbose` with cross-project deps that have run history, verify project label + schedule + status displayed.

### Implementation

- [ ] T012 [US4] Update `formatVerboseLabel` in cmd/anvil/task_pipeline.go to include project name prefix for cross-project tasks, using the task's `projectName` and `projPath` to resolve run records from the correct project directory

**Checkpoint**: Verbose mode works correctly with cross-project context.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T013 Handle edge case of same-named tasks in different projects: ensure disambiguation via project prefix in all output modes in cmd/anvil/task_pipeline.go
- [ ] T014 Run `go build ./...` and `go vet ./...` to verify compilation
- [ ] T015 Run quickstart.md validation: set up test projects per quickstart.md and verify expected output matches

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - read-only orientation
- **Foundational (Phase 2)**: Depends on Phase 1 - BLOCKS all user stories
- **US1+US2 (Phase 3)**: Depends on Phase 2 - core ASCII rendering
- **US3 (Phase 4)**: Depends on Phase 2 - can run in parallel with Phase 3
- **US4 (Phase 5)**: Depends on Phase 3 (extends verbose label formatting)
- **Polish (Phase 6)**: Depends on all user story phases

### User Story Dependencies

- **US1 + US2 (P1)**: Combined because both modify `pipelineASCII` - depends only on foundational phase
- **US3 (P2)**: Independent from US1/US2 - modifies `pipelineDOT` only - can run in parallel with Phase 3
- **US4 (P3)**: Depends on US1/US2 (extends `formatVerboseLabel` with project context)

### Parallel Opportunities

- T002 and T003 touch the same function so must be sequential
- Phase 3 (US1+US2) and Phase 4 (US3) can run in parallel after foundational phase
- T009 and T010 modify the same function so must be sequential

---

## Parallel Example: After Foundational Phase

```bash
# These can run in parallel (different functions):
Task: T005-T008 (pipelineASCII changes for US1+US2)
Task: T009-T011 (pipelineDOT changes for US3)
```

---

## Implementation Strategy

### MVP First (User Stories 1+2 Only)

1. Complete Phase 1: Setup (read code)
2. Complete Phase 2: Foundational (extend graph builder)
3. Complete Phase 3: US1+US2 (ASCII cross-project rendering)
4. **STOP and VALIDATE**: Test with two watched projects
5. Ship if ready

### Incremental Delivery

1. Setup + Foundational -> Graph builder ready
2. Add US1+US2 -> ASCII output with cross-project awareness (MVP)
3. Add US3 -> DOT output with subgraph clusters
4. Add US4 -> Verbose mode with project context
5. Polish -> Edge cases, validation

---

## Notes

- All changes are in a single file: `cmd/anvil/task_pipeline.go`
- No new files needed - this extends existing rendering functions
- `internal/project/dependencies.go` already provides `ParseDependency` - no changes needed there
- Backward compatibility is critical: single-project mode must produce identical output
- 15 total tasks, manageable scope for a single implementation pass
