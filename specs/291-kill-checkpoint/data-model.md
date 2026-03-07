# Data Model: Task Kill with Checkpoint

## Modified Entities

### RunningTask (internal/daemon)

Existing struct extended with graceful shutdown support.

| Field | Type | Description |
|-------|------|-------------|
| Project | string | Project path |
| Name | string | Task display name |
| TaskID | string | Task identifier |
| PID | int | Daemon PID (not child) |
| Started | time.Time | When task started |
| Timeout | time.Duration | Configured timeout |
| Cancel | context.CancelFunc | Context cancellation |
| **GracefulStop** | **chan struct{}** | **NEW: Closed to signal graceful shutdown request** |
| **ShuttingDown** | **bool** | **NEW: True if graceful shutdown in progress** |
| LogPath | string | Log file path |
| SessionID | string | Session identifier |
| Status | string | Dynamic status |
| WarningTriggered | bool | Timeout warning sent |
| TimeoutExtensions | int | Adaptive timeout extensions |
| OriginalTimeout | time.Duration | Original timeout before extensions |

### KillRequest (internal/daemon)

Existing struct extended with checkpoint flag.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Task ID to kill |
| **Checkpoint** | **bool** | **NEW: If true, request graceful shutdown for checkpoint save** |

### Todo (internal/project)

Existing struct extended with grace period.

| Field | Type | Description |
|-------|------|-------------|
| Checkpoint | bool | Existing: enable checkpoint capture |
| **CheckpointGracePeriod** | **time.Duration** | **NEW: Max wait time after graceful signal (default 30s)** |

### RunRecord (internal/project)

No schema changes. Uses existing fields:

| Field | Usage |
|-------|-------|
| Success | `false` for checkpoint stop |
| Error | `"stopped-with-checkpoint"` for checkpoint stop |
| CheckpointData | Already stores latest checkpoint data |

## State Transitions

```
Running Task
    │
    ├── anvil task kill (no --checkpoint)
    │   └── Cancel() → context cancelled → task exits → RunRecord{Success:false, Error:"killed"}
    │
    └── anvil task kill --checkpoint
        ├── checkpoint: false → ERROR "checkpoint not enabled"
        └── checkpoint: true
            └── close(GracefulStop) → SIGTERM to child
                ├── Child exits within grace period
                │   └── RunRecord{Success:false, Error:"stopped-with-checkpoint", CheckpointData:data}
                └── Child does NOT exit within grace period
                    └── SIGKILL → RunRecord{Success:false, Error:"killed-after-grace-period"}
```

## Frontmatter Extension

```yaml
---
schedule: "*/30 * * *"
checkpoint: true
checkpoint_grace_period: 45s  # NEW: optional, default 30s
---
```
