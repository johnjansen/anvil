# Quickstart: Git Event Trigger

## Basic Usage

Create a task that triggers on git push to any branch:

```yaml
---
subscribe: git
git_events:
  - push
---
echo "New commit detected on branch $ANVIL_GIT_BRANCH"
echo "Commit: $ANVIL_GIT_COMMIT"
echo "Previous: $ANVIL_GIT_PREV_COMMIT"
```

## Branch-Filtered Trigger

Trigger only on pushes to `main`:

```yaml
---
subscribe: git
git_events:
  - push
git_branch: main
---
echo "New commit on main: $ANVIL_GIT_COMMIT"
./deploy.sh
```

## Path-Filtered Trigger

Trigger only when frontend files change:

```yaml
---
subscribe: git
git_events:
  - push
git_branch: main
git_path: "src/frontend/**"
---
echo "Frontend changed, running tests..."
cd src/frontend && npm test
```

## Custom Polling Interval

Check for changes every 10 seconds instead of the default 30:

```yaml
---
subscribe: git
git_events:
  - push
git_poll_interval: 10s
---
echo "Detected change quickly!"
```

## Environment Variables

When a git trigger fires, these environment variables are available to the task:

| Variable | Description | Example |
|----------|-------------|---------|
| `ANVIL_GIT_EVENT` | Event type | `push` |
| `ANVIL_GIT_BRANCH` | Branch name | `main` |
| `ANVIL_GIT_COMMIT` | New HEAD SHA | `abc123def456...` |
| `ANVIL_GIT_PREV_COMMIT` | Previous HEAD SHA | `789abc012def...` |
| `ANVIL_GIT_REPO` | Repository path | `/home/user/myproject` |

## Combining with Other Features

Git triggers work with other anvil features like retry and dependencies:

```yaml
---
subscribe: git
git_events:
  - push
git_branch: main
retry: 3
retry_backoff: exponential
---
./run-tests.sh && ./deploy.sh
```
