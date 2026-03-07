# Implementation Plan: Cross-Project Pipeline Visualization

**Branch**: `263-cross-project-pipeline` | **Date**: 2026-03-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/263-cross-project-pipeline/spec.md`

## Summary

Add cross-project dependency awareness to `anvil task pipeline` command. The existing pipeline visualization treats all dependencies as flat strings. This feature integrates `ParseDependency` to distinguish local vs cross-project deps, groups tasks by project with visual boundaries, and adds distinct rendering for cross-project edges in both ASCII and DOT output formats.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `internal/project` (ParseDependency, Dependency, Todo, RunRecord), `cmd/anvil` (task_pipeline.go)
**Storage**: JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system), watched project paths via `~/.anvil/watched/`
**Testing**: `go test ./...`
**Target Platform**: macOS, Linux (CLI)
**Project Type**: CLI tool
**Performance Goals**: N/A (CLI output rendering, negligible overhead)
**Constraints**: Must be backward compatible with existing single-project pipeline output
**Scale/Scope**: Typically <50 tasks across <10 projects

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is unpopulated (template only). No gates to evaluate. Proceeding.

## Project Structure

### Documentation (this feature)

```text
specs/263-cross-project-pipeline/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/           # Phase 1 output (CLI contract)
│   └── cli.md
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
cmd/anvil/
└── task_pipeline.go     # Primary file: extend buildPipelineGraph, pipelineASCII, pipelineDOT

internal/project/
└── dependencies.go      # Existing: ParseDependency, ResolveWatchedProjectPath (no changes expected)
```

**Structure Decision**: This is a single-file change in `cmd/anvil/task_pipeline.go` with possible minor additions. The existing `internal/project/dependencies.go` already provides all the cross-project parsing infrastructure needed.

## Complexity Tracking

No constitution violations to justify.
