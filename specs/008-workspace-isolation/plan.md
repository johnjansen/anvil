# Implementation Plan: Task Workspace Isolation

**Branch**: `008-workspace-isolation` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/008-workspace-isolation/spec.md`

## Summary

Add workspace isolation to anvil tasks so that tasks can be restricted to specific directories (restricted type), run in ephemeral temp directories (temp type), or default to project-only access (project type). Workspace configuration is specified in task frontmatter YAML and enforced at the application level via working directory setup and environment variable injection.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: gopkg.in/yaml.v3 (frontmatter parsing), os/exec (task execution), filepath (path resolution)
**Storage**: Filesystem (task files with YAML frontmatter, temp directories)
**Testing**: go test (existing test infrastructure)
**Target Platform**: macOS, Linux (cross-platform CLI)
**Project Type**: CLI tool with daemon
**Performance Goals**: < 1 second overhead for workspace setup per task execution
**Constraints**: No root/elevated privileges required, no OS-specific APIs
**Scale/Scope**: Per-task configuration, no concurrency concerns beyond existing daemon model

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is unconfigured (template only). No gates to evaluate — proceeding.

## Project Structure

### Documentation (this feature)

```text
specs/008-workspace-isolation/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── cli-contract.md  # Frontmatter schema, env vars, task get output
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── project/
│   └── project.go       # WorkspaceConfig struct, Todo extension, frontmatter parsing, path validation
├── daemon/
│   └── daemon.go        # Workspace setup before task execution (temp dir, env vars)
├── runner/
│   └── runner.go        # No changes needed (already accepts dir and extraEnv)
└── workspace/
    └── workspace.go     # NEW: Workspace validation, temp dir lifecycle, path resolution helpers
cmd/
└── anvil/
    └── main.go          # task get display extension for workspace info
```

**Structure Decision**: New `internal/workspace/` package encapsulates workspace logic (validation, temp dir management, env var generation). This keeps `project.go` focused on parsing and `daemon.go` focused on orchestration. The runner needs no changes — it already accepts `dir` and `extraEnv` parameters.

## Key Design Decisions

### 1. WorkspaceConfig as embedded struct in Todo

The `WorkspaceConfig` struct is embedded in `Todo` and parsed from a `workspace` YAML block in frontmatter. This follows the existing pattern for all task config (schedule, timeout, env, etc.).

### 2. Enforcement via working directory + environment variables

- **restricted**: Task runs in project root (`cmd.Dir = proj.Path`), with `ANVIL_WORKSPACE_*` env vars injected listing allowed/blocked paths. The task (or its runner) can use these for self-enforcement.
- **temp**: Task runs in a fresh temp directory (`cmd.Dir = tempDir`). Temp dir is created before execution and cleaned up after (success or failure).
- **project** (default): Task runs in project root. No env vars added. Current behavior preserved.

### 3. Path validation at parse time

All workspace paths are validated in `LoadTodos()`:
- Resolved relative to project root via `filepath.Join` + `filepath.Clean`
- Checked for project root containment via `strings.HasPrefix` after cleaning
- Symlinks resolved via `filepath.EvalSymlinks` and re-checked
- Invalid paths trigger a warning (task preserved with ParseError-like behavior)

### 4. Temp directory lifecycle in daemon

The daemon's `runTask()` function handles:
1. Create temp dir (`os.MkdirTemp`) before calling `runner.Run()`
2. Pass temp dir as `dir` parameter instead of `proj.Path`
3. Clean up temp dir in a `defer` (runs on success, failure, timeout, or panic)
4. Post-execution size check if `workspace.size` is configured (advisory warning only)

### 5. No changes to runner.go

The runner already accepts `dir string` and `extraEnv map[string]string` parameters. The daemon prepares the correct dir and env before calling the runner. This keeps the runner simple and focused.

## Complexity Tracking

No constitution violations — no complexity justification needed.
