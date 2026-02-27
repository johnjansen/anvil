# Data Model: Health Check Endpoint

## Entities

### ReadinessResponse
Returned by /ready endpoint.

| Field  | Type    | Description                          |
|--------|---------|--------------------------------------|
| ready  | boolean | Whether daemon can accept new tasks  |
| reason | string  | Explanation when not ready (optional) |

### LivenessResponse
Returned by /live endpoint.

| Field | Type    | Description                     |
|-------|---------|--------------------------------|
| alive | boolean | Whether daemon process is alive |

### StatusResponse
Returned by /status endpoint.

| Field         | Type    | Description                          |
|---------------|---------|--------------------------------------|
| ready         | boolean | Readiness status                     |
| workers       | object  | Worker pool state                    |
| workers.available | int | Available worker slots               |
| workers.max   | int     | Maximum worker slots                 |
| projects      | int     | Number of loaded projects            |
| running_tasks | int     | Currently executing tasks            |
| pending_tasks | int     | Tasks queued/waiting for execution   |
| uptime        | string  | Daemon uptime duration               |
| draining      | boolean | Whether daemon is in drain mode      |

## State Transitions

### Readiness State
```
                    ┌──────────┐
    startup ──────► │ NOT READY│ ◄──── workers full
                    └─────┬────┘      drain mode
                          │            shutdown
                          ▼
                    ┌──────────┐
    projects ─────► │  READY   │ ◄──── worker freed
    loaded          └──────────┘       drain cleared
```
