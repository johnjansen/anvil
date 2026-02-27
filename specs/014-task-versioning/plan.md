# Implementation Plan: Task Diff and Versioning

**Branch**: `014-task-versioning` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/014-task-versioning/spec.md`

## Summary

Add automatic task file versioning and CLI commands for viewing version history, diffing versions, restoring previous versions, and git blame integration. Version snapshots are stored as JSON files in `.anvil/versions/<task-name>/` and created automatically by the daemon when task file content changes.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: internal/project (Todo, RunRecord patterns), internal/daemon (tick loop), crypto/sha256, os/exec (git blame)
**Storage**: JSON files in `.anvil/versions/<task-name>/v{N}.json`
**Testing**: `go build ./...` (compile check)
**Target Platform**: Linux/macOS CLI
**Project Type**: CLI tool with daemon
**Performance Goals**: Version history display and diff in under 1 second
**Constraints**: No external dependencies; task files typically <1KB
**Scale/Scope**: Hundreds of tasks, dozens of versions per task

## Constitution Check

No project constitution configured. Proceeding without gates.

## Project Structure

### Documentation (this feature)

```text
specs/014-task-versioning/
  plan.md              # This file
  research.md          # Phase 0 output
  data-model.md        # Phase 1 output
  quickstart.md        # Phase 1 output
  contracts/
    cli.md             # CLI command contracts
  tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
  project/
    project.go         # Add TaskVersion struct, versionsDir(), WriteTaskVersion(), ReadAllVersions(), getAuthor()
    diff.go            # NEW: Unified diff algorithm
  daemon/
    daemon.go          # Add taskHashes map, version snapshot logic in tick()
cmd/
  anvil/
    main.go            # Add diff, restore, blame subcommands; --versions flag on history
```

**Structure Decision**: Follows existing Go package layout. New `diff.go` file in project package keeps diff logic separate from the large project.go file. All other changes are additions to existing files.

## Key Design Decisions

1. **Version storage**: Full snapshots as JSON files (follows RunRecord pattern)
2. **Change detection**: SHA256 hash comparison on each daemon tick
3. **Version numbering**: Sequential integers (v1, v2, v3...)
4. **Author detection**: git config user.name with os/user fallback
5. **Diff output**: Minimal unified diff implemented in Go (no external deps)
6. **CLI structure**: `diff`/`restore`/`blame` as taskCmd subcommands; `--versions` flag on history
7. **Daemon integration**: Version check in tick() after LoadTodos(), before dispatch

## Integration Points

| Component | File | Line | Action |
|-----------|------|------|--------|
| Daemon struct | daemon.go | ~160 | Add `taskHashes map[string]string` field |
| tick() | daemon.go | ~2271 | Add hash comparison + snapshot after LoadTodos() |
| taskCmd() | main.go | ~1847 | Add diff, restore, blame cases |
| taskHistoryCmd() | main.go | ~3524 | Add --versions flag handling |
| runsDir() pattern | project.go | ~689 | Model for versionsDir() |
| WriteRunRecord() pattern | project.go | ~704 | Model for WriteTaskVersion() |
| Todo struct | project.go | ~98 | Reference for task name/path |
