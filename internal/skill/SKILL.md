---
name: anvil
description: Manage recurring tasks and todos with the anvil CLI. Use when users ask to add tasks, list todos, delete tasks, check task logs, kill running tasks, initialize a project for anvil, or watch a project. Trigger phrases include "add a task", "add a todo", "list todos", "show my tasks", "delete a task", "remove a todo", "create a recurring job", "check task log", "kill task", "stop task", "initialize anvil", "watch this project", "anvil init", "anvil add", "anvil task", "anvil project", "anvil kill", "anvil watch", "anvil status", "what tasks are running", "show running tasks".
---

# Anvil CLI

Anvil is a task dispatcher for LLM projects. It manages recurring and one-shot todos that the daemon executes on a cron schedule. Use the CLI to manage tasks — the daemon handles execution.

## Usage

anvil <command> [options]

## Commands

init [--force] [path]    Initialize a project and register it for watching
register [path]          Register a project for watching (without full init)
watch [-d|--daemonize]   Start the daemon (press 'd' to detach to background)
watch --install          Install as system service (auto-start on boot)
watch --uninstall        Remove the system service
watch --status           Show system service status
watch --stop [--graceful] Stop the daemon (--graceful waits for tasks, --force kills)
add [options] <task>     Add a task to the current project
dispatch [options] <task>  Add a one-shot task and wait for completion, returning the result
logs [<name>]            Raw worker output (all tasks if no name given)
ps [--json] [-w|--watch] Show running tasks (--watch for live dashboard)
status [--json]          Show watched projects and daemon status
project <subcommand>     Project management commands
daemon <subcommand>      Daemon management commands
prompt <subcommand>    Prompt testing and validation tools
update [--check]         Update anvil to the latest release
reload [--graceful]       Reload daemon configuration (--graceful waits for tasks)
version                  Show version
Add options:
-p, --priority int          Task priority 0-9 (default 1)
-s, --schedule string      Cron schedule (e.g., "*/15 * * * *"), "" for one-shot
-o, --once                 Create a one-shot task (no schedule)
-n, --dry-run              Validate schedule without creating task
-f, --file path            Read task content from a file
-t, --template name        Use a template for task configuration
-                          Read task content from stdin
--pre-check string    Shell command to skip task if non-zero exit
--allowed-tools string  Comma-separated tool allowlist (e.g. "Bash,Read") or scoped (e.g. "Bash(gh:*)")
--max-concurrent int    Max parallel instances (default 1)
--skip-permissions     Bypass all tool permission prompts
--strict               Fail if schedule conflicts with existing tasks
--no-overlap-check    Skip schedule overlap detection
--depends-on dep      Task dependency (repeatable; use project:task for cross-project)
Task subcommands:
create [options] <task>   Create a new task
ls [-a|--all] [--json] [--label L]  List tasks (--all for all projects, --label to filter)
find <pattern>            Find tasks by name pattern (alias for ls --match)
get <name> [--json]       Show task details including run status
log [-f] <name>           Show execution log (-f to follow)
history <name> [--json]    Show run history
rm <name>                 Remove a task (kills if running)
run <name>                Trigger immediate execution (bypass cron)
kill <name>               Kill a running task (persistent tasks auto-restart)
stop <name>               Stop a persistent task permanently (kill + prevent restart)
start <name>              Start a stopped persistent task (dispatches on next tick)
stop-on-idle <name>       Finish current run then stop rescheduling task
unlock <name>             Remove stale lock file to allow retry
queue [--json]            Show daemon queue status and skip reasons
pause <name>              Pause a task (sets disabled: true)
resume <name>             Resume a paused task (sets disabled: false)
edit <name>               Edit task (schedule, priority, content, labels, or --remove field)
timeout [name]            Show task timeout progress (--all for all tasks)
extend-timeout <name> <dur>  Extend a running task's timeout by the given duration
next [name]              Show next scheduled run time (--all for all projects)
wait <name> [--timeout D]  Block until a running task completes (exit 0=ok, 1=fail, 2=timeout)
analyze [-a|--all]         Detect scheduling conflicts and overlapping tasks
pipeline [--dot|--verbose] [--all]  Visualize task dependency pipelines
reset-budget <name>        Reset persistent task budget consumption
state <name>              View, export, import, or clear task state
dry-run <name> [options]   Validate and preview task config without executing
activity <name> [options]  Show task activity history (--type, --since, --limit, --export, --json)
snapshot <name> [--run <id>] [--file <filename>]  View task execution snapshots for debugging
snapshot-diff <name> --run1 <id1> --run2 <id2>  Compare two task execution snapshots
sla [--verbose] [--reset] [--json]  Show SLA violations (--verbose for all, --reset to clear)
export [names...] [-a] [-o file]  Export tasks to JSON for sharing or backup
import <file> [options]   Import tasks from a JSON export file
Project subcommands:
create [path]            Initialize and watch a project in one step
ls [-a|--all] [--json]   List watched projects
get [path]               Show project details and running tasks
rm [path] [--clean]      Unwatch a project (--clean removes .anvil/ too)
Daemon subcommands:
log [-f] [-n lines] [--level LEVEL] [--match PATTERN] [--since TIME] [--until TIME]
View daemon log (filtering options for level, match, since, until)
config-validate [--show]  Validate config file (--show to display parsed config)

## Configuration

~/.anvil/config.yaml   Daemon config
<project>/.anvil/      Project config and todos

## Advanced allowed_tools Syntax

The `allowed_tools` feature supports scoped permissions for fine-grained control:

```yaml
---
id: "some-uuid"
schedule: "*/5 * * * *"
allowed_tools:
  - Bash(gh:*)      # only gh subcommands
  - Read(.claude/commands/*)  # only read files in .claude/commands/
  - Write(/tmp/*)   # only write files in /tmp/
---
```

Scoped syntax allows you to restrict tools to specific command prefixes or file paths, providing least-privilege access control.

## Frontmatter Configuration

Tasks can be configured with various options in YAML frontmatter:

```yaml
---
id: "some-uuid"
schedule: "*/30 * * * *"
priority: 1
pre_check: "gh issue list --state open --label untriaged | grep -q ."
allowed_tools:
  - Bash
  - Read
  - Write
max_concurrent: 2
skip_permissions: false
disabled: false
timeout: 15m
retry: 3
retry_delay: 2m
persistent_cooldown: 5s
persistent_max_runtime: 30m
on_success: "echo 'done' >> /tmp/anvil.log"
on_failure: "curl -X POST https://slack.example.com/webhook -d '{\"text\":\"Task failed\"}'"
---
```

