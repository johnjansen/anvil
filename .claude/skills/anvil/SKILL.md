---
name: anvil
description: Manage recurring tasks and todos with the anvil CLI. Use when users ask to add tasks, list todos, delete tasks, check task logs, kill running tasks, initialize a project for anvil, or watch a project. Trigger phrases include "add a task", "add a todo", "list todos", "show my tasks", "delete a task", "remove a todo", "create a recurring job", "check task log", "kill task", "stop task", "initialize anvil", "watch this project", "anvil init", "anvil add", "anvil task", "anvil project", "anvil kill", "anvil watch", "anvil status", "what tasks are running", "show running tasks".
---

# Anvil CLI

Anvil is a task dispatcher for LLM projects. It manages recurring and one-shot todos that the daemon executes on a cron schedule. Use the CLI to manage tasks — the daemon handles execution.

## Project Setup

Before adding tasks, the project needs to be initialized:

```bash
# Initialize and register with the daemon — creates .anvil/todos/ and .claude/skills/
anvil init

# Start the daemon (once per machine)
anvil watch

# Or run in background (daemonized)
anvil watch [-d|--daemonize]
anvil watch --stop             # stop the daemon
anvil watch --stop --graceful  # stop gracefully (wait for running tasks)
anvil watch --stop --force     # stop immediately (kill running tasks)
anvil watch --restart          # restart the daemon
anvil watch --restart --graceful # restart gracefully (wait for running tasks)
anvil watch --install          # install as system service (auto-start on boot)
anvil watch --uninstall        # remove the system service
anvil watch --status           # show system service status

# Reload config without restarting
anvil reload
```

`anvil init` defaults to the current directory, pass a path to target a different project. It both initializes the project structure and registers it with the daemon.

## Adding Tasks

```bash
anvil add [options] <task text>
# or equivalently:
anvil task create [options] <task text>
```

Options:
- `-p, --priority <0-9>` — Priority level (default: 1). Lower = higher priority.
- `-s, --schedule <cron>` — Cron expression for when to run.
- `-o, --once` — Create a one-shot task (no schedule).
- `-n, --dry-run` — Validate schedule without creating the task.
- `-f, --file <path>` — Read task content from a file.
- `-` — Read task content from stdin.
- `-t, --template <name>` — Use a task template for configuration.
- `--pre-check <command>` — Shell command to gate execution (skip if non-zero exit).
- `--allowed-tools <tools>` — Comma-separated tool allowlist (e.g. "Bash,Read,Write").
- `--max-concurrent <n>` — Max parallel instances (default: 1).
- `--skip-permissions` — Bypass all tool permission prompts.
- `--strict` — Fail if the schedule conflicts with existing tasks.
- `--no-overlap-check` — Skip schedule overlap detection.

**Note:** The `runner` option is available in task frontmatter or project config, but not as a CLI flag.

**Important:** The default is a one-shot task (empty schedule — runs once on next tick, then gets deleted). Always pass `-s` with an appropriate schedule unless you intentionally want one-shot execution.

Examples:
```bash
# Run every 30 minutes at p0 (highest priority)
anvil add -p 0 -s "*/30 * * * *" "Review pending tasks and follow up"

# Run weekday mornings at 9am
anvil add -p 1 -s "0 9 * * 1-5" "Check GitHub issues and triage"

# One-shot task (no schedule — runs once on next tick, then gets deleted)
anvil add -s "" "Migrate the database schema"

# Persistent task (runs continuously, exits and re-dispatches on each tick)
anvil add -s "persistent" "Monitor a queue and process items"

# Task with pre-check to skip if nothing to do
anvil add -s "*/30 * * * *" --pre-check "gh issue list --state open --label untriaged | grep -q ." "Triage GitHub issues"

# Task with limited tools
anvil add -s "*/5 * * * *" --allowed-tools "Bash,Read,Write" "Process queued items"

# Task that skips permission prompts
anvil add -s "*/15 * * * *" --skip-permissions "Run automated checks"

# Task with max concurrent instances
anvil add -s "*/5 * * * *" --max-concurrent 2 "Process in parallel"

# Read task content from a file
anvil add -s "*/30 * * * *" -f task.md

# Read task content from stdin
echo "Process items from queue" | anvil add -s "*/30 * * * *" -
```

Task files are stored in `.anvil/todos/p<N>/<slugified-name>.md` with YAML frontmatter containing the schedule and a UUID.

## Persistent Tasks

Pass `persistent` as the schedule to run continuously:

```yaml
---
id: "some-uuid"
schedule: "persistent"
---
```

Persistent tasks exit after each unit of work and are immediately re-dispatched on the next scheduler tick. This is useful for event-driven workflows.

**Each cycle starts fresh** — the daemon generates a new session ID for each execution. There's no session persistence between cycles; each run begins as if it were the first time.

### persistent_cooldown

For persistent tasks, set `persistent_cooldown` to wait between restart cycles:

```yaml
---
id: "some-uuid"
schedule: "persistent"
persistent_cooldown: 5s
---
```

Default is 0 (immediate restart).

### persistent_max_runtime

Set `persistent_max_runtime` to enforce a maximum runtime before forced restart:

```yaml
---
id: "some-uuid"
schedule: "persistent"
persistent_max_runtime: 10m
---
```

Useful for preventing runaway tasks. Default is 0 (no limit).

### persistent_budget

Set `persistent_budget` to limit cumulative wall-clock time a persistent task can run per daemon lifetime:

```yaml
---
id: "some-uuid"
schedule: "persistent"
persistent_budget: 1h
---
```

When the cumulative runtime exceeds this budget, the task stops and requires manual restart. Default is 0 (unlimited).

### Starvation prevention

If a persistent task waits more than 5 minutes for a worker slot, it temporarily yields to let higher-priority work through. This prevents low-priority persistent tasks from blocking important cron jobs indefinitely.

## Task Templates

Create reusable task templates to standardize common patterns:

```bash
# List available templates
anvil template ls

# Show template details
anvil template get <name>
```

Templates are stored in:
- `.anvil/templates/` — project-specific templates
- `~/.anvil/templates/` — global templates shared across all projects

A template is a YAML file:

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

Use a template when creating a task:

```bash
anvil add -t daily-standup "Morning standup summary"
```

Template values can be overridden by CLI flags — flags take precedence over template values.

## Session Continuity

By default, recurring tasks resume previous sessions and one-shot tasks start fresh. Override this with `resume` in the frontmatter:

```yaml
---
id: "some-uuid"
schedule: "*/5 * * * *"
resume: true           # default for recurring — continues previous session
---
```

```yaml
---
id: "some-uuid"
schedule: "*/30 * * * *"
resume: false          # force fresh session every run
---
```

- `resume: true` (default for recurring): first run uses `--session-id`, subsequent runs use `--resume`
- `resume: false` (default for one-shot): always starts a new session
- Omit `resume` to use the default behavior

## skip_permissions

Set `skip_permissions: true` in the frontmatter to append `--dangerously-skip-permissions` to the runner command for that task:

```yaml
---
id: "some-uuid"
schedule: "*/30 * * * *"
skip_permissions: true
---
```

The flag is appended only if the runner command doesn't already include it globally.

## allowed_tools

Set `allowed_tools` in the frontmatter to pre-approve a specific list of tools without bypassing all permission checks. This is the least-privilege alternative to `skip_permissions`.

```yaml
---
id: "some-uuid"
schedule: "*/5 * * * *"
allowed_tools:
  - Bash
  - Read
  - Write
  - Edit
---
Task that can read, write, and run shell commands but nothing else.
```

The Claude CLI supports shell-globbing syntax to restrict tools to specific command prefixes:

```yaml
---
id: "some-uuid"
schedule: "*/30 * * * *"
allowed_tools:
  - Bash(gh:*)      # only gh subcommands
  - Read
---
```

`allowed_tools` and `skip_permissions` can coexist — both flags are appended independently. If both are set, `--dangerously-skip-permissions` takes precedence (it is a superset).

## max_concurrent

Set `max_concurrent` to limit how many simultaneous instances of a task the daemon will run. Useful for tasks triggered at short intervals that may take longer than the interval to complete.

```yaml
---
id: "some-uuid"
schedule: "*/5 * * * *"
max_concurrent: 2
---
Run analysis but allow at most 2 parallel instances of this task.
```

Default is 1 (no parallel runs of the same task). Omit or set to 0 to use the default.

## pre_check

Set `pre_check` to a shell command that gates task execution. If the command exits non-zero, the task is skipped silently — no log entry, no agent invocation. Use this to avoid expensive LLM calls when there's nothing to do:

```yaml
---
id: "some-uuid"
schedule: "*/15 * * * *"
pre_check: "gh issue list --state open --label untriaged | grep -q ."
---
Check GitHub for new untriaged issues and apply labels.
```

The `pre_check` command runs in the project directory. Zero exit = proceed; non-zero = skip quietly.

## Task Labels

Add labels to tasks for organization and filtering via frontmatter:

```yaml
---
id: "some-uuid"
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
anvil task edit <name> --remove-label old-label
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

## disabled

Set `disabled: true` to pause a task without deleting it:

```yaml
---
id: "some-uuid"
schedule: "*/15 * * * *"
disabled: true
---
Temporarily paused task...
```

The task is skipped during tick evaluation but remains in the project. Set `disabled: false` or remove the line to resume.

## pause / resume

Use CLI commands to pause and resume tasks without editing the file manually:

```bash
anvil task pause <name>   # sets disabled: true
anvil task resume <name>  # sets disabled: false
```

These are shortcuts for setting `disabled: true/false` in the frontmatter — equivalent to editing the file directly but more convenient.

## timeout

Set `timeout` to override the global timeout for a specific task. The global timeout is configured in `~/.anvil/config.yaml` (default: 5 minutes):

```yaml
---
id: "some-uuid"
schedule: "*/30 * * * *"
timeout: 15m
---
Run with a 15-minute timeout instead of the default.
```

Valid units: `s` (seconds), `m` (minutes), `h` (hours). Set to `0` or omit to use the global default.

## retry

Set `retry` to automatically retry failed tasks:

```yaml
---
id: "some-uuid"
schedule: "*/30 * * * *"
retry: 3
retry_delay: 2m
---
Run with up to 3 retries, waiting 2 minutes between attempts.
```

- `retry`: Number of retries on failure (0 = no retry, default: 0)
- `retry_delay`: Initial delay between retries (default: 1m)

The retry delay uses exponential backoff: the initial delay doubles after each retry attempt (delay * 2^attempt). For example, with retry_delay=1m, retries occur at 1m, 2m, 4m, etc.

## on_success / on_failure

Run shell commands after a task completes. Useful for notifications, cleanup, or chaining workflows:

```yaml
---
id: "some-uuid"
schedule: "*/30 * * * *"
on_success: "echo 'done' >> /tmp/anvil.log"
on_failure: "curl -X POST https://slack.example.com/webhook -d '{\"text\":\"Task failed\"}'"
---
```

Hooks run in the project directory with a 60-second timeout. Environment variables available:

- `ANVIL_TASK_NAME` — task filename
- `ANVIL_EXIT_CODE` — `0` for success, `1` for failure
- `ANVIL_LOG_PATH` — path to raw log file
- `ANVIL_PROJECT` — project directory path
- `ANVIL_SESSION_ID` — Claude session ID used
- `ANVIL_START_TIME` — RFC 3339 start timestamp
- `ANVIL_END_TIME` — RFC 3339 end timestamp
- `ANVIL_ELAPSED_MS` — elapsed time in milliseconds

Hook errors are logged as warnings but do not affect the task outcome.

## Webhook Notifications

Configure HTTP webhooks to receive notifications for task lifecycle events in `~/.anvil/config.yaml`:

```yaml
webhooks:
  slack:
    url: "https://hooks.slack.com/services/xxx"
    method: "POST"  # default: POST
    headers:
      Authorization: "Bearer xxx"
    events: ["success", "failure", "start", "timeout", "persistent_cycle"]
    timeout: 10s  # default: 10s
  teams:
    url: "https://outlook.office.com/webhook/xxx"
    events: ["failure", "timeout"]
```

Supported events:
- `start` — task execution started
- `success` — task completed successfully
- `failure` — task failed
- `timeout` — task timed out
- `persistent_cycle` — persistent task completed a cycle

You can use short event names in config (`success` instead of `task_success`). An empty events list means "all events".

The webhook payload includes:

| Field | Description |
|-------|-------------|
| `event` | Event type (e.g., `task_success`) |
| `task_name` | Task filename |
| `project` | Project directory path |
| `status` | Status string (`success`, `failure`, `started`, `timeout`, `force_cycled`) |
| `run_id` | Unique run identifier |
| `started_at` | RFC 3339 timestamp |
| `finished_at` | RFC 3339 timestamp |
| `duration_seconds` | Execution duration |
| `estimated_cost_usd` | Estimated LLM cost |
| `error` | Error message (if failure/timeout) |

Webhooks are sent asynchronously with 3 retry attempts and exponential backoff. Failed deliveries are logged but don't affect task outcome.

## Per-task Webhook

Override or supplement global webhooks for a specific task:

```yaml
---
schedule: "*/30 * * * *"
webhook: "https://hooks.slack.com/services/xxx"
---
Triage GitHub issues...
```

The per-task webhook URL receives the same payload as global webhooks. It fires in addition to any globally configured webhooks.

## Per-task runner override

Override the global runner chain for a specific task:

```yaml
---
id: "some-uuid"
schedule: "*/30 * * * *"
runner: "claude -p 'You are a specialized assistant'"
---
Run this task with a different runner command than the global default.
```

The task-level runner is used instead of the global `runners` list for this task only.

You can also set a default runner at the project level:

```yaml
# .anvil/config.yaml (project-level)
defaults:
  runner: "claude -p 'You are a task runner'"
```

## Environment Variables

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

## Task Dependencies

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

## Task Checkpointing

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

## Runtime Status Reporting

Tasks can report their current status to the daemon by printing a special line to stdout:

```
##anvil:status Triaging 3 new issues
```

The daemon picks up the status text and displays it in `anvil task ls` and heartbeat logs. Status lines are stripped from the task's output — they never appear in log files. Any line starting with `##anvil:status ` (note the trailing space) is intercepted. All other output passes through normally.

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

## Runner Fallback Chain

The daemon supports an ordered list of runner commands in `~/.anvil/config.yaml`. If the first runner fails, it tries the next:

```yaml
runners:
  - claude -p "you are a task runner"
  - claude --model haiku -p "you are a task runner"
  - echo
```

For backwards compatibility, a single `runner` string still works:

```yaml
runner: claude -p "you are a task runner"
```

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
rate_limit:
  max_concurrent_calls: 10    # max concurrent LLM API calls (default: unlimited)
  requests_per_minute: 60    # max API requests per minute (default: unlimited)
  requests_per_hour: 1000    # max API requests per hour (default: unlimited)
  burst: 20                  # allow short bursts above rate (default: 10)
  provider:
    claude:
      requests_per_minute: 50
hooks:
  on_success: "echo 'Task completed' >> ~/.anvil/history.log"
  on_failure: "curl -X POST https://example.com/webhook -d '{\"text\":\"Task failed\"}'"
retention:
  max_age: 7d      # delete logs older than 7 days
  max_runs: 50     # keep only last 50 runs per task
  max_log_size: 50mb  # max size per log file (0 = unlimited)
webhooks:
  slack:
    url: "https://hooks.slack.com/services/xxx"
    method: "POST"
    headers:
      Authorization: "Bearer xxx"
    events: ["success", "failure", "start", "timeout"]
    timeout: 10s
```

Global hooks run for all tasks. Task-level hooks override global hooks for that specific task.

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

### Hot Reload

Reload the daemon configuration without restarting:

```bash
anvil reload
anvil reload --graceful          # wait for running tasks before reloading
anvil reload --graceful --timeout 60s  # wait up to 60 seconds
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

## Listing Tasks

```bash
anvil task ls           # list tasks in current project
anvil task ls --all     # list tasks across all watched projects
anvil task ls -a        # short form
anvil task ls --json    # output in JSON format
anvil task ls --label triage    # filter by label
anvil task ls --label triage,github  # filter by multiple labels (OR)
anvil task ls --label !archived      # exclude tasks with label
anvil task ls --match "search-term" # search in task names and content
```

Output columns: priority, schedule, status (running/idle/disabled/locked), name, content preview. A `disabled` status means the task is paused (set `disabled: true` in frontmatter) — use `anvil task resume <name>` to re-enable. A `locked` status means a stale lock file was found — this typically indicates the daemon crashed mid-execution. Use `anvil task unlock <name>` to remove the stale lock and allow the task to run again.

## Finding Tasks

```bash
anvil task find <pattern>
```

Find tasks by name pattern. This is an alias for `anvil task ls --match <pattern>`.

## Getting Task Details

```bash
anvil task get <name>
anvil task get <name> --json
```

Shows full details: file path, ID, schedule, priority, session path (if exists), run status, and full content. The name can be with or without the `.md` extension.

## Deleting Tasks

```bash
anvil task rm <name>
```

Removes the todo file. Also kills the task if it's currently running.

## Viewing Logs

```bash
anvil task log <name>      # print session log
anvil task log -f <name>   # follow (tail -f style)
```

Shows the session log (JSONL) for a task's most recent execution. Accepts a task name or UUID directly. Follow mode (`-f`) waits for the log file to appear and streams new output until the task completes or Ctrl+C is pressed.

## Raw Worker Logs

```bash
anvil logs                 # multiplex raw output from all running tasks
anvil logs <name>          # raw output for a specific task (live or last completed)
```

The `logs` command shows raw stdout/stderr output from worker processes. Without an argument, it multiplexes output from all currently running tasks with task name prefixes, exiting when all tasks complete. With a task name, it follows the live raw log if the task is running, or prints the most recent completed run's raw log if it has already finished.

## Running Tasks

```bash
anvil ps [--json] [-w|--watch]
anvil task ls
```

Use `anvil ps [--json] [-w|--watch]` to show currently running tasks. Use `anvil task ls` to see tasks across all watched projects.

Shows tasks with their running/idle status. Use `anvil task ls --all` to see tasks across all watched projects.

## Killing Running Tasks

```bash
anvil task kill <name>
```

Sends a kill request to the daemon via unix socket. The daemon cancels the task's context directly for immediate termination. Accepts a task name or UUID.

## Viewing Run History

```bash
anvil task history <name>      # show last 10 runs
anvil task history <name> -n 5 # show last 5 runs
anvil task history <name> -f   # follow mode - watch for new runs
anvil task history <name> --failures   # show only failed runs
anvil task history <name> --json       # output in JSON format
```

Shows a table with start time, duration, and status (ok/failed) for each run.

## Editing Tasks

```bash
anvil task edit <name> -s "*/30 * * * *"  # change schedule
anvil task edit <name> -p 0                 # change priority
anvil task edit <name> --content "New task description"  # change content
anvil task edit <name> --content-file task.md  # change content from file
anvil task edit <name> --add-label triage     # add a label
anvil task edit <name> --remove-label old     # remove a label
anvil task edit <name> --remove pre_check   # remove a frontmatter field
```

Edits the task's frontmatter in place. Moving a task to a different priority moves the file to the corresponding priority directory.

The `--remove` flag (also `--clear`) removes a field from the task's frontmatter. Valid fields: `allowed_tools`, `on_failure`, `on_success`, `persistent_budget`, `persistent_cooldown`, `persistent_max_runtime`, `pre_check`, `schedule`, `timeout`.

### Bulk Edit

```bash
# Change schedule for all tasks matching pattern
anvil task edit --all -s "0 9 * * 1-5"

# Disable tasks matching pattern
anvil task edit --all "triage-*" --disabled

# Enable all tasks
anvil task edit --all --enabled

# Preview priority change without applying
anvil task edit --all -p 2 --dry-run
```

Bulk edit (`--all`) supports `-s`/`--schedule`, `-p`/`--priority`, `--disabled`, and `--enabled`. Use `--dry-run` to preview. Does not support `--content`, `--content-file`, or `--remove`.

## Stopping the Daemon

```bash
anvil stop-on-idle             # drain running tasks then exit
```

`anvil stop-on-idle` puts the daemon into drain mode — it finishes all currently running tasks and then exits cleanly. Useful for graceful shutdowns. To stop the daemon immediately, send SIGTERM directly (e.g. `kill $(cat ~/.anvil/daemon.pid)`).

## Task Subcommands

Full task management via `anvil task`:

```bash
anvil task create [options] <task>   # create a new task
anvil task ls [-a|--all]             # list tasks (--all for all projects)
anvil task ls --label <label>        # filter by label
anvil task ls --match <pattern>      # search in task names and content
anvil task get <name>                # show task details including run status
anvil task get <name> --json        # output in JSON format
anvil task log [-f] <name>           # show execution log (-f to follow)
anvil task rm <name>                 # remove task (kills if running)
anvil task run <name>                # trigger immediate execution (bypass cron)
anvil task kill <name>               # kill a running task
anvil task stop-on-idle <name>      # finish current run then stop rescheduling
anvil task unlock <name>             # remove stale lock file to allow retry
anvil task queue                     # show daemon queue status and skip reasons
anvil task pause <name>              # pause a task (sets disabled: true)
anvil task resume <name>             # resume a paused task (sets disabled: false)
anvil task timeout [name]            # show task timeout progress (--all for all tasks)
anvil task wait <name> [--timeout D] [--match PAT]  # block until task completes (exit 0=ok, 1=fail, 2=timeout)
anvil task analyze [--all]         # analyze task schedules for potential conflicts
anvil task reset-budget <name>    # reset persistent task budget consumption
anvil task next [name]              # show next scheduled run time (--all for all projects)
anvil task start <name>              # start a stopped task (re-enable rescheduling)
anvil task stop <name>               # stop a running task (disable rescheduling)
anvil task find <pattern>            # find tasks by name pattern (alias for ls --match)
anvil task edit --all [pattern] [-s|-p|--disabled|--enabled] [--dry-run]  # bulk edit tasks
anvil task export [names...] [-a|--all] [-o file]  # export tasks to JSON
anvil task import <file> [--base-path path] [-n|--dry-run] [-f|--force]  # import tasks from JSON
```

## Project Subcommands

Manage watched projects via `anvil project`:

```bash
anvil project create [path]          # init and watch a project in one step
anvil project ls [-a|--all]          # list watched projects
anvil project get [path]             # show project details and running tasks
anvil project rm [path] [--clean]    # unwatch (--clean removes .anvil/ too)
```

## Updating Anvil

```bash
anvil update             # download and install the latest release
anvil update --check     # check if an update is available without installing
```

`anvil update` fetches the latest GitHub release, downloads the platform binary, and replaces the current executable. Use `--check` to see if a newer version exists without actually updating.

## Version

```bash
anvil version    # show current version
anvil -v         # shorthand
anvil --version  # also valid
```

Shows the currently installed anvil version.

## Daemon Log

```bash
anvil daemon log           # view last 50 lines of daemon log
anvil daemon log -f       # follow daemon log in real-time
anvil daemon log -n 100   # view last 100 lines
anvil daemon log --level info     # filter by minimum level (debug, info, warn, error)
anvil daemon log --match "error"  # filter by text pattern
anvil daemon log --since "1h"     # show entries since duration ago
anvil daemon log --until "2pm"   # show entries until specific time
```

View the daemon's log output. Useful for debugging daemon issues or monitoring daemon activity.

Filtering options:
- `--level` — minimum log level to show (debug, info, warn, error)
- `--match` — text pattern to filter log lines
- `--since` — show entries since duration ago (e.g., "1h", "30m", "2026-01-15")
- `--until` — show entries until specific time (e.g., "2pm", "2026-01-15T15:00")

## Validating Configuration

```bash
anvil daemon config-validate          # validate ~/.anvil/config.yaml
anvil daemon config-validate --show   # validate and show parsed config
```

Checks the daemon config file for syntax errors and invalid values without starting the daemon.

## Task Import/Export

```bash
# Export specific tasks to stdout
anvil task export task1.md task2.md

# Export all tasks from current project to a file
anvil task export --all -o backup.json

# Import tasks from a JSON file
anvil task import backup.json

# Preview import without creating tasks
anvil task import backup.json --dry-run

# Import with path remapping
anvil task import backup.json --base-path /new/project/path

# Overwrite existing tasks
anvil task import backup.json --force
```

Export tasks to a portable JSON format for sharing between machines or backing up configurations. Use `--base-path` during import to remap project paths.

## Cleanup

```bash
anvil cleanup                         # show retention policy config
anvil cleanup --older-than=3d         # delete logs older than 3 days
anvil cleanup --older-than=3d --dry-run  # preview what would be deleted
anvil cleanup --older-than=3d -n      # shorthand for --dry-run
anvil cleanup -o=24h                  # short form
```

Prune old logs and session data. Use `--dry-run` to preview deletions without actually deleting. Without a retention policy configured, it shows how to configure one.

## Checking Status

```bash
anvil status [--json]
anvil ps [--json] [-w|--watch]
```

`anvil status [--json]` shows watched projects, daemon status, and todo counts. `anvil ps [--json] [-w|--watch]` shows currently running tasks.

## Unwatching

```bash
anvil project rm
anvil project rm [path] --clean   # also removes .anvil/ directory
```

Stops the daemon from monitoring this project. Does not delete any task files unless `--clean` is passed.

## Usage

```bash
anvil usage                     # show usage for last 7 days
anvil usage --project <path>    # filter to a specific project
anvil usage --task <name>      # filter to a specific task
anvil usage --since 2026-01-01 # show usage since a specific date
anvil usage --metrics          # show task runtime metrics (total runtime, success rate, etc.)
anvil usage --top 10           # limit to top 10 tasks (use with --metrics)
anvil usage --json             # output as JSON
```

Shows LLM token usage and estimated costs across tasks and projects. Tracks input/output tokens and calculates estimated USD costs based on the `input_token_rate` and `output_token_rate` configured in `~/.anvil/config.yaml`.

With `--metrics`, shows task runtime metrics including total runtime, average execution time, run count, and success rates. Use `--top N` to limit output to the top N tasks by runtime.
