# Anvil Specification

## Overview

Anvil is a single-binary task scheduler for LLM projects. One daemon per machine watches multiple project directories, checks each todo's cron schedule on every tick, and runs matching tasks through a configured runner command.

## Core Principles

1. **File-based state** — Todos are markdown files on disk. No database, no queue.
2. **Per-todo scheduling** — Each todo carries its own cron expression in YAML frontmatter.
3. **Priority ordering** — Todos live in `p0/`–`p9/` directories. Lower number = higher priority.
4. **Dumb daemon** — The daemon doesn't interpret todos. It just hands them to the runner.
5. **One process** — Single daemon manages all projects on the machine.

---

## Data Structures

### Todo

A markdown file in `<project>/.anvil/todos/pN/`:

```markdown
---
schedule: "*/30 * * * *"
---
Check GitHub for new untriaged issues and label them.
```

Parsed into:

```go
type Todo struct {
    Path     string // absolute path to the file
    Name     string // filename
    Priority int    // 0-9, from pN/ directory
    Content  string // file contents (after front-matter)
    Schedule string // cron expression from front-matter
}
```

### Project

```go
type Project struct {
    Path string // absolute path to the project directory
}
```

### Daemon Config

```go
type Config struct {
    TickInterval time.Duration `yaml:"tick_interval"` // how often to check (default 10s)
    Runner       string        `yaml:"runner"`         // command to run todos (default "echo")
    Timeout      time.Duration `yaml:"timeout"`        // per-execution timeout (default 5m)
    MaxTodos     int           `yaml:"max_todos"`      // parallel todos per project (default 1)
}
```

---

## Daemon Tick Loop

On every tick:

```
for each watched project:
    if project is still busy from last tick → skip
    load all todos from .anvil/todos/pN/
    for each todo:
        if todo.Schedule matches current time → collect
    run collected todos through runner (up to max_todos in parallel)
```

If a project is busy when the next tick fires, it's skipped. The cron is "when to check", not "guaranteed execution slot."

### Batch Execution

When `max_todos > 1`, todos are processed in batches:
- Run `max_todos` concurrently
- Wait for entire batch to complete
- Run next batch
- Continue until all matching todos are processed

---

## Directory Layout

### Per-project

```
<project>/
├── .anvil/
│   └── todos/
│       ├── p0/                    ← highest priority
│       │   └── triage-issues.md
│       └── p1/
│           └── review-prs.md
└── .claude/
    └── skills/                    ← embedded skills from binary
        ├── prompts/
        ├── create-skill/
        ├── install-skill/
        └── telegram-react/
```

Priority directories (`p0/`–`p9/`) are created on-demand by `anvil add`. Not pre-created.

### Global daemon

```
~/.anvil/
├── config.yaml
└── watched/
    └── <sha256[:8]>/              ← hash of project absolute path
        └── <timestamp>.md         ← YAML frontmatter with path + watched_at
```

---

## CLI Commands

| Command | Description |
|---------|-------------|
| `anvil init [path]` | Create `.anvil/todos/` and `.claude/skills/` (with embedded tools) |
| `anvil serve` | Start the daemon (one per machine) |
| `anvil watch [path]` | Register a project directory (runs init if needed) |
| `anvil unwatch [path]` | Stop watching a project |
| `anvil add [opts] <text>` | Create a todo (`-p` priority, `-s` schedule) |
| `anvil list` | List all todos in current project |
| `anvil get <name>` | Show full details of a todo |
| `anvil delete <name>` | Remove a todo |
| `anvil status` | Show all watched projects |
| `anvil ps` | Show running tasks |

### anvil add defaults

- Priority: `1`
- Schedule: `* * * * *` (every minute)
- Filename: slugified from task text, max 50 chars, `.md` extension

---

## Cron Parser

Standard 5-field format:

```
┌─── minute (0-59)
│ ┌─── hour (0-23)
│ │ ┌─── day of month (1-31)
│ │ │ ┌─── month (1-12)
│ │ │ │ ┌─── day of week (0-6, Sun=0)
│ │ │ │ │
* * * * *
```

Supports: `*`, `*/n`, `n`, `n,m`, `n-m`

---

## Embedded Tools

The `tools/skills/` directory is compiled into the binary via `//go:embed`. On `anvil init`, these files are written to `<project>/.claude/skills/`, giving each project a starter set of Claude skills:

- **prompts/** — Bootstrap, identity, soul, user templates, heartbeat
- **create-skill/** — Skill to create new skills
- **install-skill/** — Skill to search and install from skills.sh
- **telegram-react/** — Telegram reaction directive

---

## File Structure

```
anvil/
├── cmd/anvil/
│   └── main.go              ← CLI entry point, all commands
├── internal/
│   ├── config/
│   │   └── config.go        ← daemon config (~/.anvil/config.yaml)
│   ├── cron/
│   │   └── parser.go        ← cron expression matching
│   ├── daemon/
│   │   └── daemon.go        ← tick loop, project scanning, execution
│   ├── project/
│   │   └── project.go       ← todo loading, creation, init
│   └── runner/
│       └── runner.go        ← shell out to configured command
├── tools/
│   ├── embed.go             ← //go:embed for skills directory
│   └── skills/              ← embedded skill files
├── example_proj/             ← example project with sample todos
├── go.mod
├── go.sum
├── README.md
└── SPEC.md
```

---

## Future Considerations

- **Persistence** — Track execution history (last run time, success/failure)
- **Web UI** — Dashboard showing watched projects and their todos
- **Notifications** — Webhooks or messages on task failure
- **Task dependencies** — Run B after A completes
- **Log capture** — Store runner stdout/stderr per execution
