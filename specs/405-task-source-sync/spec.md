# Feature Specification: Task Source File Sync

**Feature Branch**: `405-task-source-sync`
**Created**: 2026-04-23
**Status**: Draft
**Input**: GitHub issue #405 — "Task prompts are snapshotted on registration — `anvil reload` and source-file edits have no effect"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Edited source file is picked up on reload (Priority: P1)

A user registers a task from a markdown file they control (e.g. checked into their project's repo). Later, they edit the source file to refine the prompt or adjust frontmatter. After running `anvil reload`, the next scheduled run uses the updated content. The source file is treated as the source of truth.

**Why this priority**: This is the core broken assumption that cost a user an hour of silent debugging. Without this, `anvil reload` is misleadingly named, the declarative file-based workflow is a lie, and version-controlling task prompts alongside project code is fragile. Fixing this single behavior resolves the primary confusion reported in #405.

**Independent Test**: Register a task from a source file, edit the source file, run `anvil reload`, trigger a run, and verify the executed content matches the edited source. Delivers value without requiring any other story to be implemented.

**Acceptance Scenarios**:

1. **Given** a task registered from `~/project/task.md`, **When** the user edits `~/project/task.md` and runs `anvil reload`, **Then** the next scheduled or manual run executes the updated content.
2. **Given** a task registered from a source file, **When** the source file is unchanged and `anvil reload` is run, **Then** behavior is identical to before (no unnecessary re-import, no churn).
3. **Given** a task registered from a source file, **When** the user updates only frontmatter in the source (e.g. changes `schedule:` or adds `allowed-tools:`), **Then** `anvil reload` applies the new frontmatter on the next tick.
4. **Given** a task registered without a source file (created via `anvil task new` interactively), **When** `anvil reload` runs, **Then** that task is unaffected by source-file reconciliation.

---

### User Story 2 - Source file deletion or drift is surfaced, not silent (Priority: P2)

A user has tasks registered from source files that may have moved, been deleted, or been edited in unexpected ways. When they run `anvil status` or `anvil task ls`, they see a visible indicator for any task whose registered content differs from the current source file, or whose source file is missing. The registered content keeps running (safe fallback) but the user is never surprised.

**Why this priority**: Even with reload-picks-up-edits, users may still hit drift in situations where the daemon hasn't reloaded yet, the source file was deleted, or the source file was moved without updating the registration. Surfacing drift loudly prevents the "silent divergence" class of bugs from recurring in other forms.

**Independent Test**: Register a task from a source file, delete or modify the source file without reloading, and verify `anvil status` / `anvil task ls` shows a clear warning marker. Can ship independently of Story 1.

**Acceptance Scenarios**:

1. **Given** a task registered from a source file that has been edited since the last load, **When** the user runs `anvil task ls` or `anvil status`, **Then** the task row includes a clear "source drift" indicator.
2. **Given** a task whose source file has been deleted or moved, **When** the user runs `anvil task ls` or `anvil status`, **Then** the task row includes a clear "source missing" indicator and the user is told the task is running from cached content.
3. **Given** a task registered without a source file, **When** the user runs `anvil task ls` or `anvil status`, **Then** no drift indicator is shown for that task.
4. **Given** `anvil task get <name>`, **When** the task has a source file, **Then** the output shows the source path and its current sync status (in-sync / drift / missing).

---

### User Story 3 - Frontmatter normalization is documented and reversible (Priority: P3)

A user who inspects `.anvil/todos/p<N>/<slug>.md` can understand why their frontmatter looks different from what they wrote. Specifically, they understand that `allowed-tools` is normalized to `allowed_tools`, `priority:` is encoded via the parent directory, and `max-concurrent` is normalized to `max_concurrent`. When source-file reload happens, the same normalizations are applied — so a user editing `allowed-tools:` in their source file gets the expected effect. Help text and documentation make this explicit.

**Why this priority**: This is a secondary annoyance in the issue. It only needs to be addressed after the primary divergence problem is solved, because once reload picks up source changes, users will immediately hit the normalization confusion next.

**Independent Test**: Register a task whose source file uses hyphenated frontmatter keys (`allowed-tools`, `max-concurrent`), reload, and verify those keys take effect as though the user had written `allowed_tools`. Separately, verify `anvil add --help` and `anvil task get` explain the normalization.

**Acceptance Scenarios**:

1. **Given** a source file with `allowed-tools: [Read, Bash]` in frontmatter, **When** the task is registered or reloaded, **Then** the task runs with that allowed-tools list applied (hyphenated form is accepted, not silently stripped).
2. **Given** a source file with `priority: 1` in frontmatter, **When** the task is registered, **Then** the task is placed under `.anvil/todos/p1/` (current behavior), AND the user sees `priority: 1` in `anvil task get` output (so the normalization is visible, not hidden).
3. **Given** a user runs `anvil add --help`, **When** they read the output, **Then** it explains how registration relates to the source file (live-referenced vs snapshotted) and points at `anvil reload` for applying source-file edits.

---

### Edge Cases

- What happens when the source file is deleted between two ticks? The task continues running from the last successfully-loaded content, and drift indicators flag "source missing".
- What happens when the source file contains invalid frontmatter (YAML parse error) after an edit? The daemon logs the parse error, keeps the previously-loaded content active, and drift indicators flag the task as "source invalid".
- What happens when the source file is on a mounted filesystem that's temporarily unavailable? Same as "source missing" — continue with cached content, flag drift.
- What happens when two different source paths are registered that produce the same slug? Existing collision behavior applies (first-registered wins or explicit error — whichever is current); sync behavior does not introduce new collisions.
- What happens when the user registers a task from a source file, then later runs `anvil task edit <name> --content-file <different-file>`? The registered source path is updated to the new file; future reloads pull from the new path.
- What happens when the user runs `anvil task edit <name>` and edits the registered copy directly (not via `--content-file`)? The registered copy and the source file diverge; drift indicators flag this, and the user can choose to push-to-source or pull-from-source via explicit commands (or manual resolution).
- What happens if the user's source file path contains `~` or is relative? Paths are resolved and stored as absolute at registration time; relative paths in `anvil add` are resolved relative to the current working directory at registration, not at reload time.
- What happens during a `reload` when the daemon is mid-run on a task whose source just changed? The in-flight run completes using the content it started with; the next run uses the new content.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST record the absolute path of the source file at task registration time (via `anvil add -f`, `anvil init`, or `anvil register`) so that it can be re-read later.
- **FR-002**: System MUST re-read task content from the recorded source path during `anvil reload`, applying any changes to the task's content and frontmatter on the next tick.
- **FR-003**: System MUST preserve the task's stable identity (UUID, run history, retry state, last-run timestamp) across source-file reloads — reloading content MUST NOT orphan history.
- **FR-004**: System MUST, when a recorded source path no longer exists or cannot be read, continue executing the task using the last successfully-loaded content and record a "source missing" state for that task.
- **FR-005**: System MUST expose the source path and sync status of each task via `anvil task get <name>`, including at minimum: source path (or "no source file"), last loaded at, and current sync status (in-sync / drift / missing / invalid).
- **FR-006**: System MUST indicate source drift in `anvil task ls` and `anvil status` output in a way that is visible at a glance (e.g. a marker column or inline indicator).
- **FR-007**: System MUST apply the same frontmatter normalization rules (e.g. hyphen-to-underscore for known keys, priority-to-directory mapping) during reload as during initial registration, so that source-file edits take effect identically to re-registration.
- **FR-008**: System MUST accept hyphenated frontmatter key variants that were previously silently stripped or renamed (at minimum `allowed-tools`, `max-concurrent`) and normalize them to their canonical form instead of dropping them.
- **FR-009**: `anvil add --help` MUST document the relationship between the source file and the registered task (that the source path is recorded and honored by `anvil reload`) so that users understand the workflow without reading source code.
- **FR-010**: System MUST NOT re-read source files on every daemon tick by default — reload is triggered explicitly via `anvil reload` (or on daemon start) to avoid surprising load from large task libraries and to preserve the current explicit-reload mental model for config.
- **FR-011**: System MUST handle source files that have been edited but still parse correctly (content body changes, valid frontmatter edits) by updating the task atomically on reload — partial updates MUST NOT leave a task in an inconsistent state.
- **FR-012**: System MUST handle source files that have been edited to be invalid (unparseable YAML frontmatter) by logging a clear error, leaving the last-valid content active, and surfacing the error via `anvil task get` and the drift indicator.
- **FR-013**: System MUST distinguish between "task registered from a source file" and "task created without a source file" (e.g. via interactive `anvil task new`), and only apply source-sync behavior to the former.
- **FR-014**: `anvil reload` output MUST summarize what changed: number of tasks checked, number reloaded from source, number with drift warnings, number with missing sources — so the user gets feedback that reload did something.

### Key Entities *(include if feature involves data)*

- **Task Registration Metadata**: Records the relationship between a registered task and its source file. Attributes: absolute source path (optional — may be empty for tasks created without a source), last-loaded-at timestamp, last-loaded-hash (for drift detection without re-parsing), last-load-status (ok / missing / invalid). Belongs to a single task; created at registration; updated on reload.
- **Task Sync Status**: Derived state (not persisted) shown to users. Values: `in-sync` (registered content matches current source), `drift` (source file exists and differs from registered content), `missing` (source file path recorded but file not found), `invalid` (source file exists but cannot be parsed), `no-source` (task was not registered from a source file).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user who edits a registered task's source file and runs `anvil reload` sees the edit take effect on the very next scheduled or manual run, in 100% of cases where the source file is valid and readable.
- **SC-002**: Time for a user to iterate on a task prompt (edit source → reload → run → observe) is reduced to under 30 seconds, eliminating the multi-step `anvil task rm` / `anvil add -f` / re-prime dance currently required.
- **SC-003**: Zero registered tasks lose their UUID, run history, or retry state when their source content is reloaded.
- **SC-004**: A user running `anvil task ls` or `anvil status` can identify any task whose registered content has drifted from its source file within 5 seconds of looking at the output (drift indicators are visible at a glance).
- **SC-005**: A user reading `anvil add --help` can correctly describe, in their own words, what happens to the source file after registration and how to apply later edits, without needing to read source code or ask support.
- **SC-006**: Frontmatter keys `allowed-tools` and `max-concurrent` in source files are honored (applied to task configuration) in 100% of registration and reload operations — not silently stripped.
- **SC-007**: In the failure mode where a source file is deleted or becomes unreadable between reloads, the task continues to run (no regression) and the user is notified via drift indicator in less than one `anvil status` / `anvil task ls` invocation.

## Assumptions

- The cron-style explicit reload model (`anvil reload`) is kept; per-tick auto-reload is deliberately out of scope to avoid surprising I/O and to match the existing config-reload mental model.
- Tasks created interactively via `anvil task new` (no source file) continue to work as today — they are "owned" by the `.anvil/todos/` copy, and source-sync behavior does not apply.
- The daemon continues to execute tasks from its in-memory / on-disk registered copy; source files are read only during reload, not during execution. This preserves current execution performance characteristics.
- Path resolution for source files happens at registration time (absolute path stored). Users who move their project directories will need to re-register or use an explicit relink command (out of scope for this feature; a follow-up can add `anvil task relink` if demand emerges).
- Drift detection uses content hashing, not mtime, to avoid false positives from filesystem-level timestamp churn (e.g. git checkout operations that rewrite mtimes but not content).
- The `anvil task edit <name> --content-file` command continues to work as today and is treated as equivalent to re-registering the source path.

## Out of Scope

- Automatic, per-tick re-reading of source files (deferred; would be a behavioral change with unclear performance implications on large task libraries).
- A watch mode that picks up file changes without an explicit reload (deferred; can be layered on top of the recorded-source-path mechanism later).
- A `relink` command to update a task's recorded source path after the user moves or renames the file (deferred; users can `anvil task rm` + `anvil add -f` as a workaround, though this loses run history — an explicit relink is a likely follow-up).
- Bidirectional sync (editing the registered copy and pushing back to the source file) — not requested in the issue and introduces ambiguity around source-of-truth.
- Changes to the `anvil task history` hang bug or the sandbox / `additionalDirectories` documentation gap — both flagged in the issue as separate concerns to be filed separately.
