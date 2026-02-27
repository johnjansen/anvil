# Implementation Plan: Graceful Cancel with Partial Result Capture

**Branch**: `015-graceful-cancel` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/015-graceful-cancel/spec.md`

## Summary

Add graceful task cancellation with SIGTERM/on_kill hook support, partial result capture via `##anvil:partial` protocol, and resume-from-partial capability. Extends the existing kill mechanism with a `--graceful` flag, adds partial result storage to RunRecord, and provides `anvil task partial` and `--resume` commands.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: internal/daemon (kill handler, runTask, runHook), internal/runner (statusWriter), internal/project (RunRecord, Todo, frontmatter)
**Storage**: JSON files in `.anvil/runs/` (existing RunRecord system)
**Testing**: `go build ./...` (compile check)
**Target Platform**: Linux/macOS CLI
**Project Type**: CLI tool with daemon
**Constraints**: Must preserve backward compatibility -- existing `anvil task kill` without flags must behave identically

## Constitution Check

No project constitution configured. Proceeding without gates.

## Project Structure

### Documentation (this feature)

```text
specs/015-graceful-cancel/
  plan.md
  research.md
  data-model.md
  quickstart.md
  contracts/cli.md
  tasks.md
```

### Source Code (modified files)

```text
internal/
  project/
    project.go         # Add PartialResults+TerminationMethod to RunRecord, OnKill to Todo, on_kill frontmatter
  runner/
    runner.go          # Add ##anvil:partial prefix + onPartial callback to statusWriter
  daemon/
    daemon.go          # Extend handleKill for graceful, add on_kill hook, partial capture callback, resume support
cmd/
  anvil/
    main.go            # Add --graceful/-g/--force to kill, add partial cmd, add --resume to run
```

## Key Design Decisions

1. **Graceful mechanism**: SIGTERM to child process, then SIGKILL after grace period
2. **Partial protocol**: `##anvil:partial` prefix in statusWriter (follows ##anvil:status pattern)
3. **Partial storage**: `PartialResults` string field in RunRecord (alongside CheckpointData)
4. **On-kill hook**: `on_kill` frontmatter field, executed via existing `runHook()`
5. **Kill request**: Extended `KillRequest` with `Graceful bool` field
6. **Resume**: `--resume` flag injects `ANVIL_PARTIAL_RESULTS` env var from previous RunRecord
7. **Termination tracking**: `TerminationMethod` field in RunRecord

## Integration Points

| Component | File | Line | Action |
|-----------|------|------|--------|
| RunRecord struct | project.go | ~166 | Add PartialResults, TerminationMethod fields |
| Todo struct | project.go | ~137 | Add OnKill field |
| frontmatter parsing | project.go | ~264 | Add on_kill yaml field |
| statusWriter | runner.go | ~310 | Add ##anvil:partial prefix + onPartial callback |
| Runner.Run signature | runner.go | ~56 | Add onPartial callback parameter |
| KillRequest struct | daemon.go | ~172 | Add Graceful bool field |
| handleKill | daemon.go | ~1512 | Add graceful kill logic with SIGTERM + grace period |
| runTask | daemon.go | ~814 | Wire onPartial callback, store partial in var |
| runHook | daemon.go | ~1115 | Add ANVIL_IS_KILLED env var for on_kill context |
| run record write | daemon.go | ~950 | Set PartialResults and TerminationMethod fields |
| taskKillCmd | main.go | ~3402 | Add --graceful/-g/--force flags |
| taskCmd dispatcher | main.go | ~1847 | Add partial case |
| taskRunCmd | main.go | ~3343 | Add --resume flag |
| SendKillRequest | daemon.go | ~2889 | Add graceful parameter |
