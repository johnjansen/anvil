# Contracts: Shared Task Queue

## Cluster Message Types (new)

### task_assign (Leader -> Follower)
Leader sends this to assign a task to a specific follower for execution.
```json
{
  "type": "task_assign",
  "term": 5,
  "from_id": "leader-node",
  "to_id": "follower-node",
  "payload": {
    "assignment_id": "uuid",
    "task_name": "check-disk",
    "task_id": "task-uuid",
    "project_path": "/path/to/project",
    "content": "df -h",
    "runner": "bash",
    "timeout": "30s",
    "priority": 1
  }
}
```

### task_result (Follower -> Leader)
Follower sends this after task execution completes.
```json
{
  "type": "task_result",
  "term": 5,
  "from_id": "follower-node",
  "to_id": "leader-node",
  "payload": {
    "assignment_id": "uuid",
    "task_name": "check-disk",
    "task_id": "task-uuid",
    "node_id": "follower-node",
    "success": true,
    "started": "2026-02-27T12:00:00Z",
    "finished": "2026-02-27T12:00:05Z"
  }
}
```

### heartbeat_ack (extended with worker info)
Existing heartbeat_ack message extended with worker availability in payload.
```json
{
  "type": "heartbeat_ack",
  "term": 5,
  "from_id": "follower-node",
  "payload": {
    "total_workers": 4,
    "busy_workers": 1,
    "idle_workers": 3
  }
}
```

## Modified Frontmatter Field

### node (new, optional)
```yaml
---
node: "abc123-def4-5678"
---
```

Constrains task execution to the specified cluster node ID. If omitted, the task can run on any node.
