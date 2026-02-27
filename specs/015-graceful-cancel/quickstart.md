# Quickstart: Graceful Cancel with Partial Result Capture

**Date**: 2026-02-27
**Feature**: 015-graceful-cancel

## Graceful Kill

```bash
# Gracefully stop a task (SIGTERM + hook + grace period)
anvil task kill my-task --graceful

# Force kill (immediate, no hooks)
anvil task kill my-task --force
```

## Task with On-Kill Hook

```yaml
---
schedule: "*/5 * * * *"
on_kill: "echo 'Saving state' && cp /tmp/work.json /tmp/work-partial.json"
---
Process records from database
```

## Emitting Partial Results

Tasks can emit progress markers:
```bash
echo '##anvil:partial {"records": 500, "last_id": 1234}'
```

## Viewing Partial Results

```bash
anvil task partial my-task
```

## Resuming from Partial Results

```bash
anvil task run my-task --resume
# Task receives ANVIL_PARTIAL_RESULTS env var
```
