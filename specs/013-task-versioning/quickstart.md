# Quickstart: Task Versioning

## Automatic Versioning

Task changes are automatically tracked. Every time you create or modify a task, a new version is created.

No configuration needed — versioning is automatic.

## View Version History

See all versions of a task:

```bash
anvil task history --versions my-task
```

Output:
```
VERSION   DATE                  AUTHOR    CHANGES
v3        2026-02-27 10:00     john      schedule: "*/30" -> "0 9 * *"
v2        2026-02-26 09:00     john      added retry: 3
v1        2026-02-25 08:00     john      initial version
```

## Compare Versions

See exactly what changed between two versions:

```bash
# Compare v1 and v3
anvil task diff my-task v1 v3

# Compare v1 to latest (shorthand)
anvil task diff my-task
```

Output:
```
--- v1  2026-02-25
+++ v3  2026-02-27
@@ -1,4 +1,4 @@
 schedule: "0 9 * * *"
-retry: 0
+retry: 3
```

## Restore a Version

Revert to a previous version:

```bash
anvil task restore my-task v2
```

This restores the task file to v2 and creates a new version (v4) recording the restore.

## Git Blame

If your project is in git, see who changed what:

```bash
anvil task blame my-task
```

Output:
```
abc1234 (John Jansen 2026-02-27 10:00:00) 1: schedule: "0 9 * * *"
abc1235 (Jane Smith 2026-02-26 09:00:00) 2: retry: 3
abc1236 (John Jansen 2026-02-25 08:00:00) 3:
```

## How It Works

Versions are stored in `.anvil/versions/<task-id>/`:

```
.anvil/versions/abc-123/
  versions.json    # Index of all versions
  v1.json         # First version
  v2.json         # Second version
  ...
```

Each version includes:
- Version number
- Timestamp (ISO 8601)
- Author (git user.name or system username)
- Full task file content
