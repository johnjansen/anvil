## Problem

When cancelling a running task (`anvil task kill`), all progress is lost. For long-running tasks that have done significant work, users want to:
- Capture partial results before killing
- Save checkpoint state for manual resume
- Get summary of what was accomplished

## Proposed Solution

Add graceful cancellation with partial results:

### 1. Graceful kill with capture

```bash
# Request graceful shutdown, capture state before killing
anvil task kill my-task --graceful
# or
anvil task kill my-task -g

# Force kill if graceful takes too long
anvil task kill my-task --force
```

### 2. Pre-kill hook

```yaml
---
on_kill: "echo 'Saving state...' && cp /tmp/work.json /tmp/work-partial.json"
---
```

The `on_kill` hook runs before the task is terminated, giving it a chance to save state.

### 3. Partial result capture

Tasks can emit partial results:

```
##anvil:partial {"records_processed": 500, "last_id": 1234}
```

On kill, the daemon captures partial output and stores it in the run record.

### 4. Resume from partial

```bash
# See partial results from last run
anvil task partial my-task

# Resume with partial data
anvil task run my-task --resume
```

### 5. Environment variables on kill

```
ANVIL_IS_KILLED=true
ANVIL_PARTIAL_RESULTS={"records_processed": 500}
```

Task can check `ANVIL_IS_KILLED` and save state accordingly.

## Acceptance Criteria

- [ ] `anvil task kill --graceful` allows task to save state before termination
- [ ] `on_kill` hook runs before termination
- [ ] Partial results captured in run record
- [ ] `anvil task partial` shows partial results from last run
- [ ] `ANVIL_IS_KILLED` env var signals imminent termination
