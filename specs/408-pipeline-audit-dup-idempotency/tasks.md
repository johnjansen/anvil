---
description: "Task list for Idempotent Duplicate Flagging in pipeline-audit"
---

# Tasks: Idempotent Duplicate Flagging in pipeline-audit

**Input**: Design documents from `/specs/408-pipeline-audit-dup-idempotency/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: This feature is a change to a natural-language task prompt (no compiled code), so
no automated test suite applies. Validation is behavioral, performed via `gh` and described
in `quickstart.md`. Validation tasks are included in place of unit/contract tests.

**Organization**: Tasks are grouped by user story. NOTE: all three user stories are realized
by editing the **same single file** — `.anvil/todos/p1/pipeline-audit.md` (Check 4). Tasks
that touch that file therefore CANNOT run in parallel with each other (no `[P]`), even when
they belong to different stories. They remain logically distinct clauses of one coherent edit.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1, US2, US3 maps to user stories in spec.md

## Path Conventions

The only runtime artifact edited is the task definition:
`.anvil/todos/p1/pipeline-audit.md`. The `specs/408-pipeline-audit-dup-idempotency/`
documents are reference, not shipped code.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish the working context and a measurable baseline before editing.

- [ ] T001 Open `.anvil/todos/p1/pipeline-audit.md` and locate the `## Check 4: Duplicate Detection` section; confirm its current comment wording is exactly `Possible duplicate of #<older>. Flagged by pipeline-audit.` (the marker phrase the fix depends on).
- [ ] T002 Capture a baseline of the spam using `gh`: for issues #375, #377, #380, #382, #384 record the current count of comments containing `Flagged by pipeline-audit` (per `quickstart.md` §2). Save these numbers to compare against after the change.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Confirm the building blocks the revised Check 4 relies on. MUST complete before
editing Check 4.

**⚠️ CRITICAL**: No Check 4 edit should be written until the predicate and tool permissions
are confirmed.

- [ ] T003 Verify the frontmatter `allowed_tools` in `.anvil/todos/p1/pipeline-audit.md` permits the `gh` reads/writes the new logic needs (`gh issue view`, `gh issue comment`, `gh issue edit`). The current `Bash(gh:*)` entry covers them; if any needed subcommand is not covered, add the minimal entry. (FR-009)
- [ ] T004 Validate the already-flagged predicate command against the live repo before embedding it: run `gh issue view 384 --json labels,comments --jq 'any(.labels[]; .name=="duplicate") or any(.comments[]; .body|contains("Flagged by pipeline-audit"))'` and confirm it returns `true`; run it against an unflagged open issue and confirm `false`. (research.md Decision 1; data-model.md "Flag State")

**Checkpoint**: Predicate proven and tooling confirmed — Check 4 can now be edited.

---

## Phase 3: User Story 1 - Already-flagged duplicates are left alone (Priority: P1) 🎯 MVP

**Goal**: Stop the audit from commenting on / re-labeling issues that are already flagged as
duplicates. This is the core fix that resolves issue #408.

**Independent Test**: Run the audit (or dry-run Check 4) against #375/#377/#380/#382/#384 and
confirm zero new `Flagged by pipeline-audit` comments are added and no label changes occur.

### Implementation for User Story 1

- [ ] T005 [US1] In `.anvil/todos/p1/pipeline-audit.md` Check 4, before the comment/label actions, add the **already-flagged guard**: define an issue as already flagged if it has the `duplicate` label OR any comment containing `Flagged by pipeline-audit`, using the predicate command from T004. (FR-001, FR-002; contract B1/B2)
- [ ] T006 [US1] In the same section, change the comment/label step so that when the newer issue is already flagged, the audit SKIPS it — posts no comment and makes no label change (it MAY still print the `##anvil:status WARNING: Possible duplicates` line). (FR-003; contract B3/B7)
- [ ] T007 [US1] Add the **fail-safe rule**: if the predicate command errors or its result is uncertain, treat the issue as already flagged (skip — do not comment). (FR-007; contract B6)

**Checkpoint**: Already-flagged issues are immune to new comments — the spam stops. MVP done.

---

## Phase 4: User Story 2 - Genuinely new duplicates are still flagged once (Priority: P1)

**Goal**: Preserve first-time flagging so the feature still surfaces real new duplicates.

**Independent Test**: For an issue with neither the `duplicate` label nor a marker comment,
confirm the revised Check 4 posts exactly one comment + applies the label on first detection,
and nothing on the next run.

### Implementation for User Story 2

- [ ] T008 [US2] In `.anvil/todos/p1/pipeline-audit.md` Check 4, confirm/rewrite the not-yet-flagged branch so it posts exactly one comment whose body is `Possible duplicate of #<older>. Flagged by pipeline-audit.` (preserving the marker phrase) and applies the `duplicate` label. (FR-004, FR-005; contract B4)
- [ ] T009 [US2] Re-read the full edited Check 4 to confirm the new guard wraps — but does not remove — the original flagging behavior, so previously-unflagged pairs are still handled. (contract B4; spec US2 AC1/AC2)

**Checkpoint**: New duplicates flagged exactly once; second run is silent.

---

## Phase 5: User Story 3 - An issue in multiple pairs is flagged once (Priority: P2)

**Goal**: Prevent double-commenting when one issue matches several older issues in a single
run.

**Independent Test**: Reason through (or stage) a case where the newest issue matches two
older issues; confirm the instructions flag it at most once.

### Implementation for User Story 3

- [ ] T010 [US3] In `.anvil/todos/p1/pipeline-audit.md` Check 4, add an **in-run tracking clause**: instruct the auditor to remember which issues it has flagged (or found already flagged) during this run and to skip any issue already in that set when processing later candidate pairs. (FR-006; data-model.md "In-Run Flagged Set"; contract B5)

**Checkpoint**: One issue → at most one comment per run, regardless of pair count.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Verify the whole change and confirm no scope creep.

- [ ] T011 Diff the edited `.anvil/todos/p1/pipeline-audit.md` against its original and confirm ONLY the Check 4 section (and, if required, one `allowed_tools` line) changed — all other checks and the reporting-only posture are untouched. (FR-008; contract B7)
- [ ] T012 Run the `quickstart.md` verification: predicate returns `true` for #375/#377/#380/#382/#384 (§1), and a re-count shows their `Flagged by pipeline-audit` comment counts have not increased versus the T002 baseline (§2). (SC-001, SC-002, SC-004)
- [ ] T013 Sanity-check readability: ensure the revised Check 4 reads as clear, unambiguous instructions for the executing agent (no contradictory steps; guard precedes actions). (spec "Requirements" testability)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS all story work (predicate + tooling must be proven first).
- **User Stories (Phases 3–5)**: All depend on Foundational. Because they edit the **same file**, they proceed **sequentially** in priority order (US1 → US2 → US3), not in parallel.
- **Polish (Phase 6)**: Depends on all story edits being complete.

### User Story Dependencies

- **US1 (P1)**: The core guard. Foundational only.
- **US2 (P1)**: Builds on the same edited section as US1 (the guard must wrap the existing flagging path). Do US1 first.
- **US3 (P2)**: Adds a clause to the same section; do after US1/US2.

### Within Each User Story

- Edits are to one file; apply them in listed order to avoid conflicting rewrites of the same paragraph.

### Parallel Opportunities

- **None within the implementation phases** — every implementation task edits the single file `.anvil/todos/p1/pipeline-audit.md`, so no `[P]` is marked on T005–T010.
- T002 (baseline capture) and T004 (predicate validation) are read-only `gh` checks and could be run independently of the file inspection in T001/T003, but are sequenced here for clarity.

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 (Setup) and Phase 2 (Foundational).
2. Complete Phase 3 (US1): add the already-flagged guard + fail-safe.
3. **STOP and VALIDATE**: confirm #375/#377/#380/#382/#384 receive no new comments. This alone
   resolves the reported spam in issue #408.

### Incremental Delivery

1. Setup + Foundational → predicate and tooling confirmed.
2. US1 → spam stops (MVP).
3. US2 → confirm new duplicates still flagged once.
4. US3 → guard against within-run double-flagging.
5. Polish → diff check + quickstart validation.

---

## Notes

- The entire implementation is a focused edit to one Markdown task definition; there is no
  build, no Go change, and no new file in `src/`/`internal/`.
- The marker phrase `Flagged by pipeline-audit` is the contract between the write step and the
  detect step — keep it byte-identical in both.
- Cleanup of the 100+ historical comments is explicitly OUT OF SCOPE (contracts/ "Non-goals");
  this change only prevents new ones.
- Commit the single-file change with a message referencing #408.
