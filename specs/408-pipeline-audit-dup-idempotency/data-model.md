# Phase 1 Data Model: Idempotent Duplicate Flagging in pipeline-audit

**Feature**: 408-pipeline-audit-dup-idempotency
**Date**: 2026-06-07

This feature has **no persistent data model** — it introduces no schema, file, or database.
The "entities" below are conceptual structures the revised Check 4 reasons over at runtime,
all sourced from live GitHub state via `gh`. They are documented to make the behavioral
contract precise.

## Conceptual Entities

### Issue (read-only view)

The relevant projection of a GitHub issue for duplicate detection.

| Field      | Source                                  | Used for |
|------------|-----------------------------------------|----------|
| `number`   | `gh issue list --json number`           | Identity; ordering (newer = higher number) |
| `title`    | `gh issue list --json title`            | Similarity comparison (existing behavior) |
| `labels`   | `gh issue list/view --json labels`      | Prior-flag signal (presence of `duplicate`) |
| `comments` | `gh issue view <n> --json comments` / `--comments` | Prior-flag signal (marker phrase) |

### Duplicate Candidate Pair

A transient pairing produced by the title-similarity step.

| Field     | Meaning |
|-----------|---------|
| `older`   | The lower-numbered issue (canonical; never flagged) |
| `newer`   | The higher-numbered issue (the flag target) |

Ordering rule: the **newer** issue (higher number / later `createdAt`) is the one that
receives the comment and label. This preserves existing behavior.

### Flag State (derived predicate)

A boolean computed per `newer` issue. Not stored anywhere.

```text
already_flagged(issue) :=
      ("duplicate" ∈ issue.labels)
   OR (∃ comment ∈ issue.comments : comment.body contains "Flagged by pipeline-audit")
```

Special value: **UNKNOWN** when the labels/comments cannot be retrieved. UNKNOWN is treated
as `true` for the purpose of acting (i.e., skip — fail safe), per FR-007.

### In-Run Flagged Set (ephemeral)

A set of issue numbers flagged or found-already-flagged during the current execution. Lives
only for the duration of one audit run. Prevents double-flagging within a run (FR-006).

```text
seen := {}              # at start of Check 4
... when processing pair (older, newer):
    if newer ∈ seen: skip
    if already_flagged(newer): seen.add(newer); skip (no comment)
    else: comment + label; seen.add(newer)
```

## State Transitions (for a single `newer` issue)

```text
        ┌─────────────────────────────────────────────┐
        │              NOT-FLAGGED                      │
        │ (no duplicate label, no marker comment)       │
        └───────────────┬─────────────────────────────┘
                        │ audit detects it as a duplicate (first time)
                        │ → post 1 comment + add `duplicate` label
                        ▼
        ┌─────────────────────────────────────────────┐
        │               FLAGGED                         │
        │ (has duplicate label and/or marker comment)   │
        └───────────────┬─────────────────────────────┘
                        │ any later run / later pair in same run
                        │ → already_flagged == true → SKIP (no comment)
                        ▼
                   (stable: no further comments)
```

The only transition that writes to GitHub is NOT-FLAGGED → FLAGGED, and it occurs at most
once per issue. FLAGGED is absorbing for audit purposes (a human may still remove the label,
but the marker comment keeps the issue recognized as already-handled).

## Validation Rules

- An issue is never both `older` and acted upon in the same pair (only `newer` is flagged).
- `already_flagged` must be evaluated **before** any write action for that issue.
- A write action (comment + label) is permitted only when `already_flagged == false` and the
  issue is not in the in-run `seen` set.
- No write action may produce more than one comment per issue per run.
