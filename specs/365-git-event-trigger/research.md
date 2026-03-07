# Research: Git Event Trigger for Tasks

## R1: Git Ref Polling vs Git Hooks

**Decision**: Poll-based detection using `git rev-parse` and `git for-each-ref`

**Rationale**: Polling is simpler, requires no git hook installation, works with any git workflow (local commits, remote pulls, force pushes), and aligns with the existing PollingManager pattern. Git hooks require modifying `.git/hooks/` which may conflict with user hooks or be lost on clone.

**Alternatives considered**:
- Git hooks (post-commit, post-merge, post-checkout): More immediate but invasive, fragile, and complex to manage lifecycle
- fsnotify on `.git/refs/`: Platform-dependent behavior, unreliable with packed refs
- `git log --since`: Requires timestamp tracking, timezone issues, less precise than SHA comparison

## R2: Ref Change Detection Method

**Decision**: Use `git rev-parse HEAD` for current branch and `git for-each-ref refs/heads/` for multi-branch watching

**Rationale**: `git rev-parse` is fast (<10ms), reliable, and available in all git versions. For multi-branch watching, `git for-each-ref` returns all branch HEAs in a single call, making it efficient for tasks that watch all branches.

**Alternatives considered**:
- `git log -1 --format=%H`: Slightly slower, same result
- Reading `.git/refs/heads/` directly: Breaks with packed refs
- `git ls-remote`: Would detect remote changes but adds network overhead; local ref checking is sufficient since the user must pull/push locally

## R3: State Persistence for Last-Seen Commits

**Decision**: Store last-seen refs as JSON in `.anvil/git-state/<task-id>.json`

**Rationale**: Follows the existing `.anvil/runs/` pattern of per-task state files. JSON is consistent with RunRecord storage. Persists across daemon restarts to prevent duplicate triggers.

**Format**:
```json
{
  "refs": {
    "refs/heads/main": "abc123...",
    "refs/heads/develop": "def456..."
  },
  "last_poll": "2026-03-07T12:00:00Z"
}
```

**Alternatives considered**:
- In-memory only: Loses state on restart, causes duplicate triggers
- SQLite: Overkill for simple key-value storage
- Embed in RunRecord: Conflates run history with polling state

## R4: Path Filtering Implementation

**Decision**: Use `git diff --name-only <old-sha> <new-sha>` to get changed files, then match against configured glob patterns using Go's `path/filepath.Match`

**Rationale**: `git diff --name-only` is fast and gives exactly the file list needed. Using the same glob matching as the existing FSWatcher keeps behavior consistent.

**Alternatives considered**:
- `git log --name-only`: More complex output to parse
- Full diff parsing: Unnecessary; only file names needed for filtering

## R5: Subscription Config Fields

**Decision**: Use existing `SubscriptionConfig` map pattern with these fields:
- `git_branch`: Branch name or pattern to watch (default: all branches)
- `git_path`: Glob pattern for path filtering (optional)
- `git_poll_interval`: Polling interval as duration string (default: "30s")
- `git_events`: Event types list, currently only `["push"]` (future: tag, merge)

**Rationale**: Follows the naming convention established by `fs_path`, `fs_events`, `fs_debounce`, `webhook_path`, `webhook_secret`. Prefixing with `git_` avoids field name collisions.

## R6: Environment Variables for Task Context

**Decision**: Pass git context via environment variables following the FSWatcher pattern:
- `ANVIL_GIT_EVENT`: Event type (e.g., "push")
- `ANVIL_GIT_BRANCH`: Branch name that changed
- `ANVIL_GIT_COMMIT`: New HEAD commit SHA
- `ANVIL_GIT_PREV_COMMIT`: Previous HEAD commit SHA (for diffing)
- `ANVIL_GIT_REPO`: Repository path

**Rationale**: Consistent with `ANVIL_FS_EVENT`, `ANVIL_FS_PATH`, `ANVIL_WEBHOOK_PAYLOAD` patterns. Environment variables are the established mechanism for passing trigger context to tasks.
