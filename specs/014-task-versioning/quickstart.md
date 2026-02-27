# Quickstart: Task Diff and Versioning

**Date**: 2026-02-27
**Feature**: 014-task-versioning

## Getting Started

Once the daemon is running and watching a project, all task file changes are automatically versioned.

### View Version History

```bash
# See all versions of a task
anvil task history --versions my-task

# JSON output
anvil task history --versions my-task --json
```

### Compare Versions

```bash
# Diff between two specific versions
anvil task diff my-task v1 v3

# Diff a version against the current file
anvil task diff my-task v2
```

### Restore a Previous Version

```bash
# Restore to v1 (creates a new version v4)
anvil task restore my-task v1
```

### Git Blame

```bash
# Show line-by-line git blame for a task file
anvil task blame my-task
```
