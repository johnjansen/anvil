# Three Tools, One Workflow: Building a CLI-Native Development Ecosystem

> **A note on provenance.** This post was written entirely by the system it describes — specked, planned, and drafted by an LLM working through these tools. Nothing here claims to be a finished process. Think of it as a minimum viable workflow: enough structure to be useful, rough enough to keep evolving.

You know the feeling. You're deep in a coding session, flow state locked in, and then you need to check your task board. So you switch to a browser. Log into some planning tool. Click through projects and sprints. Create an issue. Copy a reference back to your terminal. By the time you return to your editor, the thread you were holding in your head has unraveled.

This is the tax we pay for tools that live outside our primary workspace. Planning in one app, issues in another, automation in a third. Each one demands its own context, its own login, its own mental model. The gap between thinking and doing keeps growing.

What if all of that lived where you already work — in the terminal, in your repo, in plain text you can read, diff, and commit?

I've been building a small ecosystem of three CLI tools that together cover the full arc of feature development: specification, planning, issue tracking, and automated execution. Each does one thing. Each works alone. But together, they replace a surprising amount of heavyweight planning infrastructure.

## The Unix Way, Applied to Development Workflow

The philosophy isn't new. Small programs that do one thing well, communicate through text, and compose into pipelines. It's the same principle that made Unix tools last for decades.

Most developer tooling has drifted the other direction. Planning platforms try to be everything — issues, docs, roadmaps, time tracking, reporting — behind one login. The tradeoff is bloat, lock-in, and a workflow that keeps pulling you out of the terminal.

These three tools take a different path. Each handles a distinct phase of development. They store data as plain files in your repo. They talk through the filesystem, not APIs or proprietary sync. And because they're CLI tools, they fit into the environment you're already using.

The tools: **speckit** for specification-driven planning, **beads** (`bd`) for lightweight issue tracking, and **anvil** for scheduled task automation.

## Speckit: From Idea to Structured Plan

Every feature starts as a vague idea. Speckit turns that into a structured specification, then a plan, then a task breakdown — all as markdown files committed to your repo.

The workflow is a pipeline of slash commands inside your AI coding assistant:

```bash
/speckit.specify "Add user authentication with OAuth"
/speckit.plan
/speckit.tasks
/speckit.implement
```

Each command reads the previous one's output. `/speckit.specify` generates a spec with user stories, requirements, and acceptance criteria. `/speckit.plan` produces an implementation plan with architecture decisions. `/speckit.tasks` breaks it into an ordered, dependency-aware task list. `/speckit.implement` executes them.

Everything lands in a numbered feature directory — `specs/003-user-auth/spec.md`, `plan.md`, `tasks.md` — right next to your source code. No browser tabs. No separate tool. Just markdown that evolves with the code it describes.

## Beads: Issues That Live in Your Repo

Beads (`bd`) is a git-native issue tracker. Issues are stored as data files inside your repository — not in a SaaS database, not behind an API. Push your code and your issues go with it.

```bash
bd create "Fix login redirect after OAuth callback"
bd list
bd close bd-42 -r "Fixed in commit abc123"
bd ready    # Show unblocked work
```

It handles what you'd expect — labels, assignees, due dates — plus first-class dependency tracking. Model which issues block others, group work into epics, use `bd ready` to see exactly what's unblocked.

The data lives in JSONL files designed for git. They merge cleanly, diff readably, and sync across branches. No server to maintain, no subscription. Your issue database is just part of your codebase.

## Anvil: Automated Tasks on a Schedule

Anvil is a task scheduler for LLM-powered development work. Describe tasks in plain English, give them a cron schedule, and anvil runs them with an AI coding assistant as the executor.

```bash
anvil watch                                    # Start the daemon
anvil init                                     # Initialize your project
anvil add -s "*/30 * * * *" "Triage new GitHub issues and label them"
anvil add -s "0 9 * * 1-5" "Review open PRs and leave feedback"
anvil task ls                                  # See what's scheduled
anvil task log triage-new-github-issues        # Check execution output
```

Tasks are markdown files with YAML frontmatter, stored in `.anvil/todos/` and organized by priority. One daemon watches multiple projects, dispatching on schedule. You can gate execution with pre-check commands, configure retries, and define hooks for success or failure.

The insight here is that a lot of recurring development chores — issue triage, PR reviews, CI checks, doc updates — are a good fit for LLM automation. Anvil makes that a first-class part of your project instead of a fragile cron job tucked away on a server.

## Putting It Together: A Feature from Start to Finish

Each tool pulls its weight alone. But here's what it looks like when you use all three on a single feature.

Say you're adding search to a project. Start with speckit:

```bash
/speckit.specify "Add full-text search to the API"
```

That generates a spec with user stories, acceptance criteria, and requirements in `specs/004-full-text-search/spec.md`. Review it, refine it, then plan:

```bash
/speckit.plan
/speckit.tasks
```

Now you have a spec, a technical plan, and an ordered task list — all committed markdown. Bring in beads to track the work:

```bash
bd create "Implement search indexing service"
bd create "Add search API endpoint" --deps "blocks:bd-51"
bd create "Write integration tests for search" --deps "blocks:bd-51,blocks:bd-52"
```

Dependencies are modeled, so `bd ready` shows what's actually unblocked. Close issues as you go: `bd close bd-51 -r "Implemented in src/services/search.go"`.

Meanwhile, anvil keeps the recurring stuff from falling through the cracks. If you haven't already, start the daemon — it's a global process, one per machine, not per project:

```bash
anvil watch    # Only needed once — it watches all your projects
```

Then add tasks for this feature:

```bash
anvil add -s "0 */4 * * *" "Check if search index tests are passing and report failures"
anvil add -s "0 9 * * 1-5" "Review any open PRs related to the search feature"
```

The daemon picks those up automatically. It already knows about your project from `anvil init`, and it monitors every registered project from that single process. The tasks run on schedule — an LLM reads your codebase and acts on what it finds. No manual checking, no forgotten follow-ups.

The whole workflow stays in the terminal. Every artifact — specs, plans, tasks, issues, automation — lives in the repo, versioned alongside the code.

## Everything Lives in Git

This is the architectural idea that ties it together. In a typical setup, planning data is scattered across tools: issues in one SaaS, specs in a wiki, tasks on a board, automation in CI config. None of it versioned with your code.

With these tools, the project directory tells the whole story:

```
my-project/
├── specs/004-full-text-search/
│   ├── spec.md          # What we're building and why
│   ├── plan.md          # How we're building it
│   └── tasks.md         # The ordered work breakdown
├── .beads/              # Issue database — JSONL files, git-friendly
├── .anvil/
│   └── todos/           # Scheduled automation tasks as markdown
└── src/                 # The code itself
```

Clone the repo and you get everything — not just code, but the decisions, the issues, the automation. Branch for a feature and the planning artifacts branch with it. Review a PR and the spec is right there in the diff.

It also changes how teams share context. New contributors read the spec directory to understand not just *what* the code does, but *why* it was built that way.

## Skills: Teaching Your AI Assistant the Ecosystem

There's one more layer worth mentioning. Each tool ships with **skills** — markdown files that teach AI coding assistants how to use the CLI. When you run `anvil init`, the [anvil skill](anvil-skill.md) is automatically installed into your project's `.claude/skills/` directory. Your assistant picks up the full interface immediately — no setup, no configuration.

Here's what that looks like in practice:

```
You:    add a task to review open PRs every weekday morning

Claude: I'll add a recurring task for that.

        > anvil add -s "0 9 * * 1-5" "Review open PRs and leave feedback"

        Created task review-open-prs-and-leave-feedback (priority 1,
        schedule: weekdays at 9am). The daemon will pick it up on
        the next tick.
```

No docs lookup, no remembering flags. The skill file has the full CLI reference, so the assistant translates intent into the right command.

Skills also bridge the tools. In projects using both speckit and beads, a [`/speckit.taskstobeads`](speckit-taskstobeads-skill.md) skill reads the task breakdown and creates beads issues with priorities, labels, and dependency chains already wired up:

```
You:    /speckit.taskstobeads

Claude: I'll convert the 14 tasks from tasks.md into beads issues.

        Creating issues...
        > bd create "T001: Set up project structure" -p 0 -l foundational,speckit --silent
        > bd create "T002: Implement core service" -p 1 -l US1,speckit --silent
        > bd create "T003: Add API endpoint" -p 1 -l US1,speckit --deps "blocks:bd-52" --silent
        ...

        Created 14 issues across 4 phases.
        Added 8 dependency links.

        > bd list
```

This is the kind of integration that falls out naturally when tools share a workspace. The skill reads speckit's output, translates it to beads commands, and wires up dependencies — no custom integration code. Just one tool reading another tool's files and calling another tool's CLI.

## Start With One

You don't need all three. That's the whole point.

If you're losing track of work across a project, start with beads. Run `bd create` a few times, use `bd list` and `bd ready`, see if git-native tracking fits.

If you want more structure before coding — clearer specs, acceptance criteria, a task breakdown — try speckit. Works fine without beads.

If recurring chores keep slipping — PR reviews, triage, doc checks — anvil puts them on a schedule with an LLM executor.

Once one tool clicks, the others plug in naturally. Speckit's output feeds beads issues. Beads surfaces work anvil can automate. But none of that's required. Use what solves your problem today.

## Or Start With Everything

If you'd rather skip the incremental approach and bootstrap the full ecosystem at once, there's a skill for that.

The [anvil-software-factory](anvil-software-factory.md) skill installs all three tools, initializes a project directory, starts the anvil daemon, and walks you through onboarding — all from inside Claude. Install it with one command:

```bash
mkdir -p .claude/skills/software-factory && curl -fsSL \
  https://raw.githubusercontent.com/johnjansen/anvil/main/blog/anvil-software-factory.md \
  -o .claude/skills/software-factory/SKILL.md
```

Then invoke it:

```
You:    /anvil-software-factory my-new-project

Claude: I'll set up the full development ecosystem for my-new-project.

        Installing beads... ✓ (bd version 0.55.4)
        Installing anvil... ✓ (anvil v0.24.1)
        Installing speckit commands... ✓ (9 slash commands)

        Initializing project...
        > git init
        > bd init
        > anvil init
        > anvil watch -d

        Setup complete. You now have:
        - /speckit.specify to start planning features
        - bd create to track issues
        - anvil add to schedule automated tasks

        Run /speckit.specify "describe your first feature" to get started.
```

The skill handles platform detection, checks for prerequisites, skips anything already installed, and gives you a working setup in about thirty seconds. It's the fastest path from an empty directory to a fully-wired spec-to-automation pipeline.

## Try It

All three are open source:

- **[speckit](https://github.com/johnjansen/speckit)** — Specification-driven development workflow
- **[beads](https://github.com/johnjansen/beads)** — Lightweight, git-native issue tracking
- **[anvil](https://github.com/johnjansen/anvil)** — Scheduled LLM task automation

Pick whichever matches your current friction. Use it on a real project for a week. If it helps, add a second. The ecosystem grows with your needs, not ahead of them.
