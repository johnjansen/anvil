# Implementation Plan: File Watcher Trigger for Tasks

**Branch**: `364-file-watch-trigger` | **Date**: 2026-03-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/364-file-watch-trigger/spec.md`

## Summary

Extend the existing `FSWatcher` in `internal/daemon/fs.go` to support the full `file_watch` trigger type: glob pattern matching for watched paths, event type filtering (create/modify/delete), configurable debounce to collapse rapid changes, and passing batched file change information to triggered tasks. This builds on the existing `fsnotify`-based infrastructure and `SubscriptionConfig` system.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `github.com/fsnotify/fsnotify v1.9.0` (already in go.mod), `gopkg.in/yaml.v3` (frontmatter parsing)
**Storage**: JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system)
**Testing**: Go test (`go test ./...`)
**Target Platform**: macOS (FSEvents via fsnotify), Linux (inotify via fsnotify)
**Project Type**: CLI tool with background daemon
**Performance Goals**: Trigger latency <2s after debounce period; negligible idle resource usage
**Constraints**: Must work with existing `SubscriptionConfig` frontmatter; must not break existing `fs` subscription type
**Scale/Scope**: Per-project file watching; tens of watchers per daemon instance

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is unconfigured (template placeholders). No gates to evaluate. Proceeding.

## Project Structure

### Documentation (this feature)

```text
specs/364-file-watch-trigger/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── daemon/
│   └── fs.go            # Extend existing FSWatcher with glob, events, debounce
├── project/
│   └── project.go       # Extend SubscriptionConfig with events, debounce fields
└── runner/              # (no changes expected)

cmd/anvil/               # (no changes expected unless CLI status display needed)

tests/                   # New test files alongside source
├── internal/daemon/fs_test.go
└── internal/project/     # (existing tests)
```

**Structure Decision**: All changes extend existing files. No new packages or major structural changes needed. The `FSWatcher` in `internal/daemon/fs.go` is the primary implementation target.

## Complexity Tracking

No constitution violations to justify.
