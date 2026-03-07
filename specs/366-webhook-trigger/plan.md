# Implementation Plan: Webhook Trigger for Tasks

**Branch**: `366-webhook-trigger` | **Date**: 2026-03-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/366-webhook-trigger/spec.md`

## Summary

Harden and extend the existing `WebhookServer` in `internal/daemon/webhook.go` to meet full feature requirements: configurable port (default 9090), conditional server startup (only when webhook tasks exist), request body size limits, duplicate path detection, header/metadata passing to tasks, dynamic server lifecycle management, and proper HTTP status codes. The existing HMAC validation, route registration, and payload passing are already functional.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `net/http` (standard library), `crypto/hmac` + `crypto/sha256` (standard library), `gopkg.in/yaml.v3` (frontmatter parsing, already in go.mod)
**Storage**: JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system)
**Testing**: `go test` with table-driven tests
**Target Platform**: macOS, Linux
**Project Type**: CLI tool with background daemon
**Performance Goals**: Trigger latency <2s from request receipt; handle 100+ concurrent requests
**Constraints**: Must use existing `SubscriptionConfig` frontmatter; no new Go dependencies
**Scale/Scope**: Single webhook server per daemon; tens of webhook routes per instance

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is not customized for this project (template placeholders only). No specific gates to evaluate. Proceeding with standard engineering best practices.

**Post-design re-check**: Design extends existing `WebhookServer` following established subscription patterns (FSWatcher, GitWatcher). No new abstractions introduced. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/366-webhook-trigger/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── config/
│   └── config.go        # Existing: add WebhookPort field
├── daemon/
│   ├── daemon.go        # Existing: conditional webhook server startup, port from config
│   ├── webhook.go       # Existing: extend with body limits, duplicate detection, lifecycle, headers
│   └── webhook_test.go  # Existing: extend with new test cases
└── project/
    └── project.go       # Existing: SubscriptionConfig already has webhook fields (no changes)
```

**Structure Decision**: All changes extend existing files. The `WebhookServer` in `internal/daemon/webhook.go` is the primary implementation target. `config.go` gets one new field. `daemon.go` gets conditional startup logic. No new packages or files needed.

## Complexity Tracking

No constitution violations to justify.

## Implementation Gaps (Existing Code -> Spec Requirements)

| Gap | Current State | Required State | Files |
|-----|--------------|----------------|-------|
| Port configuration | Hardcoded `:8080` | Configurable via config, default 9090 | `config.go`, `daemon.go` |
| Conditional startup | Always starts | Only start when webhook tasks exist | `daemon.go` |
| Body size limit | No limit (`io.ReadAll`) | 1 MB max via `http.MaxBytesReader` | `webhook.go` |
| Duplicate paths | Silent overwrite | Error on duplicate registration | `webhook.go` |
| Dynamic lifecycle | No stop/restart | Start/stop based on active handlers | `webhook.go`, `daemon.go` |
| Request headers | Not passed | Pass as env vars | `webhook.go`, `daemon.go` |
| StopSubscription | Not implemented | Remove handler, stop server if empty | `webhook.go` |
| HTTP status codes | Basic (200, 401, 405) | Add 404 (custom), 413 (body too large) | `webhook.go` |
