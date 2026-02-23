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
                     │  tick:                   │
                     │   for each project:     │
                     │    → for each todo:     │
                     │      → cron match?      │
                     │      → shell out runner │
                     └────────────────────────┘
```

1. Start the daemon once: `anvil serve`
2. From any project, register it: `anvil watch`
3. Add recurring tasks: `anvil add -p 0 -s "*/30 * * * *" "Check GitHub for new issues and triage them"`
4. On every tick the daemon iterates all watched projects, checks each todo's cron schedule, and runs matching tasks through the configured runner
5. The daemon manages all running processes across all projects

If a project is still busy processing todos when the next tick fires, that tick is skipped. This is intentional — the work matters more than the schedule. The cron is "when to check", not "guaranteed execution slot."

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

Todos are just files. The daemon works through all matching ones — highest priority, oldest first — running up to `max_todos` in parallel. If there are 6 matching todos and `max_todos: 2`, it runs 2, waits for both to finish, runs the next 2, waits, runs the last 2.

### Todo Format

```markdown
---
schedule: "*/30 * * * *"
---
Check GitHub for new untriaged issues. For each unlabelled issue,
read the content and apply appropriate labels (bug, feature, docs, question).
```

The schedule is a standard 5-field cron expression. The body is passed directly to the configured runner command.

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
runner: "claude -p"         # command that runs todos (content passed as argument)
timeout: 5m                 # max time per execution
max_todos: 1                # max parallel todos per project (all todos still get processed)
```

## CLI

Single binary. One subcommand starts the daemon, everything else talks to it.

```
anvil init [path]                    Initialize a project (.anvil/ and .claude/skills/)
anvil serve                          Start the daemon (run once per machine)
anvil watch [path]                   Register a project directory with the daemon
anvil unwatch [path]                 Stop watching a project directory
anvil add [options] <task>           Add a recurring todo to the current project
anvil list                           List all todos in the current project
anvil get <name>                     Show details of a todo by name
anvil delete <name>                  Delete a todo by name
anvil status                         Show all watched projects and their state
anvil ps                             Show currently executing tasks
```

### anvil add

```bash
# Add a high-priority task that runs every 30 minutes
anvil add -p 0 -s "*/30 * * * *" "Check GitHub for new issues and triage them"

# Add a normal-priority task that runs weekday mornings
anvil add -p 1 -s "0 9 * * 1-5" "Review stale PRs and nudge authors"

# Defaults: priority 1, schedule every minute
anvil add "Run the health check"
```

Options:
- `-p`, `--priority` — Task priority 0-9 (default: 1)
- `-s`, `--schedule` — Cron schedule (default: `* * * * *`)

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
