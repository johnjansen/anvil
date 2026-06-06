# Quickstart & Validation: Idempotent Duplicate Flagging in pipeline-audit

**Feature**: 408-pipeline-audit-dup-idempotency
**Date**: 2026-06-07

This change is a prompt edit to one file. There is no build step. Validation is behavioral,
using `gh` against the repository.

## What changes

File: `.anvil/todos/p1/pipeline-audit.md` — the **Check 4: Duplicate Detection** section.

Before (current, simplified):

```text
If two open issues appear to cover the same feature:
- Print: ##anvil:status WARNING: Possible duplicates: #<a> and #<b>
- Comment on the newer issue: "Possible duplicate of #<older>. Flagged by pipeline-audit."
- Add duplicate label to the newer issue
```

After (idempotent — add the guard):

```text
If two open issues appear to cover the same feature, treat the higher-numbered issue as
the "newer" one and check whether it is ALREADY FLAGGED before doing anything:

  An issue is ALREADY FLAGGED if either of these is true:
    - it has the `duplicate` label, OR
    - any of its comments contains the text "Flagged by pipeline-audit".
  Determine this with:
    gh issue view <newer> --json labels,comments \
      --jq 'any(.labels[]; .name=="duplicate") or any(.comments[]; .body|contains("Flagged by pipeline-audit"))'
  If the command fails or the result is uncertain, treat the issue as already flagged
  (skip it) — never risk a duplicate comment.

- Always print: ##anvil:status WARNING: Possible duplicates: #<a> and #<b>
- If the newer issue is ALREADY FLAGGED: skip it — do NOT comment and do NOT re-label.
- Otherwise (not yet flagged):
    - Comment on the newer issue: "Possible duplicate of #<older>. Flagged by pipeline-audit."
    - Add the `duplicate` label to the newer issue.
- Track which issues you have flagged during this run and do not flag the same issue twice,
  even if it matches more than one older issue.
```

(The exact wording is finalized in implementation; the behavior must satisfy
`contracts/check4-duplicate-detection.md`.)

Also confirm the frontmatter `allowed_tools` permits the `gh` calls used. The current value
includes `Bash(gh:*)`, which already covers `gh issue view`, `gh issue comment`, and
`gh issue edit` — no change expected.

## How to verify (manual)

### 1. Already-flagged issues are recognized (AC1/AC2/AC7)

```bash
# Each of these should report "true" (already flagged) with the new predicate:
for n in 375 377 380 382 384; do
  echo -n "#$n already-flagged: "
  gh issue view "$n" --json labels,comments \
    --jq 'any(.labels[]; .name=="duplicate") or any(.comments[]; .body|contains("Flagged by pipeline-audit"))'
done
```

Expected: every issue prints `true`. A run of the audit after the change must add **no** new
duplicate comments to any of them.

### 2. Comment counts stay flat across runs (AC4/SC-002)

```bash
# Baseline count of pipeline-audit duplicate comments on a known issue:
gh issue view 384 --json comments \
  --jq '[.comments[] | select(.body|contains("Flagged by pipeline-audit"))] | length'
# Run the audit (or dry-run the Check 4 logic), then re-count. The number must not increase.
```

### 3. A genuinely new duplicate is still flagged once (AC3)

Use a scratch pair (or reason through the logic): an issue with neither the `duplicate` label
nor a marker comment must receive exactly one comment + the label on the first detection, and
zero additional comments on the next run.

### 4. Within-run single flagging (AC5)

If one issue matches two older issues in the same run, confirm the instructions flag it only
once (the in-run tracking clause).

## Done when

- `.anvil/todos/p1/pipeline-audit.md` Check 4 contains the already-flagged guard, the in-run
  tracking clause, and the fail-safe rule.
- The predicate command above returns `true` for #375/#377/#380/#382/#384.
- A subsequent audit run adds zero new duplicate comments to already-flagged issues.
- No other check in the task definition is altered; the audit remains reporting-only.
