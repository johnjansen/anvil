# Anvil

One daemon per machine. Many projects. The daemon is the central dispatcher — it watches registered project directories, checks their cron schedules, and runs their todos through a configured LLM runner.

## How It Works

```
  project A/.anvil/         project B/.anvil/         project C/.anvil/
  ├── todos/                ├── todos/                ├── todos/
  │   ├── p0/               │   ├── p0/               │   ├── p0/
  │   └── p1/               │   └── p1/               │   └── p1/
  └── anvil.yaml            └── anvil.yaml            └── anvil.yaml
        │                         │                         │
        └─────────────────────────┼─────────────────────────┘
                                  │ anvil watch
                                  ▼
                     ┌────────────────────────┐
                     │     Daemon (singleton) │
                     │       ~/.anvil/        │
                     │                        │
                     │  tick:                  │
                     │   for each project:    │
                     │    → cron match?       │
                     │    → grab top N todos  │
                     │    → shell out runner  │
                     └────────────────────────┘
```

1. Start the daemon once: `anvil serve`
2. From any project, register it: `anvil watch`
3. On every tick the daemon iterates all watched projects, checks each project's cron, and if it matches, grabs the top N todos (N from daemon config) and shells out the configured runner with each
4. The daemon manages all running processes across all projects.

If a project is still busy processing todos when the next tick fires, that tick is skipped. This is intentional — the work matters more than the schedule. The cron is "when to check", not "guaranteed execution slot."

## Project Directory

Each project has its own `.anvil/` with its schedule and work:

```
<project>/.anvil/
├── anvil.yaml          ← this project's schedule + priority
└── todos/
    ├── p0/             ← highest priority
    │   ├── fix-bug.md
    │   └── review-pr.md
    ├── p1/
    └── ...p9/          ← lowest priority
```

Todos are just files. The daemon works through all of them — highest priority, oldest first — running up to `max_todos` in parallel. If there are 6 todos and `max_todos: 2`, it runs 2, waits for both to finish, runs the next 2, waits, runs the last 2. All 6 complete before this project's tick is done.

Todos are **deleted after successful execution**. If a todo fails, it stays in place for the next tick.

### Project Config

```yaml
# <project>/.anvil/anvil.yaml
schedule: "*/15 * * * *"   # when this project is eligible to run
priority: 0                # cross-project priority (lower = first)
```

The schedule controls *when* the daemon checks this project. The priority controls *which project goes first* when multiple projects are due on the same tick.

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
anvil serve              Start the daemon (run once per machine)
anvil watch [path]       Register a project directory with the daemon
anvil unwatch [path]     Stop watching a project directory
anvil status             Show all watched projects and their state
anvil ps                 Show currently executing tasks
```

### Typical Usage

```bash
# On the machine, once:
anvil serve

# In any project directory (e.g. from an LLM session):
anvil watch

# Done. The daemon picks up todos on schedule.
# Add more projects the same way:
cd /other/project && anvil watch
```

## Cron Format

Standard 5-field cron in each project's `.anvil/anvil.yaml`:

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