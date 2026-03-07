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
id: "550e8400-e29b-41d4-a716-446655440000"
schedule: "*/30 * * * *"
resume: true
---
Check GitHub for new untriaged issues and label them.
```

Parsed into:

```go
type Todo struct {
    Path          string // absolute path to the file
    Name          string // filename
    Priority      int    // 0-9, from pN/ directory
    Content       string // file contents (after front-matter)
    Schedule      string // cron expression from front-matter (empty = one-shot)
    ID            string // UUID for session tracking
    Resume        *bool  // nil = default, explicit true/false overrides
    MaxConcurrent int    // max simultaneous instances (0 = default 1)
}
```

### RunRecord

Persists metadata for a single task dispatch, written after completion:

```go
type RunRecord struct {
    RunID     string    `json:"run_id"`
    TaskID    string    `json:"task_id"`
    SessionID string    `json:"session_id"`
    PID       int       `json:"pid"`
    Started   time.Time `json:"started"`
}
```

Run records live at `<project>/.anvil/runs/<task-id>/<run-id>.json` with a `current` file pointing to the latest run.

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
    Runners      []string      `yaml:"runners"`       // ordered runner commands, first success wins
    Runner       string        `yaml:"runner"`         // single runner (backwards compat)
    Timeout      time.Duration `yaml:"timeout"`        // per-execution timeout (default 5m)
    MaxWorkers   int           `yaml:"max_workers"`    // worker pool size (default 1)
}
```

---

## Daemon Architecture

### Worker Pool

The daemon runs a fixed-size worker pool (`max_workers` goroutines). On each cron tick, matching tasks are collected across all projects, sorted by global priority, and dispatched to a buffered work queue. Workers pull from the queue continuously — as each finishes, it picks up the next task.

### Tick Loop

On every tick (default 10s):

```
load all watched project paths from ~/.anvil/watched/
collect all todos across all projects
for each todo:
    if one-shot (empty schedule) → always matches
    if recurring → check cron against current minute (once per minute)
    skip if already in-flight (queued or executing)
    skip one-shot with stale lock file
sort matched todos by priority (p0 first), then name
dispatch to work queue (non-blocking; drop + warn if full)
```

Between cron ticks, the scheduler logs heartbeats showing running tasks and elapsed time.

### Runner Fallback Chain

The daemon tries each configured runner command in order. If the first fails (non-zero exit), it tries the next. First success wins. Each runner invocation either:

- **Resumes** a previous session: `<command> --resume <session-id> <content>`
- **Starts fresh**: `<command> --session-id <fresh-uuid> <content>`

### Session Continuity

- **Recurring tasks** (have a schedule): resume by default — subsequent runs use `--resume` to continue the previous Claude session
- **One-shot tasks** (empty schedule): start fresh by default — always use `--session-id` with a new UUID
- **Explicit override**: set `resume: true` or `resume: false` in frontmatter to override defaults

### One-shot Tasks

Tasks with an empty schedule (`schedule: ""`) run once on the next tick, then the todo file is deleted after successful execution. A lock file (`<todo>.lock`) prevents re-delivery if the daemon crashes mid-run.

### Kill Mechanism

Kill requests are sent via the unix socket (`POST /kill`). The daemon cancels the task's context directly for immediate termination.

### Unix Socket API

The daemon exposes a unix socket at `~/.anvil/daemon.sock`:

- `GET /ps` — Returns JSON array of running tasks (project, name, PID, started, elapsed)
- `POST /kill` — Accepts `{"id": "<name-or-uuid>"}`, cancels the matching task's context

### Environment Cleanup

The runner strips all `CLAUDE*` and `ANTHROPIC*` environment variables (except `ANTHROPIC_API_KEY` and `ANTHROPIC_BASE_URL`) to prevent recursive invocation detection when spawning Claude processes.

---

## Directory Layout

### Per-project

```
<project>/
├── .anvil/
│   ├── todos/
│   │   ├── p0/                    ← highest priority
│   │   │   └── triage-issues.md
│   │   └── p1/
│   │       └── review-prs.md
│   └── runs/
│       └── <task-uuid>/           ← run records per task
│           ├── <run-id>.json
│           └── current            ← points to latest run
└── .claude/
    └── skills/
        └── anvil/
            └── SKILL.md           ← anvil CLI skill (embedded from binary)
```

Priority directories (`p0/`–`p9/`) are created on-demand by `anvil add`. Not pre-created.

### Global daemon

```
~/.anvil/
├── config.yaml
├── daemon.sock                    ← unix socket for ps/kill
└── watched/
    └── <sha256[:8]>/              ← hash of project absolute path
        └── <timestamp>.md         ← YAML frontmatter with path + watched_at
```

---

## CLI Commands

### Top-level

| Command | Description |
|---------|-------------|
| `anvil init [path]` | Create `.anvil/todos/` and `.claude/skills/` |
| `anvil serve` | Start the daemon (one per machine) |
| `anvil watch [path]` | Register a project directory (runs init if needed) |
| `anvil unwatch [path]` | Stop watching a project |
| `anvil add [opts] <text>` | Create a todo (`-p` priority, `-s` schedule) |
| `anvil list` | List all todos in current project |
| `anvil get <name>` | Show full details of a todo |
| `anvil delete <name>` | Remove a todo |
| `anvil log [-f] <name>` | Show session log for a todo (`-f` to follow) |
| `anvil status` | Show all watched projects |
| `anvil ps` | Show running tasks |
| `anvil version` | Show version |

### anvil task

| Command | Description |
|---------|-------------|
| `anvil task create [opts] <text>` | Create a task (same as `anvil add`) |
| `anvil task ls [-a\|--all]` | List tasks (current project or all) |
| `anvil task get <name>` | Show task details including run status |
| `anvil task log [-f] <name>` | Show execution log |
| `anvil task rm <name>` | Remove task (kills if running) |
| `anvil task kill <name>` | Kill a running task via socket |

### anvil project

| Command | Description |
|---------|-------------|
| `anvil project create [path]` | Init + watch in one step |
| `anvil project ls [-a\|--all]` | List watched projects |
| `anvil project get [path]` | Show project details + running tasks |
| `anvil project rm [path] [--clean]` | Unwatch (`--clean` removes `.anvil/`) |

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

## Embedded Skills

The `tools/skills/` directory is compiled into the binary via `//go:embed`. On `anvil init`, these files are written to `<project>/.claude/skills/`:

- **anvil/** — CLI skill for managing tasks via Claude

---

## Logging

The daemon uses structured, colored log output (when connected to a TTY):

- **Scheduler** messages: tick summaries, dispatch events, idle/running status
- **Worker** messages: color-coded by worker ID, showing pickup/done/fail/idle
- **Priority** coloring: p0=red, p1=yellow, p2=cyan, p3+=default
- **OSC 8 links**: task names in logs link to their session JSONL files (in supported terminals)

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
│   │   ├── daemon.go        ← worker pool, tick loop, socket server
│   │   └── logger.go        ← structured colored log output
│   ├── project/
│   │   └── project.go       ← todo loading, creation, run records, init
│   └── runner/
│       └── runner.go        ← fallback chain, session management, env cleanup
├── tools/
│   ├── embed.go             ← //go:embed for skills directory
│   └── skills/
│       └── anvil/
│           └── SKILL.md     ← embedded anvil CLI skill
├── go.mod
├── go.sum
├── .gitignore
├── README.md
└── SPEC.md
```
