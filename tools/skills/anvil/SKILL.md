---
name: anvil
description: Manage recurring tasks and todos with the anvil CLI. Use when users ask to add tasks, list todos, delete tasks, check task logs, kill running tasks, initialize a project for anvil, or watch a project. Trigger phrases include "add a task", "add a todo", "list todos", "show my tasks", "delete a task", "remove a todo", "create a recurring job", "check task log", "kill task", "stop task", "initialize anvil", "watch this project", "anvil init", "anvil add", "anvil list", "anvil get", "anvil delete", "anvil log", "anvil kill", "anvil watch", "anvil status", "what tasks are running", "show running tasks".
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

## Adding Tasks

```bash
anvil add [options] <task text>
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

# One-shot task (no schedule — runs once, then gets deleted)
anvil add -s "" "Migrate the database schema"
```

Task files are stored in `.anvil/todos/p<N>/<slugified-name>.md` with YAML frontmatter containing the schedule and a UUID.

## Listing Tasks

```bash
# List todos for the current project
anvil list

# List todos across all watched projects
anvil list --all
```

Output columns: priority, schedule, name, content preview.

## Getting Task Details

```bash
anvil get <name>
```

Shows full details: file path, ID, schedule, priority, session path (if exists), and full content. The name can be with or without the `.md` extension.

## Deleting Tasks

```bash
anvil delete <name>
```

Removes the todo file. Use this for tasks you no longer want.

## Viewing Logs

```bash
anvil log <name>
```

Shows the session log (JSONL) for a task's most recent execution. Accepts a task name or UUID directly.

## Killing Running Tasks

```bash
anvil kill <name>
```

Writes a kill signal that the daemon picks up within ~100ms. The running process gets cancelled and cleaned up. Accepts a task name or UUID.

## Checking Status

```bash
# Show watched projects and todo counts
anvil status

# Show what's currently running
anvil ps
```

## Unwatching

```bash
anvil unwatch
```

Stops the daemon from monitoring this project. Does not delete any task files.
