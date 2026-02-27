# Data Model: Cluster CLI Commands

## Entities

### ClusterStatus (existing - from internal/cluster/types.go)

| Field       | Type         | Description                          |
|-------------|--------------|--------------------------------------|
| NodeID      | string       | This node's unique identifier        |
| Role        | Role         | Current role (leader/follower/candidate) |
| Term        | uint64       | Current election term                |
| LeaderID    | string       | ID of the current known leader       |
| ClusterSize | int          | Total number of known cluster members |
| Members     | []MemberInfo | List of all known members            |

### MemberInfo (existing - from internal/cluster/types.go)

| Field    | Type      | Description                    |
|----------|-----------|--------------------------------|
| ID       | string    | Member's unique identifier     |
| Address  | string    | Member's listen address        |
| Role     | Role      | Member's role                  |
| LastSeen | time.Time | Last time this member was seen |

### ClusterHealth (new - derived in CLI, not persisted)

| Field       | Type   | Description                                    |
|-------------|--------|------------------------------------------------|
| Status      | string | healthy, degraded, or unhealthy                |
| NodeID      | string | This node's ID                                 |
| Role        | string | This node's role                               |
| LeaderID    | string | Current leader ID (empty if none)              |
| ClusterSize | int    | Total member count                             |
| Stale       | int    | Number of stale members                        |

## State Transitions

Running (cluster enabled) -> Leave -> Running (cluster disabled, rejoin requires restart)
