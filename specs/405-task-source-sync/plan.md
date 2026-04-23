# Implementation Plan: Task Source File Sync

**Branch**: `405-task-source-sync` | **Date**: 2026-04-23 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/405-task-source-sync/spec.md`

## Summary

Make `anvil reload` re-read task content from the original source file used at registration, instead of running forever against a frozen `.anvil/todos/p<N>/<slug>.md` snapshot. The source path is recorded at registration time, reload honours it, drift is surfaced in `anvil task ls` / `status` / `task get`, and known hyphenated frontmatter keys (`allowed-tools`, `max-concurrent`) stop being silently stripped. The task's stable identity (UUID, run history, retry state) is preserved across reloads.

Technical approach: add a small metadata sidecar per registered task (`.anvil/todos/<priority>/<slug>.meta.json`) storing the absolute source path, last-loaded content hash, last-loaded-at, and last-load-status. Extend the reload path in `cmd/anvil/status.go` and the project loader in `internal/project/project.go` to walk tasks, re-import source files when present, and write the updated `.md` copy atomically. Extend frontmatter parsing to accept known hyphenated aliases. Extend `anvil task ls`, `status`, and `task get` rendering to include sync status derived from the metadata sidecar.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `gopkg.in/yaml.v3` (frontmatter parsing, already in go.mod), `crypto/sha256` (stdlib, content hashing for drift detection), existing `internal/project` (Todo loading), `internal/daemon` (reload trigger), `cmd/anvil` (CLI)
**Storage**: JSON sidecar file per task at `.anvil/todos/p<N>/<slug>.meta.json`; existing `.anvil/todos/p<N>/<slug>.md` retained as the executable copy
**Testing**: `go test ./...` — existing `internal/project` test patterns (table-driven, uses `t.TempDir()`); existing `cmd/anvil` dispatch tests for CLI surface
**Target Platform**: macOS + Linux CLI (existing `anvil` targets)
**Project Type**: Single Go CLI + long-running daemon (existing layout — `cmd/anvil`, `internal/*`)
**Performance Goals**: Reload walks at most a few hundred registered tasks per project; must complete in <500ms for 100 tasks (hash + stat + conditional re-parse); no per-tick I/O overhead
**Constraints**: Zero data loss on concurrent reload + run (atomic writes via temp-file + rename); reload MUST NOT orphan run history; MUST be safe when daemon is mid-executing a task whose source just changed (in-flight run completes against old content)
**Scale/Scope**: ~10 files touched (`internal/project/project.go`, new `internal/project/source_sync.go` or similar, `cmd/anvil/status.go` reload path, `cmd/anvil/task_list.go`, `cmd/anvil/task_state.go` get/show, `cmd/anvil/task_create.go` add-f help, new `_test.go` files)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Repository constitution (`.specify/memory/constitution.md`) is a template with placeholder principles — no ratified rules to evaluate against. Standard project guardrails apply:

- **Library-First / Internal Package**: New logic lives in `internal/project` (task loading is already there) — no new top-level packages. PASS.
- **CLI Interface**: All new capability surfaces via existing commands (`anvil reload`, `anvil add`, `anvil task ls/get/status`) — no new top-level commands required for the MVP. PASS.
- **Test-First**: Table-driven unit tests in `internal/project` for source-sync logic, plus dispatch tests in `cmd/anvil` for CLI output. Integration test via `quickstart.md` reproduction. PASS.
- **Backward Compatibility**: Tasks registered before this feature have no metadata sidecar — they degrade to `no-source` status (current behavior unchanged). Existing `.anvil/todos/*.md` layout unchanged. PASS.

No violations requiring complexity justification.

## Project Structure

### Documentation (this feature)

```text
specs/405-task-source-sync/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── cli.md           # CLI surface changes (reload, add, task ls/get/status)
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
cmd/anvil/
├── status.go              # MODIFY: reloadCmd — walk tasks, call project.ReloadFromSources, print summary
├── task_create.go         # MODIFY: anvil add -f writes SourcePath to meta sidecar; --help text updated
├── task_list.go           # MODIFY: add sync-status column / inline marker
├── task_state.go          # MODIFY: anvil task get <name> includes source path + sync status
└── init_cmd.go            # MODIFY: anvil init / register records source paths for discovered task files

internal/project/
├── project.go             # MODIFY: Todo loader reads .meta.json if present, populates SourceMeta
├── source_sync.go         # NEW: SourceMeta type, read/write sidecar, hash+stat drift detection,
│                          #      ReloadFromSources(rootDir) implementing FR-002, FR-004, FR-011, FR-012
├── source_sync_test.go    # NEW: table-driven tests for reload, drift, missing, invalid cases
└── frontmatter.go (or wherever frontmatter parsing lives — confirm in Phase 0)
                           # MODIFY: accept hyphenated aliases for allowed-tools, max-concurrent

internal/daemon/
└── (no changes expected — reload is initiated from cmd/anvil/status.go via IPC/signal;
   daemon picks up refreshed on-disk task files via its existing loader on the next tick)

tests/ (none — existing Go test layout keeps tests alongside source)
```

**Structure Decision**: Keep existing Go package layout (`cmd/anvil` + `internal/*`). New logic isolated in a new file `internal/project/source_sync.go` to keep `project.go` (already ~1.5k lines) from growing further. Tests colocated per Go convention.

## Complexity Tracking

No constitution violations. Table omitted.

## Open Questions (resolved in research.md)

1. How does `anvil reload` currently signal the daemon — IPC socket, SIGHUP, or file-watch? Resolved in research.md via a quick read of `cmd/anvil/status.go:reloadCmd` and `internal/daemon`.
2. Where is frontmatter parsing implemented today — `internal/project/project.go` or a separate file? Resolved in research.md.
3. What is the current behavior of `anvil add -f` when the source path is a symlink or a path with `~`? Resolved in research.md.
4. What is the precise list of frontmatter keys currently stripped or renamed during registration? Confirmed via grep; `allowed-tools` and `max-concurrent` are the two reported; research.md enumerates any others found.
