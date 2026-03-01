# Implementation Plan: Task Timeout Escalation

**Branch**: `030-task-timeout-escalation` | **Date**: 2026-03-02 | **Spec**: [link to spec](./spec.md)

**Input**: Feature specification from `/specs/030-task-timeout-escalation/spec.md`

## Summary

Add timeout escalation features to anvil tasks including advance warnings, adaptive timeouts based on progress checkpoints, and customizable escalation hooks. The implementation extends existing timeout handling in the daemon and project configuration systems.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: internal/daemon, internal/project, internal/runner
**Storage**: JSON files in .anvil/runs/ for run records, file system for checkpoint detection
**Testing**: Go testing package with integration tests
**Target Platform**: Cross-platform CLI (Linux, macOS, Windows)
**Project Type**: CLI task scheduler
**Performance Goals**: Minimal overhead - monitoring checks should complete in <10ms
**Constraints**: Must maintain backward compatibility with existing timeout behavior
**Scale/Scope**: Extends existing task system used by hundreds of tasks per daemon instance

## Constitution Check

### Core Principles Compliance

1. **Library-First**: Changes made as enhancements to existing internal packages
2. **CLI Interface**: Features exposed via task frontmatter configuration
3. **Test-First**: Implementation includes comprehensive unit and integration tests
4. **Integration Testing**: Daemon-level changes tested with integration tests
5. **Observability**: New features include appropriate logging and status reporting

GATE: All constitutional principles satisfied - no violations.

## Project Structure

### Documentation (this feature)

```text
specs/030-task-timeout-escalation/
├── spec.md             # Feature specification
├── plan.md             # This file (/speckit.plan command output)
├── research.md         # Phase 0 output (/speckit.plan command)
├── data-model.md       # Phase 1 output (/speckit.plan command)
├── quickstart.md       # Phase 1 output (/speckit.plan command)
├── contracts/          # Phase 1 output (/speckit.plan command)
└── tasks.md            # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/
├── project/
│   └── project.go      # Todo struct extensions, frontmatter parsing
├── daemon/
│   └── daemon.go       # Timeout monitoring, hook execution
cmd/
└── anvil/
    └── main.go         # CLI status display updates
```

**Structure Decision**: Extending existing internal packages following established patterns. No new top-level directories needed.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No constitutional violations - complexity is minimal and follows existing patterns.

## Phase 0: Research Complete

Technical unknowns resolved in [research.md](./research.md)

## Phase 1: Design Implementation

### 1. Extend Todo Struct

Add new fields to internal/project/project.go:
- TimeoutWarning time.Duration
- OnTimeoutWarning string
- OnTimeout string
- AdaptiveTimeout *AdaptiveTimeoutConfig

### 2. Extend Frontmatter Parsing

Update frontmatter parsing in internal/project/project.go to handle:
- timeout_warning field (parsed as duration string)
- on_timeout_warning field (parsed as string)
- on_timeout field (parsed as string)
- adaptive_timeout field (parsed as nested object with enabled, extend_if, max_extensions)

### 3. Enhance Daemon Timeout Monitoring

Modify internal/daemon/daemon.go to:
- Track timeout warning status for running tasks
- Implement goroutine to periodically check for timeout warnings
- Execute on_timeout_warning hooks when warnings are triggered
- Implement adaptive timeout logic to extend timeouts based on conditions
- Execute on_timeout hooks when tasks actually time out

### 4. Update CLI Status Display

Enhance anvil ps output to show:
- Time remaining until timeout warning
- Time remaining until actual timeout
- Number of timeout extensions used

### 5. Implement Checkpoint Detection

Add logic to detect checkpoint files for adaptive timeout:
- Check for files in task run directory
- Implement different checkpoint detection strategies

## Dependencies

None - this is an enhancement to existing functionality

## Risks & Mitigations

### Risk: Performance impact from timeout monitoring
Mitigation: Use efficient timing mechanisms and limit monitoring frequency to once per second

### Risk: Confusion between different timeout hooks
Mitigation: Clear documentation and distinct naming

### Risk: Backward compatibility issues
Mitigation: Default values ensure existing tasks work unchanged

## Success Metrics

1. Tasks can configure timeout warnings that trigger before actual timeout
2. Timeout warning hooks execute correctly with appropriate timing
3. Adaptive timeouts extend based on checkpoint detection
4. CLI status shows timeout warning information
5. All existing timeout functionality remains unchanged
6. Comprehensive test coverage for new features