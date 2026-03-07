# Data Model: Leader Election

## Entities

### Node
Represents a daemon instance in the cluster.

| Field     | Type   | Description                         |
|-----------|--------|-------------------------------------|
| ID        | string | Unique node identifier (UUID)       |
| Address   | string | TCP address (host:port) for cluster |
| Role      | enum   | leader, follower, candidate         |
| Term      | uint64 | Current election term number        |
| LeaderID  | string | ID of the current known leader      |
| JoinedAt  | time   | When this node joined the cluster   |
| LastSeen  | time   | Last heartbeat received             |

### ElectionState
Tracks the current election protocol state for a node.

| Field         | Type   | Description                              |
|---------------|--------|------------------------------------------|
| CurrentTerm   | uint64 | Monotonically increasing term number     |
| VotedFor      | string | Node ID voted for in current term        |
| Role          | enum   | leader, follower, candidate              |
| LeaderID      | string | Known leader for current term            |
| Votes         | int    | Votes received (when candidate)          |
| ClusterSize   | int    | Known number of cluster members          |

### Message
Protocol messages exchanged between nodes.

| Field     | Type   | Description                           |
|-----------|--------|---------------------------------------|
| Type      | enum   | vote_request, vote_response, heartbeat, heartbeat_ack |
| Term      | uint64 | Sender's current term                 |
| FromID    | string | Sender node ID                        |
| ToID      | string | Recipient node ID (empty for broadcast) |
| Granted   | bool   | For vote_response: whether vote granted |

## State Transitions

```
                    ┌───────────┐
    startup ──────► │ FOLLOWER  │ ◄──── higher term seen
                    └─────┬─────┘       leader heartbeat
                          │
              election    │
              timeout     │
                          ▼
                    ┌───────────┐
                    │ CANDIDATE │ ──── election timeout
                    └─────┬─────┘      (restart election)
                          │
              majority    │
              votes       │
                          ▼
                    ┌───────────┐
                    │  LEADER   │ ──── loses majority
                    └───────────┘      → FOLLOWER
```
