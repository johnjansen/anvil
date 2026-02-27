# Shared Feature Branches for Parallel Workers

When you run multiple anvil tasks concurrently against the same repo, you hit a coordination problem fast. Each worker needs its own sandbox to avoid corrupting the others' git state, but when those workers are building toward the same feature, you don't want 14 divergent branches — you want one clean PR.

This post walks through the worktree isolation pattern we developed for anvil workers, and how we extended it to support shared feature branches across related tasks.

## The problem

Anvil runs tasks on cron schedules. A spec writer generates a specification and breaks it into, say, 14 implementation tasks. An implementation worker picks up one task every 2 minutes. Each worker needs to:

1. Read and write files
2. Run tests
3. Commit and push

If two workers touch the same checkout simultaneously, you get corrupted index files, half-written commits, and merge conflicts that aren't real conflicts. The obvious fix is git worktrees — give each worker its own working directory backed by the same repository.

## Worktree isolation (the baseline)

The basic pattern: before doing any file operations, each worker creates a throwaway worktree branched from `origin/main`:

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
WORKTREE_NAME="impl-<task-slug>-$(date +%s)"
WORKTREE_BRANCH="worktree/$WORKTREE_NAME"
WORKTREE_PATH="$REPO_ROOT/.worktrees/$WORKTREE_NAME"

git worktree add -b "$WORKTREE_BRANCH" "$WORKTREE_PATH" origin/main
cd "$WORKTREE_PATH"
```

All file reads, writes, git operations, test runs — everything happens inside `$WORKTREE_PATH`. When the worker finishes, it pushes the branch and removes the worktree:

```bash
git push -u origin "$WORKTREE_BRANCH"
cd "$REPO_ROOT"
git worktree remove "$WORKTREE_PATH"
```

On failure, the worktree and its throwaway branch both get deleted:

```bash
cd "$REPO_ROOT"
git worktree remove --force "$WORKTREE_PATH" 2>/dev/null
git branch -D "$WORKTREE_BRANCH" 2>/dev/null
```

This works well for independent tasks. Each produces its own branch and (optionally) its own PR. No collisions.

## The divergence problem

But when a spec generates 14 related tasks — all part of the same feature — this baseline produces 14 independent branches. Each one starts from `origin/main` and re-implements or conflicts with the others. You end up with:

- 14 PRs instead of 1
- Later tasks duplicating or conflicting with earlier tasks' work
- No coherent history of the feature

What you actually want: all 14 tasks contribute commits to a single `feat/<spec-slug>` branch, producing one PR when the last task completes.

## Shared feature branches

The solution has three parts: labeling, deterministic branch names, and serialization.

### 1. Label related tasks

When a spec writer breaks a spec into implementation tasks, it tags every task issue with a label like `spec:my-feature-name`. This is the link that tells the implementation worker "these tasks belong together."

```bash
SPEC_SLUG="my-feature-name"
SPEC_LABEL="spec:$SPEC_SLUG"

# Create each task with the spec label
bd create "Implement X" -l "$SPEC_LABEL" -l search
bd create "Implement Y" -l "$SPEC_LABEL" -l search
bd create "Implement Z" -l "$SPEC_LABEL" -l search
```

### 2. Derive a deterministic branch name

When a worker picks up a task, it checks for a `spec:` label. If present, it derives the branch name deterministically:

```bash
SPEC_LABEL=$(echo "$ISSUE_LABELS" | grep -o 'spec:[^ ]*')
if [ -n "$SPEC_LABEL" ]; then
  SPEC_SLUG="${SPEC_LABEL#spec:}"
  FEATURE_BRANCH="feat/$SPEC_SLUG"
  MODE="feature"
else
  MODE="legacy"
fi
```

Every task from the same spec produces the same branch name. No timestamps, no task IDs in the branch — just `feat/my-feature-name`.

### 3. Branch from existing work

The worktree creation checks whether the feature branch already exists on the remote:

```bash
git fetch origin

if git rev-parse --verify "origin/$FEATURE_BRANCH" >/dev/null 2>&1; then
  # Feature branch exists — continue from previous tasks' work
  git worktree add "$WORKTREE_PATH" "origin/$FEATURE_BRANCH"
  cd "$WORKTREE_PATH"
  git checkout -B "$FEATURE_BRANCH" "origin/$FEATURE_BRANCH"
else
  # First task — create the feature branch from main
  git worktree add -b "$FEATURE_BRANCH" "$WORKTREE_PATH" origin/main
  cd "$WORKTREE_PATH"
fi
```

The first task creates `feat/my-feature-name` from `origin/main`. The second task sees that branch already exists and branches from it, inheriting all of the first task's commits. The third task inherits from the first two. And so on.

Each worker still gets its own worktree directory (unique on disk via timestamp), but they all push to the same branch.

### 4. Rebase before working

Before starting implementation, the worker rebases the feature branch onto main to pick up any changes that landed since the last task ran:

```bash
git pull --rebase origin main
```

If this produces conflicts, the worker aborts and marks the task as blocked rather than pushing a broken state:

```bash
git rebase --abort
# mark task as blocked, clean up worktree, exit
```

## Serialization

There's a subtle race condition: if two tasks from the same spec run simultaneously, they'll both check out the feature branch, both make changes, and the second one to push will hit a conflict.

The fix is a concurrency guard. Before claiming a spec-linked task, the worker checks if any sibling task is already in progress:

```bash
SPEC_LABEL=$(echo "$ISSUE_LABELS" | grep -o 'spec:[^ ]*')
if [ -n "$SPEC_LABEL" ]; then
  IN_PROGRESS=$(bd list --status open --label "$SPEC_LABEL" --label in-progress --json | jq 'length')
  if [ "$IN_PROGRESS" -gt 0 ]; then
    # Another task from this spec is running — back off
    exit 0
  fi
fi
```

Tasks within a spec run sequentially. Tasks across different specs (or standalone tasks) still run in parallel. The worker exits silently and the scheduler will retry on the next cron tick.

## Stale claim recovery

Serialization introduces a liveness risk: if a worker crashes while holding an `in-progress` claim, it blocks all subsequent tasks for that spec forever.

A separate triage task runs on a faster cadence and checks for stale claims:

```bash
# For each in-progress issue, check elapsed time
if [ "$ELAPSED_MINUTES" -gt 30 ]; then
  bd label remove <id> in-progress
  bd comments add <id> "Stale claim recovery: in-progress for ${ELAPSED_MINUTES}m with no updates."
fi
```

This ensures crashed workers don't permanently block a feature pipeline.

## Cleanup differences

The cleanup behavior differs between modes:

**Standalone tasks**: On failure, delete both the worktree and the throwaway branch. Nothing of value is lost.

**Feature branch tasks**: On failure, delete only the worktree. Never delete the feature branch — it contains commits from previously completed tasks. The next worker will pick up where it left off.

```bash
# Feature mode cleanup — preserve the branch
cd "$REPO_ROOT"
git worktree remove --force "$WORKTREE_PATH" 2>/dev/null
# Do NOT delete the feature branch

# Legacy mode cleanup — delete everything
cd "$REPO_ROOT"
git worktree remove --force "$WORKTREE_PATH" 2>/dev/null
git branch -D "$WORKTREE_BRANCH" 2>/dev/null
```

## PR creation on completion

After closing a spec-linked task, the worker checks if it was the last one:

```bash
REMAINING=$(bd list --status open --label "$SPEC_LABEL" --json | jq 'length')
if [ "$REMAINING" -eq 0 ]; then
  gh pr create --base main --head "$FEATURE_BRANCH" \
    --title "feat($SPEC_SLUG): implement $SPEC_SLUG" \
    --body "..."
fi
```

All tasks done, one branch, one PR.

## The full flow

1. Spec writer generates a spec and creates N task issues, all labeled `spec:my-feature`
2. Worker picks up task 1 (no sibling in progress). Creates `feat/my-feature` from `origin/main`. Implements, commits, pushes. Closes the task.
3. Worker picks up task 2 (no sibling in progress). Creates worktree from existing `feat/my-feature`. Rebases onto main. Implements on top of task 1's work. Pushes. Closes.
4. Repeat for tasks 3 through N.
5. Worker closes task N, sees all spec tasks are done, creates a PR.

If task 3 fails, its worktree is cleaned up but the branch survives. Task 3 gets marked as blocked. A human (or the stale-claim recovery) unblocks it, and the next worker picks it up with tasks 1 and 2's work intact.

## Key design decisions

**Why not merge instead of rebase?** Merge commits add noise to the feature branch history. Since these tasks run sequentially within a spec, rebase produces a clean linear history.

**Why serialize instead of handling merge conflicts?** Automated conflict resolution is unreliable. It's simpler and safer to run one task at a time and let each build cleanly on the last. The throughput cost is minimal — tasks within a spec have natural ordering anyway.

**Why label-based rather than a coordination database?** The issue tracker is already the source of truth for task state. Adding a separate coordination layer creates consistency problems. Labels are atomic, visible, and debuggable.

**Why 30 minutes for stale claim timeout?** It's long enough that a healthy worker won't be interrupted, and short enough that a crash doesn't block the pipeline for hours. Adjust to match your typical task duration.
