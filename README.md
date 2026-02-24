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

Pass `persistent` as the schedule to run continuously, exiting and re-dispatching on each tick:

```bash
anvil add -s "persistent" "Monitor a queue and process items as they arrive"
```

Persistent tasks exit after each unit of work and are immediately re-dispatched on the next scheduler tick. This is useful for event-driven workflows where you want the task to run frequently but not block the worker between work units.

#### Persistent task options

For persistent tasks, you can configure:

```yaml
---
schedule: "persistent"
persistent_cooldown: 5s      # wait between restart cycles (default: 0 = immediate)
persistent_max_runtime: 10m  # max runtime before forced restart (default: 0 = no limit)
---
```

- `persistent_cooldown` — wait time after a persistent task completes before re-dispatching. Default is 0 (immediate restart).
- `persistent_max_runtime` — maximum runtime before the task is forcibly restarted. Useful for preventing runaway tasks. Default is 0 (no limit).

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

## Configuration

`~/.anvil/config.yaml`:

```yaml
runners:
  - claude
max_workers: 10    # parallel tasks (max_todos is deprecated)
timeout: 15m       # max per task
tick_interval: 5s  # how often to check for work
hooks:
  on_success: "echo 'Task completed' >> ~/.anvil/history.log"
  on_failure: "curl -X POST https://example.com/webhook -d '{\"text\":\"Task failed\"}'"
```

Global hooks run for all tasks. Task-level hooks override global hooks for that specific task.

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

Or send SIGHUP manually:

```bash
kill -HUP $(cat ~/.anvil/daemon.pid)
```

The daemon will reload `~/.anvil/config.yaml` and apply changes to `max_workers`, `timeout`, `runners`, and `tick_interval`. Running tasks are not affected — only new task dispatches use the updated config.

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
```

Task-level frontmatter overrides project defaults. Global hooks from `~/.anvil/config.yaml` apply to all tasks unless overridden at the project or task level.

## CLI Reference

| Command | Description |
|---------|-------------|
| `anvil watch` | Start the daemon |
| `anvil watch [-d|--daemonize]` | Start daemon in background |
| `anvil watch --stop` | Stop the running daemon |
| `anvil init [path]` | Initialize a project |
| `anvil add [opts] <task>` | Add a task (`-s` schedule, `-p` priority 0-9, `--pre-check`, `--allowed-tools`, `--max-concurrent`, `--skip-permissions`) |
| `anvil logs [<name>]` | Raw worker output (all tasks or one) |
| `anvil daemon log` | View daemon log (-f to follow, -n for lines) |
| `anvil status` | Show watched projects |
| `anvil reload` | Reload daemon configuration (SIGHUP) |
| `anvil stop-on-idle` | Drain running tasks then exit the daemon |
| `anvil update [--check]` | Update to latest release |

**Task management:**

| Command | Description |
|---------|-------------|
| `anvil task ls` | List tasks in current project |
| `anvil task run <name>` | Trigger immediate execution (bypass cron) |
| `anvil task ls [-a|--all]` | List tasks across all projects |
| `anvil task get <name>` | Show task details |
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
| `anvil task edit <name> [-s schedule] [-p priority]` | Edit task schedule/priority |
| `anvil task pause <name>` | Pause a task (sets disabled: true) |
| `anvil task resume <name>` | Resume a paused task (sets disabled: false) |
| `anvil task timeout [name]` | Show task timeout progress (--all for all tasks) |

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
