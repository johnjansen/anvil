# 002-cli-ecosystem-blog-post Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-02-25

## Active Technologies
- Go 1.24.6 + `internal/cron` (cron parsing with `Prev()`), `internal/project` (Todo/RunRecord), `internal/config`, `internal/daemon` (004-task-sla-tracking)
- JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system) (004-task-sla-tracking)
- Go 1.24.6 + internal/cron, internal/project, internal/config, internal/daemon (340-task-wait-conditions)
- Go 1.24.6 + `internal/project` (Todo, RunRecord), `internal/daemon` (task execution, dependency checking), `internal/runner` (statusWriter, output scanning), `cmd/anvil` (CLI commands) (343-task-result-passing)

- Markdown (GitHub-Flavored Markdown compatible with dev.to, Medium, and static blog platforms) + None (standalone markdown document) (002-cli-ecosystem-blog-post)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Markdown (GitHub-Flavored Markdown compatible with dev.to, Medium, and static blog platforms)

## Code Style

Markdown (GitHub-Flavored Markdown compatible with dev.to, Medium, and static blog platforms): Follow standard conventions

## Recent Changes
- 343-task-result-passing: Added Go 1.24.6 + `internal/project` (Todo, RunRecord), `internal/daemon` (task execution, dependency checking), `internal/runner` (statusWriter, output scanning), `cmd/anvil` (CLI commands)
- 275-task-forecasting: Added Go 1.24.6 + internal/cron, internal/project, internal/config, internal/daemon
- 340-task-wait-conditions: Added Go 1.24.6 + internal/cron, internal/project, internal/config, internal/daemon


<!-- MANUAL ADDITIONS START -->

## Development Workflow

All features and non-trivial bug fixes MUST follow the speckit workflow:

1. `/speckit.specify` — Create feature spec from issue description
2. `/speckit.plan` — Generate technical implementation plan
3. `/speckit.tasks` — Break plan into dependency-ordered tasks
4. `/speckit.implement` — Execute tasks with validation

Trivial fixes (1-2 line changes) may skip speckit and implement directly.

The automated planning task in `.anvil/todos/` follows this same workflow when picking up GitHub issues.

<!-- MANUAL ADDITIONS END -->
