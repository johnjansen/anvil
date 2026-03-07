# Research: Remove Task Dependency Pipeline

**Feature**: 367-remove-dependency-pipeline
**Date**: 2026-03-07

## Findings

### Decision: Full removal of dependency pipeline code

**Rationale**: The feature is fundamentally broken — dependencies only evaluate on the dependent task's own cron tick, not event-driven. A 4-step chain at 6-hour cadence takes 24 hours. The team has already collapsed multi-task pipelines into single tasks as a workaround.

**Alternatives considered**:
- Fix with event-driven cascading: Rejected — adds significant complexity for a feature nobody uses
- Deprecation warnings before removal: Rejected — feature is already broken, no active users to migrate

### Decision: Silent ignore for stale depends_on fields

**Rationale**: YAML frontmatter parsing via `gopkg.in/yaml.v3` with struct tags will naturally ignore unknown fields. Removing the `DependsOn` and `DependencyPolicy` fields from the frontmatter struct means the parser silently drops them. No explicit ignore logic needed.

**Alternatives considered**:
- Emit deprecation warning: Rejected — creates noise, feature was never usable
- Error on unknown fields: Rejected — breaks backward compatibility unnecessarily

### Decision: Preserve historical run records

**Rationale**: RunRecord JSON files in `.anvil/runs/<task-id>/` are self-contained. Dependency-triggered runs are already recorded with their own data. No migration or cleanup needed.

## Code Inventory

### Files to delete entirely:
1. `internal/project/dependencies.go` — all dependency types, parsing, resolution, validation, graph/cycle detection
2. `internal/project/dependencies_test.go` — all dependency tests
3. `cmd/anvil/task_pipeline.go` — pipeline CLI command and visualization

### Files to modify:
1. **`internal/project/project.go`** — Remove `DependsOn`, `DependencyPolicy`, `DependencyPolicyConfig` from `Todo` struct; remove from frontmatter parsing struct; remove from task file generation
2. **`internal/daemon/daemon.go`** — Remove `depFailInfo` type, `checkDependenciesMet()` function, dependency collection in task execution, cross-project validation, cycle detection, dependency checking in dispatch
3. **`cmd/anvil/task_create.go`** — Remove `--depends-on` flag and dependency validation
4. **`cmd/anvil/task_results.go`** — Remove dependency results display
5. **`cmd/anvil/task_list.go`** — Remove dependency display in list output (JSON and text)
6. **`cmd/anvil/dryrun.go`** — Remove dependency section from dry-run output
7. **`cmd/anvil/task_router.go`** — Remove `pipeline` subcommand routing
8. **`cmd/anvil/main.go`** — Remove `pipeline` from help text
9. **`tools/skills/anvil/SKILL.md`** — Remove `--depends-on` and `pipeline` command docs
10. **`CLAUDE.md`** — Remove 263-cross-project-pipeline and 265-cross-project-queue technology references
