# Autonomous Pipeline Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Close the review loop so the autonomous engineer's code gets reviewed before merging, enforce priority ordering (fix PRs > build features > plan new ones), and add worktree isolation.

**Architecture:** Modify 4 existing anvil task definitions and add 1 new task. No Go code changes — these are markdown prompt files with YAML frontmatter. The label state machine gains one new state (`in-review`) and the pipeline gains one new agent (`pr-merge`).

**Tech Stack:** Anvil task YAML frontmatter, markdown prompts, `gh` CLI, `git worktree`

---

### Task 1: Rewrite backend-engineer.md

**Files:**
- Modify: `.anvil/todos/p2/backend-engineer.md`

**Step 1: Read the current file**

Read `.anvil/todos/p2/backend-engineer.md` to confirm current contents match what we expect.

**Step 2: Replace the file contents**

Replace the entire file with:

```markdown
---
id: "backend-engineer"
schedule: "*/5 * * * *"
max_concurrent: 1
resume: false
skip_permissions: true
pre_check: "(gh pr list --state open --author @me --json number,reviewDecision --jq '.[] | select(.reviewDecision == \"CHANGES_REQUESTED\")' | grep -q 'number') || (gh issue list --state open --label ready --label backend --limit 1 --json number | grep -q '\"number\"')"
allowed_tools:
  - Read
  - Write
  - Edit
  - Glob
  - Grep
  - "Bash(git:*)"
  - "Bash(go:*)"
  - "Bash(gh:*)"
  - "Bash(ls:*)"
  - "Bash(mkdir:*)"
  - Skill
---
You are a backend engineer for johnjansen/anvil (a Go CLI tool). Your priorities in order: fix PRs with review feedback, then implement ready issues. Never start new work if existing PRs need fixes.

## Phase 0: Fix PRs With Requested Changes (ALWAYS CHECK FIRST)

1. Check for PRs with requested changes:
   gh pr list --state open --author @me --json number,title,headRefName,reviewDecision --jq '.[] | select(.reviewDecision == "CHANGES_REQUESTED")'
   Print: ##anvil:status Checking for PRs needing fixes...

2. If any PR has CHANGES_REQUESTED:
   a) Pick the oldest one
   b) Read the review comments: gh pr view <number> --comments
   c) Checkout the branch: git checkout <headRefName>
   d) Read the relevant source files to understand the review feedback
   e) Fix the issues raised in the review
   f) Run: go build ./... && go test ./...
   g) Commit: git commit -m 'fix: address review feedback on #<number>'
   h) Push: git push
   i) Re-request review: gh pr review <number> --comment --body "Addressed review feedback. Ready for re-review."
   j) Print: ##anvil:status Fixed review feedback on PR #<number>
   k) EXIT — do not continue to Phase 1

3. If no PRs need fixes, continue to Phase 1.

## Phase 1: Issue Selection

4. Find the next issue to work on (highest priority, oldest first):
   gh issue list --state open --label ready --label backend --json number,title,body,labels,createdAt --search "sort:created-asc" --limit 5
   Priority order: p0 > p1 > p2 > p3. Within same priority, oldest first.
   Print: ##anvil:status Finding next backend issue...

5. If no ready backend issues exist, exit quietly.

6. Pick the top issue. Read the full body and check for planning artifacts:
   ls specs/ to find any matching spec directory for this issue number.
   If specs exist, read spec.md, plan.md, and tasks.md for context.

7. Add 'in-progress' label and remove 'ready':
   gh issue edit <number> --add-label in-progress --remove-label ready
   Print: ##anvil:status Working on #<number>: <title>

## Phase 2: Implementation (Worktree)

8. Determine branch name from issue labels:
   - If labels include 'bug': fix/<number>-<short-slug>
   - Otherwise: feat/<number>-<short-slug>
   Where <short-slug> is the issue title lowercased, spaces replaced with dashes, truncated to 30 chars.

9. Create a worktree for isolated work:
   git worktree add .anvil/worktrees/<branch> -b <branch>
   cd .anvil/worktrees/<branch>

10. If speckit planning artifacts exist (tasks.md), run /speckit.implement to execute the tasks.
    If no planning artifacts exist (trivial bug fix), implement directly:
    - Read the relevant source files before modifying
    - Follow existing code patterns and conventions
    - Keep changes minimal and focused

11. Verify the build:
    go build ./...
    go test ./...

## Phase 3: Delivery (Create PR)

12. Stage and commit:
    git add <changed files>
    git commit -m '<type>(#<number>): <description>'
    Where type is 'fix' for bugs, 'feat' for features/enhancements.

13. Push and create PR:
    git push -u origin <branch>
    gh pr create --title "<type>(#<number>): <description>" --body "Closes #<number>"
    Print: ##anvil:status Created PR for #<number>

14. Clean up worktree:
    cd /path/to/main/repo
    git worktree remove .anvil/worktrees/<branch>

15. Update issue labels:
    gh issue edit <number> --remove-label in-progress --add-label in-review

## Rules

- Only work on ONE issue per run
- ALWAYS check Phase 0 first — fixing PRs takes absolute priority
- Never pick an issue already labeled 'in-progress' or 'in-review'
- Use speckit for ALL features and non-trivial changes
- Trivial fixes (1-2 line changes) may skip speckit
- If stuck or unclear, add 'needs-human' label and comment asking for clarification
- Do NOT close issues — pr-merge handles that via the PR
- Do NOT create version tags — the release pipeline handles that
```

**Step 3: Verify the file is valid**

Read the file back to confirm YAML frontmatter parses correctly (has opening and closing `---`).

**Step 4: Commit**

```bash
git add .anvil/todos/p2/backend-engineer.md
git commit -m "refactor: rewrite backend-engineer for PR-based workflow with worktrees"
```

---

### Task 2: Move and update pr-review.md

**Files:**
- Delete: `.anvil/todos/p3/pr-review.md`
- Create: `.anvil/todos/p2/pr-review.md`

**Step 1: Read the current file**

Read `.anvil/todos/p3/pr-review.md` to confirm current contents.

**Step 2: Create the new file at p2/**

Write `.anvil/todos/p2/pr-review.md` with updated schedule and scope:

```markdown
---
id: "pr-review"
schedule: "*/10 * * * *"
max_concurrent: 1
resume: false
skip_permissions: true
pre_check: "gh pr list --state open --json number --limit 1 | grep -q '\"number\"'"
allowed_tools:
  - Read
  - Glob
  - Grep
  - "Bash(gh:*)"
  - "Bash(git:*)"
  - "Bash(go:*)"
---
Review open pull requests in johnjansen/anvil. Review ALL non-draft PRs — including those created by automated agents.

1. Find PRs needing review:
   gh pr list --state open --json number,title,author,reviewRequests,reviewDecision,headRefName,isDraft
   Filter to non-draft PRs only.
   Skip PRs you've already approved (unless new commits since approval).
   Skip PRs with reviewDecision == "CHANGES_REQUESTED" that have no new commits since the review.

2. If no PRs need review, exit quietly.

3. For each PR (max 3 per run):
   a) Get the diff: gh pr diff <number>
   b) Read the PR description and any linked issue
   c) Check CI status: gh pr checks <number>

   d) Review against these priorities (in order):
      - Correctness: Does the code do what it claims?
      - Safety: Any security issues, command injection, path traversal?
      - Data integrity: Any race conditions, data loss scenarios?
      - Architecture: Does it follow existing Go patterns in the codebase?
      - Error handling: Are errors properly wrapped and returned?
      - Tests: Are there tests? Do they cover edge cases?
      - Build: Does it compile? (go build ./...)

   e) Submit review:
      - If issues found: gh pr review <number> --request-changes --body "<feedback>"
      - If minor suggestions only: gh pr review <number> --comment --body "<suggestions>"
      - If looks good: gh pr review <number> --approve --body "LGTM"

   f) Use comment prefixes: [blocking], [suggestion], [nit], [question]

4. Never merge PRs — review only. The pr-merge task handles merging.
5. Exit quietly when done.
```

**Step 3: Delete the old file**

```bash
git rm .anvil/todos/p3/pr-review.md
```

**Step 4: Verify**

Confirm `.anvil/todos/p2/pr-review.md` exists and `.anvil/todos/p3/pr-review.md` is gone.

**Step 5: Commit**

```bash
git add .anvil/todos/p2/pr-review.md
git commit -m "refactor: promote pr-review to p2, review all PRs including internal"
```

---

### Task 3: Create pr-merge.md

**Files:**
- Create: `.anvil/todos/p2/pr-merge.md`

**Step 1: Write the new task file**

```markdown
---
id: "pr-merge"
schedule: "*/10 * * * *"
max_concurrent: 1
resume: false
skip_permissions: true
pre_check: "gh pr list --state open --json reviewDecision --jq '[.[] | select(.reviewDecision == \"APPROVED\")] | length' | grep -qv '^0$'"
allowed_tools:
  - Read
  - "Bash(gh:*)"
  - "Bash(git:*)"
---
You merge approved pull requests in johnjansen/anvil. You only merge — you never review or implement.

1. Find approved PRs:
   gh pr list --state open --json number,title,headRefName,reviewDecision,createdAt --jq '[.[] | select(.reviewDecision == "APPROVED")] | sort_by(.createdAt)'
   Print: ##anvil:status Checking for approved PRs to merge...

2. If no approved PRs exist, exit quietly.

3. For each approved PR (max 3 per run, oldest first):

   a) Check CI status:
      gh pr checks <number>
      If any required check is failing, skip this PR.
      If already commented about CI failure, do not comment again.
      If not yet commented: gh pr comment <number> --body "Skipping merge — CI checks are failing. Will retry when they pass."
      Continue to next PR.

   b) Check mergeability:
      gh pr view <number> --json mergeable --jq '.mergeable'
      If CONFLICTING:
        - Find the linked issue number from the PR body (look for "Closes #N")
        - gh issue edit <issue> --add-label needs-human --remove-label in-review
        - gh pr comment <number> --body "Merge conflict detected. Flagging for human resolution."
        - Continue to next PR.

   c) Squash merge:
      gh pr merge <number> --squash --delete-branch
      Print: ##anvil:status Merged PR #<number>

4. Exit quietly when done.

## Rules

- Only squash merge — keeps history clean
- Always delete the branch after merge
- Never force merge past failing CI
- The PR body's "Closes #N" will auto-close the linked issue on merge
- Do NOT create version tags — the release pipeline handles that
```

**Step 2: Verify the file**

Read it back to confirm YAML frontmatter is valid.

**Step 3: Commit**

```bash
git add .anvil/todos/p2/pr-merge.md
git commit -m "feat: add pr-merge task to auto-merge approved PRs"
```

---

### Task 4: Gate speckit-planning.md

**Files:**
- Modify: `.anvil/todos/p2/speckit-planning.md`

**Step 1: Read the current file**

Read `.anvil/todos/p2/speckit-planning.md` to confirm the current pre_check line.

**Step 2: Replace the pre_check line**

Change the `pre_check` from:

```yaml
pre_check: "gh issue list --state open --label needs-planning --json number --limit 1 | grep -q '\"number\"'"
```

To:

```yaml
pre_check: "! gh issue list --state open --label ready --label backend --json number --limit 1 | grep -q '\"number\"' && gh issue list --state open --label needs-planning --json number --limit 1 | grep -q '\"number\"'"
```

This means: only run if there are NO ready+backend issues AND there IS a needs-planning issue.

**Step 3: Verify**

Read the file back. Confirm the pre_check is the only change.

**Step 4: Commit**

```bash
git add .anvil/todos/p2/speckit-planning.md
git commit -m "feat: gate speckit-planning behind empty build queue"
```

---

### Task 5: Update stale-label-cleanup.md

**Files:**
- Modify: `.anvil/todos/p1/stale-label-cleanup.md`

**Step 1: Read the current file**

Read `.anvil/todos/p1/stale-label-cleanup.md`.

**Step 2: Update pre_check to include in-review**

Change:

```yaml
pre_check: "gh issue list --label in-progress --state open --json number | grep -q '\"number\"'"
```

To:

```yaml
pre_check: "(gh issue list --label in-progress --state open --json number | grep -q '\"number\"') || (gh issue list --label in-review --state open --json number | grep -q '\"number\"')"
```

**Step 3: Update the prompt body**

In the prompt text, update step 2 to also list in-review issues:

Change:

```
2. List all open issues with the in-progress label:
   gh issue list --label in-progress --state open --json number,title,updatedAt
```

To:

```
2. List all open issues with in-progress or in-review labels:
   gh issue list --label in-progress --state open --json number,title,updatedAt
   gh issue list --label in-review --state open --json number,title,updatedAt
```

And update step 5 to handle in-review labels:

Change:

```
5. If NO anvil task is currently working on that issue AND no recent activity:
   - Remove the 'in-progress' label
   - Add the 'ready' label (so it gets picked up again)
```

To:

```
5. If NO anvil task is currently working on that issue AND no recent activity:
   - Remove the 'in-progress' or 'in-review' label (whichever is present)
   - Add the 'ready' label (so it gets picked up again)
```

**Step 4: Verify**

Read the file back. Confirm only the pre_check and the label references changed.

**Step 5: Commit**

```bash
git add .anvil/todos/p1/stale-label-cleanup.md
git commit -m "feat: monitor in-review label for staleness alongside in-progress"
```

---

### Task 6: Verify the full pipeline

**Step 1: Check all task files exist and have valid YAML**

```bash
ls -la .anvil/todos/p0/ .anvil/todos/p1/ .anvil/todos/p2/ .anvil/todos/p3/
```

Confirm:
- `p0/tag-release.md` exists
- `p1/triage-issues.md` exists
- `p1/stale-label-cleanup.md` exists
- `p1/build-release.md` exists
- `p2/backend-engineer.md` exists (rewritten)
- `p2/pr-review.md` exists (moved from p3)
- `p2/pr-merge.md` exists (new)
- `p2/speckit-planning.md` exists (updated)
- `p2/verify-build.md` exists
- `p2/update-skill.md` exists
- `p2/generate-changelog.md` exists
- `p3/` should be empty (pr-review moved out)

**Step 2: Verify no duplicate task IDs**

```bash
grep -r '^id:' .anvil/todos/ | sort
```

All IDs should be unique.

**Step 3: Verify the .anvil/worktrees directory exists**

```bash
mkdir -p .anvil/worktrees
```

**Step 4: Commit the design doc**

```bash
git add docs/plans/2026-03-01-autonomous-pipeline-redesign.md
git add docs/plans/2026-03-01-autonomous-pipeline-implementation.md
git commit -m "docs: add autonomous pipeline redesign design and implementation plan"
```
