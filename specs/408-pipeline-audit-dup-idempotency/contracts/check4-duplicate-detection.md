# Contract: Revised Check 4 — Duplicate Detection (idempotent)

**Feature**: 408-pipeline-audit-dup-idempotency
**Artifact under contract**: Check 4 section of `.anvil/todos/p1/pipeline-audit.md`
**Type**: Behavioral contract for an LLM-executed task prompt (not a code API)

This contract defines the required externally-observable behavior of the duplicate-detection
step after the change. Implementation is natural-language instructions executed via `gh`.

## Inputs

- The set of open issues in `johnjansen/anvil`, each with `number`, `title`, and `labels`
  (obtainable from `gh issue list --json number,title,labels`).
- On demand, an issue's comments (`gh issue view <number> --json comments` or `--comments`).

## Definitions

- **marker phrase** = the exact substring `Flagged by pipeline-audit`.
- **already-flagged(issue)** = the issue's labels contain `duplicate` OR any of its comments
  contains the marker phrase. If the issue's labels/comments cannot be retrieved, the result
  is treated as already-flagged (skip / fail safe).
- **newer / older** of a candidate pair = higher / lower issue number respectively.

## Required behavior

### B1 — Detect candidates (unchanged)
The step MUST compare titles of open issues and identify candidate duplicate pairs using the
same similarity heuristic as today (same key nouns / similar phrasing).

### B2 — Guard before acting
For each candidate pair, BEFORE posting any comment or changing any label, the step MUST
evaluate `already-flagged(newer)`.

### B3 — Skip already-flagged
If `already-flagged(newer)` is true, the step MUST NOT post a comment and MUST NOT change the
issue's labels. It MAY print a status line noting the pair was already flagged.

### B4 — Flag new duplicates exactly once
If `already-flagged(newer)` is false, the step MUST:
- post exactly one comment on `newer` whose body is
  `Possible duplicate of #<older>. Flagged by pipeline-audit.` (containing the marker phrase), and
- apply the `duplicate` label to `newer`.

### B5 — Within-run single-flag
If the same issue is the `newer` member of more than one candidate pair in a single run, the
step MUST act on it at most once (subsequent pairs treat it as already-flagged).

### B6 — Fail safe
If label/comment state for a candidate cannot be reliably determined, the step MUST skip
flagging (no comment) for that candidate in this run.

### B7 — No scope creep
The step MUST NOT modify code, MUST NOT alter other checks, and MUST keep the audit
reporting-only. Existing run-log status output (e.g.,
`##anvil:status WARNING: Possible duplicates: #<a> and #<b>`) MAY be retained.

## Observable acceptance checks

| ID  | Precondition | Action | Expected result |
|-----|--------------|--------|-----------------|
| AC1 | `newer` has `duplicate` label | run audit | 0 new comments, no label change on `newer` |
| AC2 | `newer` has a marker comment, no `duplicate` label | run audit | 0 new comments on `newer` |
| AC3 | `newer` has neither signal | run audit | exactly 1 marker comment + `duplicate` label added |
| AC4 | pair flagged in run N | run audit in run N+1 (no changes) | comment count on `newer` unchanged |
| AC5 | `newer` matches two older issues in one run | run audit once | `newer` commented at most once |
| AC6 | `gh` read for `newer` fails | run audit | no comment posted to `newer` |
| AC7 | known issues #375/#377/#380/#382/#384 | run audit | no new audit duplicate comments |

## Non-goals (explicitly out of contract)

- Removing or editing the 100+ historical duplicate comments already posted (cleanup is a
  separate decision; this change only stops new ones).
- Changing the title-similarity heuristic itself.
- Adding machine-readable hidden markers to comments (possible future enhancement).
