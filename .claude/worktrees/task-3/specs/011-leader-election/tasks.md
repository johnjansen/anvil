# Tasks: Leader Election for Cluster Coordination

**Input**: Design documents from `/specs/011-leader-election/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Not explicitly requested. Tests omitted.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1, US2, US3)

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create cluster package, define types, add config

- [x] T001 Create `internal/cluster/` package directory
- [x] T002 Define Role constants, Message struct, and ElectionState struct in `internal/cluster/types.go` — Role enum (Leader, Follower, Candidate), Message with Type/Term/FromID/ToID/Granted fields, ElectionState with CurrentTerm/VotedFor/Role/LeaderID
- [x] T003 [P] Add ClusterConfig struct to `internal/config/config.go` — fields: Enabled bool, Name string, Listen string (default ":9091"), Peers []string, HeartbeatInterval duration (default 5s), ElectionTimeout duration (default 30s); add `Cluster ClusterConfig` field to Config struct

---

## Phase 2: Foundational (Transport Layer)

**Purpose**: TCP communication between cluster nodes

- [x] T004 Implement TCP transport in `internal/cluster/transport.go` — Transport struct with Listen(addr) to accept connections, Connect(addr) to dial peers, Send(conn, Message) and Receive(conn) using JSON encoder/decoder, Close() to shut down listener
- [x] T005 Implement peer management in `internal/cluster/transport.go` — maintain map of peer connections, auto-reconnect on disconnect, broadcast(Message) to all peers

**Checkpoint**: Transport can send and receive JSON messages between two nodes over TCP.

---

## Phase 3: User Story 1 — Prevent Duplicate Task Execution (Priority: P1)

**Goal**: Leader election ensures exactly one coordinator. Only leader schedules tasks.

**Independent Test**: Start 3 nodes with static peer list. Verify exactly one becomes leader. Verify only leader runs scheduler.

### Implementation

- [x] T006 [US1] Implement node identity in `internal/cluster/node.go` — Node struct with ID (UUID), config, transport, electionState, mutex; LoadOrCreateNodeID(path) generates UUID and persists to .anvil/node-id
- [x] T007 [US1] Implement Node.Start() in `internal/cluster/node.go` — initialize as Follower, start transport listener, connect to peers, launch election timer goroutine, launch message handler goroutine
- [x] T008 [US1] Implement Node.Stop() in `internal/cluster/node.go` — close transport, stop goroutines via context cancellation, clean shutdown
- [x] T009 [US1] Implement election state machine in `internal/cluster/election.go` — startElection(): increment term, vote for self, transition to Candidate, broadcast VoteRequest to all peers; resetElectionTimer() with random jitter (1x to 2x election timeout)
- [x] T010 [US1] Implement vote handling in `internal/cluster/election.go` — handleVoteRequest(msg): grant vote if term >= current and not yet voted in this term, reply VoteResponse; handleVoteResponse(msg): count votes, if majority received transition to Leader
- [x] T011 [US1] Implement term management in `internal/cluster/election.go` — stepDown(term): if incoming term > current, update term, clear votedFor, transition to Follower; called when any message with higher term is received
- [x] T012 [US1] Implement Node.IsLeader() and Node.LeaderID() in `internal/cluster/node.go` — thread-safe accessors for current role and leader identity

**Checkpoint**: Three nodes elect exactly one leader. Only IsLeader() returns true on one node.

---

## Phase 4: User Story 2 — Automatic Leader Failover (Priority: P1)

**Goal**: Followers detect leader failure and elect a new leader automatically.

**Independent Test**: Start 3 nodes, kill leader, verify new leader elected within timeout period.

### Implementation

- [x] T013 [US2] Implement leader heartbeat sender in `internal/cluster/heartbeat.go` — when role==Leader, broadcast Heartbeat message at HeartbeatInterval; stop sending when stepping down
- [x] T014 [US2] Implement follower heartbeat timeout in `internal/cluster/heartbeat.go` — followers track last heartbeat time; if ElectionTimeout elapses without heartbeat, call startElection()
- [x] T015 [US2] Implement heartbeat handler in `internal/cluster/heartbeat.go` — handleHeartbeat(msg): if term >= current, reset election timer, update leaderID, ensure role=Follower; reply HeartbeatAck
- [x] T016 [US2] Implement majority tracking for leader in `internal/cluster/heartbeat.go` — leader tracks HeartbeatAck responses; if less than majority respond within election timeout, leader steps down to Follower (split-brain prevention per FR5)

**Checkpoint**: Kill leader node, remaining nodes elect new leader within election timeout.

---

## Phase 5: User Story 3 — Leadership Visibility (Priority: P2)

**Goal**: Operators can query leadership status via API and see election history in logs.

**Independent Test**: Query /cluster/status on leader and follower, verify correct role/term/members.

### Implementation

- [x] T017 [US3] Implement Node.Status() in `internal/cluster/node.go` — returns ClusterStatus struct with NodeID, Role, Term, LeaderID, ClusterSize, Members list (each with ID, Address, Role, LastSeen)
- [x] T018 [US3] Add /cluster/status endpoint in `internal/daemon/daemon.go` — handleClusterStatus() handler that calls cluster.Node.Status() and returns JSON; register in startSocketServer() mux; also register in healthServer mux if health_port is configured
- [x] T019 [US3] Wire cluster.Node into daemon lifecycle in `internal/daemon/daemon.go` — if config.Cluster.Enabled: create cluster.Node in NewDaemon(), call node.Start() in Run(), call node.Stop() on shutdown; add clusterNode field to Daemon struct
- [x] T020 [US3] Guard task scheduling with IsLeader() in `internal/daemon/daemon.go` — in tick(), if cluster is enabled and node is not leader, skip task scheduling (log "not leader, skipping tick"); followers only execute tasks assigned by leader
- [x] T021 [US3] Add election event logging in `internal/cluster/election.go` — log role transitions (elected leader, stepped down, started election) with term number and reason using dlog-style format

**Checkpoint**: /cluster/status returns correct JSON. Task scheduling only runs on leader.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [x] T022 Ensure backward compatibility in `internal/daemon/daemon.go` — when cluster.enabled=false (default), daemon behavior is identical to pre-cluster; no cluster goroutines started, no cluster routes registered, task scheduling runs unconditionally
- [x] T023 Add .anvil/node-id to documentation/comments noting it should not be shared between nodes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies
- **Foundational (Phase 2)**: Depends on Setup (needs types)
- **US1 (Phase 3)**: Depends on Foundational (needs transport)
- **US2 (Phase 4)**: Depends on US1 (needs election state machine)
- **US3 (Phase 5)**: Depends on US1+US2 (needs working election + heartbeat)
- **Polish (Phase 6)**: Depends on all user stories

### Parallel Opportunities

- T003 (config) can run in parallel with T002 (types) — different files
- T006-T008 (node lifecycle) can run in parallel with T009-T011 (election logic) — different files
- T017 (Status method) can run in parallel with T018 (HTTP endpoint) — different packages

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1: Setup (T001-T003)
2. Phase 2: Foundational (T004-T005)
3. Phase 3: US1 (T006-T012)
4. **STOP and VALIDATE**: Exactly one leader elected in 3-node cluster

### Incremental Delivery

1. Setup + Foundational -> Transport infrastructure
2. US1 -> Leader election works -> Deploy (MVP!)
3. US2 -> Heartbeat + automatic failover -> Deploy
4. US3 -> Status API + daemon integration -> Deploy
5. Polish -> Backward compatibility verified -> Deploy

---

## Notes

- All code uses Go stdlib only (net, encoding/json, sync, crypto/rand, time)
- Cluster mode is opt-in via config — zero impact when disabled
- runner.go needs NO changes
- Total tasks: 23
