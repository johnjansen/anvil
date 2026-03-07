# Tasks: Cluster CLI Commands

**Input**: Design documents from `/specs/012-cluster-cli/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: No new files needed. All changes go in existing files. This phase adds the shared daemon endpoint and client functions.

- [x] T001 Add /cluster/leave POST endpoint handler in internal/daemon/daemon.go
- [x] T002 Register /cluster/leave route in daemon HTTP mux in internal/daemon/daemon.go
- [x] T003 [P] Add SendClusterStatusRequest() function using socketClient() in internal/daemon/daemon.go
- [x] T004 [P] Add SendClusterLeaveRequest() function using socketClient() in internal/daemon/daemon.go
- [x] T005 Add clusterCmd() dispatcher (status/health/leave) with "cluster" case in main switch in cmd/anvil/main.go

**Checkpoint**: Foundation ready - shared infrastructure for all 3 CLI commands exists.

---

## Phase 2: User Story 1 - View Cluster Status (Priority: P1)

**Goal**: Operators can run `anvil cluster status` to see cluster topology.

**Independent Test**: Run `anvil cluster status` against a running daemon and verify output shows node ID, role, term, leader, and member list.

### Implementation for User Story 1

- [x] T006 [US1] Implement clusterStatusCmd() in cmd/anvil/main.go that calls SendClusterStatusRequest() and prints human-readable status table (node, role, term, leader, member list with ID/ROLE/LAST SEEN columns)
- [x] T007 [US1] Add --json flag support to clusterStatusCmd() for raw JSON output in cmd/anvil/main.go
- [x] T008 [US1] Handle error cases: daemon not running (connection refused), cluster disabled (enabled: false in response) in cmd/anvil/main.go

**Checkpoint**: `anvil cluster status` and `anvil cluster status --json` work independently.

---

## Phase 3: User Story 2 - Check Cluster Health (Priority: P2)

**Goal**: Operators can run `anvil cluster health` to get a healthy/degraded/unhealthy assessment.

**Independent Test**: Run `anvil cluster health` and verify it returns a clear health assessment based on leader presence and member staleness.

### Implementation for User Story 2

- [x] T009 [US2] Implement clusterHealthCmd() in cmd/anvil/main.go that calls SendClusterStatusRequest() and derives health: healthy (leader exists, all members seen within 3x heartbeat), degraded (leader exists but stale members), unhealthy (no leader)
- [x] T010 [US2] Add --json flag support to clusterHealthCmd() for JSON output with status/node_id/leader_id/cluster_size/stale_count fields in cmd/anvil/main.go
- [x] T011 [US2] Handle error cases: daemon not running, cluster disabled in cmd/anvil/main.go

**Checkpoint**: `anvil cluster health` and `anvil cluster health --json` work independently.

---

## Phase 4: User Story 3 - Leave Cluster (Priority: P3)

**Goal**: Operators can run `anvil cluster leave` to gracefully remove daemon from cluster.

**Independent Test**: Run `anvil cluster leave` and verify the daemon stops cluster participation.

### Implementation for User Story 3

- [x] T012 [US3] Implement clusterLeaveCmd() in cmd/anvil/main.go that calls SendClusterLeaveRequest() and prints confirmation
- [x] T013 [US3] Handle error cases: daemon not running, cluster disabled, already left in cmd/anvil/main.go

**Checkpoint**: `anvil cluster leave` works independently.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Build verification

- [x] T014 Run go build ./cmd/anvil/ to verify all changes compile cleanly

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **User Story 1 (Phase 2)**: Depends on T003 (SendClusterStatusRequest) and T005 (clusterCmd dispatcher)
- **User Story 2 (Phase 3)**: Depends on T003 (SendClusterStatusRequest) and T005 (clusterCmd dispatcher)
- **User Story 3 (Phase 4)**: Depends on T001, T002, T004 (leave endpoint + SendClusterLeaveRequest) and T005 (clusterCmd dispatcher)
- **Polish (Phase 5)**: Depends on all user stories being complete

### Parallel Opportunities

- T003 and T004 can run in parallel (different functions, same file but independent)
- US1 and US2 both depend on T003 but can be implemented sequentially
- US3 is independent of US1/US2 (uses different endpoint)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T005)
2. Complete Phase 2: User Story 1 (T006-T008)
3. **STOP and VALIDATE**: Test `anvil cluster status`
4. This alone delivers the most critical cluster visibility command

### Incremental Delivery

1. Setup -> US1 (status) -> US2 (health) -> US3 (leave) -> Build verification
2. Each story adds value without breaking previous stories

---

## Notes

- All changes go in exactly 2 existing files: cmd/anvil/main.go and internal/daemon/daemon.go
- The cluster package (internal/cluster/) already exists from issue #299
- The /cluster/status endpoint already exists from issue #299
- Only /cluster/leave is a new daemon endpoint
- Total: 14 tasks across 5 phases
