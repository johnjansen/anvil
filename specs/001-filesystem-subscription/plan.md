# Implementation Plan: Filesystem Subscription for Task Triggers

**Branch**: `001-filesystem-subscription` | **Date**: 2026-03-01 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-filesystem-subscription/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add filesystem subscription support to allow tasks to be triggered automatically when files matching a configured pattern are created, modified, or deleted in a specified directory. Follows the subscription framework defined in spec 016-task-subscriptions.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: Standard library + `gopkg.in/yaml.v3`, fsnotify (for cross-platform file watching)
**Storage**: JSON files in `.anvil/subscriptions/` (following existing pattern for alerts/circuits)
**Testing**: Go testing framework (`go test`)
**Target Platform**: Cross-platform (Linux, macOS, Windows)
**Project Type**: CLI tool / daemon
**Performance Goals**: Handle 100+ rapid file changes without missing events (SC-005)
**Constraints**: Response time under 5 seconds from event to task trigger (SC-001)
**Scale/Scope**: Single project daemon, typically watching 1-10 directories

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Single Project**: This is a CLI tool with a daemon component - fits within existing project structure
- **No External Services**: Filesystem watching uses OS-level notifications (inotify/FSEvents) - no external dependencies
- **TDD Approach**: Implementation should follow Test-First development

## Project Structure

### Documentation (this feature)

```text
specs/001-filesystem-subscription/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
cmd/
└── anvil/               # CLI entry point

internal/
├── daemon/              # Daemon for running tasks and watching subscriptions
├── config/              # Configuration parsing
├── project/             # Project model (includes Todo, Task, etc.)
├── subscription/        # NEW: Subscription management (fs, webhook, amqp)
│   └── fs/              # NEW: Filesystem watcher implementation
```

**Structure Decision**: Following the existing pattern in internal/daemon/ and internal/project/. New subscription package will house all subscription types with fs subpackage for filesystem-specific code.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
