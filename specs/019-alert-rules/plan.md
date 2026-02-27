# Implementation Plan: Task Alerting Rules

**Branch**: `019-alert-rules` | **Date**: 2026-02-28 | **Spec**: [spec.md](spec.md)

## Summary

Add alerting rules to task frontmatter. Conditions (cost, duration, output) are evaluated after each run. Fired alerts stored as JSONL. CLI commands for viewing and acknowledging alerts.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: internal/project, internal/daemon, internal/webhook
**Storage**: `.anvil/alerts/<task-id>.jsonl`

## Project Structure

```text
cmd/anvil/
  main.go         # Modified: add alertsCmd dispatcher
  alerts.go       # New: CLI commands

internal/project/
  project.go      # Modified: AlertRule, AlertRecord, Todo.Alerts, read/write

internal/daemon/
  daemon.go       # Modified: evaluateAlerts() post-run
```

## Constitution Check

No constitution defined.

## Complexity Tracking

No violations.
