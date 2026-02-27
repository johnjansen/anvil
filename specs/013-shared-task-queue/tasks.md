# Tasks: Shared Task Queue Across Cluster Members

**Input**: Design documents from `/specs/013-shared-task-queue/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Extend cluster message types and create shared data structures for task distribution.

- [x] T001 Add Payload field (json.RawMessage) to Message struct in internal/cluster/types.go
- [x] T002 Add new message type constants (MsgTaskAssign, MsgTaskResult) in internal/cluster/types.go
- [x] T003 [P] Create internal/cluster/queue.go with TaskAssignment and TaskResult structs
- [x] T004 [P] Add NodeAffinity field to Todo struct and parse from frontmatter "node" key in internal/project/project.go
- [x] T005 [P] Add NodeID field to RunRecord struct in internal/project/project.go

**Checkpoint**: All shared types exist. Cluster messages can carry task payloads.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Worker availability reporting and task message handling in cluster node.

- [x] T006 Add WorkerReport callback field to Node struct and include worker data in heartbeat_ack payload in internal/cluster/node.go and internal/cluster/heartbeat.go
- [x] T007 Add peerWorkers map (nodeID -> idle count) to Node struct, parse WorkerReport from incoming heartbeat_ack payloads in internal/cluster/heartbeat.go
- [x] T008 Add task_assign and task_result cases to handleMessage switch in internal/cluster/node.go
- [x] T009 Add OnTaskAssign and OnTaskResult callback fields to Node struct for daemon to hook into in internal/cluster/node.go
- [x] T010 Add PeerWorkers() method to Node that returns map of node ID to idle worker count in internal/cluster/node.go
- [x] T011 Add AssignTask(nodeAddr string, assignment TaskAssignment) method to Node that sends task_assign message in internal/cluster/node.go
- [x] T012 Add ReportResult(leaderAddr string, result TaskResult) method to Node that sends task_result message in internal/cluster/node.go

**Checkpoint**: Cluster nodes can exchange worker reports, task assignments, and results.

---

## Phase 3: User Story 1 - Distributed Task Execution (Priority: P1)

**Goal**: Leader distributes due tasks to followers based on worker availability.

**Independent Test**: Schedule a task on a multi-node cluster. Verify the leader assigns it to a follower with idle workers.

### Implementation for User Story 1

- [x] T013 [US1] Set WorkerReport callback on clusterNode in daemon Run() to report idle worker count from workQueue capacity in internal/daemon/daemon.go
- [x] T014 [US1] Set OnTaskAssign callback on clusterNode to enqueue received TaskAssignment into local workQueue as a workItem in internal/daemon/daemon.go
- [x] T015 [US1] Set OnTaskResult callback on clusterNode to write RunRecord from TaskResult on the leader in internal/daemon/daemon.go
- [x] T016 [US1] Modify tick() to check PeerWorkers() and distribute tasks to remote nodes with idle workers via AssignTask() instead of only local dispatch in internal/daemon/daemon.go
- [x] T017 [US1] After follower completes a remote task, call ReportResult() to send TaskResult back to leader in internal/daemon/daemon.go
- [x] T018 [US1] Set NodeID field on RunRecord when writing results for both local and remote executions in internal/daemon/daemon.go

**Checkpoint**: Tasks scheduled on leader are distributed to followers. Results flow back to leader.

---

## Phase 4: User Story 2 - Node Affinity (Priority: P2)

**Goal**: Tasks with node affinity only execute on the designated node.

**Independent Test**: Set node affinity on a task. Verify it only runs on the specified node.

### Implementation for User Story 2

- [x] T019 [US2] In tick() distribution logic, check todo.NodeAffinity: if set, only assign to matching node; if that node has no idle workers, queue until it does in internal/daemon/daemon.go
- [x] T020 [US2] If NodeAffinity matches the leader's own node ID, dispatch locally instead of remotely in internal/daemon/daemon.go

**Checkpoint**: Tasks with node affinity execute exclusively on the designated node.

---

## Phase 5: User Story 3 - Cross-Node Result Access (Priority: P3)

**Goal**: Task results are visible from any node via CLI.

**Independent Test**: Execute a task on a follower. Query history from the leader. Verify the result is accessible.

### Implementation for User Story 3

- [x] T021 [US3] When leader writes RunRecord from remote TaskResult, include NodeID so history shows which node executed in internal/daemon/daemon.go
- [x] T022 [US3] Add NodeID column to "anvil task history" output when NodeID is present in RunRecord in cmd/anvil/main.go

**Checkpoint**: Task history shows execution node for distributed tasks.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Build verification and cleanup

- [x] T023 Run go build ./cmd/anvil/ to verify all changes compile cleanly

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (types and structs)
- **US1 (Phase 3)**: Depends on Phase 2 (node callbacks and methods)
- **US2 (Phase 4)**: Depends on Phase 3 (distribution logic to filter by affinity)
- **US3 (Phase 5)**: Depends on Phase 3 (results flowing back to leader)
- **Polish (Phase 6)**: Depends on all user stories

### Parallel Opportunities

- T003, T004, T005 can run in parallel (different files)
- US2 and US3 can be implemented in parallel after US1

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T005)
2. Complete Phase 2: Foundational (T006-T012)
3. Complete Phase 3: US1 Distributed Execution (T013-T018)
4. **STOP and VALIDATE**: Tasks flow from leader to followers and results come back

### Incremental Delivery

1. Setup + Foundational -> US1 (distributed execution) -> US2 (affinity) -> US3 (history) -> Build
2. Each story adds value without breaking previous stories

---

## Notes

- Files modified: internal/cluster/types.go, internal/cluster/node.go, internal/cluster/heartbeat.go, internal/project/project.go, internal/daemon/daemon.go, cmd/anvil/main.go
- One new file: internal/cluster/queue.go
- Total: 23 tasks across 6 phases
