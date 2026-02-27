# Specification: Task Failure Prediction

## 1. Project Overview

**Project Name:** Task Failure Prediction
**Type:** CLI Feature (anvil)
**Core Functionality:** Predict task failures using historical run patterns, enabling users to take preventive action before failures occur.
**Target Users:** Anvil users running scheduled/persistent tasks who want proactive failure prevention.

## 2. Problem Statement

Users currently have no way to anticipate task failures before they happen. By the time a task fails, damage may be done (corrupted data, missed notifications, etc.). With historical run data available in run records, patterns could predict failures.

## 3. Technical Design

### 3.1 Data Model

**New fields in Todo struct:**
```go
// In internal/project/project.go
type Todo struct {
    // ... existing fields
    OnRiskHigh    string        // hook: shell command when risk becomes HIGH
    RiskThreshold RiskThresholds // risk scoring configuration
}

type RiskThresholds struct {
    HighThreshold   float64 // default: 0.7
    MediumThreshold float64 // default: 0.4
    MinRunsForAnalysis int   // default: 5
    LookbackPeriod  time.Duration // default: 30 days
}
```

**New fields in RunRecord:**
```go
type RunRecord struct {
    // ... existing fields
    RiskFactors []RiskFactor // detected risk factors for this run
}

type RiskFactor struct {
    Type    string // "high_tokens", "time_of_day", "failure_correlation"
    Value   string
    Score   float64
}
```

**RiskScore persisted in task state:**
```go
type TaskState struct {
    // ... existing fields
    CurrentRisk      RiskLevel // LOW, MEDIUM, HIGH
    RiskScore        float64   // 0.0-1.0
    RiskFactors      []RiskFactor
    RiskLastUpdated  time.Time
    HistoricalStats  TaskHistoricalStats
}

type TaskHistoricalStats struct {
    TotalRuns        int
    SuccessRate      float64
    RecentFailures   int // last 10 runs
    TrendDirection   string // "improving", "stable", "declining"
}
```

### 3.2 Pattern Analysis Engine

**Analysis dimensions:**
1. **Success Rate** - Overall historical success rate
2. **Recent Failure Frequency** - Failures in last N runs
3. **Trend Analysis** - Success rate trajectory over time
4. **Correlation Detection** - Correlate failures with:
   - Token count (input/output)
   - Time of day / day of week
   - Runtime duration
   - Specific error patterns
5. **External Factors** - Time-based patterns (edge cases)

**Risk Score Calculation:**
```
riskScore = (
    (1.0 - successRate) * 0.3 +
    (recentFailures / 10.0) * 0.3 +
    trendPenalty * 0.2 +
    correlationPenalty * 0.2
)

Where:
- successRate = successes / total_runs (0.0-1.0)
- recentFailures = failures in last 10 runs (0-10)
- trendPenalty = 0.0 (stable), 0.2 (declining), 0.4 (sharply declining)
- correlationPenalty = max(correlation_scores) if significant correlation found
```

**Risk Levels:**
- LOW: riskScore < 0.4
- MEDIUM: 0.4 <= riskScore < 0.7
- HIGH: riskScore >= 0.7

### 3.3 CLI Commands

**New command: `anvil task predict <task-name>`**
```bash
anvil task predict fetch-data

# Output:
Analysis of last 30 runs (min 5 required):
─────────────────────────────────────────────
Success rate: 87% (26/30)
Recent failures: 4 of last 10 runs
Trend: Declining (was 95% 2 weeks ago)
─────────────────────────────────────────────
Risk Factors:
  • High token correlation: Failures occur when input >100K tokens
  • Time correlation: Failures more common after 2pm
─────────────────────────────────────────────
Risk Score: 0.73 (HIGH)
Prediction: ~80% chance of failure in next 5 runs
─────────────────────────────────────────────
Recommendation: Check token usage, consider adding timeout
```

**Risk display in `anvil ps`:**
```bash
$ anvil ps
TASK            STATUS    RISK     LAST_RUN
fetch-data      running   [HIGH]   2m ago
process-data    idle      [MEDIUM] 5m ago
backup          idle      [LOW]    1h ago
```

**Risk display in `anvil task get`:**
```bash
$ anvil task get fetch-data
Name: fetch-data
Schedule: */15 * * * *
Risk: HIGH (0.73)
  - 4 failures in last 10 runs
  - Correlation: high token count
```

### 3.4 Hook Integration

**New hook: `on_risk_high`**
```yaml
---
schedule: "0 9 * * *"
on_risk_high: "echo 'Warning: Task fetch-data is at high risk!' | slack"
```

**Hook environment variables:**
```
ANVIL_TASK_NAME
ANVIL_PROJECT
ANVIL_RISK_LEVEL (LOW, MEDIUM, HIGH)
ANVIL_RISK_SCORE (0.0-1.0)
ANVIL_RISK_FACTORS (JSON array of risk factors)
```

### 3.5 Storage

**Risk state stored in:** `.anvil/tasks/<task-id>/risk.json`
```json
{
  "current_risk": "HIGH",
  "risk_score": 0.73,
  "last_updated": "2026-02-27T10:00:00Z",
  "historical_stats": {
    "total_runs": 30,
    "success_rate": 0.87,
    "recent_failures": 4,
    "trend": "declining"
  },
  "risk_factors": [
    {"type": "high_tokens", "value": ">100K", "score": 0.5},
    {"type": "time_of_day", "value": "after_2pm", "score": 0.3}
  ]
}
```

### 3.6 Risk Update Trigger

Risk is recalculated:
1. After each task run completes (in daemon)
2. On-demand via `anvil task predict` (always fresh)
3. Periodic check every 6 hours for active tasks

## 4. Acceptance Criteria

- [ ] `anvil task predict <name>` shows failure prediction analysis with success rate, trend, risk factors
- [ ] Risk scores (LOW/MEDIUM/HIGH) computed from historical patterns
- [ ] Detected patterns shown (correlation with tokens, timing, errors)
- [ ] Risk shown in `anvil ps` output with [RISK] indicator
- [ ] Risk shown in `anvil task get` output
- [ ] `on_risk_high` hook fires when risk transitions to HIGH
- [ ] Risk state persists across daemon restarts
- [ ] Minimum 5 runs required before prediction (configurable)

## 5. Files to Modify

1. `internal/project/project.go` - Add risk fields to Todo, TaskState, create RiskAnalyzer
2. `internal/daemon/daemon.go` - Add risk calculation after runs, trigger on_risk_high hook
3. `cmd/anvil/main.go` - Add `predict` subcommand, modify `ps` and `task get` for risk display
4. `internal/config/config.go` - Add risk threshold configuration

## 6. Out of Scope

- ML-based predictions (use statistical analysis only)
- Cross-task correlation (task dependencies)
- Auto-remediation suggestions (beyond showing risk factors)
