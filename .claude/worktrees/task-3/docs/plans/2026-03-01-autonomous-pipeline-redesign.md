# Autonomous Pipeline Redesign

Close the review loop, enforce priority ordering, and add worktree isolation to the anvil autonomous engineering pipeline.

## Problem

backend-engineer pushes directly to main with no review gate. pr-review exists but only reviews external PRs. Review feedback is never acted upon. speckit-planning creates work faster than it gets built, and there's no priority ordering between fixing existing PRs and starting new features.

## Design Decisions

- **Approach A (minimal patch)**: modify existing tasks in-place, add one new task (pr-merge). No role separation — the single-agent-per-run model with phase ordering within the prompt *is* the priority system.
- **Auto-merge with exceptions**: pr-merge auto-merges approved PRs. If pr-review requests changes, backend-engineer fixes them in Phase 0 before starting new work. Only `needs-human` escalations require a person.
- **Worktrees**: backend-engineer creates a git worktree per implementation, works on a feature branch, creates a PR. Clean isolation.
- **Planning gate**: speckit-planning only runs when the build queue (ready+backend issues) is empty.

## Label State Machine

```
[new issue]
    |
    v (triage-issues, every 10min)
[p0-p3 + area + type]
    |
    +-- [needs-planning] --(speckit-planning, gated)--> [ready]
    |
    +-- [ready] (trivial fix)
            |
            v (backend-engineer, every 5min)
        [in-progress]
            |
            v (backend-engineer creates PR)
        [in-review]
            |
            v (pr-review, every 10min)
            |
            +-- approved --(pr-merge, every 10min)--> [closed]
            |
            +-- changes-requested --(backend-engineer Phase 0)--> [in-review]

[stale-label-cleanup]: in-progress OR in-review (no activity 2hr) --> ready
```

## Changes

### 1. backend-engineer.md — Major Rewrite

New phase ordering: fix PRs > build features > exit.

**Phase 0: Fix PRs with requested changes**
- Check: `gh pr list --state open --author @me --json number,reviewDecision --jq '.[] | select(.reviewDecision == "CHANGES_REQUESTED")'`
- If found: checkout that PR's branch, read review comments via `gh pr view <number> --comments`, fix issues, push, re-request review via `gh pr review <number> --comment --body "Addressed feedback, re-requesting review"`, done for this run

**Phase 1: Issue Selection** (unchanged logic)
- Find highest priority oldest ready+backend issue
- Add in-progress label, remove ready

**Phase 2: Implementation** (worktree-based)
- Determine branch name: `fix/<number>-<slug>` for bugs, `feat/<number>-<slug>` for features
- Create worktree: `git worktree add .anvil/worktrees/<branch> -b <branch>`
- Work entirely in worktree directory
- Run `/speckit.implement` if tasks.md exists, otherwise implement directly
- Run `go build ./...` and `go test ./...`

**Phase 3: Delivery** (PR instead of push-to-main)
- Commit with conventional message: `fix #<number>: <desc>` or `feat #<number>: <desc>`
- Push branch: `git push -u origin <branch>`
- Create PR: `gh pr create --title "<type> #<number>: <desc>" --body "Closes #<number>"`
- Remove worktree: `git worktree remove .anvil/worktrees/<branch>`
- Move issue label: remove in-progress, add in-review
- Do NOT close the issue, tag, or version bump

### 2. pr-review.md — Promote and Broaden

- Move from `p3/` to `p2/`
- Schedule: `*/15` to `*/10`
- Review ALL open non-draft PRs including internal ones
- Keep "never merge" constraint

### 3. pr-merge.md — New Task

```yaml
id: "pr-merge"
schedule: "*/10 * * * *"
max_concurrent: 1
resume: false
skip_permissions: true
pre_check: "gh pr list --state open --json reviewDecision --jq '[.[] | select(.reviewDecision == \"APPROVED\")] | length > 0' | grep -q true"
allowed_tools:
  - Read
  - "Bash(gh:*)"
  - "Bash(git:*)"
```

Behavior:
1. Find approved PRs, process oldest first (max 3 per run)
2. Verify CI passing: `gh pr checks <number>` — skip if failing
3. Verify mergeable: `gh pr view <number> --json mergeable` — add needs-human if CONFLICTING
4. Squash merge: `gh pr merge <number> --squash --delete-branch`
5. The PR body's `Closes #<number>` auto-closes the linked issue

### 4. speckit-planning.md — Gate Behind Build Queue

New pre_check:
```bash
gh issue list --state open --label ready --label backend --json number --limit 1 | grep -qv '"number"' && gh issue list --state open --label needs-planning --json number --limit 1 | grep -q '"number"'
```

Only plans when: zero ready+backend issues AND at least one needs-planning issue.

### 5. stale-label-cleanup.md — Monitor in-review

Add `in-review` to the set of labels checked for staleness. Same 2-hour inactivity threshold. If stale, remove in-review and add ready so the issue gets re-queued.

## File Changes Summary

| File | Action |
|------|--------|
| `.anvil/todos/p2/backend-engineer.md` | Major rewrite |
| `.anvil/todos/p3/pr-review.md` | Move to `p2/pr-review.md` |
| `.anvil/todos/p2/pr-merge.md` | New file |
| `.anvil/todos/p2/speckit-planning.md` | Update pre_check |
| `.anvil/todos/p1/stale-label-cleanup.md` | Add in-review monitoring |

## What Doesn't Change

- Triage pipeline (p1/triage-issues.md)
- Release pipeline (verify-build, update-skill, generate-changelog, build-release, tag-release)
- Task scheduling model (cron + pre_check + max_concurrent)
- Run record system
- Speckit skill invocations (specify, plan, tasks, implement)
