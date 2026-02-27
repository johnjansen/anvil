# Implementation Plan: Task Dry-Run Impact Analysis

**Branch**: `010-dry-run-impact-analysis` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/010-dry-run-impact-analysis/spec.md`

## Summary

Add `--dry-run` flag to `anvil add` command that shows impact analysis (scheduling conflicts, cost estimates, worker load) without creating the task. Allows users to make informed scheduling decisions before committing. Includes interactive confirmation to proceed with adding the task.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: gopkg.in/yaml.v3 (frontmatter), cron parser (internal), fmt, os
**Storage**: Filesystem (task files) - no new storage needed
**Testing**: Go testing framework (existing project pattern)
**Target Platform**: CLI tool (cross-platform)
**Performance Goals**: <1 second for impact analysis
**Constraints**: Must work in non-interactive (CI) mode
**Scale/Scope**: Single CLI command modification

## Constitution Check

*No constitution file found - skipping check.*

## Project Structure

### Source Code (repository root)

The feature modifies existing code in `cmd/anvil/main.go`:

```text
cmd/anvil/
└── main.go              # Add --dry-run flag, impact analysis, confirmation prompt

internal/
├── cron/
│   └── parser.go       # No changes needed (already used for conflict detection)
├── project/
│   └── project.go       # No changes needed (already used)
└── config/
    └── config.go        # No changes needed (cost rates already available)
```

## Implementation Phases

### Phase 1: Core Infrastructure

#### Task 1.1 - Add ImpactAnalysis types
Create data structures for impact analysis results.

**Files to modify**: `cmd/anvil/main.go`

**Implementation**:
- Add `ImpactAnalysis` struct with fields: Conflicts, CostEstimate, WorkerLoad, Alternatives
- Add `ConflictInfo` struct with TaskName and Schedule fields
- Add `CostEstimate` struct with PerRun and Monthly fields
- Add `AlternativeSchedule` struct with Cron and Reason fields

#### Task 1.2 - Implement conflict detection
Reuse existing conflict detection logic for impact analysis.

**Files to modify**: `cmd/anvil/main.go`

**Implementation**:
- Extract conflict detection from `taskCreateCmd` into separate function `checkConflicts(schedule string) []ConflictInfo`
- This function returns list of conflicts found (reuses existing logic at lines 2163-2223)

#### Task 1.3 - Implement cost estimation
Calculate estimated cost based on task content.

**Files to modify**: `cmd/anvil/main.go`

**Implementation**:
- Add `estimateCost(content string, schedule string) CostEstimate` function
- Estimate tokens from content: len(content) / 4 (4 chars per token)
- Calculate monthly runs from cron: daily=30, hourly=720, weekly=4, etc.
- Use config rates: input_token_rate ($3.00/1M), output_token_rate ($15.00/1M)
- Return per-run and monthly estimates

#### Task 1.4 - Implement alternative schedule suggestions
Generate alternative schedules when conflicts exist.

**Files to modify**: `cmd/anvil/main.go`

**Implementation**:
- Add `suggestAlternatives(schedule string, conflicts []ConflictInfo) []AlternativeSchedule`
- Generate alternatives by offsetting minute field
- Filter alternatives that don't conflict with existing tasks

### Phase 2: CLI Integration

#### Task 2.1 - Add --dry-run flag
Add the flag to anvil add command.

**Files to modify**: `cmd/anvil/main.go`

**Implementation**:
- Add `dryRun` bool flag in `taskCreateCmd` (around line 1930)
- Flag: `--dry-run`, short `-n`
- Add help text explaining the flag

#### Task 2.2 - Implement dry-run workflow
When --dry-run is specified, show impact and optionally prompt.

**Files to modify**: `cmd/anvil/main.go`

**Implementation**:
- In `taskCreateCmd`, when `dryRun` is true:
  1. Run conflict detection
  2. Calculate cost estimate
  3. Generate alternative suggestions if conflicts exist
  4. Print formatted impact analysis
  5. If interactive (TTY) and conflicts exist, prompt "Add anyway? [y/N]"
  6. If user confirms, proceed with normal AddTodo
  7. If user declines or non-interactive with conflicts, exit without creating

#### Task 2.3 - Format impact output
Create formatted output for impact analysis.

**Files to modify**: `cmd/anvil/main.go`

**Implementation**:
- Add `printImpactAnalysis(impact ImpactAnalysis)` function
- Match the format shown in the issue:
  ```
  Impact Analysis:
  ─────────────────────────
  Schedule: 0 9 * * *
  Conflicts: Conflicts with 3 tasks at 09:00
    - fetch-data (same time)
    - process-data (same time)
    - report (same time)
  Monthly Cost: +$15.00 (estimated)
  Worker Load: +10% at 09:00

  Suggested alternatives to avoid conflicts:
    - 0 9,15,21 * * * (spread across day)
    - */30 * * * * (every 30 min)
    - 0 10 * * * (shift to 10am)
  ```

### Phase 3: Edge Cases & Polish

#### Task 3.1 - Handle non-interactive mode
Exit appropriately in CI/non-TTY environments.

**Files to modify**: `cmd/anvil/main.go`

**Implementation**:
- Check `terminal.IsTerminal` or similar for TTY detection
- In non-interactive mode with conflicts: exit code 1
- In non-interactive mode without conflicts: exit code 0 (or proceed based on flag)

#### Task 3.2 - Handle edge cases
Implement handling for edge cases from spec.

**Files to modify**: `cmd/anvil/main.go`

**Implementation**:
- Empty content: show error before impact
- Invalid schedule: show parse error before impact
- Many conflicts: truncate to 10, show "and X more"
- One-shot task: show per-run cost instead of monthly

## Open Questions

1. **Worker load calculation**: Should worker load use configured `max_workers` or just show concurrent task count? *Decision: Show both - concurrent count and percentage if max_workers is configured.*

2. **Confirmation prompt**: Should confirmation be automatic in non-interactive mode when there are no conflicts? *Decision: Yes - skip prompt in non-interactive mode if no conflicts.*

3. **Alternative generation**: How many alternatives to generate? *Decision: Generate up to 3, filter for valid non-conflicting schedules.*

## Dependencies

- All required functionality already exists in codebase:
  - Cron parsing: `internal/cron/parser.go`
  - Conflict detection: existing code in `taskCreateCmd`
  - Cost rates: `internal/config/config.go`
  - Task loading: `internal/project/project.go`

## Testing Strategy

1. **Unit tests**: Test cost estimation function with known inputs
2. **Integration tests**: Test `--dry-run` flag end-to-end
3. **Manual testing**: Test interactive confirmation flow
