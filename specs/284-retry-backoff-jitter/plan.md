# Implementation Plan: Advanced Task Retry with Backoff Strategies and Jitter

**Branch**: `284-retry-backoff-jitter` | **Date**: 2026-03-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/284-retry-backoff-jitter/spec.md`

## Summary

Extend the existing retry mechanism to support configurable backoff strategies (exponential, linear, constant), jitter for thundering-herd prevention, a maximum total retry duration, and enhanced observability in task history. The implementation modifies the Todo struct, YAML frontmatter parsing, daemon retry loop, RunRecord, and task history display. Full backward compatibility with existing `retry`/`retry_delay` syntax is maintained.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `internal/project` (Todo, RunRecord, frontmatter parsing), `internal/daemon` (retry loop in `executeTask`), `cmd/anvil` (task history/list display)
**Storage**: JSON files in `.anvil/runs/<task-id>/` (RunRecord system)
**Testing**: `go test ./...`
**Target Platform**: macOS, Linux (CLI tool)
**Project Type**: CLI
**Performance Goals**: N/A (retry delays are seconds-to-minutes scale)
**Constraints**: Must be backward compatible with existing `retry: N` and `retry_delay: Xm` YAML syntax
**Scale/Scope**: Changes touch 3 packages: `internal/project`, `internal/daemon`, `cmd/anvil`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

No constitution is defined for this project (template only). No gates to enforce.

## Project Structure

### Documentation (this feature)

```text
specs/284-retry-backoff-jitter/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── project/
│   └── project.go       # Todo struct (RetryConfig fields), RunRecord, frontmatter parsing
├── daemon/
│   └── daemon.go        # executeTask retry loop, backoff calculation, jitter application
cmd/anvil/
├── task_lifecycle.go    # task history display
├── task_list.go         # task list display
└── dryrun.go            # dry-run retry info display
```

**Structure Decision**: All changes extend existing files in the established project structure. No new packages or files are needed (except potentially a small retry helper if backoff calculation warrants extraction).
