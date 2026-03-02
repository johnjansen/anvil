## Problem

Low-priority tasks can get starved in a busy system. High-priority tasks always run first, but a low-priority task that's been waiting a long time never gets a chance. Users need a way to ensure tasks don't wait forever.

## Proposed Solution

Add priority aging:

### 1. Automatic priority boost

Tasks that wait too long get automatic priority boost:

```yaml
# ~/.anvil/config.yaml
priority_aging:
  enabled: true
  threshold: 30m    # after waiting 30 minutes
  boost_by: 2        # lower priority by 2 levels
```

So a p5 task waiting 30 minutes becomes effectively p3.

### 2. Boost limits

```yaml
priority_aging:
  enabled: true
  threshold: 30m
  boost_by: 2
  max_boost: 4       # never boost more than 4 levels
```

### 3. Visibility

```bash
$ anvil task queue
TASK           WAITED   PRIORITY   EFFECTIVE
low-task       45m      p5         p3 (aged)
high-task      1m       p1         p1
```

### 4. Disable per-task

```yaml
---
schedule: "*/30 * * *"
priority_aging: false
---

## Acceptance Criteria

- [ ] Tasks waiting longer than threshold get automatic priority boost
- [ ] `priority_aging` config in config.yaml
- [ ] `boost_by` and `max_boost` configurable
- [ ] Visible in `anvil task queue`
- [ ] Can disable per-task with `priority_aging: false`
