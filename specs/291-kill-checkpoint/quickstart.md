# Quickstart: Task Kill with Checkpoint

## Overview

This feature adds a `--checkpoint` flag to `anvil task kill` that gracefully stops a running task, giving it time to save checkpoint data before exiting. The next run automatically resumes from the saved checkpoint.

## Prerequisites

- Task must have `checkpoint: true` in its frontmatter
- Task must handle SIGTERM signal to save checkpoint before exiting

## Setup

### 1. Enable checkpoint on your task

```yaml
---
schedule: "*/30 * * *"
checkpoint: true
checkpoint_grace_period: 45s  # optional, default 30s
---
Process items from data queue...
```

### 2. Task writes checkpoint data

The task emits checkpoint data via stdout:

```bash
echo "##anvil:checkpoint {\"last_item\":5000,\"total\":10000}"
```

### 3. Task handles SIGTERM for graceful shutdown

```python
import signal, sys, json

checkpoint = {"last_item": 0}

def handle_sigterm(signum, frame):
    print(f"##anvil:checkpoint {json.dumps(checkpoint)}")
    sys.exit(0)

signal.signal(signal.SIGTERM, handle_sigterm)
```

### 4. Task resumes from checkpoint

On startup, check for `ANVIL_CHECKPOINT_DATA` environment variable:

```python
import os, json

checkpoint_data = os.environ.get("ANVIL_CHECKPOINT_DATA")
if checkpoint_data:
    checkpoint = json.loads(checkpoint_data)
    print(f"Resuming from item {checkpoint['last_item']}")
```

## Usage

```bash
# Gracefully stop with checkpoint
anvil task kill my-task --checkpoint

# View checkpoint status in history
anvil task history my-task

# Regular kill (unchanged, no checkpoint save)
anvil task kill my-task
```

## Key Files to Modify

| File | Change |
|------|--------|
| `cmd/anvil/task_lifecycle.go` | Add `--checkpoint` flag to kill command |
| `internal/daemon/daemon.go` | Extend `KillRequest`, `RunningTask`, `handleKill` |
| `internal/daemon/daemon.go` | Add graceful shutdown detection in `runTask` |
| `internal/project/project.go` | Add `CheckpointGracePeriod` to Todo, parse frontmatter |
