# Tasks: Health Check Endpoint for Container Orchestration

**Input**: Design documents from `/specs/010-health-check-endpoint/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Not explicitly requested. Tests omitted.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1, US2, US3)

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add helper method for readiness evaluation and project counting

- [x] T001 Add `isReady()` method to Daemon struct in `internal/daemon/daemon.go` — returns (ready bool, reason string) by checking: workers available > 0, projects loaded > 0, not draining, not shutting down
- [x] T002 Add `projectCount()` method to Daemon struct in `internal/daemon/daemon.go` — returns int count of currently watched projects from the daemon's project list

---

## Phase 2: User Story 1 — Readiness Probe (Priority: P1)

**Goal**: /ready endpoint returns 200 when daemon can accept tasks, 503 when it cannot.

**Independent Test**: Start daemon with projects loaded, curl /ready -> 200. Fill all worker slots, curl /ready -> 503. Free a slot, curl /ready -> 200.

### Implementation

- [x] T003 [US1] Implement `handleReady()` HTTP handler in `internal/daemon/daemon.go` — calls isReady(), returns 200 with `{"ready": true}` or 503 with `{"ready": false, "reason": "..."}`, sets Content-Type: application/json
- [x] T004 [US1] Register `/ready` route in `startSocketServer()` mux in `internal/daemon/daemon.go` — add `mux.HandleFunc("/ready", d.handleReady)`

**Checkpoint**: /ready returns proper HTTP status codes based on daemon state.

---

## Phase 3: User Story 2 — Liveness Probe (Priority: P1)

**Goal**: /live endpoint returns 200 when daemon process is responsive.

**Independent Test**: Start daemon, curl /live -> 200. Kill daemon, curl /live -> connection refused.

### Implementation

- [x] T005 [P] [US2] Implement `handleLive()` HTTP handler in `internal/daemon/daemon.go` — always returns 200 with `{"alive": true}`, sets Content-Type: application/json, minimal overhead
- [x] T006 [US2] Register `/live` route in `startSocketServer()` mux in `internal/daemon/daemon.go` — add `mux.HandleFunc("/live", d.handleLive)`

**Checkpoint**: /live returns 200 as long as daemon process is running.

---

## Phase 4: User Story 3 — Detailed Status Endpoint (Priority: P1)

**Goal**: Enhanced /status returns JSON with readiness, worker counts, project/task metrics.

**Independent Test**: Start daemon with projects, curl /status -> JSON with ready, workers, projects, running_tasks, pending_tasks, uptime, draining fields.

### Implementation

- [x] T007 [US3] Enhance `handleStatus()` in `internal/daemon/daemon.go` — replace or extend existing status response to include: ready (bool), workers (object with available/max), projects (int), running_tasks (int), pending_tasks (int), uptime (string), draining (bool)

**Checkpoint**: /status returns comprehensive JSON suitable for monitoring dashboards.

---

## Phase 5: Transport — Optional TCP Health Port

**Goal**: Allow health endpoints to be accessible via TCP for Kubernetes probes without socat sidecar.

- [x] T008 Add `HealthPort` field to Config struct in `internal/config/config.go` — int field with yaml tag `health_port`, defaults to 0 (disabled)
- [x] T009 Implement `startHealthServer()` method in `internal/daemon/daemon.go` — if config.HealthPort > 0, start a separate `net.Listen("tcp", ":port")` with mux containing only /ready, /live, /status routes; start in goroutine alongside socket server
- [x] T010 Call `startHealthServer()` from `Daemon.Run()` in `internal/daemon/daemon.go` — launch alongside existing `startSocketServer()`; gracefully shutdown on daemon stop

---

## Phase 6: Polish & Cross-Cutting Concerns

- [x] T011 Ensure backward compatibility of existing `/health` endpoint in `internal/daemon/daemon.go` — verify handleHealth() is unchanged and still returns the same response format

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **US1 (Phase 2)**: Depends on Setup (needs isReady helper)
- **US2 (Phase 3)**: No dependencies on Setup (self-contained)
- **US3 (Phase 4)**: Depends on Setup (needs isReady and projectCount helpers)
- **Transport (Phase 5)**: Depends on US1+US2+US3 (routes must exist to register)
- **Polish (Phase 6)**: Depends on all phases

### Parallel Opportunities

- T005 (handleLive) can run in parallel with T001-T004 — it has no dependencies
- Phase 2 (US1) and Phase 3 (US2) can run in parallel after Setup
- T008 (config field) can run in parallel with T001-T007

---

## Implementation Strategy

### MVP First (User Story 1 + 2)

1. Phase 1: Setup (T001-T002)
2. Phase 2: US1 (T003-T004) + Phase 3: US2 (T005-T006) in parallel
3. **STOP and VALIDATE**: /ready and /live work via Unix socket

### Incremental Delivery

1. Setup + US1 + US2 -> Readiness and liveness probes via socket -> Deploy (MVP!)
2. US3 -> Enhanced /status endpoint -> Deploy
3. Transport -> Optional TCP health port -> Deploy
4. Polish -> Backward compatibility verification -> Deploy

---

## Notes

- All code uses Go stdlib only (net/http, encoding/json, sync/atomic)
- Health endpoints never block task execution
- Existing /health endpoint remains unchanged
- runner.go needs NO changes
- Total tasks: 11
