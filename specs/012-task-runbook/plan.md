# Implementation Plan: Task Runbook Linking

**Branch**: `[012-task-runbook]` | **Date**: 2026-02-27 | **Spec**: [spec.md](./spec.md)

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add task runbook linking feature that allows users to associate troubleshooting instructions with tasks. When tasks fail, users can quickly access runbook content via CLI or automatic display in error output. Supports both URL references and inline markdown content.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `gopkg.in/yaml.v3` (existing), no new dependencies needed
**Storage**: Task frontmatter (YAML) - runbook stored inline or as URL reference
**Testing**: Go testing (`go test`)
**Target Platform**: CLI tool (macOS/Linux)
**Project Type**: CLI daemon/task runner
**Performance Goals**: Minimal - runbook is static content loaded with task metadata
**Constraints**: None identified
**Scale/Scope**: Per-task configuration, single-user CLI

## Constitution Check

*No constitutional issues identified for this feature.*

## Project Structure

### Documentation (this feature)

```text
specs/012-task-runbook/
├── plan.md              # This file
├── spec.md              # Feature specification
└── tasks.md             # Task breakdown (future)
```

### Source Code (repository root)

```text
internal/
├── project/
│   └── project.go       # Add Runbook field to Todo struct
cmd/anvil/
│   ├── main.go          # Add "runbook" subcommand to task CLI
│   └── task.go          # (or relevant file for task commands)
internal/
├── daemon/
│   └── daemon.go        # Display runbook on task failure
```

**Structure Decision**: Minimal changes to existing structure. Add `Runbook` field to `Todo` struct in `internal/project/project.go`. Add new CLI subcommand in `cmd/anvil/main.go`.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | - | - |

## Implementation Approach

### 1. Add Runbook Field to Todo Struct

In `internal/project/project.go`, add `Runbook string` field to the `Todo` struct. This field will be populated from task frontmatter during task loading.

### 2. Add CLI Command

Add `anvil task runbook <name>` command that displays the runbook content. Handle both:
- URL runbooks: Display URL with suggestion to open in browser
- Inline runbooks: Render markdown in terminal

### 3. Integrate with Task Get

Modify `taskGetCmd` to display runbook when present.

### 4. Display on Failure

Modify task execution result handling in daemon to display runbook when task fails. This requires passing runbook info to the result or having the CLI fetch it on failure display.

### 5. Browser Open (Optional)

Add `--open` flag to `anvil task runbook` command that opens URL in default browser (using `os/exec` with `open` command on macOS).

## Files to Modify

1. `internal/project/project.go` - Add `Runbook` field to `Todo` struct
2. `cmd/anvil/main.go` - Add `runbook` subcommand to task CLI
3. Potentially `internal/daemon/` - Add runbook to failure output

## Testing Approach

1. Unit tests for Runbook field parsing in project.go
2. Integration tests for CLI command
3. Manual testing for terminal rendering and browser open
