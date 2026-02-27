# Implementation Plan: Task Alerting Rules

**Branch**: `015-task-alerting` | **Date**: 2026-02-28 | **Spec**: [spec.md](./spec.md)

## Summary

Add task alerting rules feature that allows users to define custom alert conditions on tasks. Alerts trigger based on cost thresholds, duration limits, or output pattern matching. Users can view active alerts, acknowledge them, and configure automatic actions like webhooks.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `gopkg.in/yaml.v3` (existing), `regexp` (standard library)
**Storage**: Task frontmatter (YAML) + JSON alert storage in `.anvil/alerts/`
**Testing**: Go testing (`go test`)
**Target Platform**: CLI tool (macOS/Linux)
**Project Type**: CLI daemon/task runner
**Performance Goals**: Alert evaluation < 100ms per task, webhook delivery < 5s
**Constraints**: None identified
**Scale/Scope**: Per-task configuration, single-user CLI

## Constitution Check

*No constitutional issues identified for this feature.*

## Project Structure

### Documentation (this feature)

```text
specs/015-task-alerting/
├── plan.md              # This file
├── spec.md              # Feature specification
├── data-model.md        # Alert data structures
└── tasks.md             # Task breakdown (future)
```

### Source Code (repository root)

```text
internal/
├── project/
│   └── project.go       # Add AlertRules field to Todo struct
├── alerts/
│   ├── alerts.go        # Alert evaluation and storage
│   └── storage.go       # Alert persistence
cmd/anvil/
│   └── main.go          # Add "alerts" subcommand
```

**Structure Decision**: Add `Alerts` field to existing `Todo` struct. Create new `internal/alerts/` package for alert logic. Add CLI subcommands under existing task hierarchy.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | - | - |

## Implementation Approach

### 1. Add Alert Configuration to Todo Struct

In `internal/project/project.go`, add `Alerts []AlertRule` field to the `Todo` struct with:
- `Name` - identifier for the alert rule
- `Condition` - expression (cost > X, duration > Xm, output =~ pattern)
- `Message` - alert message template
- `Severity` - warning, error, critical
- `Action` - webhook, notify list, retry count

### 2. Create Alert Evaluation Package

Create `internal/alerts/alerts.go` with:
- `EvaluateAlerts(task *Todo, runResult RunResult) []Alert` - evaluate rules against run result
- `AlertCondition` interface for different condition types
- Condition parsers: `parseCostCondition`, `parseDurationCondition`, `parseOutputCondition`

### 3. Create Alert Storage

Create `internal/alerts/storage.go` with:
- `AlertStore` struct for persisting alerts
- JSON file storage in `.anvil/alerts/`
- Methods: `SaveAlert`, `GetActiveAlerts`, `AcknowledgeAlert`, `GetAlertHistory`

### 4. Integrate Alert Evaluation into Daemon

Modify task execution flow in daemon to:
- After task completes, call alert evaluation
- Store triggered alerts
- Execute alert actions (webhook, notify)

### 5. Add CLI Commands

Add `anvil alerts` subcommand:
- `anvil alerts` - list active alerts
- `anvil alerts ack <alert-id>` - acknowledge alert
- `anvil alerts history` - show past alerts
- `anvil alerts get <alert-id>` - show alert details

### 6. Implement Webhook Action

Add webhook delivery with:
- JSON payload with alert details
- Retry logic (configurable attempts)
- Timeout handling (10s default)

## Files to Modify

1. `internal/project/project.go` - Add `Alerts []AlertRule` field to `Todo` struct
2. `internal/alerts/alerts.go` (NEW) - Alert evaluation logic
3. `internal/alerts/storage.go` (NEW) - Alert persistence
4. `cmd/anvil/main.go` - Add `alerts` subcommand
5. `internal/daemon/daemon.go` - Integrate alert evaluation after task completion

## Files to Create

1. `internal/alerts/alerts.go` - Alert evaluation
2. `internal/alerts/storage.go` - Alert storage
3. `specs/015-task-alerting/data-model.md` - Data model documentation

## Testing Approach

1. Unit tests for condition parsing (cost, duration, output regex)
2. Unit tests for alert evaluation
3. Integration tests for CLI commands
4. Manual testing for webhook delivery
