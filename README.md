# Anvil

One daemon per machine. Many projects. The daemon is the central dispatcher — it watches registered project directories, checks their cron schedules, and runs their todos through a configured LLM runner.

## How It Works

```
  project A/.anvil/         project B/.anvil/         project C/.anvil/
  └── todos/                └── todos/                └── todos/
      ├── p0/                   ├── p0/                   ├── p0/
      └── p1/                   └── p1/                   └── p1/
        │                         │                         │
        └─────────────────────────┼─────────────────────────┘
                                  │ anvil watch
                                  ▼
                     ┌────────────────────────┐
                     │    Daemon (singleton)   │
                     │       ~/.anvil/         │
                     │                         │
                     │  tick:                  │
                     │   for each project:     │
                     │    → for each todo:     │
                     │      → cron match?      │
                     │      → dispatch to pool │
                     └────────────────────────┘
```

1. Start the daemon once: `anvil serve`
2. From any project, register it: `anvil watch`
3. Add recurring tasks: `anvil add -p 0 -s "*/30 * * * *" "Check GitHub for new issues and triage them"`
4. On every tick the daemon iterates all watched projects, checks each todo's cron schedule, and runs matching tasks through the configured runner
5. The daemon manages all running processes across all projects

If a task is already in-flight (queued or executing) when the next tick fires, it's skipped for that tick. The cron is "when to check", not "guaranteed execution slot."

## Project Directory

Each project has a `.anvil/todos/` tree. Each todo is a markdown file with its own cron schedule in YAML frontmatter:

```
<project>/.anvil/
└── todos/
    ├── p0/                          ← highest priority
    │   └── triage-github-issues.md
    ├── p1/
    │   └── review-stale-prs.md
    └── ...                          ← up to p9 (lowest priority)
```

Priority directories are created on-demand when tasks are added. You don't need to pre-create them.

Todos are just files. The daemon works through all matching ones — highest priority, oldest first — running up to `max_workers` in parallel. With `max_workers: 2` and 6 matching todos, 2 workers pull from the queue continuously: as each worker finishes, it picks up the next available todo. All 6 get processed, 2 at a time.

### Todo Format

```markdown
---
id: "550e8400-e29b-41d4-a716-446655440000"
schedule: "*/30 * * * *"
---
Check GitHub for new untriaged issues. For each unlabelled issue,
read the content and apply appropriate labels (bug, feature, docs, question).
```

The `id` is generated automatically by `anvil add` and used for session tracking. The schedule is a standard 5-field cron expression. The body is passed directly to the configured runner command.

### Session Continuity

By default, recurring tasks resume their previous Claude session and one-shot tasks start fresh. Override this with `resume` in the frontmatter:

```markdown
---
id: "550e8400-e29b-41d4-a716-446655440000"
schedule: "*/30 * * * *"
resume: false
---
```

- `resume: true` (default for recurring tasks): subsequent runs use `--resume <session-id>` to continue the previous session. The first run starts fresh.
- `resume: false` (default for one-shot tasks): always starts a new session with `--session-id`.
- Omit `resume` to use the default behavior.

One-shot tasks (empty schedule) are automatically deleted from disk after successful execution.

## Daemon Directory

Lives at `~/.anvil/`. One per machine. No project-specific anything.

```
~/.anvil/
├── config.yaml
└── watched/
    ├── a1b2c3d4/                  ← sha256[:8] of project path
    │   └── 2026-02-23T13-07-49.md
    └── e5f6g7h8/
        └── 2026-02-23T14-00-00.md
```

Each watched file has YAML frontmatter with the project path and timestamp:

```yaml
---
path: /Users/you/projects/my-app
watched_at: 2026-02-23T13:07:49Z
---
```

`anvil watch` creates these files. `anvil unwatch` deletes the hash directory. The daemon scans `watched/` on every tick to discover registered projects.

### Daemon Config

```yaml
# ~/.anvil/config.yaml
tick_interval: 10s          # how often to check for work
runners:                    # ordered list of runner commands; first success wins
  - "claude -p"
timeout: 5m                 # max time per execution
max_workers: 4              # worker pool size (all tasks still get processed)
```

For backwards compatibility, a single `runner:` string is still accepted and treated as a one-entry list.

## CLI

Single binary. One subcommand starts the daemon, everything else talks to it.

```
anvil init [path]                    Initialize a project (.anvil/ and .claude/skills/)
anvil serve                          Start the daemon (run once per machine)
anvil watch [path]                   Register a project directory with the daemon
anvil unwatch [path]                 Stop watching a project directory
anvil add [options] <task>           Add a todo to the current project
anvil list                           List all todos in the current project
anvil get <name>                     Show details of a todo by name
anvil delete <name>                  Delete a todo by name
anvil log [-f] <name>                Show session log for a todo (-f to follow)
anvil logs [<name>]                  Raw worker output (live if running, last run if not)
anvil status                         Show all watched projects and their state
anvil ps                             Show currently executing tasks
anvil task <subcommand>              Task management commands
anvil project <subcommand>           Project management commands
```

### anvil add

```bash
# Add a high-priority task that runs every 30 minutes
anvil add -p 0 -s "*/30 * * * *" "Check GitHub for new issues and triage them"

# Add a normal-priority task that runs weekday mornings
anvil add -p 1 -s "0 9 * * 1-5" "Review stale PRs and nudge authors"

# Defaults: priority 1, schedule every minute
anvil add "Run the health check"

# One-shot task (no schedule — runs once on next tick, then gets deleted)
anvil add -s "" "Migrate the database schema"
```

Options:
- `-p`, `--priority` — Task priority 0-9 (default: 1)
- `-s`, `--schedule` — Cron schedule (default: `* * * * *`); pass `""` for one-shot

### anvil task

Full task lifecycle management:

```
anvil task create [options] <task>   Create a new task
anvil task ls [-a|--all]             List tasks (--all for all watched projects)
anvil task get <name>                Show task details including run status
anvil task log [-f] <name>           Show execution log (-f to follow)
anvil task rm <name>                 Remove a task (kills if running)
anvil task kill <name>               Kill a running task
```

### anvil project

Manage watched projects:

```
anvil project create [path]          Initialize and watch a project in one step
anvil project ls [-a|--all]          List watched projects
anvil project get [path]             Show project details and running tasks
anvil project rm [path] [--clean]    Unwatch a project (--clean removes .anvil/ too)
```

### Typical Usage

```bash
# On the machine, once:
anvil serve

# In any project directory:
anvil watch
anvil add -p 0 -s "*/30 * * * *" "Check GitHub for new issues and triage them"
anvil add -p 1 -s "0 9 * * 1-5" "Review stale PRs and nudge authors"

# Done. The daemon picks up todos on schedule.
```

## Cron Format

Standard 5-field cron, set per-todo in YAML frontmatter:

```
┌─── minute (0-59)
│ ┌─── hour (0-23)
│ │ ┌─── day of month (1-31)
│ │ │ ┌─── month (1-12)
│ │ │ │ ┌─── day of week (0-6, Sun=0)
│ │ │ │ │
* * * * *
```

`*/15 * * * *` = every 15 min · `0 9 * * 1-5` = weekdays at 9am · `* * * * *` = every minute

## Philosophy

The daemon is dumb on purpose. It doesn't interpret todos — it just hands them to the runner. Projects own their work. The daemon owns the schedule. One process, one machine, many projects.
