# Implementation Plan: Task Activity Log

**Branch**: `017-task-activity-log` | **Date**: 2026-02-28 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/017-task-activity-log/spec.md`

## Summary

Add a task activity logging system that records all lifecycle events (created, run, paused, resumed, edited, killed, unlocked, force-run) to per-task JSONL files. Implement a new `anvil task activity` CLI command to view, filter, and export the activity log.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: internal/project (Todo, ActivityEntry, WriteActivity, ReadActivities), internal/daemon (run tracking), cmd/anvil (CLI commands)
**Storage**: JSONL files at .anvil/activities/<task-id>.jsonl
**Testing**: go test ./... (existing pattern)
**Target Platform**: CLI (macOS, Linux)
**Project Type**: CLI tool
**Performance Goals**: < 1 second for 1000 entries
**Constraints**: Append-only logging, must not slow down task execution

## Constitution Check

Constitution is a template (not project-specific). No gates to evaluate.

## Project Structure

### Source Code Changes

```text
internal/project/
├── project.go        # Modified: Add ActivityEntry struct, WriteActivity(), ReadActivities(), ActivitiesPath()

internal/daemon/
├── daemon.go         # Modified: Add activity logging in runTask(), handleKill(), handleRun()

cmd/anvil/
├── activity.go       # NEW: taskActivityCmd() with filtering, export, JSON output
├── main.go           # Modified: Add "activity" case to taskCmd(), add activity logging in create/pause/resume/edit/unlock commands
```

## Complexity Tracking

No new dependencies. Minimal new code (~300 lines for activity.go, ~50 lines for project.go additions, ~30 lines of logging calls scattered across existing code).
