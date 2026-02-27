# CLI Contract: Audit Log

## New Commands

### `anvil audit`

Show task operation audit trail.

```
anvil audit [flags]

Flags:
  --task <name>       Filter by task name
  --since <date>      Show entries since date (YYYY-MM-DD or RFC3339)
  --show-diff         Show change details for modifications
  --export <file>     Export audit trail to file (JSONL format)
  --verify            Verify chain integrity of the audit log
  --json              Output in JSON format
```

**Default output (human-readable)**:
```
TIMESTAMP              OPERATION        ACTOR     TASK             CHANGES
2026-02-27 10:00:00    task.created     johnj     fetch-data       schedule: "0 9 * * *"
2026-02-27 10:05:00    task.modified    johnj     fetch-data       priority: 0 (was 1)
2026-02-27 10:30:00    task.completed   daemon    fetch-data       exit_code: 0
2026-02-27 10:35:00    task.paused      johnj     fetch-data
```

**Verify output**:
```
Audit log: 42 entries
Chain integrity: VALID
All signatures: VALID
```

Or on failure:
```
Audit log: 42 entries
Chain integrity: BROKEN at entry #17 (expected hash abc123, got def456)
Signature errors: 1 entry with invalid signature (entry #17)
```

### `anvil task history <name> --verify`

Extended existing history command with signature verification.

```
RUN ID       SIGNATURE    STATUS     STARTED              ELAPSED
abc12345     valid        success    2026-02-27 10:00     5m30s
def67890     TAMPERED     failed     2026-02-27 09:00     2m15s
```

### `anvil task verify-logs <name>`

Verify log integrity for a specific task.

```
anvil task verify-logs <name> [flags]

Flags:
  --verbose    Show detailed verification for each run record
```

**Output**:
```
Task: fetch-data
Run records: 15
Verified: 15/15 valid
```

## Run Record Extension

The existing run record JSON is extended with a `signature` field:

```json
{
  "run_id": "abc123",
  "task_id": "task-uuid",
  "session_id": "session-uuid",
  "started": "2026-02-27T10:00:00Z",
  "finished": "2026-02-27T10:05:00Z",
  "success": true,
  "signature": "hmac-sha256:a1b2c3d4..."
}
```

## Audit Log Entry Format (JSONL)

```json
{"timestamp":"2026-02-27T10:00:00Z","operation":"task.created","actor":"johnj","task":"fetch-data","project":"/path/to/project","details":{"schedule":"0 9 * * *"},"prev_hash":"","signature":"hmac-sha256:..."}
```

## Signing Key Storage

- Location: `.anvil/audit-key`
- Format: 32 bytes, hex-encoded
- Permissions: 0600 (owner read/write only)
- Auto-generated on first audit operation
- Should be added to .gitignore (sensitive)
