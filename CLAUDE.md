# 002-cli-ecosystem-blog-post Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-02-25

## Active Technologies
- Go 1.24.6 + `internal/daemon` (RunningTask, workItem, handlers), `internal/project` (Todo, RunRecord), `internal/config`, `internal/runner` (checkpoint callback) (005-timeout-extension)
- In-memory for runtime state (RunningTask), JSON files for persistence (RunRecord) (005-timeout-extension)
- Go 1.24.6 + `internal/runner` (Runner.Run), `internal/project` (Todo, LoadTodos), `internal/config` (cost rates), `cmd/anvil/main.go` (CLI dispatch) (006-prompt-sandbox)
- N/A (sandbox produces no persistent state) (006-prompt-sandbox)

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
- 006-prompt-sandbox: Added Go 1.24.6 + `internal/runner` (Runner.Run), `internal/project` (Todo, LoadTodos), `internal/config` (cost rates), `cmd/anvil/main.go` (CLI dispatch)
- 005-timeout-extension: Added Go 1.24.6 + `internal/daemon` (RunningTask, workItem, handlers), `internal/project` (Todo, RunRecord), `internal/config`, `internal/runner` (checkpoint callback)

- 002-cli-ecosystem-blog-post: Added Markdown (GitHub-Flavored Markdown compatible with dev.to, Medium, and static blog platforms) + None (standalone markdown document)

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
