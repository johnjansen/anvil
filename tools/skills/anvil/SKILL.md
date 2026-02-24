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

**Important:** The default schedule is `* * * * *` (every minute). Always pass `-s` with an appropriate schedule unless you intentionally want every-minute execution.

Examples:
```bash
# Run every 30 minutes at p0 (highest priority)
anvil add -p 0 -s "*/30 * * * *" "Review pending tasks and follow up"

# Run weekday mornings at 9am
anvil add -p 1 -s "0 9 * * 1-5" "Check GitHub issues and triage"

# One-shot task (no schedule — runs once on next tick, then gets deleted)
anvil add -s "" "Migrate the database schema"
```

Task files are stored in `.anvil/todos/p<N>/<slugified-name>.md` with YAML frontmatter containing the schedule and a UUID.

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

## Listing Tasks

```bash
anvil task ls           # list tasks in current project
anvil task ls --all     # list tasks across all watched projects
anvil task ls -a        # short form
```

Output columns: priority, schedule, status (running/idle), name, content preview.

## Getting Task Details

```bash
anvil task get <name>
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
anvil task ls
```

Shows tasks with their running/idle status. Use `anvil task ls --all` to see tasks across all watched projects.

## Killing Running Tasks

```bash
anvil task kill <name>
```

Sends a kill request to the daemon via unix socket. The daemon cancels the task's context directly for immediate termination. Accepts a task name or UUID.

## Stop on Idle

```bash
anvil stop-on-idle                   # daemon will finish running tasks then exit
anvil task stop-on-idle <name>       # task will not be rescheduled after its current run
```

`anvil stop-on-idle` puts the whole daemon into drain mode — it finishes all currently running tasks and then exits cleanly. Useful for graceful shutdowns.

`anvil task stop-on-idle <name>` marks a single task: it completes its current run but is not rescheduled. Other tasks continue normally.

## Task Subcommands

Full task management via `anvil task`:

```bash
anvil task create [options] <task>   # create a new task
anvil task ls [-a|--all]             # list tasks (--all for all projects)
anvil task get <name>                # show task details including run status
anvil task log [-f] <name>           # show execution log (-f to follow)
anvil task rm <name>                 # remove task (kills if running)
anvil task kill <name>               # kill a running task
anvil task stop-on-idle <name>       # finish current run then stop rescheduling
```

## Project Subcommands

Manage watched projects via `anvil project`:

```bash
anvil project create [path]          # init and watch a project in one step
anvil project ls [-a|--all]          # list watched projects
anvil project get [path]             # show project details and running tasks
anvil project rm [path] [--clean]    # unwatch (--clean removes .anvil/ too)
```

## Checking Status

```bash
anvil status
```

Shows watched projects and todo counts.

## Unwatching

```bash
anvil project rm
anvil project rm [path] --clean   # also removes .anvil/ directory
```

Stops the daemon from monitoring this project. Does not delete any task files unless `--clean` is passed.
