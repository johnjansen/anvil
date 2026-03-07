# Contract: Git Trigger Frontmatter Schema

## Subscription Type

```
subscribe: git
```

## Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `git_events` | list of string | Yes | - | Event types to watch. Values: `push` |
| `git_branch` | string | No | (all branches) | Branch name to filter. Exact match only. |
| `git_path` | string | No | (all paths) | Glob pattern to filter by changed file paths |
| `git_poll_interval` | duration string | No | `30s` | How often to check for ref changes |

## Environment Variables Provided

| Variable | Type | Always Set | Description |
|----------|------|------------|-------------|
| `ANVIL_GIT_EVENT` | string | Yes | The event type that fired (e.g., `push`) |
| `ANVIL_GIT_BRANCH` | string | Yes | The branch where the change was detected |
| `ANVIL_GIT_COMMIT` | string | Yes | The new HEAD commit SHA (full 40-char) |
| `ANVIL_GIT_PREV_COMMIT` | string | Yes | The previous HEAD commit SHA (full 40-char). Empty string on first detection after baseline. |
| `ANVIL_GIT_REPO` | string | Yes | Absolute path to the repository root |

## State File Format

Persisted at `.anvil/git-state/<task-id>.json`:

```json
{
  "refs": {
    "refs/heads/main": "abc123...",
    "refs/heads/develop": "def456..."
  },
  "last_poll": "2026-03-07T12:00:00Z"
}
```

## Behavior Rules

1. On first daemon start with no state file: record current refs as baseline, do NOT trigger
2. On subsequent polls: compare current refs to stored refs; trigger if any watched ref changed
3. Branch filter: if `git_branch` is set, only trigger for changes on that exact branch
4. Path filter: if `git_path` is set, run `git diff --name-only <old> <new>` and only trigger if any changed file matches the glob
5. Multiple commits pushed at once: trigger exactly once with the latest SHA
6. Force push: detected as ref change, triggers normally
7. Git errors (unreachable, permissions): log warning, skip cycle, retry next interval
