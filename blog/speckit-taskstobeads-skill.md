---
description: Convert existing tasks into actionable, dependency-ordered beads issues for the feature based on available design artifacts.
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty).

## Outline

1. Run `.specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks` from repo root and parse FEATURE_DIR and AVAILABLE_DOCS list. All paths must be absolute. For single quotes in args like "I'm Groot", use escape syntax: e.g 'I'\''m Groot' (or double-quote if possible: "I'm Groot").

2. From the executed script, extract the path to **tasks**.

3. Verify beads is initialized by running:

```bash
bd info --json 2>/dev/null || echo "BEADS_NOT_INITIALIZED"
```

> [!CAUTION]
> ONLY PROCEED IF beads is initialized (bd info succeeds). If not, run `bd init` first and confirm with the user.

4. Read the tasks file and parse all tasks. Each task follows the format:
```
- [ ] [TaskID] [P?] [Story?] Description with file path
```

Extract from each task:
- **TaskID** (e.g., T001, T002) — used for dependency tracking
- **Phase** — the phase heading the task belongs to (e.g., "Phase 1: Foundational")
- **Parallelizable** — whether `[P]` marker is present
- **Story label** — e.g., `[US1]`, `[US2]` if present
- **Description** — the full task description including file paths
- **Priority** — derive from phase: Phase 1 = P0, Phase 2 = P1, Phase 3 = P2, Phase 4+ = P3

5. Also extract dependency information from the "Dependencies & Execution Order" section of tasks.md if present. Map task dependencies to beads `dep add` commands.

6. For each task, create a beads issue using `bd create`:

```bash
bd create "T001: <description>" \
  -p <priority> \
  -t task \
  -l <labels> \
  -d "<detailed description with context from phase>"
```

**Label mapping:**
- Phase tasks (no story label): label with phase name, e.g., `foundational`, `polish`
- Story tasks: label with story, e.g., `US1`, `US2`
- Parallelizable tasks: add `parallel` label
- All tasks: add `speckit` label

**Priority mapping:**
- Phase 1 (Foundational/Setup): `-p 0`
- Phase 2 (MVP / P1 story): `-p 1`
- Phase 3 (P2 story): `-p 2`
- Phase 4+ (Polish): `-p 3`

7. After creating all issues, add dependency links using `bd dep add`:

```bash
# child depends on parent (parent blocks child)
bd dep add <child-id> <parent-id>
```

Map the dependency information from tasks.md:
- Phase 2 tasks depend on Phase 1 completion
- Tasks within a phase that have noted dependencies (e.g., "depends on T001")
- Cross-phase dependencies noted in the Dependencies section

8. After all issues are created, display a summary:

```bash
bd list --pretty
```

Report:
- Total issues created
- Issues per phase/story
- Dependencies added
- Any issues that failed to create

> [!IMPORTANT]
> Use `bd create --silent` to capture just the issue ID for dependency wiring. Store a mapping of TaskID → beads issue ID as you create issues so dependencies can be linked correctly.

## Example Flow

```bash
# Create issue, capture ID
ISSUE_ID=$(bd create "T001: Fix remove_job_actor in waikato/core/job_registry.py" \
  -p 0 -t task -l foundational,speckit \
  -d "Check abort status before removing: if aborted=true, set FAILED and keep; if not aborted, set COMPLETED then remove." \
  --silent)

# Store mapping: T001 -> $ISSUE_ID

# Create dependent issue
ISSUE_ID_2=$(bd create "T003: Update remove_job_actor unit test" \
  -p 0 -t task -l foundational,speckit \
  -d "Verify COMPLETED set before removal for success; verify FAILED set and job persists for aborted jobs." \
  --silent)

# Add dependency: T003 depends on T001
bd dep add $ISSUE_ID_2 $ISSUE_ID
```
