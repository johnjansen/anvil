# Feature Specification: Task Execution Audit Log with Tamper Detection

**Feature Branch**: `009-audit-log`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Add task execution audit log with tamper detection"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Verify Task Run Integrity (Priority: P1)

As a project owner, I want to verify that task run records have not been tampered with so that I can trust the execution history for compliance and debugging purposes.

**Why this priority**: This is the core security promise of the feature. Without integrity verification, all other audit capabilities have no trustworthiness. Signed run records are the foundation everything else builds on.

**Independent Test**: Can be fully tested by running a task, then running the verify command and confirming the signature is valid. Tampering with the run record file and re-running verify should show the tampering was detected.

**Acceptance Scenarios**:

1. **Given** a task has completed execution, **When** the user runs the history command with verification, **Then** each run record shows a valid signature status.
2. **Given** a run record file has been manually edited, **When** the user runs verification, **Then** the modified record is flagged as tampered.
3. **Given** a run record has been deleted from the chain, **When** the user runs verification, **Then** the missing record is detected via chain break.

---

### User Story 2 - View Task Operation History (Priority: P1)

As a project administrator, I want to see a chronological log of all operations performed on tasks (created, modified, run, paused, deleted) so that I can understand what happened and when for debugging and accountability.

**Why this priority**: The audit trail is the primary user-facing value — users need to see what happened to their tasks over time. This is equally critical to verification because without the log, there is nothing to verify.

**Independent Test**: Can be tested by creating a task, modifying it, running it, pausing it, and then viewing the audit log to confirm all operations are recorded with timestamps and details.

**Acceptance Scenarios**:

1. **Given** multiple operations have been performed on tasks, **When** the user runs the audit command, **Then** all operations are listed chronologically with timestamp, operation type, and details.
2. **Given** the user wants to see operations for a specific task, **When** they filter by task name, **Then** only operations for that task are shown.
3. **Given** the user wants to see recent operations, **When** they filter by date, **Then** only operations since that date are shown.
4. **Given** a task's configuration was changed, **When** the user views the audit log with diff mode, **Then** the specific changes (old value vs new value) are shown.

---

### User Story 3 - Export Audit Trail for Compliance (Priority: P2)

As a compliance officer, I want to export the full audit trail as a signed, machine-readable file so that I can provide verifiable evidence of task execution for audits and regulatory requirements.

**Why this priority**: Compliance export is important but less frequently used than day-to-day viewing and verification. It builds on the audit log (US2) and signing (US1) capabilities.

**Independent Test**: Can be tested by exporting the audit trail to a file and verifying the export contains all records with valid signatures in a standard format.

**Acceptance Scenarios**:

1. **Given** an audit trail exists, **When** the user exports it, **Then** a file is created containing all audit entries in a machine-readable format.
2. **Given** the export includes signatures, **When** a third party receives the file, **Then** they can independently verify each entry's integrity.
3. **Given** an export has been created, **When** any entry in the export file is modified, **Then** re-verification detects the tampering.

---

### Edge Cases

- What happens when the signing key is lost or rotated? Old records remain verifiable with the old key; new records use the new key. Key rotation is logged in the audit trail itself.
- What happens when the audit log file is deleted entirely? The system detects the absence and reports it. A fresh audit log is started, and the gap is noted.
- What happens when disk space is exhausted? The audit log write fails gracefully with an error message; the task operation itself is not blocked by audit log failures.
- What happens when two daemon instances write to the audit log simultaneously? Each daemon instance has its own audit log; entries include a daemon identifier for disambiguation.
- What happens when a task has no run history? The verify command reports no records found rather than an error.
- What happens when the audit log grows very large? The audit command supports date-based filtering to limit output. Log rotation is handled through standard file management.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST generate a cryptographic signature for each run record upon task completion.
- **FR-002**: System MUST store signatures alongside run records in a verifiable format.
- **FR-003**: System MUST support a verification command that validates all run record signatures for a given task.
- **FR-004**: System MUST detect modifications to signed run records and report them as tampered.
- **FR-005**: System MUST maintain an append-only audit log recording all task operations (create, modify, delete, run, pause, resume).
- **FR-006**: Each audit log entry MUST include: timestamp, operation type, actor (user or daemon), and a description of changes.
- **FR-007**: Each audit log entry MUST include a hash of the previous entry to form a verifiable chain.
- **FR-008**: System MUST support viewing the audit log with filtering by task name and date range.
- **FR-009**: System MUST support showing change diffs in the audit log (old value vs new value for modifications).
- **FR-010**: System MUST support exporting the audit trail to a machine-readable file format with embedded signatures.
- **FR-011**: System MUST generate a signing key automatically on first use and store it securely in the project configuration.
- **FR-012**: System MUST display verification status (valid/tampered/missing) for each run record in the history view.
- **FR-013**: System MUST detect chain breaks (deleted entries) in the audit log during verification.
- **FR-014**: Audit log failures MUST NOT block task execution — logging is best-effort with error reporting.

### Key Entities

- **AuditEntry**: A single record in the audit log. Contains timestamp, operation type, actor, task name, change details, previous entry hash, and signature.
- **SigningKey**: The cryptographic key used to sign audit entries and run records. Stored in project configuration, auto-generated on first use.
- **RunRecord (extended)**: The existing run record entity, extended with a cryptographic signature field.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of task run records include a cryptographic signature that can be independently verified.
- **SC-002**: Users can view the complete audit trail for any task within normal command response time (under 2 seconds for up to 10,000 entries).
- **SC-003**: Verification detects 100% of modifications to signed records (no false negatives).
- **SC-004**: Audit log export produces a file that can be verified by a third party without access to the running system.
- **SC-005**: Audit log operations add less than 100 milliseconds of overhead to task lifecycle operations.

## Assumptions

- HMAC-based signing is sufficient for the initial implementation. The signing key is stored locally in the project's .anvil directory. This protects against accidental modification but not against an attacker with full system access (who could also access the signing key). Full PKI-based signing with external key storage is deferred to a future iteration.
- The audit log is stored as a single append-only file per project in the .anvil directory. No external database or service is required.
- The "actor" field uses the system username from the operating system. No user authentication system is assumed.
- The audit log records task-level operations only (create, modify, delete, run, pause, resume). It does not record daemon-level operations (start, stop, config changes).
- Export format is JSON with one entry per line (JSONL), which is both human-readable and machine-parseable.
