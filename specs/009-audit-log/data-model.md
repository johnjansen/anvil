# Data Model: Task Execution Audit Log

## Entities

### AuditEntry

A single record in the append-only audit log.

| Field | Type | Description |
|-------|------|-------------|
| Timestamp | string (RFC3339) | When the operation occurred |
| Operation | string | Operation type: task.created, task.modified, task.deleted, task.run, task.completed, task.paused, task.resumed |
| Actor | string | Who performed it (username or "daemon") |
| TaskName | string | Name of the affected task |
| ProjectPath | string | Project the task belongs to |
| Details | map[string]any | Operation-specific data (e.g., old/new values for modifications, exit code for runs) |
| PrevHash | string | SHA256 hash of the previous entry's JSON bytes (empty for first entry) |
| Signature | string | HMAC-SHA256 signature of this entry (hex-encoded) |

### RunRecord (extended)

The existing RunRecord entity with an added signature field.

| Field | Type | Description |
|-------|------|-------------|
| Signature | string | HMAC-SHA256 signature of all other fields (hex-encoded) |

### SigningKey

| Field | Type | Description |
|-------|------|-------------|
| Key | []byte | 32 random bytes, auto-generated on first use |
| Location | string | Stored at .anvil/audit-key |

### Validation Rules

1. Timestamp must be valid RFC3339
2. Operation must be one of the defined operation types
3. Actor must be non-empty
4. PrevHash must match SHA256 of previous entry (or empty for first)
5. Signature must be valid HMAC-SHA256 of entry content
6. Audit log is append-only — entries cannot be modified or deleted

## Relationships

- AuditEntry references a task by TaskName and ProjectPath
- AuditEntry forms a chain via PrevHash (each entry links to previous)
- RunRecord.Signature is independent of AuditEntry (both are signed separately)
- SigningKey is shared between AuditEntry and RunRecord signing
