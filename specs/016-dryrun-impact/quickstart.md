# Quickstart: Dry-Run Impact Analysis

## Overview
The dry-run impact analysis enhances `anvil add --dry-run` to show scheduling conflicts, worker load, and alternative schedule suggestions before adding a task.

## Usage

### Basic Impact Analysis
```bash
anvil add -s "0 9 * * *" "Daily report" --dry-run
```

### JSON Output
```bash
anvil add -s "0 9 * * *" "Daily report" --dry-run --json
```

### One-Shot Task (no impact analysis)
```bash
anvil add --once "Migrate database" --dry-run
```

## What You See
1. **Schedule validation** — confirms cron syntax is valid and shows next run time
2. **Scheduling conflicts** — lists existing tasks that fire at the same times
3. **Peak concurrency** — shows maximum concurrent tasks at the busiest time slot
4. **Suggested alternatives** — offers 2-3 nearby schedules with fewer conflicts

## When to Use
- Before adding any scheduled task to check for bottlenecks
- When planning task schedules across a project
- To find optimal time slots with minimal contention
