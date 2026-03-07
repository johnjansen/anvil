# Research: Task Result Passing

## R1: Result Capture Mechanism

**Decision**: Extend the existing `statusWriter` pattern to scan for `##anvil:result` prefix lines, mirroring how `##anvil:checkpoint` already works.

**Rationale**: The `statusWriter` in `internal/runner/runner.go` already scans stdout for `##anvil:status` and `##anvil:checkpoint` prefixes. Adding a third prefix (`##anvil:result`) follows the established pattern exactly, requires minimal code, and reuses the line-scanning infrastructure.

**Alternatives considered**:
- Separate output capture pipe: More complex, no benefit over existing pattern
- Capture all stdout: Too noisy, no way to distinguish result data from logs
- File-based results: Requires additional cleanup, doesn't integrate with RunRecord

## R2: Result Storage

**Decision**: Add a `ResultData` field to the existing `RunRecord` struct. Results are stored as a raw JSON string in the run record file.

**Rationale**: RunRecords are already written per-run as JSON files in `.anvil/runs/<task-id>/`. Adding a field is backward compatible (empty string for old records). No schema migration needed — Go's JSON unmarshaler ignores unknown/missing fields.

**Alternatives considered**:
- Separate result files: Adds filesystem complexity, harder to correlate with runs
- Database storage: Over-engineered for a single-user CLI tool

## R3: Result Passing to Dependents

**Decision**: Pass results via `ANVIL_DEPENDENCY_RESULTS` environment variable as a JSON object keyed by dependency task name. Also support Go template variables in task body.

**Rationale**: Environment variables are the standard mechanism for passing data to child processes in anvil (see `ANVIL_CHECKPOINT_DATA`, `ANVIL_WEBHOOK_PAYLOAD`, `ANVIL_AMQP_PAYLOAD`). Template variables extend the existing frontmatter template system.

**Alternatives considered**:
- Stdin piping: Would require changes to all runners, breaks existing runner contracts
- Shared memory / temp files: More complex, platform-dependent

## R4: Result Size Limit

**Decision**: Cap result data at 1MB. If `##anvil:result` data exceeds 1MB, truncate and log a warning.

**Rationale**: Environment variables have OS-specific limits (typically 128KB-2MB). 1MB is generous for structured data while staying within safe limits. The result is meant for metadata/summary data, not bulk transfer.

**Alternatives considered**:
- No limit: Risk of hitting OS env var limits or OOM on large outputs
- Smaller limit (64KB): Too restrictive for some use cases

## R5: Template Variable Access

**Decision**: Use `.DependencyResults.<task-name>` in Go templates. Task names with hyphens are accessed via `index` function: `{{ index .DependencyResults "fetch-data" }}`.

**Rationale**: Go templates natively support map access. The dependency results map uses task names (sans `.md` extension) as keys. For names with special characters, Go's `index` function provides reliable access.

**Alternatives considered**:
- Custom template functions: Over-engineered for map access
- Dot notation only: Breaks on hyphenated task names (most task names use hyphens)

## R6: Cross-Project Results

**Decision**: Use existing `ResolveDependencyRunRecord()` which already handles cross-project dependencies via `project:task` syntax. Simply read the `ResultData` field from the resolved run record.

**Rationale**: The cross-project dependency resolution is already implemented. Result passing just reads an additional field from the same record.

**Alternatives considered**:
- Separate cross-project result API: Unnecessary, existing infrastructure handles it
