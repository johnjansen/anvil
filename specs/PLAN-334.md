# Implementation Plan: Task Subscriptions for External Event Triggers

**Branch**: `[016-task-subscriptions]` | **Date**: 2026-02-28 | **Spec**: [SPEC-334.md](./SPEC-334.md)

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add task subscriptions feature that enables tasks to be triggered by external events including HTTP webhooks, AMQP message queues, and file system events. Includes CLI commands for managing subscriptions.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: Existing deps + `github.com/rabbitmq/amqp091-go` (AMQP), no new dependencies needed for webhook (stdlib `net/http`), `github.com/fsnotify/fsnotify` (FS events)
**Storage**: Task frontmatter (YAML) + state in `.anvil/subscriptions/` JSON files
**Testing**: Go testing (`go test`)
**Target Platform**: CLI tool (macOS/Linux)
**Project Type**: CLI daemon/task runner
**Performance Goals**: Subscriptions run asynchronously, minimal impact on task execution
**Constraints**: None identified
**Scale/Scope**: Per-task configuration, single-user CLI

## Constitution Check

*No constitutional issues identified for this feature.*

## Project Structure

### Documentation (this feature)

```text
specs/016-task-subscriptions/
├── plan.md              # This file
├── SPEC-334.md          # Feature specification
└── tasks.md             # Task breakdown (future)
```

### Source Code (repository root)

```text
internal/
├── subscription/        # NEW: Subscription management package
│   ├── manager.go     # SubscriptionManager implementation
│   ├── webhook.go     # Webhook subscription handler
│   ├── amqp.go        # AMQP subscription handler
│   ├── fs.go          # File system subscription handler
│   └── state.go       # Subscription state persistence
├── project/
│   └── project.go     # Add Subscription field to Todo struct
cmd/anvil/
│   └── main.go        # Add "subscription" subcommand
internal/
└── daemon/
    └── daemon.go       # Integrate subscriptions with task queue
```

**Structure Decision**: New `internal/subscription/` package to encapsulate all subscription logic. Minimal changes to existing daemon architecture.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | - | - |

## Implementation Approach

### 1. Add Subscription Field to Todo Struct

In `internal/project/project.go`, add `Subscription` struct and field to `Todo`. Support three types: webhook, amqp, fs.

### 2. Create Subscription Package

Create `internal/subscription/` with:
- `SubscriptionManager` - coordinates all subscription types
- `WebhookHandler` - HTTP server for webhooks
- `AMQPHandler` - RabbitMQ client
- `FSHandler` - File system watcher using fsnotify

### 3. Implement Webhook Handler

- Start HTTP server on configurable port (or share daemon port)
- Validate webhook paths from task config
- Verify secret if configured
- Pass payload via environment variable

### 4. Implement AMQP Handler

- Connect to AMQP broker
- Subscribe to configured queue
- Pass message body via environment variable
- Handle reconnection on disconnect

### 5. Implement FS Handler

- Use fsnotify to watch configured paths
- Support glob patterns for file matching
- Debounce rapid events
- Pass file path via environment variable

### 6. Add CLI Commands

- `anvil subscription ls` - list all subscriptions
- `anvil subscription pause <task>` - pause subscription
- `anvil subscription resume <task>` - resume subscription

### 7. Persist State

Store subscription state (active/paused) in `.anvil/subscriptions/state.json` for persistence across restarts.

### 8. Integrate with Daemon

Modify daemon to start subscription manager alongside task scheduler. When subscription triggers, add task to queue.

## Files to Modify

1. `internal/project/project.go` - Add `Subscription` field to `Todo` struct
2. `internal/subscription/` - NEW package for subscription handling
3. `cmd/anvil/main.go` - Add `subscription` subcommand
4. `internal/daemon/daemon.go` - Start/stop subscription manager

## Files to Create

1. `internal/subscription/manager.go` - Main subscription coordinator
2. `internal/subscription/webhook.go` - HTTP webhook handler
3. `internal/subscription/amqp.go` - AMQP message handler
4. `internal/subscription/fs.go` - File system watcher
5. `internal/subscription/state.go` - State persistence

## Testing Approach

1. Unit tests for Subscription struct parsing
2. Integration tests for each subscription type
3. Manual testing for CLI commands
4. End-to-end testing: trigger -> queue -> execute
