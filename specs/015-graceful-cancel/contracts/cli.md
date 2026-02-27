# CLI Contract: Graceful Cancel with Partial Result Capture

**Date**: 2026-02-27
**Feature**: 015-graceful-cancel

## Modified Commands

### `anvil task kill <name> [--graceful|-g] [--force]`

Kill a running task.

**Flags**:
- `--graceful` / `-g`: Send SIGTERM, run on_kill hook, wait grace period (30s default), then force kill
- `--force`: Immediately kill without hooks (current behavior)
- No flags: Same as `--force` (backward compatible)

**Output**:
```text
# Graceful
gracefully killing task: my-task (30s grace period)

# Force
killed task: my-task
```

---

### `anvil task partial <name>`

Display partial results from the most recent run.

**Output**:
```text
# With partial results
{"records_processed": 500, "last_id": 1234}

# Without partial results
no partial results found for task: my-task
```

---

### `anvil task run <name> [--force] [--resume]`

Run a task manually.

**New Flag**:
- `--resume`: Inject previous run's partial results as `ANVIL_PARTIAL_RESULTS` env var

**Output**:
```text
# With partial results from previous run
resuming task: my-task (with partial results from previous run)

# Without partial results
resuming task: my-task (no partial results from previous run)
```

## Task Frontmatter

### New Field: `on_kill`

```yaml
---
on_kill: "echo 'Saving state...' && cp /tmp/work.json /tmp/work-partial.json"
---
```

### Task Output Protocol

New magic prefix for partial results:
```text
##anvil:partial {"records_processed": 500, "last_id": 1234}
```

### Environment Variables

| Variable | Context | Description |
|----------|---------|-------------|
| `ANVIL_IS_KILLED` | on_kill hook | Set to "true" during graceful kill |
| `ANVIL_PARTIAL_RESULTS` | --resume run | JSON partial results from previous run |
