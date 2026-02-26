<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg" width="120">
    <source media="(prefers-color-scheme: light)" srcset="assets/logo.svg" width="120">
    <img alt="Anvil" src="assets/logo.svg" width="120">
  </picture>
</p>

# Anvil

Scheduled LLM tasks for your projects. Write a task in plain English, give it a cron schedule, and anvil runs it automatically through Claude.

## Claude Skill

Anvil includes a [Claude skill](https://github.com/johnjansen/anvil/blob/main/tools/skills/anvil/SKILL.md) that can automatically create scheduled jobs for you. Just describe what you want automated, and the skill will generate the task file with the proper schedule and content.

**Note:** Tasks only run when `anvil watch` is running. Task files persist on disk, so one-shot tasks will execute when the daemon next starts. Scheduled tasks only run at their cron times — missed windows are not replayed.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/johnjansen/anvil/main/install.sh | sh
```

Or with Go:

```bash
go install github.com/johnjansen/anvil/cmd/anvil@latest
```

## Get Started

```bash
# 1. Start the daemon (once per machine)
anvil watch

# 2. Initialize any project
cd my-project
anvil init

# 3. Add a task
anvil add -s "*/30 * * * *" "Check GitHub for new issues and triage them with labels"
```

That's it. Anvil picks up the task on schedule and runs it through Claude.

## What You Can Automate

```bash
# Triage GitHub issues every 30 minutes
anvil add -s "*/30 * * * *" "Check GitHub for new untriaged issues. For each unlabelled issue, read the content and apply appropriate labels (bug, feature, docs, question)."

# Review stale PRs on weekday mornings
anvil add -s "0 9 * * 1-5" "Review open pull requests older than 3 days. Post a summary comment on each and ping the author if no activity in 48h."

# Sync documentation after hours
anvil add -s "0 2 * * *" "Read the latest CHANGELOG.md and update README sections that reference version numbers or new features."

# Run once, right now
anvil add -s "" "Audit all TODO comments in the codebase and file GitHub issues for the ones that need attention."
```

## How It Works

One daemon runs on your machine and watches all your projects. Each project has a `.anvil/todos/` directory with task files. The daemon checks schedules every few seconds and dispatches tasks to a worker pool.

- **One daemon, many projects** — `anvil watch` once per machine
- **Tasks are markdown files** — your prompt is the file body
- **Priority queue** — p0 runs before p1, p1 before p2, etc.
- **Already running? Skipped** — no duplicate runs
- **Session continuity** — recurring tasks resume where they left off

## Task Options

### One-shot tasks

Pass an empty schedule to run once and delete:

```bash
anvil add -s "" "Migrate the database schema to add the new users table"
```

### Persistent tasks

Pass `persistent` as the schedule to run continuously:

```bash
anvil add -s "persistent" "Monitor a queue and process items as they arrive"
```

Persistent tasks are designed for event-driven workflows. Here's how they work:

1. **Each cycle is a fresh run** — The task executes, completes its work, and exits. A new process starts on the next scheduler tick.

2. **Immediate re-dispatch** — When the task exits, the scheduler re-dispatches it on the next tick (by default, every 10 seconds). This allows the task to check for new work frequently without blocking a worker between jobs.

3. **Configure behavior** — Use `persistent_cooldown` to wait between cycles, `persistent_max_runtime` to force restart after a maximum runtime, and `persistent_budget` to limit cumulative runtime:

```yaml
---
schedule: "persistent"
persistent_cooldown: 5s      # wait between restart cycles (default: 0 = immediate)
persistent_max_runtime: 10m  # max runtime before forced restart (default: 0 = no limit)
persistent_budget: 1h        # cumulative budget per daemon lifetime (default: 0 = unlimited)
---
```

- `persistent_cooldown` — wait time after a persistent task completes before re-dispatching. Default is 0 (immediate restart).
- `persistent_max_runtime` — maximum runtime before the task is forcibly restarted. Useful for preventing runaway tasks. Default is 0 (no limit).
- `persistent_budget` — cumulative wall-clock time budget per daemon lifetime. Once exhausted, the task stops and requires manual restart. Default is 0 (unlimited).

#### Starvation prevention

If a persistent task waits more than 5 minutes for a worker slot, it temporarily yields to let higher-priority work through. This prevents low-priority persistent tasks from blocking important cron jobs indefinitely.

### Task Templates

Use templates to create tasks with predefined configurations:

```bash
# List available templates
anvil template ls

# Show template details
anvil template get <name>

# Create a task from a template
anvil add -s "*/30 * * * *" -t <template-name> "Task description"
```

Templates are stored in:
- `.anvil/templates/` — project-specific templates
- `~/.anvil/templates/` — global templates shared across all projects

A template is a YAML file with task configuration:

```yaml
# .anvil/templates/daily-standup.yaml
name: daily-standup
schedule: "0 9 * * 1-5"
priority: 1
allowed_tools:
  - Bash
  - Read
  - Write
```

Template values can be overridden by CLI flags. For example, `-t daily-standup -s "0 10 * * * *"` uses the template's allowed_tools but overrides the schedule.

### Priority

Lower number = higher priority. Default is `p1`:

```bash
anvil add -p 0 -s "*/5 * * * *" "Critical: alert if production error rate exceeds threshold"
anvil add -p 5 -s "0 9 * * 1"   "Low priority: weekly dependency audit"
```

### Limit concurrent runs

Prevent a task from running more than N instances simultaneously:

```yaml
---
schedule: "*/5 * * * *"
max_concurrent: 2
---
Run analysis jobs but cap at 2 parallel instances.
```

Default is 1 (no parallel runs of the same task). Omit or set to 0 to use the default.

### Gate execution with a pre-check

Skip the LLM call entirely when there's nothing to do:

```yaml
---
schedule: "*/30 * * * *"
pre_check: "gh issue list --state open --label untriaged | grep -q ."
---
Triage untriaged GitHub issues...
```

The task only runs if `pre_check` exits 0. Saves LLM calls when the queue is empty.

### Tool permissions

Pre-approve specific tools instead of bypassing all checks:

```yaml
allowed_tools:
  - Bash(gh:*)   # only gh subcommands
  - Read
  - Write
```

Or skip all permission prompts:

```yaml
skip_permissions: true
```

### Lifecycle hooks

Run shell commands after a task succeeds or fails:

```yaml
---
schedule: "*/30 * * * *"
on_success: "echo 'Task completed' >> /tmp/anvil.log"
on_failure: "curl -X POST https://slack.example.com/webhook -d '{\"text\":\"Task failed\"}'"
---
Triage GitHub issues...
```

Hooks run in the project directory with a 60-second timeout. The following environment variables are available:

| Variable | Description |
|----------|-------------|
| `ANVIL_TASK_NAME` | Task filename |
| `ANVIL_EXIT_CODE` | `0` for success, `1` for failure |
| `ANVIL_LOG_PATH` | Path to the raw log file |
| `ANVIL_PROJECT` | Project directory path |
| `ANVIL_SESSION_ID` | Claude session ID used |
| `ANVIL_START_TIME` | RFC 3339 start timestamp |
| `ANVIL_END_TIME` | RFC 3339 end timestamp |
| `ANVIL_ELAPSED_MS` | Elapsed time in milliseconds |

Hook errors are logged as warnings but do not affect the task outcome.

### Pause/Resume tasks

Set `disabled: true` in the frontmatter to pause a task without deleting it:

```yaml
---
schedule: "*/30 * * * *"
disabled: true
---
Temporarily paused task...
```

The task is skipped during tick evaluation but remains in the system. Set `disabled: false` or remove the line to resume.

### Task timeout

Override the global timeout for a specific task:

```yaml
---
schedule: "*/30 * * * *"
timeout: 15m
---
Run with a 15-minute timeout instead of the default.
```

Valid units: `s` (seconds), `m` (minutes), `h` (hours). Set to `0` or omit to use the global default (configured in `~/.anvil/config.yaml`, default: 5 minutes).

### Retry on failure

Automatically retry failed tasks with exponential backoff:

```yaml
---
schedule: "*/30 * * * *"
retry: 3
retry_delay: 2m
---
Run with up to 3 retries, waiting 2 minutes between attempts.
```

- `retry`: Number of retries on failure (0 = no retry, default: 0)
- `retry_delay`: Initial delay between retries (default: 1m)

The retry delay uses exponential backoff: the initial delay doubles after each retry attempt (delay * 2^attempt). For example, with retry_delay=1m, retries occur at 1m, 2m, 4m, etc.

### Task labels

Add labels to tasks for organization and filtering:

```yaml
---
schedule: "*/30 * * * *"
labels:
  - triage
  - github
  - automated
---
Triage GitHub issues...
```

Or set labels via `anvil task edit`:

```bash
anvil task edit <name> --add-label triage
anvil task edit <name> --remove-label archived
```

Labels can be used to filter tasks in `anvil task ls`:

```bash
# Show only tasks with the "triage" label
anvil task ls --label triage

# Show tasks with any of these labels
anvil task ls --label triage,github

# Exclude tasks with a label
anvil task ls --label !archived
```

### Per-task runner override

Override the global runner chain for a specific task:

```yaml
---
schedule: "*/30 * * * *"
runner: "claude -p 'You are a specialized assistant'"
---
Run this task with a different runner command than the global default.
```

The task-level runner is used instead of the global `runners` list for this task only.

### Environment Variables

Set environment variables that will be injected into the task's execution environment:

```yaml
---
schedule: "*/30 * * * *"
env:
  GITHUB_TOKEN: "env:GITHUB_TOKEN"
  CUSTOM_VAR: "my-value"
---
Run with environment variables...
```

- Prefix a value with `env:` to inherit from the current environment (e.g., `env:GITHUB_TOKEN` reads the `GITHUB_TOKEN` env var)
- Use literal strings directly for custom values
- Environment variables are available in hooks, pre_check commands, and the task itself

### Task Dependencies

Set task dependencies to ensure a task only runs after its dependencies have completed successfully:

```yaml
---
schedule: "*/30 * * * *"
depends_on:
  - fetch-data
  - process-data
---
Run after dependencies complete...
```

The task will only be dispatched when all tasks in `depends_on` have completed successfully (exit code 0) in their most recent run. If any dependency failed, the dependent task is skipped silently.

### Task Checkpointing

Enable checkpoint to persist state between runs. Tasks emit checkpoint data via a special stdout prefix, and the daemon injects it as an environment variable on the next run:

```yaml
---
schedule: "*/30 * * * *"
checkpoint: true
---
Process items starting from where we left off...
```

During execution, the task emits state:

```
##anvil:checkpoint {"last_processed_id": 42, "cursor": "abc123"}
```

On the next run, the daemon sets `ANVIL_CHECKPOINT_DATA` to the last emitted checkpoint value. The task reads this to resume from where it left off. Checkpoint lines are stripped from log output — they never appear in log files.

- `checkpoint: true` — enable checkpointing for this task (default: false)
- Multiple `##anvil:checkpoint` lines can be emitted; the last one wins
- Checkpoint data is stored in the run record and visible via `anvil task get`

### Task State Management

For long-running data processing tasks, use state buckets to persist arbitrary JSON state between runs:

```yaml
---
schedule: "*/30 * * * *"
state:
  bucket: "data-processing"
  key: "cursor-{{ .TaskID }}"
---
Process items from where we left off...
```

The task reads/writes a JSON state file via `ANVIL_STATE_FILE` environment variable:

```python
import json
import os

# Read previous state
with open(os.environ['ANVIL_STATE_FILE'], 'r') as f:
    state = json.load(f)

# Update state
state['last_processed_id'] = 42

# Write back (automatically persisted after task completes)
with open(os.environ['ANVIL_STATE_FILE'], 'w') as f:
    json.dump(state, f)
```

- `bucket` — named bucket for grouping related state (shared across tasks)
- `key` — unique key within the bucket (supports `{{ .TaskID }}` template variable)

State is automatically persisted to `.anvil/state/<bucket>/<key>.json` in the project directory after each run. Multiple tasks can share the same bucket but have different keys.

### Webhook Notifications

Configure HTTP webhooks to receive notifications for task lifecycle events:

```yaml
webhooks:
  slack:
    url: "https://hooks.slack.com/services/xxx"
    method: "POST"  # default: POST
    headers:
      Authorization: "Bearer xxx"
    events: ["success", "failure", "start", "timeout", "persistent_cycle"]
    timeout: 10s  # default: 10s
```

Supported events:
- `start` — task execution started
- `success` — task completed successfully
- `failure` — task failed
- `timeout` — task timed out
- `persistent_cycle` — persistent task completed a cycle

You can also set a per-task webhook:

```yaml
---
schedule: "*/30 * * * *"
webhook: "https://hooks.slack.com/services/xxx"
---
Triage GitHub issues...
```

## Desktop Notifications

Configure native desktop notifications for task events:

```yaml
notifications:
  enabled: true           # master switch
  on_failure: true        # notify on task failure (default when enabled)
  on_success: false       # notify on task success
  on_budget_warning: true # notify when persistent task budget is exhausted
  persistent_cycle: false  # notify on persistent task cycle completion
  # command: ""  # optional custom command (overrides platform default)
```

Supported platforms:
- **macOS**: Uses `osascript` to display notifications
- **Linux**: Uses `notify-send`
- **Windows**: Uses PowerShell toast notifications
- **Custom**: Set `command` to use a custom notification tool

Custom command supports `{title}` and `{message}` placeholders:

```yaml
notifications:
  enabled: true
  command: "echo '{title}: {message}' | logger"
```

You can also override notifications per-task:

```yaml
---
schedule: "*/30 * * * *"
notify_on_failure: false
notify_on_success: true
---
Triage GitHub issues...
```

Notifications fire asynchronously after webhooks, so they never block task execution.

## Configuration

`~/.anvil/config.yaml`:

```yaml
runners:
  - claude
max_workers: 10    # parallel tasks (max_todos is deprecated)
timeout: 15m       # max per task
tick_interval: 10s  # how often to check for work
input_token_rate: 3.0    # cost per 1M input tokens in USD (default: 3.0)
output_token_rate: 15.0  # cost per 1M output tokens in USD (default: 15.0)
auto_update: false       # opt-in: auto-update binary on daemon startup
graceful_shutdown_timeout: 5m  # max wait for running tasks on graceful stop (default: 5m)
retention:
  max_age: 7d      # delete logs older than 7 days
  max_runs: 50     # keep only last 50 runs per task
  max_log_size: 50mb  # max size per log file (0 = unlimited)
hooks:
  on_success: "echo 'Task completed' >> ~/.anvil/history.log"
  on_failure: "curl -X POST https://example.com/webhook -d '{\"text\":\"Task failed\"}'"
env:
  GITHUB_TOKEN: "env:GITHUB_TOKEN"
  CUSTOM_VAR: "my-value"
```

Global hooks run for all tasks. Task-level hooks override global hooks for that specific task.

Global `env` sets environment variables for all tasks. Prefix a value with `env:` to inherit from the current environment (e.g., `env:GITHUB_TOKEN` reads the `GITHUB_TOKEN` env var). Task-level `env` overrides global `env` for that specific task.

Multiple runners with fallback:

```yaml
runners:
  - claude -p "you are a helpful task runner"
  - claude --model haiku -p "you are a helpful task runner"
```

### Hot Reload

Reload the daemon configuration without restarting:

```bash
anvil reload
```

Or with graceful option to wait for running tasks:

```bash
anvil reload --graceful
anvil reload --graceful --timeout 60s
```

Or send SIGHUP manually:

```bash
kill -HUP $(cat ~/.anvil/daemon.pid)
```

The daemon will reload `~/.anvil/config.yaml` and apply changes to `max_workers`, `timeout`, `runners`, and `tick_interval`. Running tasks are not affected unless `--graceful` is used — in that case, the daemon waits for running tasks to complete before reloading.

### Rate Limiting

Configure rate limiting for LLM API calls to prevent exceeding provider limits:

```yaml
rate_limit:
  max_concurrent_calls: 10    # max concurrent LLM API calls (default: unlimited)
  requests_per_minute: 60    # max API requests per minute (default: unlimited)
  requests_per_hour: 1000    # max API requests per hour (default: unlimited)
  burst: 20                  # allow short bursts above rate (default: 10)
  provider:
    claude:
      requests_per_minute: 50
    openai:
      requests_per_hour: 500
```

- `max_concurrent_calls` — Maximum number of tasks that can make LLM API calls simultaneously
- `requests_per_minute` — Maximum API requests allowed per minute across all tasks
- `requests_per_hour` — Maximum API requests allowed per hour across all tasks
- `burst` — Allows short bursts above the configured rate (default: 10)
- `provider` — Set different limits per LLM provider (uses the first word of the runner command)

When rate limited, tasks are skipped and re-queued on the next tick. The skip reason is visible in `anvil task queue`.

### Project-level Configuration

Each project can have its own `.anvil/config.yaml` to set defaults for all tasks in that project:

```yaml
# .anvil/config.yaml (project-level)
defaults:
  timeout: 10m
  retry: 2
  retry_delay: 1m
  max_concurrent: 2
  allowed_tools:
    - Bash
    - Read
    - Write
  pre_check: "test -f /tmp/running"
  on_success: "echo 'done'"
  on_failure: "echo 'failed'"
  skip_permissions: false
  persistent_cooldown: 5s
  persistent_max_runtime: 30m
  persistent_budget: 1h
  max_log_size: 50mb
  runner: "claude -p 'You are a task runner'"
```

Task-level frontmatter overrides project defaults. Global hooks from `~/.anvil/config.yaml` apply to all tasks unless overridden at the project or task level.

### Log Retention

Configure automatic cleanup of old logs and session data in `~/.anvil/config.yaml`:

```yaml
retention:
  max_age: 7d    # delete logs older than 7 days
  max_runs: 50   # keep only last 50 runs per task
  max_log_size: 50mb  # max size per log file (0 = unlimited)
```

Or run cleanup manually:

```bash
# Preview what would be deleted
anvil cleanup --older-than=3d --dry-run

# Shorthand for dry-run
anvil cleanup --older-than=3d -n

# Actually delete logs older than 3 days
anvil cleanup --older-than=3d

# Use shorter duration syntax
anvil cleanup -o=24h
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `anvil init [path]` | Initialize a project |
| `anvil register [path]` | Register a project for watching (without full init) |
| `anvil watch` | Start the daemon |
| `anvil watch [-d|--daemonize]` | Start daemon in background |
| `anvil watch --stop` | Stop the running daemon |
| `anvil watch --stop --graceful` | Stop daemon gracefully (wait for running tasks) |
| `anvil watch --stop --force` | Stop daemon immediately (kill running tasks) |
| `anvil watch --restart` | Restart the daemon |
| `anvil watch --restart --graceful` | Restart gracefully (wait for running tasks) |
| `anvil watch --install` | Install as system service (auto-start on boot) |
| `anvil watch --uninstall` | Remove the system service |
| `anvil watch --status` | Show system service status |
| `anvil add [opts] <task>` | Add a task (`-s schedule`, `-p priority 0-9`, `-o|--once`, `-f file`, `-` stdin, `-n|--dry-run`, `-t|--template`, `--pre-check`, `--allowed-tools`, `--max-concurrent`, `--skip-permissions`, `--strict`, `--no-overlap-check`) |
| `anvil template ls` | List available task templates |
| `anvil template get <name>` | Show template details |
| `anvil logs [<name>]` | Raw worker output (all tasks or one) |
| `anvil daemon log` | View daemon log (-f to follow, -n for lines, --level, --match, --since, --until) |
| `anvil daemon config-validate [--show]` | Validate config file (--show to display parsed config) |
| `anvil ps [--json] [-w|--watch]` | Show running tasks (--watch for live updates) |
| `anvil status` | Show watched projects |
| `anvil reload` | Reload daemon configuration |
| `anvil reload --graceful` | Reload daemon gracefully (wait for running tasks) |
| `anvil stop-on-idle` | Drain running tasks then exit the daemon |
| `anvil cleanup [--older-than=<duration>] [-n\|--dry-run]` | Prune old logs and session data (use --older-than=3d format with equals sign) |
| `anvil update [--check]` | Update to latest release |
| `anvil usage [--project <path>] [--task <name>] [--since <date>]` | Show LLM token usage and estimated costs |
| `anvil version` | Show version |

**Task management:**

| Command | Description |
|---------|-------------|
| `anvil task ls` | List tasks in current project |
| `anvil task ls [-a|--all] [--json] [--label L] [--match P]` | List tasks across all projects with optional filtering |
| `anvil task get <name> [--json]` | Show task details |
| `anvil task run <name>` | Trigger immediate execution (bypass cron) |
| `anvil task log <name>` | Show execution log |
| `anvil task log -f <name>` | Follow live log output |
| `anvil task rm <name>` | Remove a task |
| `anvil task kill <name>` | Kill a running task |
| `anvil task stop-on-idle <name>` | Finish current run then stop rescheduling |
| `anvil task unlock <name>` | Remove stale lock file to allow retry |
| `anvil task history <name> [-n limit]` | Show run history (default 10 runs) |
| `anvil task history <name> -f` | Follow mode - watch for new runs |
| `anvil task history <name> --failures` | Show only failed runs |
| `anvil task history <name> --json` | Output in JSON format |
| `anvil task queue` | Show daemon queue status and skip reasons |
| `anvil task edit <name> [-s schedule] [-p priority] [--content text] [--content-file path] [--add-label L] [--remove-label L]` | Edit task schedule, priority, content, or labels |
| `anvil task pause <name>` | Pause a task (sets disabled: true) |
| `anvil task resume <name>` | Resume a paused task (sets disabled: false) |
| `anvil task timeout [name]` | Show task timeout progress (--all for all tasks) |
| `anvil task next [name]` | Show next scheduled run time (--all for all projects) |
| `anvil task wait <name> [--timeout D] [--match PAT]` | Block until a running task completes (exit 0=ok, 1=fail, 2=timeout) |
| `anvil task analyze [--all]` | Analyze task schedules for potential conflicts |
| `anvil task pipeline [--dot|--verbose] [--all]` | Visualize task dependency pipelines |
| `anvil task reset-budget <name>` | Reset persistent task budget consumption |
| `anvil task state <bucket> [key]` | Show task state bucket contents |
| `anvil task state <name> [--get|--set <key=value>] [--delete]` | Manage task state buckets |
| `anvil task start <name>` | Start a stopped task (re-enable rescheduling) |
| `anvil task stop <name>` | Stop a running task (disable rescheduling) |
| `anvil task find <pattern>` | Find tasks by name pattern |
| `anvil task export [names...] [-a\|--all] [-o file]` | Export tasks to JSON |
| `anvil task import <file> [--base-path path] [-n\|--dry-run] [-f\|--force]` | Import tasks from JSON |

**Task status:**

| Status | Meaning |
|--------|---------|
| `idle` | Task is queued but not running |
| `running` | Task is currently executing |
| `disabled` | Task is paused (set `disabled: true` in frontmatter); use `anvil task resume <name>` to re-enable |
| `locked` | Stale lock file found (daemon crashed mid-execution); use `unlock` to allow retry |

**Project management:**

| Command | Description |
|---------|-------------|
| `anvil project create [path]` | Initialize and watch a project in one step |
| `anvil project ls` | List watched projects |
| `anvil project ls [-a|--all]` | List watched projects |
| `anvil project get [path]` | Show project and running tasks |
| `anvil project rm [path] [--clean]` | Unwatch a project (--clean removes .anvil/ too) |

## Runtime Status Reporting

Tasks can report their current status to the daemon by printing a special line to stdout:

```
##anvil:status Triaging 3 new issues
```

The daemon picks up the status text and displays it in `anvil task ls` and heartbeat logs. Status lines are stripped from the task's output — they never appear in log files. Use this to give visibility into long-running tasks:

```python
print("##anvil:status Scanning repository...")
# ... do work ...
print("##anvil:status Found 5 issues, triaging...")
```

Any line starting with `##anvil:status ` (note the trailing space) is intercepted. All other output passes through normally.

## Pipeline Visualization

Visualize task dependency pipelines to understand how tasks are connected:

```bash
anvil task pipeline                    # show pipeline for current project
anvil task pipeline --verbose          # show detailed pipeline info
anvil task pipeline --dot             # output in GraphViz DOT format
anvil task pipeline --all             # show pipelines across all watched projects
```

Output formats:
- **Default** — ASCII tree showing task dependencies
- `--verbose` — detailed view with schedule and last run status for each task
- `--dot` — GraphViz DOT format for rendering with tools like `dot`

```bash
# Export to DOT and render with GraphViz
anvil task pipeline --dot > pipeline.dot
dot -Tpng pipeline.dot -o pipeline.png

# Show all project pipelines
anvil task pipeline --all --verbose
```

## Prometheus Metrics

The daemon exposes a Prometheus-compatible `/metrics` endpoint at the daemon socket:

```bash
# Fetch metrics from the daemon
curl --unix-socket ~/.anvil/daemon.sock http://daemon/metrics
```

Available metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `anvil_workers_available` | gauge | Number of available worker slots |
| `anvil_workers_max` | gauge | Maximum number of workers |
| `anvil_tasks_in_flight` | gauge | Number of tasks currently running |
| `anvil_tasks_pending` | gauge | Number of tasks queued but not running |
| `anvil_projects_watched` | gauge | Number of projects being watched |
| `anvil_uptime_seconds` | gauge | Daemon uptime in seconds |
| `anvil_task_runs_total` | counter | Total number of task runs |
| `anvil_task_success_total` | counter | Number of successful task runs |
| `anvil_task_failure_total` | counter | Number of failed task runs |
| `anvil_task_duration_seconds_bucket` | histogram | Task duration buckets |

## Cron Format

Standard 5-field: `minute hour day month weekday`

```
*/15 * * * *      every 15 minutes
0 9 * * 1-5       weekdays at 9am
0 */6 * * *       every 6 hours
0 2 * * *         daily at 2am
```

---

Built by [anvil](https://github.com/johnjansen/anvil), with [anvil](https://github.com/johnjansen/anvil) and [Claude Code](https://claude.com/claude-code).
