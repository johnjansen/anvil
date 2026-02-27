# Implementation Plan: Task Execution Sandbox

**Branch**: `006-prompt-sandbox` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/006-prompt-sandbox/spec.md`

## Summary

Add a `prompt sandbox` CLI subcommand that runs a task's prompt against the LLM without creating run records, triggering hooks, or consuming budgets. Includes `--compare` for testing prompt variations and `--watch` for iterative development. Implemented as a new CLI command tree (`anvil prompt sandbox`) that calls the existing runner directly, bypassing daemon dispatch.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `internal/runner` (Runner.Run), `internal/project` (Todo, LoadTodos), `internal/config` (cost rates), `cmd/anvil/main.go` (CLI dispatch)
**Storage**: N/A (sandbox produces no persistent state)
**Testing**: `go test` with unit tests for sandbox result formatting; integration tested via CLI
**Target Platform**: macOS/Linux CLI
**Project Type**: CLI tool
**Performance Goals**: Sandbox execution adds no meaningful overhead beyond the LLM call itself
**Constraints**: Must not write to `.anvil/runs/`, must not trigger hooks, must not alter daemon state
**Scale/Scope**: Single-user CLI command, one sandbox execution at a time

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is not customized for this project. No specific gates apply. Following project conventions:
- CLI subcommand pattern (existing `taskCmd` switch pattern)
- Go test coverage for new logic
- No new dependencies

## Project Structure

### Documentation (this feature)

```text
specs/006-prompt-sandbox/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (via /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/anvil/main.go           # New promptCmd, promptSandboxCmd functions
internal/runner/runner.go   # Existing — used as-is for sandbox execution
internal/project/project.go # Existing — Todo loading used as-is
internal/config/config.go   # Existing — cost rate config used for estimates
```

**Structure Decision**: No new packages needed. The sandbox is a CLI-only feature that composes existing runner and project packages. All new code lives in `cmd/anvil/main.go` as command functions, following the established pattern for all other `anvil task *` and `anvil *` commands.
