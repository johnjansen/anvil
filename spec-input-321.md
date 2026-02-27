## Problem

Users often need to compare task outputs between runs to identify changes:
- Did the output change from yesterday?
- What changed between successful and failed runs?
- How do outputs differ between similar tasks?

Currently there's no way to diff task outputs.

## Proposed Solution

Add output diffing:

### 1. Compare runs

```bash
# Compare last two runs
anvil task diff my-task

# Compare specific runs
anvil task diff my-task --run1 abc123 --run2 def456

# Compare across tasks
anvil task diff my-task fetch-data
```

### 2. Output format

```
--- Run abc123 (2026-02-27 10:00) SUCCESS
+++ Run def456 (2026-02-27 11:00) FAILED
@@ -1,5 +1,5 @@
-Record count: 100
+Record count: 0
 Processing complete
-Items: [a, b, c]
+Items: []
```

### 3. Options

```bash
--context N     # show N lines of context
--ignore-whitespace
--json         # output diff as JSON
```

## Acceptance Criteria

- [ ] `anvil task diff <name>` compares last two runs
- [ ] `--run1` and `--run2` compare specific runs
- [ ] Unified diff format with context
- [ ] `--json` for programmatic access