# Phase 1 Data Model: Task Source File Sync

## New Entity: `SourceMeta`

Sidecar metadata describing a registered task's relationship to its origin source file. Persisted per task.

**File location**: `.anvil/todos/p<N>/<slug>.meta.json` (alongside the existing `.anvil/todos/p<N>/<slug>.md`).

**Go struct** (new, in `internal/project/source_sync.go`):

```go
// SourceMeta records the relationship between a registered task and the
// source file it was imported from, so that `anvil reload` can re-read the
// source on demand and drift can be surfaced to the user.
type SourceMeta struct {
    SourcePath       string    `json:"source_path"`        // absolute path; empty if task has no source file
    SourceHashSHA256 string    `json:"source_hash_sha256"` // hex; hash of source bytes at last successful load
    LastLoadedAt     time.Time `json:"last_loaded_at"`     // RFC3339; when we last successfully imported
    LastLoadStatus   string    `json:"last_load_status"`   // "ok" | "missing" | "invalid"
    LastLoadError    string    `json:"last_load_error,omitempty"` // human-readable; populated when status != "ok"
}
```

### Field Rules

- `SourcePath` — MUST be absolute when non-empty. Set at registration time from `filepath.Abs(userInput)`. Symlinks followed via `filepath.EvalSymlinks` so reload reads the real file.
- `SourceHashSHA256` — 64-char hex string. Computed over raw file bytes (frontmatter + body, not post-normalization). Empty when no successful load has ever occurred.
- `LastLoadedAt` — UTC, RFC3339. Set every successful reload; NOT updated on failure.
- `LastLoadStatus` — enum:
  - `"ok"` — last load succeeded; `SourceHashSHA256` reflects the source at that time.
  - `"missing"` — source file not found or unreadable at last reload; running from cached `.md` content.
  - `"invalid"` — source file exists but frontmatter is unparseable at last reload; running from cached `.md` content.
- `LastLoadError` — populated with the error string when status is `missing` or `invalid`; omitted when `ok`.

### Invariants

1. A task MAY exist without a sidecar file — treated as `no-source` (backward compatibility with tasks registered before this feature).
2. A sidecar MAY exist with `SourcePath == ""` only if the task was explicitly created without a source (e.g. via `anvil task new`); this is a valid state meaning "this task does not have a source to sync against".
3. `SourceHashSHA256` non-empty ⟹ `LastLoadedAt` non-zero ⟹ at least one successful load has happened.
4. Sidecar writes are atomic: write to `<slug>.meta.json.tmp` then `os.Rename` over the destination. Readers that see a missing or malformed sidecar treat it as `no-source`.

## Derived Entity: `SyncStatus` (not persisted)

Computed at display time by comparing the current on-disk source file against the sidecar.

```go
type SyncStatus string

const (
    SyncStatusInSync   SyncStatus = "in-sync"   // source exists, hash matches sidecar
    SyncStatusDrift    SyncStatus = "drift"     // source exists, hash differs from sidecar (user edited but didn't reload)
    SyncStatusMissing  SyncStatus = "missing"   // sidecar has SourcePath but file not readable
    SyncStatusInvalid  SyncStatus = "invalid"   // source readable but frontmatter unparseable (carried from LastLoadStatus)
    SyncStatusNoSource SyncStatus = "no-source" // no sidecar, or sidecar with empty SourcePath
)
```

### Computation Rules

1. No sidecar or `SourcePath == ""` → `no-source`.
2. `SourcePath` set, file unreadable → `missing`.
3. `SourcePath` set, file readable, `LastLoadStatus == "invalid"` AND source hash still matches the failed attempt → `invalid`. (Prevents noisy "drift" flag on a source we already know we couldn't parse.)
4. `SourcePath` set, file readable, hash matches `SourceHashSHA256` → `in-sync`.
5. `SourcePath` set, file readable, hash differs → `drift`.

### Display

- `anvil task ls`: adds a compact marker column: `✓` in-sync, `~` drift, `!` missing, `?` invalid, blank for no-source. (Exact glyphs chosen in implementation; contract below pins the textual form for `--json` output.)
- `anvil status`: summary line `tasks: N total, X drift, Y missing, Z invalid`.
- `anvil task get <name>`: adds three fields — `Source:`, `Last loaded:`, `Status:`.

## Modification to existing Entity: `Todo` (in `internal/project/project.go`)

Existing `Todo` struct gains one optional field:

```go
type Todo struct {
    // ... existing fields ...
    SourceMeta *SourceMeta // nil if no sidecar exists (backward-compatible)
}
```

### Backward Compatibility

- Old tasks (no `.meta.json`): `SourceMeta` is nil. All new commands that inspect `SourceMeta` must handle nil as `no-source`. Existing code paths that iterate `Todo` fields are unaffected.
- Old `.anvil/todos/*.md` files: unchanged. The canonical `.md` remains the execution artifact; the sidecar is pure sync metadata.

## State Transitions

### On `anvil add -f <file>`

1. Read source bytes.
2. Write canonical `.anvil/todos/p<N>/<slug>.md` (existing code path).
3. Write `<slug>.meta.json`:
   - `SourcePath = filepath.EvalSymlinks(filepath.Abs(file))`
   - `SourceHashSHA256 = sha256(sourceBytes)`
   - `LastLoadedAt = time.Now().UTC()`
   - `LastLoadStatus = "ok"`
4. Atomic rename `.tmp` → `.meta.json`.

### On `anvil reload` (daemon side, new path inside `handleReload` / reload loop)

For each task with a sidecar that has non-empty `SourcePath`:

1. Stat the source file.
   - Not found → update `LastLoadStatus = "missing"`, `LastLoadError = err.String()`, keep `SourceHashSHA256` and `LastLoadedAt` untouched, write sidecar. Skip to next task.
2. Read source bytes. Compute new hash.
3. If `newHash == SourceHashSHA256` and `LastLoadStatus == "ok"` → no-op (fast path). Skip.
4. Parse frontmatter.
   - Parse error → update `LastLoadStatus = "invalid"`, `LastLoadError = err.String()`, leave `.md` unchanged, write sidecar. Skip.
5. Re-normalize frontmatter (hyphen aliases → canonical keys), apply priority-to-directory mapping (may require moving between `p<N>/` dirs if `priority:` changed).
6. Write new `.anvil/todos/p<N>/<slug>.md` atomically (tmp + rename). Preserves the task's UUID because the UUID is embedded in the file content from a prior registration (verify in Phase 2 code — if UUID is regenerated on each write, carry it forward explicitly).
7. Update sidecar: `SourceHashSHA256 = newHash`, `LastLoadedAt = now`, `LastLoadStatus = "ok"`, clear `LastLoadError`.
8. Emit a summary line to the reload response.

### On `anvil reload` (CLI side)

1. Send HTTP POST `/reload` as today (unchanged).
2. Read the summary response (daemon adds structured counts: `checked`, `reloaded`, `drift`, `missing`, `invalid`).
3. Print to stdout: `reloaded: N of M tasks updated; K drift, J missing, I invalid`.

### On `anvil task rm <name>`

Existing behavior removes `<slug>.md`; extend to also remove `<slug>.meta.json` if present. (No functional impact on sync — the task is gone.)
