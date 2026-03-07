# 002-cli-ecosystem-blog-post Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-02-25

## Active Technologies
- Go 1.24.6 + `internal/cron` (cron parsing with `Prev()`), `internal/project` (Todo/RunRecord), `internal/config`, `internal/daemon` (004-task-sla-tracking)
- JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system) (004-task-sla-tracking)
- Go 1.24.6 + internal/cron, internal/project, internal/config, internal/daemon (340-task-wait-conditions)
- Go 1.24.6 + `internal/project` (Todo, RunRecord), `internal/daemon` (task execution, dependency checking), `internal/runner` (statusWriter, output scanning), `cmd/anvil` (CLI commands) (343-task-result-passing)
- Go 1.24.6 + `internal/project` (Todo, Dependency, RunRecord, ParseDependency, ResolveDependencyRunRecord), `internal/daemon` (TaskQueueInfo, handleQueue), `cmd/anvil` (taskQueueCmd) (265-cross-project-queue)
- JSON files in `.anvil/runs/<task-id>/` (existing RunRecord system), watched project paths via `~/.anvil/watched/` (265-cross-project-queue)
- Go 1.24.6 + `internal/project` (ParseDependency, Dependency, Todo, RunRecord), `cmd/anvil` (task_pipeline.go) (263-cross-project-pipeline)
- Go 1.24.6 + `internal/project` (Todo, RunRecord, frontmatter parsing), `internal/daemon` (retry loop in `executeTask`), `cmd/anvil` (task history/list display) (284-retry-backoff-jitter)
- JSON files in `.anvil/runs/<task-id>/` (RunRecord system) (284-retry-backoff-jitter)
- Go 1.24.6 + `net/http` (GitHub API), `gopkg.in/yaml.v3` (template parsing), existing `internal/project` (Template, TemplateSpec, LoadTemplate, ListTemplates) (303-template-registry)
- Local YAML files in `.anvil/templates/` (project) and `~/.anvil/templates/` (global); registry metadata stored alongside installed templates (303-template-registry)
- Go 1.24.6 + `internal/daemon` (task execution, kill handling), `internal/project` (Todo, RunRecord, frontmatter parsing), `internal/runner` (checkpoint capture), `cmd/anvil` (CLI commands) (291-kill-checkpoint)
- Go 1.24.6 + `github.com/fsnotify/fsnotify v1.9.0` (already in go.mod), `gopkg.in/yaml.v3` (frontmatter parsing) (364-file-watch-trigger)

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
- 364-file-watch-trigger: Added Go 1.24.6 + `github.com/fsnotify/fsnotify v1.9.0` (already in go.mod), `gopkg.in/yaml.v3` (frontmatter parsing)
- 291-kill-checkpoint: Added Go 1.24.6 + `internal/daemon` (task execution, kill handling), `internal/project` (Todo, RunRecord, frontmatter parsing), `internal/runner` (checkpoint capture), `cmd/anvil` (CLI commands)
- 303-template-registry: Added Go 1.24.6 + `net/http` (GitHub API), `gopkg.in/yaml.v3` (template parsing), existing `internal/project` (Template, TemplateSpec, LoadTemplate, ListTemplates)


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
