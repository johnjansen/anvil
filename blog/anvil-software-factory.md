---
name: anvil-software-factory
description: Bootstrap a complete CLI-native development ecosystem. Installs speckit, beads, and anvil, initializes a project directory, and walks through onboarding for each tool. Use when setting up a new project with the full spec-to-automation workflow described in the CLI ecosystem blog post. Trigger phrases include "set up the software factory", "bootstrap the ecosystem", "install speckit beads anvil", "set up cli workflow", "new project with full toolchain".
---

## User Input

```text
$ARGUMENTS
```

You **MUST** consider the user input before proceeding (if not empty). The user may specify a project name, directory, or which tools to install.

## Overview

This skill installs three CLI tools — **speckit**, **beads**, and **anvil** — and bootstraps a project directory with all three configured. Each tool handles a distinct phase of development:

- **speckit** — specification-driven planning (specs, plans, task breakdowns)
- **beads** (`bd`) — lightweight, git-native issue tracking
- **anvil** — scheduled LLM task automation

## Prerequisites

Before running, verify the basics are in place:

```bash
# Check for required tooling
which git || echo "MISSING: git"
which brew || echo "MISSING: homebrew (https://brew.sh)"
which claude || echo "MISSING: claude CLI (npm install -g @anthropic-ai/claude-code)"
which uv || echo "MISSING: uv (curl -LsSf https://astral.sh/uv/install.sh | sh)"
```

> [!CAUTION]
> If `git`, `brew`, `claude`, or `uv` are missing, stop and help the user install them first. Claude CLI is required for anvil's task execution. Homebrew is needed for beads installation. uv is needed for speckit's `specify` CLI.

## Step 1: Install Beads

Beads provides git-native issue tracking via the `bd` command.

```bash
# Check if already installed
if command -v bd &>/dev/null; then
  echo "beads already installed: $(bd version)"
else
  brew install beads
fi
```

Verify it works:

```bash
bd version
```

## Step 2: Install Anvil

Anvil is a task scheduler for LLM-powered development work. It ships as a platform binary from GitHub releases.

```bash
# Check if already installed
if command -v anvil &>/dev/null; then
  echo "anvil already installed: $(anvil version)"
  # Check for updates
  anvil update --check
else
  # Detect platform
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
  esac

  ASSET="anvil-${OS}-${ARCH}"
  echo "Downloading anvil for ${OS}/${ARCH}..."

  # Download latest release
  gh release download --repo johnjansen/anvil --pattern "$ASSET" --dir /tmp
  chmod +x "/tmp/$ASSET"
  sudo mv "/tmp/$ASSET" /usr/local/bin/anvil

  anvil version
fi
```

## Step 3: Install Speckit

Speckit uses the `specify` CLI to scaffold spec-driven development into a project. It installs commands, templates, and scripts directly into the working directory.

```bash
# Install the specify CLI if not present
if command -v specify &>/dev/null; then
  echo "specify already installed: $(specify version 2>&1 | grep 'CLI Version' | awk '{print $NF}')"
else
  uv tool install specify-cli --from git+https://github.com/github/spec-kit.git
fi
```

Then initialize speckit in the project directory. This downloads templates, installs Claude slash commands, and sets up the `.specify/` scaffolding:

```bash
specify init . --ai claude --force
```

> [!NOTE]
> The `--force` flag skips the confirmation prompt when the directory is not empty. If the project is brand new, you can omit it.

## Step 4: Initialize the Project

Determine the target directory. If the user provided a project name, create it. Otherwise use the current directory.

```bash
# If user specified a project name, create and enter the directory
# PROJECT_DIR should be set from user input, or default to "."

# Initialize git if needed
if [ ! -d .git ]; then
  git init
  echo "Initialized git repository"
fi
```

### Initialize Beads

```bash
if [ ! -d .beads ]; then
  bd init
  echo "Initialized beads issue tracker"
else
  echo "beads already initialized"
fi
```

### Initialize Anvil

```bash
if [ ! -d .anvil ]; then
  anvil init
  echo "Initialized anvil task scheduler"
else
  echo "anvil already initialized"
fi
```

### Start the Anvil Daemon

The anvil daemon is a global process — one per machine, watches all projects.

```bash
# Check if daemon is already running
if anvil status &>/dev/null; then
  echo "anvil daemon already running"
  anvil status
else
  anvil watch -d
  echo "Started anvil daemon in background"
fi
```

### Initialize Speckit Constitution

Once speckit is scaffolded and all tools are initialized, establish the project's development principles by running the constitution skill. This creates a constitution file that guides how speckit generates specs, plans, and tasks for this project.

> [!IMPORTANT]
> After all installations and initializations are complete, tell the user you are about to bootstrap their project constitution and then run:
>
> `/speckit.constitution Create principles focused on code quality, testing standards, user experience consistency, and performance requirements`
>
> This step is interactive — speckit will ask the user about their project's priorities, coding standards, and workflow preferences. Do not skip this step.

## Step 5: Verify the Setup

Run a quick health check on everything:

```bash
echo "=== Beads ==="
bd info --json 2>/dev/null && echo "OK" || echo "ISSUE: beads not initialized"

echo ""
echo "=== Anvil ==="
anvil doctor 2>/dev/null || echo "Run 'anvil doctor' to troubleshoot"

echo ""
echo "=== Speckit ==="
if [ -d .specify ] && ls .claude/commands/speckit.* &>/dev/null; then
  echo "OK — speckit commands installed"
  ls .claude/commands/speckit.* | sed 's|.*/||; s|\.md$||'
  if [ -f .specify/memory/constitution.md ]; then
    echo "Constitution: created"
  else
    echo "Constitution: not yet created (run /speckit.constitution)"
  fi
else
  echo "Not installed (run: specify init . --ai claude)"
fi

echo ""
echo "=== Project Structure ==="
ls -la .beads/ .anvil/ .specify/ .claude/commands/ 2>/dev/null
```

## Step 6: Quick Onboarding

Show the user what they can do now:

```
Setup complete. Here's what you have:

PLANNING (speckit)
  /speckit.specify "describe your feature"  → generates spec.md
  /speckit.plan                              → generates plan.md
  /speckit.tasks                             → generates tasks.md
  /speckit.implement                         → executes the tasks

ISSUE TRACKING (beads)
  bd create "Fix the login bug"             → create an issue
  bd list                                    → see all issues
  bd ready                                   → see unblocked work
  bd close <id> -r "reason"                 → close an issue

AUTOMATION (anvil)
  anvil add -s "0 9 * * 1-5" "Review PRs"  → scheduled task
  anvil task ls                              → see scheduled tasks
  anvil ps                                   → see running tasks
  anvil task log <name>                      → check execution output

BRIDGING TOOLS
  /speckit.taskstobeads                      → convert tasks to beads issues
```

> [!IMPORTANT]
> If any step failed, report the specific error to the user and suggest a fix before continuing. Do not silently skip broken installations.
