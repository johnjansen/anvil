# Tasks: Task Execution Audit Log with Tamper Detection

**Input**: Design documents from `/specs/009-audit-log/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Not explicitly requested. Tests omitted.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1, US2, US3)

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create audit package, define types, implement signing

- [x] T001 Create `internal/audit/` package directory
- [x] T002 Implement signing key management in `internal/audit/signing.go` — LoadOrCreateKey(projectPath) returns 32-byte key from .anvil/audit-key, auto-generates with crypto/rand if missing, sets 0600 permissions
- [x] T003 Implement HMAC-SHA256 signing functions in `internal/audit/signing.go` — Sign(key, data) returns hex-encoded signature, Verify(key, data, signature) returns bool
- [x] T004 Define AuditEntry struct and operation constants in `internal/audit/audit.go` — struct with Timestamp, Operation, Actor, TaskName, ProjectPath, Details, PrevHash, Signature fields; operation constants for task.created, task.modified, task.deleted, task.run, task.completed, task.paused, task.resumed

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core audit log operations — append, read, chain hashing

- [x] T005 Implement AppendEntry(projectPath string, entry AuditEntry) error in `internal/audit/audit.go` — compute prev_hash from last entry, sign entry with HMAC, serialize as JSON, append line to .anvil/audit.jsonl
- [x] T006 Implement ReadEntries(projectPath string) ([]AuditEntry, error) in `internal/audit/audit.go` — read .anvil/audit.jsonl line by line, parse JSON, return slice
- [x] T007 Implement VerifyChain(projectPath string) (valid bool, errors []string) in `internal/audit/audit.go` — walk entries, verify each prev_hash matches SHA256 of previous entry JSON, verify each signature with HMAC
- [x] T008 [P] Implement helper function currentActor() string in `internal/audit/audit.go` — returns os/user.Current().Username or "unknown"
- [x] T009 [P] Implement helper function LogOperation(projectPath, operation, taskName string, details map[string]any) in `internal/audit/audit.go` — convenience wrapper that creates AuditEntry with timestamp and actor, calls AppendEntry, logs errors to stderr (never returns error to caller)

**Checkpoint**: Audit package can append, read, and verify chained signed entries.

---

## Phase 3: User Story 1 — Verify Task Run Integrity (Priority: P1) MVP

**Goal**: Run records are signed. Users can verify they haven't been tampered with.

**Independent Test**: Run a task, check run record has signature field. Run verify-logs, confirm valid. Edit the JSON file, re-run verify-logs, confirm tampered.

### Implementation

- [x] T010 [US1] Add Signature field to RunRecord struct in `internal/project/project.go` — add `Signature string` json tag `signature,omitempty`
- [x] T011 [US1] Implement SignRunRecord(projectPath string, rec *RunRecord) error in `internal/audit/signing.go` — serialize record without signature field, compute HMAC, set rec.Signature
- [x] T012 [US1] Implement VerifyRunRecord(projectPath string, rec RunRecord) bool in `internal/audit/signing.go` — recompute HMAC from record fields (excluding Signature), compare
- [x] T013 [US1] Wire signing into WriteRunRecord in `internal/project/project.go` — before json.Marshal, call audit.SignRunRecord to populate Signature field
- [x] T014 [US1] Add `anvil task verify-logs <name>` subcommand in `cmd/anvil/main.go` — read all run records for task, verify each signature, print summary (N/N valid or list tampered records)
- [x] T015 [US1] Add --verify flag to `anvil task history <name>` in `cmd/anvil/main.go` — show SIGNATURE column (valid/TAMPERED) next to each run record

**Checkpoint**: Run records are signed on creation. verify-logs and history --verify detect tampering.

---

## Phase 4: User Story 2 — View Task Operation History (Priority: P1)

**Goal**: All task operations are recorded in the audit log. Users can view and filter the log.

**Independent Test**: Create/modify/run/pause a task. Run anvil audit and confirm all operations listed.

### Implementation

- [x] T016 [US2] Emit audit entries from daemon task lifecycle in `internal/daemon/daemon.go` — call audit.LogOperation for task.run and task.completed events in runTask()
- [x] T017 [US2] Emit audit entries from CLI task operations in `cmd/anvil/main.go` — call audit.LogOperation for task.created (in add command), task.paused/task.resumed (in disable/enable), task.deleted (in remove)
- [x] T018 [US2] Add `anvil audit` command in `cmd/anvil/main.go` — read audit log, display table with TIMESTAMP, OPERATION, ACTOR, TASK, CHANGES columns
- [x] T019 [US2] Add --task and --since flags to `anvil audit` in `cmd/anvil/main.go` — filter entries by task name and date
- [x] T020 [US2] Add --show-diff flag to `anvil audit` in `cmd/anvil/main.go` — show Details field contents for each entry
- [x] T021 [US2] Add --verify flag to `anvil audit` in `cmd/anvil/main.go` — call VerifyChain and print integrity summary
- [x] T022 [US2] Add --json flag to `anvil audit` in `cmd/anvil/main.go` — output entries as JSON array

**Checkpoint**: All task operations are logged. anvil audit shows full history with filtering.

---

## Phase 5: User Story 3 — Export Audit Trail (Priority: P2)

**Goal**: Export audit trail as signed JSONL file for compliance.

**Independent Test**: Run anvil audit --export file.json, verify file contains all entries with valid signatures.

### Implementation

- [x] T023 [US3] Add --export flag to `anvil audit` in `cmd/anvil/main.go` — write filtered entries to specified file in JSONL format (preserving signatures and chain hashes)

**Checkpoint**: Audit trail can be exported for external compliance verification.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [x] T024 Add .anvil/audit-key to .gitignore in project root `.gitignore`
- [x] T025 Emit audit entry for task.modified when task file content changes in `internal/daemon/daemon.go` — detect file modifications during LoadTodos by comparing with previous load, log changes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Depends on Setup
- **US1 (Phase 3)**: Depends on Foundational (needs signing functions)
- **US2 (Phase 4)**: Depends on Foundational (needs LogOperation)
- **US3 (Phase 5)**: Depends on US2 (needs audit command to add --export flag)
- **Polish (Phase 6)**: Depends on all user stories

### Parallel Opportunities

- T008 and T009 can run in parallel within Foundational
- Phase 3 (US1) and Phase 4 (US2) can run in parallel after Foundational
- T010-T013 (signing infrastructure) can run parallel with T016-T017 (emit events)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1: Setup (T001-T004)
2. Phase 2: Foundational (T005-T009)
3. Phase 3: US1 (T010-T015)
4. **STOP and VALIDATE**: Run records are signed, verify-logs works

### Incremental Delivery

1. Setup + Foundational -> Core audit infrastructure
2. US1 -> Signed run records + verification -> Deploy (MVP!)
3. US2 -> Full audit trail with CLI viewing -> Deploy
4. US3 -> Compliance export -> Deploy
5. Polish -> .gitignore, change detection -> Deploy

---

## Notes

- All signing uses Go stdlib crypto (no external dependencies)
- Audit log failures never block task execution (best-effort)
- runner.go needs NO changes
- Total tasks: 25
