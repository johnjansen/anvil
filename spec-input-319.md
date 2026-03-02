## Problem

Currently when adding a task, users can't see the impact before committing:
- Will this cause scheduling conflicts?
- How much will it add to monthly costs?
- Will it create worker bottlenecks?

Users must add the task first, then run separate commands to see the impact.

## Proposed Solution

Add `--dry-run` or `--preview` to `anvil add`:

```bash
$ anvil add -s \"0 9 * * *\" \"New task\" --dry-run

Impact Analysis:
─────────────────────────────────────────
Schedule: 0 9 * * *
Conflicts: Conflicts with 3 tasks at 09:00
  - fetch-data (same time)
  - process-data (same time)
  - report (same time)
Monthly Cost: +$15.00 (estimated)
Worker Load: +10% at 09:00

Add anyway? [y/N]
```

Also suggest alternatives:

```bash
Suggested alternatives to avoid conflicts:
  - 0 9,15,21 * * * (spread across day)
  - */30 * * * * (every 30 min)
  - 0 10 * * * (shift to 10am)
```

## Acceptance Criteria

- [ ] `anvil add --dry-run` shows impact before adding
- [ ] Shows scheduling conflicts with existing tasks
- [ ] Shows cost estimate for new task
- [ ] Suggests alternative schedules to avoid conflicts
- [ ] Can proceed with adding after seeing impact
