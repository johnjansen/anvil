# CLI Contract: Task Output Diffing

## Command: anvil task diff

### Output Diff Mode (1 arg: task name only, or with --run1/--run2)

```
anvil task diff <task-name> [flags]
```

**Flags**:
- `--run1 <id>` - First run ID (or prefix) to compare
- `--run2 <id>` - Second run ID (or prefix) to compare
- `--context N` - Number of context lines (default: 3)
- `--ignore-whitespace` - Ignore whitespace differences
- `--json` - Output as JSON

**Output (text mode)**:
```
--- Run abc12345 (2026-02-27 10:00:00) SUCCESS
+++ Run def67890 (2026-02-27 11:00:00) FAILED
@@ -1,3 +1,3 @@
-Record count: 100
+Record count: 0
 Processing complete
-Items: [a, b, c]
+Items: []
```

**Error cases**:
- Task not found: exit 1, "Error: task '<name>' not found"
- No runs: exit 1, "Error: no runs found for task '<name>'"
- Only one run: exit 1, "Error: need at least 2 runs to diff (found 1)"
- Invalid run ID: exit 1, "Error: run '<id>' not found"
- Identical outputs: exit 0, "Outputs are identical"

### Version Diff Mode (existing, 2+ args)

```
anvil task diff <task-name> <v1> [v2]
```

Existing behavior preserved.
