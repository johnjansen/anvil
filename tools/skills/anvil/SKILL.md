---
name: anvil
description: Manage recurring tasks and todos with the anvil CLI. Use when users ask to add tasks, list todos, delete tasks, check task logs, kill running tasks, initialize a project for anvil, or watch a project. Trigger phrases include "add a task", "add a todo", "list todos", "show my tasks", "delete a task", "remove a todo", "create a recurring job", "check task log", "kill task", "stop task", "initialize anvil", "watch this project", "anvil init", "anvil add", "anvil list", "anvil get", "anvil delete", "anvil log", "anvil kill", "anvil watch", "anvil status", "anvil ps", "what tasks are running", "show running tasks".
---

# Anvil CLI

Anvil is a task dispatcher for LLM projects. It manages recurring and one-shot todos that the daemon executes on a cron schedule. Use the CLI to manage tasks — the daemon handles execution.

## Project Setup

Before adding tasks, the project needs to be initialized and watched:

```bash
# Initialize — creates .anvil/todos/ and .claude/skills/
anvil init

# Register with the daemon so it picks up your todos
anvil watch
```

Both commands default to the current directory. Pass a path to target a different project.

Alternatively, use `anvil project create` to initialize and register in one step.

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
# List todos for the current project
anvil list

# Use the task subcommand for more options
anvil task ls           # list tasks in current project
anvil task ls --all     # list tasks across all watched projects
anvil task ls -a        # short form
```

Output columns: priority, schedule, status, name, content preview.

## Getting Task Details

```bash
anvil get <name>
# or (also shows run status):
anvil task get <name>
```

Shows full details: file path, ID, schedule, priority, session path (if exists), and full content. `anvil task get` additionally shows whether the task is currently running. The name can be with or without the `.md` extension.

## Deleting Tasks

```bash
anvil delete <name>
# or (also kills if running):
anvil task rm <name>
```

Removes the todo file. `anvil task rm` will also kill the task if it's currently running.

## Viewing Logs

```bash
anvil log <name>           # print session log
anvil log -f <name>        # follow (tail -f style)
anvil task log <name>      # same via task subcommand
anvil task log -f <name>   # follow mode
```

Shows the session log (JSONL) for a task's most recent execution. Accepts a task name or UUID directly. Follow mode (`-f`) waits for the log file to appear and streams new output until the task completes or Ctrl+C is pressed.

## Raw Worker Logs

```bash
anvil logs                 # follow raw output from all running tasks
anvil logs <name>          # follow raw output for a specific task
```

The `logs` command shows raw stdout/stderr output from worker processes. Without an argument, it multiplexes output from all running tasks with task name prefixes. With a task name, it follows the raw log for that specific task.

## Running Tasks

```bash
anvil ps
```

Shows currently running tasks with project, name, PID, start time, and elapsed time. Queries the daemon via unix socket at `~/.anvil/daemon.sock`.

## Killing Running Tasks

```bash
anvil task kill <name>
```

Sends a kill request to the daemon via unix socket. The daemon cancels the task's context directly for immediate termination. Accepts a task name or UUID.

## Task Subcommands

Full task management via `anvil task`:

```bash
anvil task create [options] <task>   # create a new task
anvil task ls [-a|--all]             # list tasks (--all for all projects)
anvil task get <name>                # show task details including run status
anvil task log [-f] <name>           # show execution log (-f to follow)
anvil task rm <name>                 # remove task (kills if running)
anvil task kill <name>               # kill a running task
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
anvil unwatch
# or:
anvil project rm
```

Stops the daemon from monitoring this project. Does not delete any task files.
