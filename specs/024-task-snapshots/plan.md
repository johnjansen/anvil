# Implementation Plan: Task Execution Snapshots for Debugging

**Branch**: `024-task-snapshots` | **Date**: 2026-03-01 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/024-task-snapshots/spec.md`

## Summary

Add task execution snapshots to capture complete runtime context (config, env vars, expanded prompt, directory listing) for every task run. Users can view snapshots via new CLI commands `anvil task snapshot` and compare snapshots with `anvil task snapshot-diff`. Snapshots are automatically pruned alongside existing run retention.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: Standard library + gopkg.in/yaml.v3 (existing)
**Storage**: JSON/YAML files in `.anvil/runs/<task-id>/<run-id>/snapshot/`
**Testing**: Go testing (go test)
**Target Platform**: macOS/Linux (CLI tool)
**Project Type**: CLI tool / daemon
**Performance Goals**: Snapshot creation < 2 seconds per run; view commands < 5 seconds
**Constraints**: Must work with existing daemon architecture; backward compatible
**Scale/Scope**: Single project, hundreds of tasks

## Constitution Check

*This is a small feature addition to an existing CLI. No complex gates apply.*

- **Single Project**: Yes - Go CLI with internal packages
- **No External Services**: Yes - all local file storage
- **Simple Testing**: Yes - standard Go tests

## Project Structure

### Source Code (repository root)

```text
cmd/anvil/
├── main.go              # CLI entry point
├── snapshot.go          # NEW: snapshot command
└── snapshot_diff.go     # NEW: snapshot-diff command

internal/
├── project/
│   ├── project.go       # RunRecord, task management
│   ├── retention.go     # Pruning logic (extend)
│   └── snapshot.go      # NEW: snapshot capture & storage
├── daemon/
│   └── daemon.go       # Task execution (add snapshot capture)
└── runner/
    └── runner.go       # Task runner (capture env vars, prompt)
```

**Structure Decision**: Add snapshot capture in daemon during task execution, store in new `snapshot/` subdirectory alongside existing run records, add CLI commands in `cmd/anvil/`.

## Phase 1: Design

### Data Model

**Snapshot**: Collection of files for a single task run
- `config.yaml` - Task configuration (frontmatter)
- `env.yaml` - Resolved environment variables
- `prompt.txt` - Expanded prompt text
- `files.json` - Directory listing at start
- `run_record.json` - Execution metadata (reuse existing RunRecord)

### CLI Commands

```
anvil task snapshot <name> [--run <id>] [--file <filename>]
anvil task snapshot-diff <name> --run1 <id1> --run2 <id2>
```

### Integration Points

1. **Daemon** (internal/daemon/daemon.go): Call snapshot capture after each run completes
2. **Retention** (internal/project/retention.go): Extend to prune snapshot directories
3. **CLI** (cmd/anvil/): Add new subcommands
