# Tasks: Task Source File Sync

**Feature**: 405-task-source-sync
**Input**: [plan.md](./plan.md), [spec.md](./spec.md), [data-model.md](./data-model.md), [contracts/cli.md](./contracts/cli.md), [research.md](./research.md), [quickstart.md](./quickstart.md)

## Conventions

- File paths are absolute from repo root.
- `[P]` marks a task that can run in parallel with sibling `[P]` tasks (different files, no shared in-memory state).
- `[US1]`, `[US2]`, `[US3]` map to spec.md user stories (P1 reload-picks-up-edits, P2 drift-surfacing, P3 frontmatter-aliases).
- Tests are expected per Go convention (alongside source as `_test.go`); test tasks are included.

## Phase 1: Setup

- [ ] T001 Confirm worktree is on branch `405-task-source-sync` and `go build ./...` succeeds before any changes
- [ ] T002 [P] Add a short note in `internal/project/doc.go` (create if missing) describing the new `SourceMeta` sidecar concept for package-level discoverability

## Phase 2: Foundational (blocking)

These must land before any user story work because every story depends on the `SourceMeta` type and sidecar I/O.

- [ ] T003 Create `internal/project/source_sync.go` with the `SourceMeta` struct (fields per `data-model.md`), JSON tags, and constructor `NewSourceMeta(sourcePath string) (*SourceMeta, error)` that resolves `filepath.Abs` + `filepath.EvalSymlinks`
- [ ] T004 In `internal/project/source_sync.go`, add `sidecarPath(mdPath string) string` helper (returns `<slug>.meta.json` alongside an `<slug>.md`)
- [ ] T005 In `internal/project/source_sync.go`, add `ReadSourceMeta(mdPath string) (*SourceMeta, error)` — returns `(nil, nil)` when sidecar absent (backward compat), returns error only on malformed JSON
- [ ] T006 In `internal/project/source_sync.go`, add `WriteSourceMeta(mdPath string, meta *SourceMeta) error` with atomic tmp-file + `os.Rename` semantics
- [ ] T007 In `internal/project/source_sync.go`, add `HashSource(path string) (string, error)` computing hex sha256 of raw file bytes
- [ ] T008 [P] Create `internal/project/source_sync_test.go` with table-driven tests covering: absent sidecar, malformed sidecar, round-trip write/read, atomicity (no partial writes visible), symlink resolution at construction
- [ ] T009 In `internal/project/project.go`, extend `Todo` struct to add `SourceMeta *SourceMeta` field (nil-safe default)
- [ ] T010 In `internal/project/project.go`, modify `LoadTodos` to populate `todo.SourceMeta` via `ReadSourceMeta` for each discovered `.md` (no behavior change when sidecar absent)
- [ ] T011 [P] Add unit test in `internal/project/project_config_test.go` (or new `todo_sourcemeta_test.go`) verifying `LoadTodos` populates `SourceMeta` correctly and leaves it nil when no sidecar exists

## Phase 3: User Story 1 — Edited source file is picked up on reload (P1)

**Goal**: `anvil reload` re-reads task content from the recorded source path and applies changes atomically; task identity (UUID, history) is preserved.

**Independent Test**: Quickstart verifications 1, 3, 8 (`quickstart.md`): register task with source file, edit source, reload, verify next run uses updated content, run history preserved.

- [ ] T012 [US1] In `cmd/anvil/task_create.go`, modify the `anvil add -f` path to call `NewSourceMeta(sourcePath)` and `WriteSourceMeta(mdPath, meta)` after the existing `.md` write; compute initial hash from the source bytes already read
- [ ] T013 [US1] In `cmd/anvil/init_cmd.go`, modify `anvil init` / `anvil register` discovery of pre-existing task files to similarly record `SourceMeta` for each registered task
- [ ] T014 [P] [US1] In `internal/project/source_sync.go`, add `ReloadFromSource(mdPath string, todo *Todo) (changed bool, status string, err error)` implementing the reload algorithm from `data-model.md` (stat → hash → compare → parse → re-normalize → atomic rewrite → update sidecar)
- [ ] T015 [US1] In `internal/project/source_sync.go`, ensure UUID preservation: `ReloadFromSource` MUST read the existing `.md`'s UUID from its frontmatter/body and write the same UUID into the regenerated `.md`; add explicit test for this
- [ ] T016 [P] [US1] In `internal/project/source_sync.go`, add `ReloadAllFromSources(rootDir string) (summary ReloadSummary, err error)` iterating `LoadTodos` and calling `ReloadFromSource` per task; return counts (checked, reloaded, drift, missing, invalid)
- [ ] T017 [US1] In `internal/daemon/daemon.go` `handleReload` (line ~2460) and/or the reload loop, call `project.ReloadAllFromSources` for each watched project after the existing config reload; include the summary in the HTTP response body as JSON
- [ ] T018 [US1] In `cmd/anvil/status.go` `reloadCmd`, parse the daemon's JSON response and print `tasks: N checked, X reloaded, Y drift, Z missing, W invalid`; fall back to `reloaded config` only if the daemon response lacks the summary (older daemon compatibility)
- [ ] T019 [P] [US1] Add test `internal/project/source_sync_reload_test.go` covering: successful reload (content + frontmatter change), no-op when source unchanged (fast path via hash), `SourceMeta == nil` tasks are skipped, reload preserves `Todo.ID`
- [ ] T020 [US1] Add dispatch test in `cmd/anvil/dispatch_test.go` for `anvil reload` output format when the daemon returns a reload summary (stub daemon response)

**Checkpoint**: After T020, US1 is independently shippable. A user can edit a source file, reload, and see the change applied.

## Phase 4: User Story 2 — Source drift is surfaced (P2)

**Goal**: `anvil task ls`, `anvil status`, and `anvil task get` show sync status clearly so users are never surprised by silent divergence.

**Independent Test**: Quickstart verifications 4, 6 (`quickstart.md`): edit source without reloading → drift shown; delete source → missing shown.

- [ ] T021 [P] [US2] In `internal/project/source_sync.go`, add `ComputeSyncStatus(todo *Todo) (SyncStatus, error)` per rules in `data-model.md` (no sidecar → no-source; file missing → missing; hash match → in-sync; else drift; carry invalid from LastLoadStatus)
- [ ] T022 [P] [US2] Add test `internal/project/sync_status_test.go` with table-driven cases for every branch of `ComputeSyncStatus`
- [ ] T023 [US2] In `cmd/anvil/task_list.go`, extend text rendering to add a `SYNC` column (text values `ok`, `drift`, `missing`, `invalid`, blank for no-source); use compact markers in any short-form output if present
- [ ] T024 [P] [US2] In `cmd/anvil/task_list.go`, extend JSON output (or add `--json` flag if absent) to include `sync_status`, `source_path`, `last_loaded_at` per task
- [ ] T025 [US2] In `cmd/anvil/task_state.go` (or wherever `anvil task get` is implemented — confirm during implementation), add `Source:`, `Last loaded:`, `Sync status:` lines to the output; show `(none)` when no sidecar
- [ ] T026 [US2] In `cmd/anvil/status.go` `statusCmd`, extend the per-project line to append `(X drift, Y missing)` when any non-`ok` tasks exist (omit when all `ok` or `no-source`)
- [ ] T027 [P] [US2] Add dispatch test in `cmd/anvil/dispatch_test.go` verifying `anvil task ls` shows `drift` column when a task's source has been edited (use a fixture with mismatched hash)

**Checkpoint**: After T027, US2 is shippable independently of US1 (even without reload-picks-up-edits, drift surfacing alone gives users visibility).

## Phase 5: User Story 3 — Frontmatter aliases honored + docs (P3)

**Goal**: `allowed-tools` and `max-concurrent` in source files stop being silently stripped; help text and `anvil task get` make the registration relationship explicit.

**Independent Test**: Quickstart verification 2 (`quickstart.md`): register a task with hyphenated keys, verify they take effect.

- [ ] T028 [US3] In `internal/project/project.go`, add `normalizeFrontmatterAliases(raw []byte) []byte` that rewrites known hyphenated keys (`allowed-tools`, `max-concurrent`) to canonical snake_case on a line-by-line basis (only at column 0, only for exact key matches); call this before `yaml.Unmarshal` at line 548 and before the typed struct unmarshal starting near line 485
- [ ] T029 [P] [US3] Add table-driven test in `internal/project/project_config_test.go` (or new `frontmatter_alias_test.go`) covering: hyphenated key accepted, canonical key still accepted, hyphenated key inside a nested value (e.g. inside `env:`) NOT rewritten, key appearing in body text NOT rewritten
- [ ] T030 [US3] In `cmd/anvil/task_create.go`, update the `anvil add --help` usage string (line ~177, line ~256) to document that `-f <file>` records the source path and that `anvil reload` re-reads it; match the wording in `contracts/cli.md`
- [ ] T031 [P] [US3] In `cmd/anvil/task_edit.go`, update help for `--content-file` to note that the registered `source_path` is updated when a new `--content-file` is supplied
- [ ] T032 [US3] In `cmd/anvil/task_edit.go`, implement the behavior from T031: when `--content-file <path>` is passed, call `NewSourceMeta(path)` and `WriteSourceMeta` so the sidecar now points at the new source
- [ ] T033 [P] [US3] In `cmd/anvil/task_lifecycle.go` (or wherever `anvil task rm` is implemented), after deleting `.md` also delete `.meta.json` if present; test for this in existing task_rm tests

**Checkpoint**: After T033, US3 ships independently. Hyphenated keys work; help text is accurate.

## Phase 6: Polish & Cross-Cutting

- [ ] T034 [P] Update `cmd/anvil/dispatch_test.go` or add a new integration test that runs the full `quickstart.md` sequence end-to-end against a temp `.anvil/` directory
- [ ] T035 [P] Add exit-code-2-on-invalid behavior to `cmd/anvil/status.go` `reloadCmd` per `contracts/cli.md`; unit test via dispatch test fixture
- [ ] T036 [P] Run `go vet ./...` and `go test ./...`; fix any introduced warnings
- [ ] T037 [P] Update `docs/` (if a user-facing docs tree exists — confirm `docs/plans/` vs `docs/user/`) with a short note about the new source-sync behavior linking to `anvil reload`
- [ ] T038 Run the full `quickstart.md` manually against a built `anvil` binary and confirm all 9 verifications pass; record results in the PR description
- [ ] T039 Close issue #405 with a PR link and a short summary of the fix; reference the spec file path in the commit message

## Dependency Graph

```
Phase 1 (T001, T002)
        ↓
Phase 2 (T003→T004→T005→T006, T007) ──┬─→ T008 [P test]
        ↓                               └─→ T009→T010→T011 [P test]
        │
        ├─→ Phase 3 US1 (T012, T013, T014→T015, T016, T017→T018, T019 [P], T020)
        │
        ├─→ Phase 4 US2 (T021→T022 [P], T023, T024 [P], T025, T026, T027 [P])
        │           (US2 depends on T010 populating SourceMeta but does NOT need US1)
        │
        └─→ Phase 5 US3 (T028→T029 [P], T030, T031 [P], T032, T033 [P])
                    (US3 is fully independent of US1/US2 except for shared foundation)
        ↓
Phase 6 Polish (T034–T039, mostly [P])
```

## Parallel Execution Examples

**After Phase 2 completes**, the following three swarms can proceed in parallel (separate contributors or separate agent sessions):

- Swarm A — US1: T012, T013, T014, T015, T016, T017, T018, T019, T020
- Swarm B — US2: T021, T022, T023, T024, T025, T026, T027
- Swarm C — US3: T028, T029, T030, T031, T032, T033

Within each swarm, `[P]` tasks can be claimed in parallel. Cross-swarm merge conflicts are expected only in `cmd/anvil/task_list.go` (US2 changes) and `cmd/anvil/task_create.go` (US1 + US3 both touch help text) — resolve via sequencing within Phase 5 T030 after Phase 3 T012.

## Implementation Strategy

- **MVP scope**: Phase 1 + Phase 2 + Phase 3 (US1 only). This alone fixes the reported bug in #405 — editing source files and running `anvil reload` takes effect. Ship as a first PR.
- **Follow-up PR**: Phase 4 (US2 drift surfacing) — polish that prevents future silent-divergence bugs.
- **Follow-up PR**: Phase 5 (US3 frontmatter aliases + docs) — cleanup of the secondary annoyance.
- **Final PR**: Phase 6 polish.

Each phase is independently mergeable. Users see progressively more value with each ship. Total task count: 39.
