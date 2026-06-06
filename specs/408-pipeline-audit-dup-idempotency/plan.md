# Implementation Plan: Idempotent Duplicate Flagging in pipeline-audit

**Branch**: `408-pipeline-audit-dup-idempotency` | **Date**: 2026-06-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/408-pipeline-audit-dup-idempotency/spec.md`

## Summary

Make Check 4 (Duplicate Detection) of the `pipeline-audit` task idempotent. Today the
check posts a "Possible duplicate" comment and applies the `duplicate` label on every daily
run for every detected pair, with no memory of prior flagging — so issues accrue one
redundant comment per day. The fix adds a pre-flagging guard to the task's natural-language
instructions: before commenting/labeling the newer issue, check whether it is *already
flagged* (it carries the `duplicate` label **or** has a comment containing the stable marker
phrase "Flagged by pipeline-audit"); if so, skip it. The change is confined to the prompt
text of `.anvil/todos/p1/pipeline-audit.md` Check 4, plus any `allowed_tools` additions the
new `gh` invocations require. No Go code changes.

## Technical Context

**Language/Version**: N/A — the artifact is a Markdown task definition (frontmatter + natural-language prompt) executed by an LLM agent, not compiled code. Repo language is Go 1.24.6 but is untouched here.
**Primary Dependencies**: `gh` CLI (GitHub issue/label/comment operations); the anvil task runner that executes the prompt with the declared `allowed_tools`.
**Storage**: GitHub issue state (labels + comments) is the source of truth for prior-flag detection. No local persistence/sidecar needed.
**Testing**: Manual/scenario validation via `gh` against the live repo plus a dry-run reading of the affected issues (#375, #377, #380, #382, #384). No Go unit tests apply (prompt change).
**Target Platform**: anvil daemon running the scheduled `pipeline-audit` task (`schedule: "0 9 * * *"`).
**Project Type**: Single-repo automation task definition (prompt engineering), not an application feature.
**Performance Goals**: Negligible — at most a few extra `gh` reads per detected duplicate pair per daily run; well within rate limits at current issue volume.
**Constraints**: Must stay reporting-only (no code fixes); must not regress other checks; new `gh` subcommands must be covered by `allowed_tools` (currently `Bash(gh:*)` covers all `gh` usage); must remain idempotent under repeated runs.
**Scale/Scope**: A single repository's open-issue set (tens to low hundreds of issues); a handful of duplicate pairs per run.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

The project constitution (`.specify/memory/constitution.md`) is an unpopulated template
(placeholder principles only); there are no ratified, concrete gates to enforce. Applying
the spirit of typical gates:

- **Simplicity**: PASS — the change adds a single guard clause to one task's instructions; no
  new files, services, dependencies, or persisted state.
- **Scope discipline**: PASS — confined to Check 4 of one task definition; preserves the
  task's reporting-only posture.
- **No new abstractions**: PASS — reuses existing `gh` + `duplicate` label conventions and
  the existing comment marker phrase.

No violations; Complexity Tracking not required.

## Project Structure

### Documentation (this feature)

```text
specs/408-pipeline-audit-dup-idempotency/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/
│   └── check4-duplicate-detection.md   # Behavioral contract for the revised Check 4
├── checklists/
│   └── requirements.md  # Spec quality checklist (/speckit.specify output)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created here)
```

### Source Code (repository root)

The only artifact changed by implementation:

```text
.anvil/
└── todos/
    └── p1/
        └── pipeline-audit.md   # Task definition; edit Check 4 + (if needed) allowed_tools
```

No `src/`, `internal/`, or `cmd/` Go code is touched. There is no test directory impact;
validation is behavioral (see quickstart.md).

**Structure Decision**: This is a prompt/task-definition change, not application code. The
implementation surface is exactly one Markdown file (`.anvil/todos/p1/pipeline-audit.md`).
The speckit artifacts in `specs/408-...` document the intended behavior and validation;
they do not ship as runtime code.

## Implementation Approach

1. **Define the "already-flagged" predicate** (natural-language, executed via `gh`):
   - The newer issue's labels include `duplicate`, **OR**
   - The newer issue has at least one comment whose body contains the marker substring
     `Flagged by pipeline-audit`.
   - Determinable with the data already fetched plus one `gh issue view <n> --json labels,comments`
     (or `--comments`) per candidate. Errors/uncertainty ⇒ treat as "skip" (FR-007).

2. **Insert the guard into Check 4** before the comment/label actions:
   - Replace the unconditional "Comment … / Add duplicate label …" steps with:
     "If the newer issue is already flagged (see predicate), skip it — do not comment or
     re-label; optionally note it in the run log. Otherwise, post exactly one comment and
     apply the `duplicate` label."

3. **Track within-run flagging** (FR-006): once an issue is flagged (or found already
   flagged) in this run, treat it as flagged for any later candidate pair in the same run so
   it is never commented twice.

4. **Preserve the marker phrase** (FR-005): keep the comment wording
   "Possible duplicate of #&lt;older&gt;. Flagged by pipeline-audit." so the write and the
   detect step share one canonical string, and legacy comments are recognized.

5. **Confirm `allowed_tools` coverage** (FR-009): current frontmatter declares `Bash(gh:*)`,
   which already permits `gh issue view`/`gh issue comment`/`gh issue edit`. No change
   expected; verify during implementation and add only if a needed subcommand is not covered.

## Complexity Tracking

No constitution violations; section intentionally empty.
