# Research: Cross-Project Dependency Status in Task Queue

## R1: How to resolve cross-project dependency status from the daemon

**Decision**: Use existing `ResolveDependencyRunRecord` from `internal/project/dependencies.go` within the daemon's `handleQueue` handler. The daemon already has access to project paths and can call this function to look up the last run record for cross-project dependencies.

**Rationale**: The `ResolveDependencyRunRecord` function already handles the full resolution chain: parse dependency string → resolve watched project path → find task ID → read run record from disk. No new resolution logic needed.

**Alternatives considered**:
- Querying remote daemon via HTTP: Rejected because it would require the remote daemon to be running, adding fragility.
- Caching cross-project status in the local daemon: Rejected as unnecessary complexity; disk-based run records are fast enough for <10 projects.

## R2: Where to resolve dependencies — daemon vs CLI

**Decision**: Resolve in the daemon's `handleQueue` handler, not in the CLI command.

**Rationale**: The daemon already has the project context (loaded todos with `DependsOn` fields) and manages the queue state. The CLI is a thin display layer. Resolving in the daemon keeps the architecture consistent and allows the JSON API to serve complete data to any consumer (TUI, scripts, etc.).

**Alternatives considered**:
- Resolving in `taskQueueCmd` (CLI side): Rejected because it would require the CLI to load project state independently, duplicating logic and making JSON output incomplete for other consumers.

## R3: How to handle --all flag behavior

**Decision**: The `--all` flag will cause the daemon to include cross-project dependency entries as additional queue items (not just metadata on existing tasks). Each cross-project dependency referenced by any local task will appear as a separate entry with its project name, task name, and last run status.

**Rationale**: This gives users a flat, scannable view of all tasks (local and cross-project) that affect the current project's execution. Without `--all`, cross-project info is shown only as metadata on local tasks that have cross-project dependencies.

**Alternatives considered**:
- Showing cross-project deps only as columns on local tasks: Considered but insufficient — users want to see the cross-project tasks as first-class entries to understand the full dependency picture.

## R4: Error handling for unreachable cross-project dependencies

**Decision**: Use descriptive status strings for error cases: "unknown project" when the project is not in watched directory, "task not found" when the task doesn't exist, "no runs" when no run records exist.

**Rationale**: The queue command should never crash due to broken cross-project references. Graceful degradation with clear status messages helps users diagnose configuration issues.

**Alternatives considered**:
- Omitting broken dependencies silently: Rejected because it would hide configuration errors from users.
