---
title: "Three Tools, One Workflow: Building a CLI-Native Development Ecosystem"
date: 2026-02-25
tags: [cli, developer-tools, workflow, automation, unix-philosophy, open-source]
description: "How three composable CLI tools — speckit, beads, and anvil — replace heavyweight planning software by keeping specs, issues, and automated tasks in the terminal and in version control."
---

# Three Tools, One Workflow: Building a CLI-Native Development Ecosystem

You know the feeling. You're deep in a coding session, flow state fully engaged, and then you need to check your task board. So you switch to a browser tab. Log into a planning tool. Click through a hierarchy of projects and sprints. Create an issue. Copy a reference back into your terminal. By the time you return to your editor, the thread you were holding in your head has unraveled.

This is the tax developers pay for using tools that live outside their primary workspace. Planning happens in one application, issue tracking in another, task automation in yet another. Each tool demands its own context, its own login, its own mental model. The result is a fragmented workflow where the space between thinking and doing keeps growing.

What if your entire project management workflow lived where you already work — in the terminal, in your repository, in plain text files you can read, diff, and commit?

That's the idea behind a small ecosystem of three CLI tools that, together, cover the full arc of feature development: from specification to planning to issue tracking to automated execution. Each tool does one thing. Each tool works alone. But when they work together, they replace a surprising amount of heavyweight planning infrastructure.

## The Unix Way, Applied to Development Workflow

The philosophy here isn't new. It's the same principle that made Unix tools endure for decades: small programs that do one thing well, communicate through text, and compose into pipelines greater than their parts.

Most developer tooling has drifted in the opposite direction. Planning platforms try to be everything — issue tracking, documentation, roadmaps, time tracking, reporting — all behind a single login. The tradeoff is bloat, lock-in, and a workflow that pulls you out of the terminal where the actual work happens.

These three tools take a different path. Each one handles a distinct phase of the development lifecycle. They store their data as plain files in your repository. They communicate through the filesystem, not through APIs or proprietary sync protocols. And because they're CLI tools, they slot into the environment you're already in — your shell, your scripts, your automation.

The tools are: **speckit** for specification-driven planning, **beads** (or `bd`) for lightweight issue tracking, and **anvil** for scheduled task automation.

## Speckit: From Idea to Structured Plan

Every feature starts as a vague idea. Speckit turns that idea into a structured specification, then an implementation plan, then a task breakdown — all as markdown files in your repository.

The workflow is a pipeline of slash commands inside your AI coding assistant:

```bash
/speckit.specify "Add user authentication with OAuth"
/speckit.plan
/speckit.tasks
/speckit.implement
```

Each command reads the output of the previous one. `/speckit.specify` generates a spec with user stories, requirements, and acceptance criteria. `/speckit.plan` produces a technical plan with architecture decisions and research. `/speckit.tasks` breaks the plan into an ordered, dependency-aware task list. `/speckit.implement` executes the tasks.

The artifacts land in a numbered feature directory — `specs/003-user-auth/spec.md`, `specs/003-user-auth/plan.md`, `specs/003-user-auth/tasks.md` — right next to your source code. No browser tabs. No separate planning tool. Just markdown files that evolve alongside the code they describe.

## Beads: Issues That Live in Your Repo

Beads (`bd`) is a git-native issue tracker. Issues are stored as data files inside your repository — not in a SaaS database, not behind an API. When you push your code, your issues go with it.

```bash
bd create "Fix login redirect after OAuth callback"
bd list
bd close bd-42 -r "Fixed in commit abc123"
bd ready    # Show unblocked work — no blocked dependencies
```

Beads supports the things you'd expect from an issue tracker — labels, assignees, due dates — plus first-class dependency tracking. You can model which issues block others, group work into epics, and use `bd ready` to see exactly what's unblocked and ready to work on.

The data lives in JSONL files that are designed for git. They merge cleanly, they diff readably, and they sync across branches. There's no server to maintain, no subscription to manage. Your issue database is part of your codebase.

## Anvil: Automated Tasks on a Schedule

Anvil is a task scheduler for LLM-powered development work. You describe tasks in plain English, give them a cron schedule, and anvil runs them using an AI coding assistant as the executor.

```bash
anvil watch                                    # Start the daemon
anvil init                                     # Initialize your project
anvil add -s "*/30 * * * *" "Triage new GitHub issues and label them"
anvil add -s "0 9 * * 1-5" "Review open PRs and leave feedback"
anvil task ls                                  # See what's scheduled
anvil task log triage-new-github-issues        # Check execution output
```

Tasks are markdown files with YAML frontmatter, stored in `.anvil/todos/` and organized by priority. A single daemon watches multiple projects, dispatching tasks on schedule. You can set pre-check commands (only run if the repo has uncommitted changes), configure retry policies, and define lifecycle hooks for success or failure.

The key insight is that many recurring development chores — triaging issues, reviewing PRs, checking CI status, syncing documentation — are well-suited to LLM automation. Anvil makes that automation a first-class part of your project, not a fragile cron job hidden in a server somewhere.

## The Composed Workflow: A Feature from Start to Finish

Here's where it gets interesting. Each tool is useful on its own, but watch what happens when you use all three on a single feature.

Say you're adding search functionality to a project. You start with speckit:

```bash
/speckit.specify "Add full-text search to the API"
```

Speckit generates a spec with user stories, acceptance criteria, and requirements — all in `specs/004-full-text-search/spec.md`. You review it, refine it, then move to planning:

```bash
/speckit.plan
```

Now you have a technical plan covering architecture decisions, data model changes, and a project structure. The plan lives in `specs/004-full-text-search/plan.md`. Next, break it into tasks:

```bash
/speckit.tasks
```

This produces an ordered task list in `specs/004-full-text-search/tasks.md`, organized by user story with dependency markers showing what can run in parallel. At this point, you have a complete blueprint — spec, plan, tasks — all as committed markdown in your repo.

Now bring in beads. The tasks from speckit become tracked issues:

```bash
bd create "Implement search indexing service"
bd create "Add search API endpoint" --deps "blocks:bd-51"
bd create "Write integration tests for search" --deps "blocks:bd-51,blocks:bd-52"
```

Each issue has dependencies, so `bd ready` always shows you exactly what's unblocked. As you work through the issues, `bd close bd-51 -r "Implemented in src/services/search.go"` closes them with a traceable reason.

Meanwhile, anvil handles the recurring work that accumulates during development:

```bash
anvil add -s "0 */4 * * *" "Check if search index tests are passing and report failures"
anvil add -s "0 9 * * 1-5" "Review any open PRs related to the search feature"
```

These tasks run on schedule, handled by an LLM that reads your codebase and acts on what it finds. No manual checking, no forgotten follow-ups.

The workflow stays in the terminal the entire time. You specify, plan, track, and automate without leaving your shell. And every artifact — specs, plans, tasks, issues, automation configs — lives in the repository, versioned alongside the code it describes.

## Everything Lives in Git

This is the architectural insight that ties the ecosystem together. In a traditional setup, your planning data is scattered: issues in one SaaS tool, specs in a wiki, tasks in a project board, automation in a CI config. None of it is versioned with your code. None of it shows up in `git log`.

With these tools, your project directory tells the full story:

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

When you clone the repo, you get the project's entire history — not just code changes, but the decisions, the issues, the automation. When you branch for a feature, your planning artifacts branch with it. When you review a PR, the spec and task list are right there in the diff.

This isn't just convenient. It changes how teams share context. New contributors can read the spec directory to understand not just *what* the code does, but *why* it was built that way.

## Start With One

You don't need to adopt all three tools to get value. That's the point of composability — each piece works independently.

If your biggest pain point is losing track of what needs doing across a project, start with beads. Run `bd create` a few times, use `bd list` and `bd ready` to manage your work, and see if git-native issue tracking fits your workflow.

If you find yourself wishing for more structure before you start coding — clearer specs, explicit acceptance criteria, an ordered task breakdown — try speckit. It works with or without beads.

If you have recurring development chores that keep falling through the cracks — PR reviews, issue triage, documentation checks — anvil gives those tasks a schedule and an executor.

Once one tool is part of your workflow, the others plug in naturally. Speckit's task output feeds into beads issues. Beads' tracked work surfaces things anvil can automate. But none of that integration is required. Use what solves your problem today.

## Try It

All three tools are open source and available now:

- **[speckit](https://github.com/johnjansen/speckit)** — Specification-driven development workflow
- **[beads](https://github.com/johnjansen/beads)** — Lightweight, git-native issue tracking
- **[anvil](https://github.com/johnjansen/anvil)** — Scheduled LLM task automation

Pick the one that matches your current friction. Install it. Use it on a real project for a week. If it helps, consider adding a second tool. The ecosystem grows with your needs, not ahead of them.
