# CLI Contract: Task Diff and Versioning

**Date**: 2026-02-27
**Feature**: 014-task-versioning

## Commands

### `anvil task history --versions <name>`

Display version history for a task file.

**Arguments**:
- `<name>` (required): Task name

**Flags**:
- `--versions`: Show file version history instead of run history
- `--json`: Output in JSON format

**Output (human-readable)**:
```text
VERSION  DATE                 AUTHOR        SUMMARY
v3       2026-02-27 10:30:00  johnjansen    modified schedule
v2       2026-02-26 14:00:00  johnjansen    added retry settings
v1       2026-02-25 09:00:00  johnjansen    initial version
```

**Output (JSON)**:
```json
[
  {"version_number": 3, "timestamp": "2026-02-27T10:30:00Z", "author": "johnjansen", "summary": "modified schedule"},
  {"version_number": 2, "timestamp": "2026-02-26T14:00:00Z", "author": "johnjansen", "summary": "added retry settings"},
  {"version_number": 1, "timestamp": "2026-02-25T09:00:00Z", "author": "johnjansen", "summary": "initial version"}
]
```

**Errors**:
- Task not found: `error: task not found: <name>` (exit 1)
- No versions: `no versions found for task: <name>` (exit 0)

---

### `anvil task diff <name> <v1> [v2]`

Show unified diff between two versions of a task.

**Arguments**:
- `<name>` (required): Task name
- `<v1>` (required): First version (e.g., `v1` or `1`)
- `[v2]` (optional): Second version; defaults to current file content

**Output**:
```text
--- v1  2026-02-25 09:00:00
+++ v2  2026-02-26 14:00:00
@@ -1,3 +1,4 @@
 ---
-schedule: "*/10 * * * *"
+schedule: "*/5 * * * *"
+retries: 3
 ---
```

**Errors**:
- Task not found: `error: task not found: <name>` (exit 1)
- Version not found: `error: version not found: v99` (exit 1)

---

### `anvil task restore <name> <version>`

Restore a task to a previous version.

**Arguments**:
- `<name>` (required): Task name
- `<version>` (required): Version to restore (e.g., `v1` or `1`)

**Output**:
```text
restored my-task to v1 (created v4)
```

**Errors**:
- Task not found: `error: task not found: <name>` (exit 1)
- Version not found: `error: version not found: v99` (exit 1)
- Already at version: `task is already at v2` (exit 0)
- No changes: `no changes: current content matches v1` (exit 0)

---

### `anvil task blame <name>`

Show git blame output for a task file.

**Arguments**:
- `<name>` (required): Task name

**Output**: Raw `git blame` output for the task file.

**Errors**:
- Task not found: `error: task not found: <name>` (exit 1)
- Not a git repo: `git blame not available: project is not in a git repository` (exit 1)
