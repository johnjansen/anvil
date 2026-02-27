# Implementation Plan: Fix Raw Mode Line Break Output

**Branch**: `001-fix-raw-mode-linebreaks` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-fix-raw-mode-linebreaks/spec.md`

## Summary

Fix broken line breaks in daemon terminal output when running in raw terminal mode. The daemon logger (`dlog.println()`) writes `\n` directly to `os.Stdout` via `fmt.Fprintln`, bypassing the `rawLineWriter` wrapper that was only applied to `log.SetOutput()`. In raw terminal mode (OPOST disabled), bare `\n` doesn't return the cursor to column 0, causing horizontal drift. The fix adds a `rawMode` flag to `daemonLogger` so `println()` writes `\r\n` when raw mode is active.

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**: `golang.org/x/term` (terminal raw mode), standard library only
**Storage**: N/A (log file output via `daemonLogger`)
**Testing**: `go test ./...`
**Target Platform**: macOS, Linux (terminal-based CLI)
**Project Type**: CLI tool / daemon
**Performance Goals**: N/A (logging output, not performance-sensitive)
**Constraints**: Must not regress daemonized/background mode output
**Scale/Scope**: 2-file change (logger.go + main.go)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is a blank template — no project-specific gates defined. No violations possible. Proceeding.

## Project Structure

### Documentation (this feature)

```text
specs/001-fix-raw-mode-linebreaks/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── spec.md              # Feature specification
└── checklists/
    └── requirements.md  # Quality checklist
```

### Source Code (repository root)

```text
internal/
├── daemon/
│   ├── daemon.go        # Worker goroutines, Run() entry point
│   ├── logger.go        # daemonLogger — PRIMARY CHANGE TARGET
│   ├── logger_test.go   # Existing tests
│   └── notify.go        # Desktop notifications
├── runner/
│   └── runner.go        # Task execution (uses log.Printf — already handled)
└── ...

cmd/
└── anvil/
    └── main.go          # serveCmd() raw mode setup — SECONDARY CHANGE TARGET
```

**Structure Decision**: Existing project structure. Changes confined to `internal/daemon/logger.go` (add rawMode flag + SetRawMode method) and `cmd/anvil/main.go` (call SetRawMode alongside existing log.SetOutput wrapper).

## Complexity Tracking

No violations to justify — this is a minimal 2-file bug fix.
