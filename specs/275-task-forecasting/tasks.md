# Tasks: Task Forecasting

**Input**: Design documents from `/specs/275-task-forecasting/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Not explicitly requested in the feature specification. Test tasks are omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the new forecast package and wire it into the CLI

- [ ] T001 Create `internal/forecast/` package directory and `internal/forecast/forecast.go` with package declaration and ForecastRun, ForecastSummary, TaskStats structs from data-model.md
- [ ] T002 Create `internal/forecast/contention.go` with package declaration and ContentionWindow struct from data-model.md
- [ ] T003 Add `"forecast"` case to task subcommand router in `cmd/anvil/task_router.go` calling a stub `taskForecastCmd(args[1:])`
- [ ] T004 Create `cmd/anvil/task_forecast.go` with stub `taskForecastCmd` function, flag parsing for `--days`, `--task`, `--contention`, `--cost`, `--all`, `--json`, `--verbose`

**Checkpoint**: Package structure exists, CLI command is wired up (prints usage/stub output)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core forecast engine that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 Implement `ComputeTaskStats(projectPath string, taskID string, maxSamples int) (TaskStats, error)` in `internal/forecast/cost.go` — reads RunRecords via `project.ReadAllRunRecords()`, averages duration/cost/tokens from up to last N successful runs
- [ ] T006 Implement `ProjectRuns(todos []project.Todo, stats map[string]TaskStats, cfg *config.Config, start time.Time, end time.Time) (ForecastSummary, error)` in `internal/forecast/forecast.go` — iterates each todo's cron schedule using `cron.Parse().Next()` combined with `daemon.NextAllowedRun()` to generate ForecastRun entries, skips manual-only tasks (empty schedule), populates estimates from TaskStats, returns sorted ForecastSummary with totals

**Checkpoint**: Foundation ready — forecast engine can project runs and compute stats

---

## Phase 3: User Story 1 — View Upcoming Scheduled Runs (Priority: P1) MVP

**Goal**: Users can run `anvil task forecast` and see a chronological list of upcoming task executions with names, times, and estimated durations.

**Independent Test**: Configure tasks with known cron schedules, run `anvil task forecast`, verify output matches expected execution times.

### Implementation for User Story 1

- [ ] T007 [US1] Implement human-readable table output in `cmd/anvil/task_forecast.go` — format ForecastSummary.Runs as `TIME / TASK / DURATION` table with summary line showing total runs and total estimated runtime
- [ ] T008 [US1] Implement JSON output in `cmd/anvil/task_forecast.go` — marshal ForecastSummary to JSON when `--json` flag is set, using field names from cli-contract.md
- [ ] T009 [US1] Implement `--task` filter in `cmd/anvil/task_forecast.go` — filter todos list before passing to ProjectRuns
- [ ] T010 [US1] Implement `--days` flag in `cmd/anvil/task_forecast.go` — compute end time as `start + days*24h`, validate days > 0
- [ ] T011 [US1] Implement `--all` flag in `cmd/anvil/task_forecast.go` — load all watched projects via config when set, otherwise use current project only
- [ ] T012 [US1] Implement output summarization in `internal/forecast/forecast.go` — when a task has >50 runs in the forecast, group by day showing runs/day, daily duration, daily cost instead of individual lines (respect `--verbose` to show all)
- [ ] T013 [US1] Implement empty-state messages in `cmd/anvil/task_forecast.go` — "No scheduled tasks found" when no todos, "No runs in forecast period" when schedule doesn't match horizon
- [ ] T014 [US1] Handle invalid cron expressions gracefully in `internal/forecast/forecast.go` — log warning `"skipping task X: invalid schedule Y"` and continue with remaining tasks

**Checkpoint**: `anvil task forecast` fully functional with table/JSON output, filtering, summarization, and error handling

---

## Phase 4: User Story 2 — Predict Resource Contention (Priority: P2)

**Goal**: Users can run `anvil task forecast --contention` and see time windows where concurrent tasks exceed worker pool capacity.

**Independent Test**: Configure overlapping tasks with worker pool smaller than overlap count, verify bottleneck windows are identified.

### Implementation for User Story 2

- [ ] T015 [US2] Implement `DetectContention(summary ForecastSummary, workerCount int) []ContentionWindow` in `internal/forecast/contention.go` — build sorted interval list from ForecastRun (start + estimated duration), sweep-line to find windows where concurrent count > workerCount, populate ContentionWindow with peak count and task names
- [ ] T016 [US2] Implement contention human-readable output in `cmd/anvil/task_forecast.go` — when `--contention` flag is set, display contention windows as `TIME / CONCURRENT / WORKERS / OVERFLOW / TASKS` table, show "No contention detected" message when none found
- [ ] T017 [US2] Add contention_windows array to JSON output in `cmd/anvil/task_forecast.go` — include ContentionWindow fields in JSON when `--contention` and `--json` are both set

**Checkpoint**: `anvil task forecast --contention` identifies and displays resource bottlenecks

---

## Phase 5: User Story 3 — Project Cost Estimates (Priority: P3)

**Goal**: Users can run `anvil task forecast --cost` and see estimated token usage and costs based on historical run averages.

**Independent Test**: Run tasks that generate known token usage, verify forecast cost projection matches expected values.

### Implementation for User Story 3

- [ ] T018 [US3] Add cost columns to human-readable output in `cmd/anvil/task_forecast.go` — when `--cost` flag is set, add COST column to the forecast table, show "no data" for tasks without historical data, include cost total in summary line
- [ ] T019 [US3] Add token and cost fields to JSON output in `cmd/anvil/task_forecast.go` — include `input_tokens`, `output_tokens`, `estimated_cost_usd`, `total_cost_usd`, `total_input_tokens`, `total_output_tokens` fields when `--cost` and `--json` are both set
- [ ] T020 [US3] Add `has_historical_data` field to ForecastRun in `internal/forecast/forecast.go` — set to false when TaskStats has zero RunCount so CLI can display "no data"

**Checkpoint**: `anvil task forecast --cost` shows per-task and total cost projections

---

## Phase 6: User Story 4 — What-If Analysis (Priority: P4)

**Goal**: Users can run `anvil add --dry-run` to see forecast impact of a new task without persisting it.

**Independent Test**: Run `anvil add --dry-run` with a schedule that creates contention, verify forecast shows impact without modifying task list.

### Implementation for User Story 4

- [ ] T021 [US4] Add `--dry-run` flag to `cmd/anvil/add.go` — when set, construct a Todo from provided args (schedule, name) with `IsHypothetical: true` marker, skip file write
- [ ] T022 [US4] Wire dry-run to forecast engine in `cmd/anvil/add.go` — append hypothetical Todo to loaded todos list, call `forecast.ProjectRuns()` and `forecast.DetectContention()`, display combined forecast with impact comparison (before/after run counts, runtime, cost, new contention)
- [ ] T023 [US4] Mark hypothetical tasks in output in `cmd/anvil/task_forecast.go` — prefix with `*` in human output, set `"is_hypothetical": true` in JSON output

**Checkpoint**: `anvil add --dry-run` shows forecast impact without persisting any changes

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T024 Add `forecast` to CLI help text and usage output in `cmd/anvil/task_router.go`
- [ ] T025 Run quickstart.md validation — execute each command from quickstart.md against a test project and verify expected output format

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001, T002 for structs)
- **User Story 1 (Phase 3)**: Depends on Phase 2 (T005, T006 for forecast engine)
- **User Story 2 (Phase 4)**: Depends on Phase 2 (needs ForecastSummary) — can run in parallel with US1
- **User Story 3 (Phase 5)**: Depends on Phase 2 (needs TaskStats and ForecastSummary) — can run in parallel with US1/US2
- **User Story 4 (Phase 6)**: Depends on Phase 3 completion (needs forecast CLI output wired up) and Phase 4 (contention detection)
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: After Foundational — no other story dependencies
- **User Story 2 (P2)**: After Foundational — independent of US1 (uses forecast engine directly)
- **User Story 3 (P3)**: After Foundational — independent of US1/US2 (adds cost columns)
- **User Story 4 (P4)**: After US1 + US2 — needs both forecast display and contention detection

### Within Each User Story

- Models/structs before logic
- Core logic before CLI output
- Human output before JSON output

### Parallel Opportunities

- **Phase 1**: T001 and T002 can run in parallel (different files). T003 and T004 can run in parallel (different files).
- **Phase 2**: T005 and T006 are sequential (T006 depends on TaskStats from T005).
- **Phase 3**: T007 and T008 are sequential (JSON output reuses table logic). T009, T010, T011 can run in parallel (independent flag implementations). T012, T013, T014 can run in parallel (independent concerns).
- **Phase 4**: T015 before T016/T017. T016 and T017 can run in parallel.
- **Phase 5**: T018 and T019 can run in parallel. T020 before T018/T019.
- **Phase 6**: T021 before T022 before T023.

---

## Parallel Example: User Story 1

```bash
# After T007/T008 (core output), these can run in parallel:
Task: "T009 [US1] Implement --task filter in cmd/anvil/task_forecast.go"
Task: "T010 [US1] Implement --days flag in cmd/anvil/task_forecast.go"
Task: "T011 [US1] Implement --all flag in cmd/anvil/task_forecast.go"

# These can also run in parallel (independent concerns):
Task: "T012 [US1] Implement output summarization in internal/forecast/forecast.go"
Task: "T013 [US1] Implement empty-state messages in cmd/anvil/task_forecast.go"
Task: "T014 [US1] Handle invalid cron expressions in internal/forecast/forecast.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T006)
3. Complete Phase 3: User Story 1 (T007-T014)
4. **STOP and VALIDATE**: Run `anvil task forecast` against a project with tasks and verify chronological output
5. Deploy if ready — users get schedule visibility immediately

### Incremental Delivery

1. Complete Setup + Foundational → forecast engine ready
2. Add User Story 1 → Test `anvil task forecast` → Deploy (MVP!)
3. Add User Story 2 → Test `--contention` → Deploy
4. Add User Story 3 → Test `--cost` → Deploy
5. Add User Story 4 → Test `anvil add --dry-run` → Deploy
6. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- All new code goes in `internal/forecast/` (new package) and `cmd/anvil/` (CLI layer)
- No modifications needed to existing `internal/project`, `internal/config`, `internal/cron`, or `internal/daemon` packages
