# Research: Task Execution Audit Log with Tamper Detection

## Decision 1: Signing Mechanism

**Decision**: HMAC-SHA256 using a per-project secret key stored in `.anvil/audit-key`.

**Rationale**: HMAC-SHA256 is fast, well-supported in Go's stdlib (`crypto/hmac` + `crypto/sha256`), and sufficient for detecting accidental or deliberate modification. The key is auto-generated on first use (32 random bytes). This avoids external dependencies and complex key management.

**Alternatives considered**:
- Ed25519 digital signatures: Stronger (asymmetric), but overkill for local verification. Would require key pair management.
- SHA256 hash only (no key): Provides integrity but not authenticity — anyone with file access can recompute hashes.
- External signing service: Too complex for a CLI tool.

## Decision 2: Audit Log Storage Format

**Decision**: JSONL (JSON Lines) file at `.anvil/audit.jsonl`. One JSON object per line, append-only.

**Rationale**: JSONL is append-friendly (just write a new line), human-readable, machine-parseable, and the same format used by Claude session logs. Each entry contains a `prev_hash` field linking to the previous entry, forming a hash chain.

**Alternatives considered**:
- SQLite: More complex, requires CGo or pure-Go driver, not easily inspectable.
- Separate JSON files per entry: Too many files, harder to chain.
- Binary format: Not human-readable, harder to debug.

## Decision 3: Chain Integrity

**Decision**: Each audit entry includes `prev_hash` (SHA256 of the previous entry's JSON bytes). The first entry has `prev_hash: ""` (empty). Verification walks the chain forward, recomputing hashes.

**Rationale**: Simple blockchain-style chaining. Detects deletions (chain break), insertions (hash mismatch), and modifications (signature mismatch). No external service needed.

## Decision 4: Run Record Signing

**Decision**: Extend the existing `RunRecord` JSON with a `signature` field. The signature is computed over all other fields (excluding the signature field itself) serialized as canonical JSON.

**Rationale**: Minimal change to existing code. The `WriteRunRecord` function in `internal/project/project.go` already serializes and writes run records. Adding a signature field is a single-line extension.

## Decision 5: Actor Identification

**Decision**: Use `os/user.Current().Username` for CLI operations and `"daemon"` for daemon-initiated operations.

**Rationale**: Simple, requires no authentication system. Sufficient for local auditing.
