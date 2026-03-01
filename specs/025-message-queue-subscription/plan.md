# Implementation Plan: Message Queue Subscription for Task Triggers

**Branch**: `025-message-queue-subscription` | **Date**: 2026-03-01 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/025-message-queue-subscription/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add message queue subscription support to allow tasks to be triggered automatically when messages are published to a configured AMQP queue. Follows the subscription framework pattern established in spec 016-task-subscriptions and follows the same implementation approach as the filesystem subscription feature (spec 001).

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: Standard library + `gopkg.in/yaml.v3`, rabbitmq/amqp091-go (AMQP 0.9.1 client)
**Storage**: JSON files in `.anvil/subscriptions/` (following existing pattern for alerts/circuits/filesystem)
**Testing**: Go testing framework (`go test`)
**Target Platform**: Cross-platform (Linux, macOS, Windows)
**Project Type**: CLI tool / daemon
**Performance Goals**: Handle 100+ messages per second without missing events (SC-004)
**Constraints**: Response time under 5 seconds from message to task trigger (SC-001)
**Scale/Scope**: Single project daemon, typically connecting to 1-5 queues

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Single Project**: This is a CLI tool with a daemon component - fits within existing project structure
- **External Service**: AMQP message queue - standard RabbitMQ/broker connectivity with retry logic
- **TDD Approach**: Implementation should follow Test-First development

## Project Structure

### Documentation (this feature)

```text
specs/025-message-queue-subscription/
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
├── subscription/        # Subscription management (fs, webhook, amqp)
    └── amqp/            # NEW: AMQP message queue implementation
```

**Structure Decision**: Following the existing pattern in internal/daemon/ and internal/project/. New subscription package will house all subscription types with amqp subpackage for AMQP-specific code, mirroring the fs implementation.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
