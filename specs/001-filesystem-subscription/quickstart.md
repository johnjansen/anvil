# Quickstart: Filesystem Subscription for Task Triggers

**Feature**: Filesystem Subscription for Task Triggers
**Date**: 2026-03-01

## Overview

Filesystem subscriptions allow tasks to automatically run when files matching a pattern are created, modified, or deleted in a watched directory.

## Configuration

Add a filesystem subscription to your task frontmatter:

```yaml
# tasks/process-data.yaml
name: process-data
schedule: ""  # Empty = not on cron, triggered only by subscription
subscription:
  type: fs
  path: ./data/input/*.json
  events: [create, modify]
run: ./scripts/process.sh
```

## Environment Variables

When triggered, your task has access to file event data via environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| ANVIL_FS_PATH | Path to the file that triggered the event | ./data/input/data.json |
| ANVIL_FS_EVENT | Type of event | create, modify, delete |
| ANVIL_FS_TIMESTAMP | Unix timestamp of event | 1709234567 |
| ANVIL_FS_PATTERN | Pattern that matched | ./data/input/*.json |

## Examples

### Process new JSON files

```yaml
name: process-json
subscription:
  type: fs
  path: ./data/incoming/*.json
  events: [create]
run: node process.js
```

### Watch multiple patterns

```yaml
name: watch-logs
subscription:
  type: fs
  path: ./logs/**/*.log
  events: [create, modify]
run: tail -f $ANVIL_FS_PATH
```

### Watch for file deletions

```yaml
name: cleanup-index
subscription:
  type: fs
  path: ./data/*.json
  events: [delete]
run: ./scripts/reindex.sh
```

## CLI Commands

```bash
# List all subscriptions including filesystem subscriptions
anvil subscription ls

# Pause a filesystem subscription
anvil subscription pause process-json

# Resume a filesystem subscription
anvil subscription resume process-json
```

## How It Works

1. When the daemon starts, it reads task frontmatter for subscription configs
2. For filesystem subscriptions, it starts watching the specified directory
3. When a matching file event occurs, the watcher triggers the associated task
4. The task runs with ANVIL_FS_* environment variables populated
