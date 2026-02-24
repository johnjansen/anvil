# Anvil

Scheduled LLM tasks for your projects. Write a task in plain English, give it a cron schedule, and anvil runs it automatically through Claude.

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

## Configuration

`~/.anvil/config.yaml`:

```yaml
runners:
  - claude
max_workers: 10    # parallel tasks
timeout: 15m       # max per task
tick_interval: 5s  # how often to check for work
```

Multiple runners with fallback:

```yaml
runners:
  - claude -p "you are a helpful task runner"
  - claude --model haiku -p "you are a helpful task runner"
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `anvil watch` | Start the daemon |
| `anvil init [path]` | Initialize a project |
| `anvil add [opts] <task>` | Add a task (`-s` schedule, `-p` priority) |
| `anvil status` | Show watched projects |
| `anvil update` | Update to latest release |

**Task management:**

| Command | Description |
|---------|-------------|
| `anvil task ls` | List tasks in current project |
| `anvil task ls --all` | List tasks across all projects |
| `anvil task get <name>` | Show task details |
| `anvil task log <name>` | Show execution log |
| `anvil task log -f <name>` | Follow live log output |
| `anvil task rm <name>` | Remove a task |
| `anvil task kill <name>` | Kill a running task |

**Project management:**

| Command | Description |
|---------|-------------|
| `anvil project ls` | List watched projects |
| `anvil project get [path]` | Show project and running tasks |
| `anvil project rm [path]` | Unwatch a project |

## Cron Format

Standard 5-field: `minute hour day month weekday`

```
*/15 * * * *      every 15 minutes
0 9 * * 1-5       weekdays at 9am
0 */6 * * *       every 6 hours
0 2 * * *         daily at 2am
```
