# Implementation Plan: Task Execution Audit Log with Tamper Detection

**Branch**: `009-audit-log` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)

## Summary

Add tamper-evident audit logging for task operations. Each task operation (create, modify, delete, run, pause, resume) is recorded in an append-only JSONL log with hash chaining and HMAC-SHA256 signatures. Run records are also individually signed. Users can view, filter, verify, and export the audit trail via CLI commands.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: crypto/hmac, crypto/sha256, encoding/json, encoding/hex (all stdlib)
**Storage**: Filesystem (JSONL audit log, signing key file)
**Project Type**: CLI tool with daemon
**Performance Goals**: < 100ms overhead per audit operation

## Constitution Check

Constitution is unconfigured (template only). No gates to evaluate.

## Project Structure

```text
internal/
  audit/
    audit.go          # NEW: AuditEntry, AuditLog, append/read/verify/export
    signing.go        # NEW: HMAC signing, key management
  project/
    project.go        # MODIFY: Add signature to RunRecord, sign on write
  daemon/
    daemon.go         # MODIFY: Emit audit entries for task lifecycle events
cmd/
  anvil/
    main.go           # MODIFY: Add audit command, verify-logs subcommand, --verify flag
```

## Key Design Decisions

### 1. New internal/audit/ package
Encapsulates all audit logic: entry creation, log management, chain verification, signing. Keeps project.go and daemon.go focused.

### 2. HMAC-SHA256 signing
Uses Go stdlib crypto/hmac + crypto/sha256. Key auto-generated (32 random bytes) on first use, stored at .anvil/audit-key with 0600 permissions.

### 3. JSONL append-only log
Single file at .anvil/audit.jsonl. Each line is a complete JSON entry with prev_hash linking to previous entry. Append-only by convention (verification detects tampering).

### 4. Run record signing
Existing RunRecord struct extended with Signature field. WriteRunRecord computes HMAC before serialization.

### 5. Best-effort logging
Audit log failures do not block task operations. Errors are logged to stderr.
