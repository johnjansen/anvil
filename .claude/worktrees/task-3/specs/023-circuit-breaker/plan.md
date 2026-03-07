# Implementation Plan: Task Circuit Breaker

**Branch**: `023-circuit-breaker` | **Date**: 2026-03-01 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/023-circuit-breaker/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add circuit breaker pattern to tasks that automatically stops task execution after too many consecutive failures, then automatically recovers after a timeout. This prevents resource waste during external service outages and reduces alert fatigue.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: Standard library + `gopkg.in/yaml.v3` + internal packages (config, project, daemon)
**Storage**: JSON files in `.anvil/circuits/` (similar to alerts storage pattern)
**Testing**: Go test framework (`go test ./...`)
**Target Platform**: CLI tool / daemon (macOS, Linux)
**Project Type**: CLI tool with daemon background service
**Performance Goals**: Sub-millisecond circuit state checks, minimal overhead on task dispatch
**Constraints**: Must persist state across daemon restarts, handle concurrent access
**Scale/Scope**: Per-task circuit breakers, typically <100 tasks per project

## Constitution Check

*This is a CLI tool project following existing patterns. No formal constitution file exists.*

## Project Structure

### Documentation (this feature)

```text
specs/023-circuit-breaker/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (CLI contracts)
└── checklists/         # Quality validation
```

### Source Code (repository root)

```text
internal/
├── config/              # Config parsing (add circuit_breaker config)
├── project/             # Todo struct (add CircuitBreakerConfig)
├── daemon/              # Core daemon logic
│   ├── circuit.go      # NEW: Circuit breaker state machine
│   ├── sla.go          # Similar pattern for reference
│   └── alerts.go       # Similar pattern for reference
cmd/anvil/              # CLI commands (add task circuit subcommand)

.anvil/                 # Runtime data
├── circuits/           # NEW: Persisted circuit breaker state
└── alerts/             # Similar pattern for reference
```

**Structure Decision**: Following the same pattern as SLA (004-task-sla-tracking) and alerts (021-task-alerts):
- Config in `internal/config/config.go`
- Todo struct extension in `internal/project/project.go`
- Logic in `internal/daemon/circuit.go`
- CLI in `cmd/anvil/`
- Persisted state in `.anvil/circuits/<task-name>.json`

## Complexity Tracking

> **No complexity violations - following established patterns exactly**

| Component | Approach | Why |
|-----------|----------|-----|
| State persistence | JSON files in .anvil/circuits/ | Matches alerts pattern |
| State machine | CLOSED → OPEN → HALF_OPEN | Standard circuit breaker pattern |
| Configuration | Per-task frontmatter | Matches SLA, alerts pattern |
| CLI visibility | `anvil task status` | Extends existing command |

## Phase 0: Research

No NEEDS CLARIFICATION items - all technical decisions are straightforward following existing patterns.

## Phase 1: Design Artifacts

### Data Model

- **CircuitBreakerConfig**: Per-task configuration (failures, timeout, half_open_max)
- **CircuitState**: Enum (Closed, Open, HalfOpen)
- **CircuitBreakerRecord**: Persisted state (state, failure count, timestamps)

### Contracts

- CLI: `anvil task status <name>` shows circuit state
- Config: YAML frontmatter `circuit_breaker:` block
- Hooks: `on_circuit_open`, `on_circuit_close` shell commands

### Quickstart

Simple 3-step guide:
1. Add circuit_breaker config to task frontmatter
2. Run task until failures threshold
3. View status with `anvil task status`
