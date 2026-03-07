# Implementation Plan: Leader Election for Cluster Coordination

## Technical Context

- **Language**: Go 1.24.6 (stdlib net, encoding/json, sync, crypto/rand, time)
- **Architecture**: New internal/cluster/ package, integrated into daemon via config flag
- **Transport**: TCP with JSON-encoded messages between nodes
- **Node identity**: UUID persisted to .anvil/node-id
- **Key data**: ElectionState (term, role, votedFor, leaderID), peer list, heartbeat timers

## File Structure

```text
internal/cluster/types.go      — Message, Node, ElectionState types and constants
internal/cluster/node.go        — Node lifecycle: Start, Stop, IsLeader, Status
internal/cluster/election.go    — Election logic: startElection, handleVoteRequest/Response
internal/cluster/transport.go   — TCP listener, peer connections, send/receive messages
internal/cluster/heartbeat.go   — Leader heartbeat sender, follower timeout detector
internal/config/config.go       — ClusterConfig struct
internal/daemon/daemon.go       — Wire cluster into daemon lifecycle, /cluster/status endpoint
```

## Implementation Approach

### Phase 1: Types and Transport (Setup)
1. Define Message, Role, ElectionState types
2. Implement TCP transport (listen, connect, send, receive)
3. Add ClusterConfig to config package

### Phase 2: Election Core (US1 - P1)
4. Implement node identity (generate/load UUID)
5. Implement election state machine (follower → candidate → leader)
6. Implement vote request/response handling
7. Implement term management and majority counting

### Phase 3: Heartbeat and Failover (US2 - P1)
8. Leader heartbeat sender (periodic)
9. Follower heartbeat timeout detection
10. Leader step-down on majority loss
11. Automatic re-election on timeout

### Phase 4: Integration and Status (US3 - P2)
12. Wire cluster.Node into daemon lifecycle
13. Add /cluster/status HTTP endpoint
14. Guard task scheduling with IsLeader() check

## Dependencies
- internal/config/config.go (cluster config)
- internal/daemon/daemon.go (integration point)
- No external dependencies

## Constitution Check
- All stdlib, no new dependencies ✓
- Backward compatible (cluster is opt-in via config) ✓
- Daemon still works standalone when cluster.enabled=false ✓
