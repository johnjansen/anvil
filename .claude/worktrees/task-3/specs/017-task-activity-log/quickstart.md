# Quickstart: Task Activity Log

## Overview
Task activity logging records all lifecycle events for each task, providing a complete audit trail for debugging and compliance.

## Usage

### View Activity
```bash
anvil task activity my-task
```

### Filter by Type
```bash
anvil task activity my-task --type run
anvil task activity my-task --type edit
```

### Filter by Date
```bash
anvil task activity my-task --since 2026-01-01
```

### Export for Auditing
```bash
anvil task activity my-task --export audit.json
```

### JSON Output
```bash
anvil task activity my-task --json
```

## What Gets Tracked
All 7 activity types are tracked automatically:
- **created** — when a task is first added
- **run** — each task execution (with run ID, exit code, duration)
- **paused** — task disabled
- **resumed** — task re-enabled
- **edited** — configuration changes (with old/new values)
- **killed** — manual task termination
- **unlocked** — stale lock removal
- **force-run** — manual forced execution
