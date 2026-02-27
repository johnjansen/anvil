# Data Model: Shared Task Queue

## Entities

### TaskAssignment (new - serialized for cluster transport)

| Field         | Type          | Description                                    |
|---------------|---------------|------------------------------------------------|
| AssignmentID  | string        | Unique ID for this assignment                  |
| TaskName      | string        | Task name (todo filename)                      |
| TaskID        | string        | Task UUID for session tracking                 |
| ProjectPath   | string        | Source project path on leader                   |
| Content       | string        | Task content/command to execute                |
| Runner        | string        | Runner command (e.g., "bash", "claude")        |
| RunnerChain   | []string      | Fallback runner chain                          |
| Timeout       | Duration      | Task timeout                                   |
| Env           | map[str]str   | Environment variables                          |
| Priority      | int           | Task priority (0-9)                            |
| Labels        | []string      | Task labels                                    |
| NodeAffinity  | string        | Target node ID (empty = any)                   |
| TargetNodeID  | string        | Assigned node ID                               |

### TaskResult (new - sent from follower to leader)

| Field         | Type          | Description                                    |
|---------------|---------------|------------------------------------------------|
| AssignmentID  | string        | Matches the TaskAssignment                     |
| TaskName      | string        | Task name                                      |
| TaskID        | string        | Task UUID                                      |
| NodeID        | string        | Executing node ID                              |
| Success       | bool          | Whether execution succeeded                    |
| Started       | time.Time     | Execution start time                           |
| Finished      | time.Time     | Execution end time                             |
| Error         | string        | Error message if failed                        |
| OutputSummary | string        | Brief output summary                           |

### WorkerReport (new - piggybacked on heartbeat_ack)

| Field       | Type   | Description                    |
|-------------|--------|--------------------------------|
| NodeID      | string | Reporting node ID              |
| TotalWorkers| int    | Total worker pool size         |
| BusyWorkers | int    | Currently executing tasks      |
| IdleWorkers | int    | Available for new tasks        |

## Modified Entities

### Todo (existing - internal/project/project.go:97)

Added field:
| Field        | Type   | Description                         |
|--------------|--------|-------------------------------------|
| NodeAffinity | string | Target node ID for execution ("" = any) |

### RunRecord (existing - internal/project/project.go:140)

Added field:
| Field  | Type   | Description                           |
|--------|--------|---------------------------------------|
| NodeID | string | Cluster node that executed this run   |

### Message (existing - internal/cluster/types.go:44)

Added field:
| Field   | Type            | Description                     |
|---------|-----------------|---------------------------------|
| Payload | json.RawMessage | Message-type-specific data      |

## State Transitions

### Task Assignment Lifecycle
```
Due (leader tick) -> Assigned (sent to node) -> Running (node executing) -> Completed (result received)
                                              -> Failed (result with error)
```

### Worker Availability
```
Idle (reported in heartbeat_ack) -> Busy (assigned task) -> Idle (task complete)
```
