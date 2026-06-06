# Feature Specification: Idempotent Duplicate Flagging in pipeline-audit

**Feature Branch**: `408-pipeline-audit-dup-idempotency`
**Created**: 2026-06-07
**Status**: Draft
**Input**: GitHub issue #408 — "pipeline-audit re-posts duplicate-flag comments every run (no idempotency)"

## Overview

The `pipeline-audit` task runs daily and, in its duplicate-detection step (Check 4),
flags pairs of open issues that appear to cover the same work. For each detected pair
it posts a "Possible duplicate" comment on the newer issue and applies the `duplicate`
label. The label application is naturally idempotent, but the comment is not: the task
has no memory of having already flagged a pair, so it re-posts the same comment on every
daily run. Issues that have been duplicates for weeks have accumulated 20+ identical
audit comments each, burying real discussion and contradicting the audit's own rule to
"avoid issue spam."

This feature makes duplicate flagging idempotent: each duplicate pair is flagged **at
most once**, and already-flagged issues receive **no further** audit comments.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Already-flagged duplicates are left alone (Priority: P1)

A maintainer watches a repository where several issues were previously flagged as
duplicates by `pipeline-audit`. They want the daily audit to stop adding redundant
"Possible duplicate" comments to issues that have already been flagged, so issue threads
stay readable.

**Why this priority**: This is the core defect. It directly stops the ongoing comment
spam (≈5 redundant comments/day across the known duplicate set) and is the minimum change
that resolves issue #408.

**Independent Test**: Take an issue that already carries the `duplicate` label and a prior
"Flagged by pipeline-audit" comment. Run the audit. Confirm the audit adds zero new
comments and makes no label changes to that issue.

**Acceptance Scenarios**:

1. **Given** an open issue that already has the `duplicate` label, **When** the audit's
   duplicate-detection step evaluates a pair containing that issue, **Then** no new comment
   is posted and no label change is made to that issue.
2. **Given** an open issue that already has a prior "Flagged by pipeline-audit" duplicate
   comment but (for any reason) no longer has the `duplicate` label, **When** the audit
   evaluates a pair containing that issue, **Then** no new comment is posted.
3. **Given** a duplicate pair that the audit flags in one run, **When** the audit runs
   again the next day with no changes to the pair, **Then** the issue's count of
   pipeline-audit duplicate comments does not increase.

---

### User Story 2 - Genuinely new duplicates are still flagged once (Priority: P1)

A maintainer relies on the audit to surface newly created duplicate issues. They need the
idempotency change to suppress only *repeat* flagging, not first-time flagging.

**Why this priority**: The fix must not regress the feature's purpose. Suppressing all
flagging would "fix" the spam by removing the capability, which is unacceptable.

**Independent Test**: Create two open issues with near-identical titles, neither carrying
the `duplicate` label nor any prior audit duplicate comment. Run the audit. Confirm the
newer issue receives exactly one "Possible duplicate" comment and the `duplicate` label.

**Acceptance Scenarios**:

1. **Given** a previously-unflagged duplicate pair, **When** the audit detects it, **Then**
   the newer issue receives exactly one "Possible duplicate of #&lt;older&gt;" comment and
   the `duplicate` label.
2. **Given** the same newly-flagged pair, **When** the audit runs the following day, **Then**
   no additional comment is posted (the pair is now in the already-flagged state from
   User Story 1).

---

### User Story 3 - An issue appearing in multiple duplicate pairs is flagged once (Priority: P2)

The audit may detect the same issue as the "newer" member of more than one candidate pair
within a single run. The issue should be flagged at most once.

**Why this priority**: Prevents within-run double-commenting; less common than the daily
re-run case but still produces visible spam when it occurs.

**Independent Test**: Construct three open issues with similar titles such that the newest
matches both older ones. Run the audit once. Confirm the newest issue receives at most one
duplicate comment.

**Acceptance Scenarios**:

1. **Given** an issue that matches two older issues in the same run, **When** the audit
   processes both candidate pairs, **Then** the issue is commented on at most once and
   labeled at most once.

---

### Edge Cases

- **Label present, no prior comment**: An issue manually labeled `duplicate` (not by the
  audit) is treated as already-flagged — the audit does not add a comment. (The label is an
  authoritative "already handled" signal.)
- **Comment present, label manually removed**: Treated as already-flagged — no new comment.
  The audit does not re-apply a label a human deliberately removed.
- **Neither signal present**: Treated as a new duplicate — flagged exactly once.
- **Existing legacy comments**: The detection must recognize the historical comment wording
  already present on issues #375, #377, #380, #382, #384 so those issues are immediately
  treated as already-flagged and stop accruing comments on the next run.
- **API/rate-limit interruption mid-run**: If the audit cannot determine an issue's current
  labels or comments, it MUST err toward *not* commenting (skip) rather than risk a
  duplicate comment.
- **Daily-flag budget interaction**: The audit's existing "at most 2 new issues per run"
  rule is unaffected; this feature governs duplicate comments/labels, not issue creation.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The duplicate-detection step MUST determine, before commenting or labeling,
  whether the candidate (newer) issue has already been flagged as a duplicate.
- **FR-002**: An issue MUST be considered already-flagged if it carries the `duplicate`
  label **or** has at least one existing comment containing the stable pipeline-audit
  duplicate marker phrase.
- **FR-003**: For an already-flagged issue, the step MUST NOT post a new comment and MUST
  NOT make redundant label changes.
- **FR-004**: For a not-yet-flagged duplicate pair, the step MUST post exactly one
  "Possible duplicate of #&lt;older&gt;" comment on the newer issue and apply the
  `duplicate` label.
- **FR-005**: The duplicate comment MUST contain a stable, recognizable marker phrase
  ("Flagged by pipeline-audit") so future runs can detect prior flagging. The marker MUST
  match the wording already present on historical audit comments so legacy-flagged issues
  are recognized.
- **FR-006**: Within a single run, an issue that appears as the newer member of multiple
  candidate pairs MUST be flagged at most once.
- **FR-007**: If the issue's label or comment state cannot be reliably determined, the step
  MUST skip flagging that issue (no comment) rather than risk a redundant comment.
- **FR-008**: The change MUST be confined to the natural-language instructions of Check 4
  in the `pipeline-audit` task definition; it MUST NOT alter the task's other checks or its
  reporting-only posture (the audit still does not fix code).
- **FR-009**: The task's `allowed_tools` MUST permit every command the revised Check 4
  instructions require (e.g., reading an issue's labels and comments via `gh`).

### Key Entities

- **Duplicate candidate pair**: Two open issues judged to cover the same work — an older
  issue and a newer issue. Only the newer issue is flagged.
- **Flag state**: A derived property of an issue indicating whether it has already been
  marked as a duplicate, determined from its `duplicate` label and its comments containing
  the marker phrase.
- **Duplicate marker phrase**: The stable substring ("Flagged by pipeline-audit") embedded
  in each duplicate comment, used both to write and to detect prior flagging.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An already-flagged duplicate issue receives **zero** new pipeline-audit
  duplicate comments on every subsequent run (down from one per run).
- **SC-002**: Across consecutive daily runs with no change to the issue set, the total count
  of pipeline-audit duplicate comments on the repository does **not increase**.
- **SC-003**: A genuinely new duplicate pair is still flagged exactly once — the newer issue
  gains exactly one duplicate comment and the `duplicate` label on the first run that
  detects it.
- **SC-004**: The five known affected issues (#375, #377, #380, #382, #384) accrue no
  further audit duplicate comments after the change ships.
- **SC-005**: A single audit run never posts more than one duplicate comment to the same
  issue, regardless of how many candidate pairs include it.

## Assumptions

- The historical duplicate comments share the wording "Flagged by pipeline-audit", making a
  substring match a reliable detection signal for legacy comments. (Confirmed by the comment
  template in the current task definition.)
- The `gh` CLI is available to the task (it is already in `allowed_tools`) and can list an
  issue's labels and comments.
- The `duplicate` label is treated as authoritative for "already handled"; the audit will
  not contradict a human who applied or removed it.
- Re-printing the ephemeral `##anvil:status WARNING: Possible duplicates` line to the run log
  is not considered spam (it is not persisted to the issue); suppressing the persistent
  comment is what matters. The instructions may still surface already-flagged pairs in the
  run output for visibility without commenting.
