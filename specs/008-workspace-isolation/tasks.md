# Tasks: Task Workspace Isolation

**Input**: Design documents from `/specs/008-workspace-isolation/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Not explicitly requested in the feature specification. Tests omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the new workspace package and define core types

- [x] T001 Create `internal/workspace/` package directory
- [x] T002 Define `WorkspaceConfig` struct and `WorkspaceType` constants in `internal/workspace/workspace.go`
- [x] T003 Add `Workspace WorkspaceConfig` field to `Todo` struct in `internal/project/project.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Workspace parsing, validation, and environment variable generation — needed by ALL user stories

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Add `workspace` YAML block to frontmatter parsing struct in `internal/project/project.go` (LoadTodos function, fmData struct around line 230)
- [x] T005 Implement `ValidateConfig(projectRoot string, cfg *WorkspaceConfig) error` in `internal/workspace/workspace.go` — resolve paths relative to project root, reject paths escaping root, resolve symlinks, validate type field, validate field applicability per type
- [x] T006 [P] Implement `EnvVars(projectRoot string, cfg WorkspaceConfig) map[string]string` in `internal/workspace/workspace.go` — generate ANVIL_WORKSPACE_TYPE, ANVIL_WORKSPACE_ROOT, ANVIL_WORKSPACE_ALLOWED, ANVIL_WORKSPACE_READONLY, ANVIL_WORKSPACE_BLOCKED environment variables per CLI contract
- [x] T007 [P] Implement `ParseSize(s string) (int64, error)` in `internal/workspace/workspace.go` — parse human-readable size strings like 100mb, 1gb to bytes
- [x] T008 Wire workspace validation into `LoadTodos()` in `internal/project/project.go` — after frontmatter parsing, call workspace.ValidateConfig(), log warnings on invalid config, set ParseError for fatal validation errors

**Checkpoint**: Foundation ready — workspace config is parsed, validated, and available on Todo structs

---

## Phase 3: User Story 1+4 — Restrict Task File Access + Default Project-Only (Priority: P1) MVP

**Goal**: Tasks with restricted workspace can only access specified directories. Tasks with no workspace config default to project-only access. Both P1 stories share the same enforcement mechanism.

**Independent Test**: Create a task with workspace.allowed_paths and blocked_paths, run it, verify environment variables are set correctly. Create a task with no workspace config and verify default ANVIL_WORKSPACE_TYPE=project is set.

### Implementation for User Story 1+4

- [x] T009 [US1] Inject workspace environment variables in daemon.runTask() in `internal/daemon/daemon.go` — after mergeEnv(), call workspace.EnvVars() and merge into extraEnv map before passing to runner.Run()
- [x] T010 [US1] Add default workspace config handling in daemon.runTask() in `internal/daemon/daemon.go` — when t.Workspace.Type is empty, treat as project type and still inject ANVIL_WORKSPACE_TYPE=project and ANVIL_WORKSPACE_ROOT=proj.Path
- [x] T011 [US1] Log warning when workspace restrictions are active in `internal/daemon/daemon.go` — add dlog.Info for restricted and temp types showing the workspace config summary

**Checkpoint**: Tasks with restricted workspace config get correct ANVIL_WORKSPACE_* env vars. Tasks with no config get default project type env vars.

---

## Phase 4: User Story 2 — Temporary Isolated Workspace (Priority: P2)

**Goal**: Tasks with workspace.type: temp execute in a fresh temporary directory that is cleaned up after completion.

**Independent Test**: Create a task with workspace.type: temp, run it, verify cmd.Dir is a temp directory and it is removed after completion.

### Implementation for User Story 2

- [x] T012 [P] [US2] Implement `CreateTempWorkspace(prefix string) (path string, cleanup func(), err error)` in `internal/workspace/workspace.go` — creates temp dir via os.MkdirTemp, returns cleanup function that removes it
- [x] T013 [P] [US2] Implement `CheckSize(dir string, maxBytes int64) (actualBytes int64, exceeded bool)` in `internal/workspace/workspace.go` — walks temp dir to calculate total size for post-execution advisory check
- [x] T014 [US2] Add temp workspace lifecycle to daemon.runTask() in `internal/daemon/daemon.go` — before runner.Run(), if workspace type is temp, create temp dir, use it as dir parameter instead of proj.Path, defer cleanup, post-execution size check if workspace size is set

**Checkpoint**: Tasks with temp workspace type run in isolated temp directories that are cleaned up after execution.

---

## Phase 5: User Story 3 — View Workspace Configuration (Priority: P2)

**Goal**: Users can see workspace configuration when running anvil task get.

**Independent Test**: Run anvil task get on a task with workspace config and verify output includes workspace details per CLI contract.

### Implementation for User Story 3

- [x] T015 [US3] Add workspace display to taskGetCmd() in `cmd/anvil/main.go` — after existing field display, show Workspace type, Allowed paths, Read-only paths, Blocked paths, and Size limit per CLI contract format

**Checkpoint**: All user stories should now be independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final integration and project defaults

- [x] T016 Add Workspace field to TaskDefaults struct in `internal/project/project.go` so workspace config can be set as project-level default in .anvil/config.yaml
- [x] T017 Wire workspace defaults into applyDefaults() in `internal/project/project.go` — if task has no workspace config, inherit from project defaults

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Story 1+4 (Phase 3)**: Depends on Foundational phase completion
- **User Story 2 (Phase 4)**: Depends on Foundational phase completion (can run parallel with Phase 3)
- **User Story 3 (Phase 5)**: Depends on Phase 1 (needs Workspace field on Todo struct), can run parallel with Phase 3/4
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1+4 (P1)**: Can start after Foundational — No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational — No dependencies on US1/US4
- **User Story 3 (P2)**: Can start after Setup — Only needs WorkspaceConfig struct defined

### Within Each User Story

- Models/structs before services/functions
- Validation before enforcement
- Core implementation before display/polish

### Parallel Opportunities

- T006 and T007 can run in parallel (different functions, same file)
- Phase 3, Phase 4, and Phase 5 can all run in parallel after Foundational
- T012 and T013 can run in parallel within Phase 4

---

## Implementation Strategy

### MVP First (User Stories 1+4 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T008)
3. Complete Phase 3: User Stories 1+4 (T009-T011)
4. **STOP and VALIDATE**: Verify env vars are injected for restricted and default workspace types
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational -> Foundation ready
2. Add US1+US4 -> Restricted + default workspace -> Deploy (MVP!)
3. Add US2 -> Temp workspace isolation -> Deploy
4. Add US3 -> Workspace visibility in task get -> Deploy
5. Add Polish -> Project defaults inheritance -> Deploy
6. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- runner.go needs NO changes — it already accepts dir and extraEnv parameters
- Total tasks: 17
