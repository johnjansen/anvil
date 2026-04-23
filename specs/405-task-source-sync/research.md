# Phase 0 Research: Task Source File Sync

## Question 1 — How does `anvil reload` signal the daemon?

**Decision**: Piggyback on the existing reload mechanism (no new IPC).

**Findings**:

- `cmd/anvil/status.go:132 reloadCmd` calls `daemon.SendReloadRequest()` (see `internal/daemon/daemon.go:4722`), which posts to an HTTP endpoint `/reload` served by the daemon.
- `daemon.handleReload` (`internal/daemon/daemon.go:2460`) pushes a value onto the daemon's `reload` channel.
- The daemon also traps `SIGHUP` (`daemon.go:401`, `daemon.go:569`) as a secondary trigger.
- Today, the reload loop reloads `~/.anvil/config.yaml` (max_workers, tick_interval, runners) but does NOT re-scan task files.

**Rationale**: Extending the existing reload loop to also walk registered tasks and re-import from recorded source paths is the minimum-surface change. No new IPC verbs, no new channels. The CLI-side `anvil reload` call remains unchanged.

**Alternatives considered**:

- Dedicated `anvil reload --tasks` subcommand or separate HTTP endpoint — rejected: splits the mental model further, and the issue explicitly says users expect a single `reload` to pick up task changes.
- Per-tick auto-reload (file watch via `fsnotify`) — rejected for MVP (explicitly out of scope in the spec) to avoid surprising I/O on large task libraries and to preserve the explicit-reload mental model.

## Question 2 — Where is frontmatter parsing implemented today?

**Decision**: Extend frontmatter handling in `internal/project/project.go` (inline in `LoadTodos`, near lines 473–548).

**Findings**:

- `internal/project/project.go:473` defines an inner struct for YAML unmarshalling with tags `yaml:"allowed_tools"` (line 487) and `yaml:"max_concurrent"` (line 485).
- `fmKeys map[string]interface{}` (line 473) is the untyped bag of every top-level frontmatter key, populated at line 548 via `yaml.Unmarshal([]byte(fm), &fmKeys)`.
- `applyDefaults` (line 736) uses `fmKeys` to decide whether to inherit project defaults — so the "key presence" detection happens here.
- Write-side: `.anvil/todos/*.md` files written at lines 965 and 1050 use the canonical `allowed_tools:` and `max_concurrent:` keys.

**Rationale**: Accept hyphenated aliases (`allowed-tools`, `max-concurrent`) by:

1. Before unmarshalling, normalize the raw frontmatter bytes: for the known aliases, substitute `allowed-tools:` → `allowed_tools:` and `max-concurrent:` → `max_concurrent:` on any line whose key is exactly one of those. This keeps the typed struct unchanged and keeps `fmKeys` consistent with the canonical form.
2. Keep the canonical form as the only on-disk output (what the daemon writes to `.anvil/todos/*.md`). Users who write either form get the same behavior.

**Alternatives considered**:

- Add `yaml:"allowed_tools" yaml:"allowed-tools"` dual tags — rejected: go-yaml does not support multi-value yaml tags on a single field, and a wrapper type adds complexity.
- Pre-process via a full YAML parse → re-serialize — rejected: slower and more fragile for comments/ordering.

## Question 3 — Behavior of `anvil add -f` on symlinks and `~` paths?

**Decision**: Resolve to absolute path at registration time using `filepath.Abs` + `os.Readlink`-aware handling (follow symlinks to the real file).

**Findings**:

- `cmd/anvil/task_create.go:177` shows `anvil add -f <file>` is parsed and passed through; no evidence of path normalization beyond what Go stdlib does when reading the file.
- Relative paths today work at registration time (because `os.ReadFile` resolves against cwd), but there's no recording of *which* path was used.

**Rationale**: Store the absolute, symlink-resolved path in the meta sidecar so that reload works regardless of the user's cwd at reload time. If the user intentionally registered a symlink, following it at registration means reload reads the target — this matches the user's expected behavior ("I pointed anvil at this file"). `~` is expanded by the shell before reaching our code, so no extra handling needed.

**Alternatives considered**:

- Store the exact string the user typed — rejected: makes reload position-dependent (different cwd → different file) and is a footgun.
- Store both raw and resolved paths — rejected as premature; single absolute path covers all documented scenarios.

## Question 4 — Precise list of frontmatter keys stripped or renamed during registration?

**Decision**: MVP handles the two reported in issue #405 (`allowed-tools`, `max-concurrent`). A secondary sweep during implementation will scan `internal/project/project.go` and add aliases for any other known-hyphen variants found during code review.

**Findings**:

- Canonical snake_case keys defined in `internal/project/project.go:37–48` include: `allowed_tools`, `max_concurrent`, `skip_permissions`, `pre_check`, `on_success`, `on_failure`, `retry_delay`, `retry_strategy`, `retry_jitter`, `retry_max_time`, `max_log_size`, `capture_output`, `checkpoint_grace_period`, `force_window`, `sla`, `priority_aging`, `on_sla_violation`, `circuit_breaker`, `on_circuit_open`, `on_circuit_close`, `notify_on_failure`, `notify_on_success`, `node_affinity`, `on_rollback`, `health_check`, `rate_limit`, `on_timeout_warning`, `on_timeout`, `adaptive_timeout`, `pinned_run`.
- Only two are explicitly called out in the issue as silently stripped/renamed: `allowed-tools` and `max-concurrent`. Users with other variants have not reported issues.

**Rationale**: Solve the reported breakage. For other potential hyphen variants, add a small helper (`normalizeFrontmatterAliases`) that can grow a curated alias map over time; start with two entries.

**Alternatives considered**:

- Blanket hyphen-to-underscore normalization on all YAML keys before parsing — rejected: too aggressive; could break user-defined keys in `env:` blocks or future extensions.

## Question 5 — Where do per-task sidecars live and what's the on-disk format?

**Decision**: JSON sidecar file next to each task's `.md`: `.anvil/todos/p<N>/<slug>.meta.json`.

**Format**:

```json
{
  "source_path": "/Users/alice/project/task.md",
  "source_hash_sha256": "abc123…",
  "last_loaded_at": "2026-04-23T12:34:56Z",
  "last_load_status": "ok"
}
```

**Rationale**:

- Sidecar file (not inside the `.md` frontmatter) because we don't want reload-metadata polluting what the user sees in `.anvil/todos/*.md` (which is already a normalized copy of their source).
- JSON not YAML to avoid conflating with user-authored YAML frontmatter and to keep a distinct parser/error surface.
- Per-task file, not a single catalog, because task-level writes are atomic (`tmp` + `rename`) and don't require a global lock.

**Alternatives considered**:

- Single `.anvil/todos/meta.json` catalog — rejected: every registration/reload becomes a global write; bad concurrency story.
- Embed `source_path` in the `.md` frontmatter as a reserved key — rejected: user-visible, easy to accidentally delete on edit, and it muddies the "source file is yours, `.md` copy is ours" boundary.

## Question 6 — Hash or mtime for drift detection?

**Decision**: SHA-256 of the raw source file bytes.

**Rationale**: mtime produces false positives from `git checkout`, `rsync`, editor-save patterns, and cross-filesystem copies. A hash is a few ms per task even on large libraries and is robust. Cost is negligible — reload is an explicit, user-invoked operation.

**Alternatives considered**:

- `mtime + size` — rejected per above.
- `crc32` — rejected: collision risk is higher and SHA-256 is already a stdlib dependency used elsewhere in anvil.
