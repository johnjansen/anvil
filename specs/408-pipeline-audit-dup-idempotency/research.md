# Phase 0 Research: Idempotent Duplicate Flagging in pipeline-audit

**Feature**: 408-pipeline-audit-dup-idempotency
**Date**: 2026-06-07

The spec contained no `[NEEDS CLARIFICATION]` markers. Research below records the design
decisions that resolve the open implementation choices.

## Decision 1 — Signal(s) used to detect prior flagging

**Decision**: Treat an issue as already-flagged if **either** (a) it carries the `duplicate`
label, **or** (b) any of its comments contains the substring `Flagged by pipeline-audit`.

**Rationale**:
- The `duplicate` label is the cheapest signal — it is already returned when listing issues
  with `--json labels`, requiring no extra call in the common case.
- The comment-marker signal makes the guard robust to the label being absent (e.g., a human
  removed it, or a future label rename). It is the only signal that recognizes the existing
  legacy spam comments on #375/#377/#380/#382/#384 even if their labels were stripped.
- Combining with OR means the guard is conservative: any evidence of prior flagging
  suppresses re-flagging. This matches the spam-prevention goal.

**Alternatives considered**:
- *Label only*: simplest, but would re-spam any issue whose `duplicate` label was removed,
  and ignores the historical comments. Rejected — incomplete.
- *Comment only*: robust to label removal but pays a per-issue `gh issue view` even when the
  label already answers the question. Rejected as the sole signal; used as the second signal.
- *External state file (e.g., `.anvil/audit-state.json`)*: would track flagged pairs locally.
  Rejected — adds persisted state, drift risk, and a new failure mode; GitHub already stores
  the authoritative signal (labels/comments). YAGNI.

## Decision 2 — Marker phrase as the contract between write and detect

**Decision**: Standardize on the existing comment wording
`Possible duplicate of #<older>. Flagged by pipeline-audit.` and use the stable substring
`Flagged by pipeline-audit` for detection.

**Rationale**: The current task already emits this exact phrasing, so all historical comments
already contain the marker — no migration/backfill is needed and legacy-flagged issues are
recognized on the very next run. Keeping one canonical string for both writing and detecting
avoids drift.

**Alternatives considered**:
- *Hidden HTML-comment marker* (e.g., `<!-- pipeline-audit:duplicate -->`): cleaner for
  machine parsing, but historical comments lack it, so legacy spam would not be detected
  without a backfill. Rejected to keep the change minimal and backward-compatible. (Could be
  added later as an enhancement; not required.)

## Decision 3 — Failure / uncertainty handling

**Decision**: If labels or comments for a candidate cannot be reliably retrieved (API error,
rate limit, malformed output), skip flagging that issue for this run (do not comment).

**Rationale**: The cost of a missed first-time flag (it gets caught tomorrow) is far lower
than the cost of a duplicate comment (permanent spam, the exact bug being fixed). Fail safe
toward silence. (FR-007)

**Alternatives considered**:
- *Proceed and comment on error*: risks re-introducing the spam this feature removes.
  Rejected.

## Decision 4 — Within-run deduplication

**Decision**: Maintain an in-run set of issues already flagged/skipped; consult it before
acting on any later candidate pair so a single issue is never commented twice in one run.

**Rationale**: An issue can be the "newer" member of multiple candidate pairs (e.g., it
resembles two older issues). Without an in-run guard the same issue could get two comments in
one execution. (FR-006)

**Alternatives considered**:
- *Re-query GitHub after each comment*: works because the just-posted comment now contains the
  marker, but costs an extra round-trip per pair and is racier than a local set. The local set
  is simpler and sufficient.

## Decision 5 — Scope of the edit & tool permissions

**Decision**: Edit only Check 4's instructions (and the frontmatter `allowed_tools` if a
required `gh` subcommand is not already permitted) in `.anvil/todos/p1/pipeline-audit.md`.

**Rationale**: The bug is entirely within Check 4. Other checks and the reporting-only posture
must remain unchanged (FR-008). The frontmatter currently declares `Bash(gh:*)`, which already
covers `gh issue view`, `gh issue comment`, and `gh issue edit`; no permission change is
expected, but this will be verified during implementation (FR-009).

**Alternatives considered**:
- *Move duplicate detection into Go code*: larger change, contradicts the task's
  prompt-driven design, and out of scope for a p1 spam fix. Rejected.

## Open Questions

None. All decisions above are settled; the spec has no remaining clarifications.
