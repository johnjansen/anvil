# Implementation Plan: Git Event Trigger for Tasks

**Branch**: `365-git-event-trigger` | **Date**: 2026-03-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/365-git-event-trigger/spec.md`

## Summary

Add a `git` subscription type to the daemon that polls git refs to detect new commits. When a ref change is detected, the daemon evaluates branch and path filters, then triggers the associated task with git context (commit SHA, branch, previous SHA) passed as environment variables. Follows the existing subscription pattern established by FSWatcher, WebhookServer, and AMQPConsumer.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `os/exec` (git CLI invocation), `gopkg.in/yaml.v3` (frontmatter parsing), `path/filepath` (glob matching)
**Storage**: JSON files in `.anvil/runs/<task-id>/` (RunRecord system); git ref state persisted in `.anvil/git-state/` as JSON
**Testing**: `go test` with table-driven tests
**Target Platform**: macOS, Linux (anywhere git CLI is available)
**Project Type**: CLI tool with background daemon
**Performance Goals**: Each poll cycle completes in under 1 second for a typical repository
**Constraints**: Must not require additional Go dependencies; uses git CLI already available on PATH
**Scale/Scope**: Single repository per daemon instance; multiple git-triggered tasks per repo

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is not customized for this project (template placeholders only). No specific gates to evaluate. Proceeding with standard engineering best practices.

**Post-design re-check**: Design follows existing subscription patterns (FSWatcher, WebhookServer, AMQPConsumer). No new abstractions introduced. No violations.

## Project Structure

### Documentation (this feature)

```text
specs/365-git-event-trigger/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── daemon/
│   ├── daemon.go        # Existing: add GitWatcher field, startSubscriptions case "git"
│   └── git.go           # NEW: GitWatcher struct, polling loop, ref tracking
├── project/
│   └── project.go       # Existing: SubscriptionConfig already has flexible fields
│   └── trigger.go       # Existing: no changes needed (uses SubscriptionConfig)
```

**Structure Decision**: Single new file `internal/daemon/git.go` following the FSWatcher pattern. Minimal changes to `daemon.go` for registration. No new packages or abstractions.

## Complexity Tracking

No constitution violations to justify.
