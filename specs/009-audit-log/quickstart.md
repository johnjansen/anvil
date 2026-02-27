# Quickstart: Audit Log

## View Audit Trail

```bash
# Show all task operations
anvil audit

# Filter by task
anvil audit --task fetch-data

# Filter by date
anvil audit --since 2026-01-01

# Show change details
anvil audit --task fetch-data --show-diff
```

## Verify Integrity

```bash
# Verify audit log chain integrity
anvil audit --verify

# Verify run records for a specific task
anvil task verify-logs my-task

# Verify with detailed output
anvil task verify-logs my-task --verbose

# Show history with signature status
anvil task history my-task --verify
```

## Export for Compliance

```bash
# Export full audit trail
anvil audit --export audit.json

# Export filtered by task
anvil audit --task fetch-data --export fetch-data-audit.json
```
