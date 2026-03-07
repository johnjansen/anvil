## Problem

Currently, task files are just markdown with YAML frontmatter. There's no version history or diff — if you accidentally modify a task or want to see what changed, you can't.

Example scenarios:
- "Why did this task start failing?" → Need to see what changed
- "Who changed the schedule?" → No audit trail
- "I accidentally overwrote my task" → Can't recover

## Proposed Solution

Add task versioning:

### 1. Auto-versioning

Task files are stored in git (if project is git-initialized), but also tracked internally:

```bash
# See task history
anvil task history --versions my-task

VERSION   DATE          AUTHOR    CHANGES
v3        2026-02-27   john     schedule: "*/30" -> "0 9 * *"
v2        2026-02-26   john     added retry: 3
v1        2026-02-25   john     initial version
```

### 2. Diff between versions

```bash
# See what changed
anvil task diff my-task v1 v3

--- v1
+++ v3
@@ -1,4 +1,4 @@
 schedule: "0 9 * * *"
-retry: 0
+retry: 3
```

### 3. Rollback

```bash
# Restore previous version
anvil task restore my-task v2
```

### 4. Git integration

If task is in git, show git blame for task file:

```bash
anvil task blame my-task
```

## Acceptance Criteria

- [ ] Task versions stored in `.anvil/versions/`
- [ ] `anvil task history --versions` shows version list
- [ ] `anvil task diff` shows changes between versions
- [ ] `anvil task restore` reverts to previous version
- [ ] Version metadata includes timestamp, author (from git or system user)
