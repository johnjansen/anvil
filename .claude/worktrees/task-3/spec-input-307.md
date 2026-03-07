## Problem

Users have no way to anticipate task failures before they happen. By the time a task fails, damage may be done (corrupted data, missed notifications, etc.). With historical data, patterns could predict failures.

Examples:
- Task always fails on Mondays after a full moon (edge cases)
- Task fails when input file exceeds certain size
- Task times out when API rate limit is hit

## Proposed Solution

Add failure prediction:

### 1. Pattern analysis

```bash
$ anvil task predict fetch-data
Analysis of last 30 runs:
- Success rate: 87% (26/30)
- Pattern detected: Failures correlate with high token count (>100K)
- Trend: Success rate decreasing over time
- Risk: HIGH (predicted failure in next 5 runs)
```

### 2. Risk scoring

Each task gets a risk score (LOW/MEDIUM/HIGH) based on:
- Historical success rate
- Recent failure frequency
- External factors (time, dependencies)

### 3. Warnings

```bash
$ anvil ps
TASK            STATUS    RISK
fetch-data      running   [HIGH]
process-data    idle      [MEDIUM]
```

### 4. Preventive actions

```yaml
---
schedule: "0 9 * * *"
on_risk_high: "echo 'Task fetch-data is at high risk of failure'"
```

## Acceptance Criteria

- [ ] `anvil task predict <name>` shows failure prediction analysis
- [ ] Risk scores (LOW/MEDIUM/HIGH) based on historical patterns
- [ ] Show detected patterns (correlation with inputs, timing)
- [ ] Risk shown in `anvil ps` output
- [ ] `on_risk_high` hook fires when risk is high
