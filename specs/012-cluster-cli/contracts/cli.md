# CLI Contracts: Cluster Commands

## anvil cluster status

**Input**: No arguments. Optional --json flag.

**Human-readable output**:
```
Cluster Status
  Node:    abc123-def4-5678
  Role:    leader
  Term:    5
  Leader:  abc123-def4-5678
  Members: 3

  ID                  ROLE       LAST SEEN
  abc123-def4-5678    leader     now
  bcd234-ef56-7890    follower   2s ago
```

**JSON output**: Raw ClusterStatus JSON from daemon endpoint.

**Error (daemon not running)**: Error: cannot connect to daemon
**Error (cluster disabled)**: Cluster mode is not enabled.

## anvil cluster health

**Input**: No arguments. Optional --json flag.

**Output**: healthy/degraded/unhealthy with member counts and stale member details.

**JSON output**:
```json
{
  "status": "healthy",
  "node_id": "abc123",
  "leader_id": "abc123",
  "cluster_size": 3,
  "stale_count": 0
}
```

## anvil cluster leave

**Input**: No arguments.
**Output**: Node abc123-def4-5678 has left the cluster.
**Error (cluster disabled)**: Cluster mode is not enabled.

## Daemon Endpoints

### GET /cluster/status (existing)
Returns ClusterStatus JSON.

### POST /cluster/leave (new)
Stops cluster node participation. Returns {"left": true, "node_id": "..."}.
