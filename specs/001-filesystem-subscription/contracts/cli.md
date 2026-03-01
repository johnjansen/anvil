# Contracts: Filesystem Subscription CLI

**Feature**: Filesystem Subscription for Task Triggers
**Date**: 2026-03-01

## CLI Contracts

This is a CLI tool - no external API contracts. The following documents the user-facing CLI interface.

### Subscription List Command

```bash
anvil subscription ls [--json]
```

Output format (human-readable):

```
NAME           TYPE    PATTERN              EVENTS         STATUS
process-json   fs      ./data/*.json        [create]       active
watch-logs     fs      ./logs/**/*.log      [create,mod]   paused
```

Output format (JSON):

```json
{
  "subscriptions": [
    {
      "name": "process-json",
      "type": "fs",
      "pattern": "./data/*.json",
      "events": ["create"],
      "status": "active"
    }
  ]
}
```

### Subscription Pause Command

```bash
anvil subscription pause <task-name>
```

### Subscription Resume Command

```bash
anvil subscription resume <task-name>
```

## Task Frontmatter Contract

```yaml
subscription:
  type: fs                              # Required: "fs"
  path: ./data/*.json                   # Required: glob pattern
  events: [create, modify, delete]      # Optional: defaults to [create, modify]
```

## Environment Variables Contract

When a task is triggered by a filesystem event, these environment variables are set:

| Variable | Type | Description |
|----------|------|-------------|
| ANVIL_FS_PATH | string | Absolute path to the file |
| ANVIL_FS_EVENT | string | Event type: "create", "modify", "delete" |
| ANVIL_FS_TIMESTAMP | int64 | Unix timestamp |
| ANVIL_FS_PATTERN | string | The pattern that matched |
