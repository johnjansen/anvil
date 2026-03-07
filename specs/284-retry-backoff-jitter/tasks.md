# Tasks: Advanced Task Retry with Backoff Strategies and Jitter

**Input**: Design documents from `/specs/284-retry-backoff-jitter/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No new project setup needed — this feature extends existing code in `internal/project`, `internal/daemon`, and `cmd/anvil`.

*(No setup tasks — existing project structure is sufficient)*

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add new retry configuration fields to the data model and YAML parsing. These changes are required before any user story can be implemented.

**CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T001 Add RetryStrategy (string), RetryJitter (float64), and RetryMaxTime (time.Duration) fields to the Todo struct in internal/project/project.go
- [ ] T002 Add retry_strategy (string), retry_jitter (float64), and retry_max_time (string) fields to the YAML frontmatter parsing struct in internal/project/project.go (around line 479) and wire them to the Todo struct during parsing (around line 548)
- [ ] T003 Add RetryStrategy (string), RetryJitter (float64), and RetryMaxTime (string) fields to the TaskDefaults struct in internal/project/project.go (around line 43) and apply defaults in the defaults-merging logic (around line 730)
- [ ] T004 Add RetryStrategy (string) and RetryDelaysUsed ([]string) fields to the RunRecord struct in internal/project/project.go (around line 335)
- [ ] T005 Add RetryStrategy (string), RetryJitter (float64), and RetryMaxTime (string) fields to the TaskExportData struct in internal/project/project.go (around line 1508) for export/import compatibility
- [ ] T006 Add jitter clamping validation: when parsing retry_jitter from frontmatter, clamp to [0.0, 1.0] and log a warning if out of range, in internal/project/project.go
- [ ] T007 Add retry_strategy validation: when parsing retry_strategy from frontmatter, accept "exponential", "linear", "constant" (case-insensitive), default to "exponential" with warning for invalid values, in internal/project/project.go

**Checkpoint**: Data model ready — all new fields are parsed, validated, and stored. User story implementation can now begin.

---

## Phase 3: User Story 1 - Configure Backoff Strategy (Priority: P1) MVP

**Goal**: Users can choose between exponential, linear, and constant backoff strategies for task retries.

**Independent Test**: Create tasks with each strategy and verify the delay between retry attempts matches the expected pattern (exponential: 1m/2m/4m, linear: 1m/2m/3m, constant: 1m/1m/1m).

### Implementation for User Story 1

- [ ] T008 [US1] Refactor the backoff calculation in the retry loop in internal/daemon/daemon.go (around line 1455-1459) to use a helper function that accepts strategy, base delay, and attempt number and returns the computed delay. Support "exponential" (delay * 2^attempt), "linear" (delay * (attempt+1)), and "constant" (delay). Apply 1-hour maximum cap to all strategies.
- [ ] T009 [US1] Wire the new backoff helper into the retry loop: read t.RetryStrategy (default "exponential" when empty) and pass it to the backoff calculation instead of the hardcoded exponential logic in internal/daemon/daemon.go
- [ ] T010 [US1] Update the retry log message (around line 1461) to include the strategy name in the output, e.g., "retry 1/3 for proj/task after error (exponential backoff 2m)" in internal/daemon/daemon.go
- [ ] T011 [US1] Add unit tests for the backoff helper function: verify exponential (1m,2m,4m,8m), linear (1m,2m,3m,4m), constant (1m,1m,1m,1m), 1-hour cap, and default-to-exponential behavior in internal/daemon/daemon_test.go
- [ ] T012 [US1] Update the dry-run output in cmd/anvil/dryrun.go to display the configured retry_strategy alongside existing retry and retry_delay fields

**Checkpoint**: Users can configure backoff strategies. Legacy tasks continue to work with exponential backoff (default).

---

## Phase 4: User Story 2 - Add Jitter (Priority: P2)

**Goal**: Users can add randomization to retry delays to prevent thundering herd.

**Independent Test**: Configure a task with jitter: 0.5 and delay: 1m, trigger retries, and verify actual delays fall within [30s, 90s].

### Implementation for User Story 2

- [ ] T013 [US2] Add jitter application to the backoff helper: after computing the strategy-based delay and applying the 1-hour cap, apply jitter using the formula `delay * (1 + jitter * (2*rand - 1))` with a floor of 1 second. Use math/rand for randomization. In internal/daemon/daemon.go
- [ ] T014 [US2] Wire t.RetryJitter into the backoff calculation in the retry loop in internal/daemon/daemon.go
- [ ] T015 [US2] Update the retry log message to show "jitter" when jitter is applied, e.g., "retry 1/3 for proj/task after error (exponential backoff 2m3s +jitter)" in internal/daemon/daemon.go
- [ ] T016 [US2] Add unit tests for jitter: verify delay stays within [delay*(1-jitter), delay*(1+jitter)] range over multiple samples, verify jitter=0 produces deterministic delay, verify floor of 1 second in internal/daemon/daemon_test.go

**Checkpoint**: Jitter works. Tasks with retry_jitter configured show randomized delays.

---

## Phase 5: User Story 3 - Limit Total Retry Duration (Priority: P2)

**Goal**: Users can set a max_total_time to limit how long retries continue.

**Independent Test**: Set retry: 10, delay: 5m, retry_max_time: 15m and verify retries stop after ~15 minutes even with attempts remaining.

### Implementation for User Story 3

- [ ] T017 [US3] Add max_total_time check to the retry loop in internal/daemon/daemon.go: before each retry attempt, check if elapsed time since first attempt exceeds t.RetryMaxTime (when > 0). If exceeded, log "retry time budget exhausted" and break the loop.
- [ ] T018 [US3] Add unit test for max_total_time: verify retries stop when time budget is exceeded, verify unlimited retries when RetryMaxTime is 0 in internal/daemon/daemon_test.go

**Checkpoint**: Total retry duration limiting works independently.

---

## Phase 6: User Story 4 - Show Retry Strategy in Task History (Priority: P3)

**Goal**: Task history displays retry strategy, delays used, and attempt details.

**Independent Test**: Run a task that retries, then run `anvil task history <task>` and verify retry strategy info appears.

### Implementation for User Story 4

- [ ] T019 [US4] Record retry metadata in RunRecord: in the retry loop in internal/daemon/daemon.go, track actual delays used in a []string slice. After the loop, populate RunRecord.RetryStrategy and RunRecord.RetryDelaysUsed (around line 1547-1550).
- [ ] T020 [US4] Update task history display in cmd/anvil/task_lifecycle.go to show RetryStrategy and RetryDelaysUsed when the run had retries (Attempt > 1 or MaxRetries > 0). Format as "Strategy: exponential, Delays: 1m, 2m3s, 4m1s".
- [ ] T021 [US4] Update the on_failure/on_success hook environment variables in internal/daemon/daemon.go (around line 1941-1944) to include ANVIL_RETRY_STRATEGY

**Checkpoint**: Full observability. Users can see retry strategy details in task history.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Ensure backward compatibility and clean integration across all stories.

- [ ] T022 Verify backward compatibility: ensure existing tasks with only `retry: N` and `retry_delay: Xm` (no new fields) behave identically to current behavior (exponential backoff, no jitter, no time limit) by running existing tests in internal/project/project_config_test.go
- [ ] T023 Update the persistent task failure backoff logic in internal/daemon/daemon.go (around line 1768-1771) to use the same backoff helper with strategy support instead of its own hardcoded exponential calculation
- [ ] T024 Run full test suite (`go test ./...`) and verify no regressions

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies - can start immediately (extends existing code)
- **User Stories (Phase 3-6)**: All depend on Foundational phase completion
  - US1 (backoff strategy) can start after Phase 2
  - US2 (jitter) depends on US1 (builds on the backoff helper)
  - US3 (max_total_time) can start after Phase 2 (independent of US1/US2)
  - US4 (history display) depends on US1 (needs RetryStrategy in RunRecord)
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Depends on Phase 2 only — no dependencies on other stories
- **US2 (P2)**: Depends on US1 (adds jitter to the backoff helper created in US1)
- **US3 (P2)**: Depends on Phase 2 only — independent of US1/US2
- **US4 (P3)**: Depends on US1 (needs RetryStrategy field populated in RunRecord)

### Parallel Opportunities

- T001-T007 (foundational): T001, T004, T005 can run in parallel (different structs). T002, T003 depend on T001. T006, T007 depend on T002.
- US1 and US3 can be implemented in parallel after Phase 2
- US2 must follow US1
- US4 can start after US1

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 2: Foundational (data model + parsing)
2. Complete Phase 3: User Story 1 (backoff strategies)
3. **STOP and VALIDATE**: Test all three strategies with existing retry tasks
4. This alone delivers significant value — users can choose their backoff pattern

### Incremental Delivery

1. Phase 2 (Foundational) → Data model ready
2. US1 (Backoff strategies) → MVP: configurable strategies
3. US3 (Max total time) → Time-bounded retries (independent of jitter)
4. US2 (Jitter) → Thundering herd prevention
5. US4 (History display) → Full observability
6. Phase 7 (Polish) → Backward compat verification, cleanup

---

## Notes

- All changes are in existing files — no new packages or files created
- The backoff helper function (T008) is the key abstraction that US1, US2, and US3 build on
- Persistent task backoff (T023) reuses the same helper for consistency
- Total tasks: 24
