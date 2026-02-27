# Research: Task Workspace Isolation

## Decision 1: Enforcement Mechanism

**Decision**: Application-level enforcement via working directory and environment variables, not OS-level sandboxing.

**Rationale**: Anvil tasks are executed via `sh -c` with `cmd.Dir` set to the project root (runner.go:123). The simplest and most portable enforcement is:
- For `restricted` type: Validate paths at parse time, then set `cmd.Dir` to the project root and inject `ANVIL_WORKSPACE_*` env vars so tasks can self-enforce. The daemon validates workspace config on load.
- For `temp` type: Create a temp directory, set `cmd.Dir` to it, clean up after.
- For `project` (default): Set `cmd.Dir` to project root (current behavior, no change).

**Alternatives considered**:
- OS-level sandboxing (Capsicum, namespaces): Too platform-specific, deferred to future `jail` type.
- chroot: Requires root on Linux, not available on macOS without SIP complications.
- LD_PRELOAD interception: Fragile, doesn't work with statically linked binaries or Go programs.

## Decision 2: Workspace Config Location

**Decision**: Add `workspace` block to existing task frontmatter YAML, parsed into a `WorkspaceConfig` struct embedded in `Todo`.

**Rationale**: Follows existing pattern — all task config lives in frontmatter (schedule, timeout, env, allowed_tools, etc.). No new files needed. The `WorkspaceConfig` struct is parsed by `LoadTodos()` alongside existing fields.

**Alternatives considered**:
- Separate workspace config file: More complexity, breaks convention.
- Project-level only config: Doesn't allow per-task customization.

## Decision 3: Temp Workspace Size Enforcement

**Decision**: Pre-create temp directory with no runtime monitoring. Size limit is advisory — checked via a post-execution size audit logged as a warning.

**Rationale**: Real-time disk usage monitoring requires either polling (expensive) or filesystem quotas (OS-specific, requires privileges). A post-execution check is simple and sufficient for the advisory use case described in the spec.

**Alternatives considered**:
- Real-time polling with `du`: Performance overhead on large temp dirs.
- Filesystem quotas: Requires root, not portable.
- tmpfs with size limit: Linux-only, not available on macOS.

## Decision 4: Symlink Protection

**Decision**: Resolve symlinks at parse time using `filepath.EvalSymlinks()` and reject any that resolve outside the allowed workspace.

**Rationale**: Go stdlib provides `filepath.EvalSymlinks()` which resolves all symlinks to their real paths. Checking at config parse time (in `LoadTodos()`) catches issues early with a clear error message.

## Decision 5: Path Resolution

**Decision**: All workspace paths are resolved relative to the project root using `filepath.Join(proj.Path, configuredPath)` and then cleaned with `filepath.Clean()`. Paths that escape the project root after resolution are rejected.

**Rationale**: Consistent with how anvil already resolves task file paths. The `filepath.Clean` + prefix check pattern is standard Go for path containment validation.
