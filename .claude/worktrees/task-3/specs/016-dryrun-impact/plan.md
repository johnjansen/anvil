# Implementation Plan: Dry-Run Impact Analysis

**Branch**: `016-dryrun-impact` | **Date**: 2026-02-28 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/016-dryrun-impact/spec.md`

## Summary

Enhance the existing `anvil add --dry-run` command to show a full impact analysis including scheduling conflicts with existing tasks, peak worker concurrency at proposed time slots, and alternative schedule suggestions. The existing overlap detection logic (main.go:2165-2226) will be refactored into reusable functions in a new `cmd/anvil/impact.go` file.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: internal/cron (schedule parsing, Next()), internal/project (Todo, LoadTodos)
**Storage**: N/A (stateless — reads existing tasks, produces report, no persistence)
**Testing**: go test ./cmd/anvil/ (existing pattern)
**Target Platform**: CLI (macOS, Linux)
**Project Type**: CLI tool
**Performance Goals**: < 2 seconds for 100 tasks
**Constraints**: Must preserve existing --dry-run behavior (schedule validation + next run display)
**Scale/Scope**: Projects with up to 100 tasks

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is a template (not project-specific). No gates to evaluate.

## Project Structure

### Documentation (this feature)

```text
specs/016-dryrun-impact/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/
│   └── cli.md           # CLI contract
└── tasks.md             # Phase 2 output (speckit.tasks)
```

### Source Code (repository root)

```text
cmd/anvil/
├── main.go              # Modified: --dry-run flow calls impact analysis, --json flag
├── impact.go            # NEW: impact analysis functions
└── dryrun.go            # Unchanged (existing task dry-run)
```

**Structure Decision**: Single new file (impact.go) in the existing cmd/anvil/ package. Keeps impact analysis self-contained while sharing access to project types and cron parser.

## Complexity Tracking

No constitution violations — no new dependencies, no new packages, minimal new code.
