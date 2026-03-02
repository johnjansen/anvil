# SPEC.md - Task Failure Prediction

## Project Overview
- **Project**: anvil
- **Feature**: Task failure prediction using historical patterns
- **Issue**: #307
- **Goal**: Analyze historical task runs to predict failures before they happen

## Problem Statement

Users have no way to anticipate task failures before they happen. By the time a task fails, damage may be done (corrupted data, missed notifications, etc.). With historical data, patterns could predict failures.

## Proposed Solution

### 1. Pattern Analysis

The system analyzes historical run records to detect:
- Success/failure rate
- Correlation with inputs (token counts, timing)
- Recent trends (success rate over time)

### 2. Risk Scoring

Each task gets a risk score:
- **LOW**: Success rate >= 90% or < 5 runs
- **MEDIUM**: Success rate 70-90%
- **HIGH**: Success rate < 70% or declining trend

### 3. Commands

```bash
# Show prediction analysis
anvil task predict <task-name>

# Risk shown in process list
anvil ps
```

### 4. Preventive Hook

```yaml
---
schedule: "0 9 * * *"
on_risk_high: "echo 'Task is at high risk of failure'"
```

## Technical Design

### Data Model

**RiskAssessment** (new struct in `internal/project/project.go`):
```go
type RiskAssessment struct {
    TaskID           string
    TotalRuns        int
    SuccessCount     int
    FailureCount     int
    SuccessRate      float64
    RiskLevel        RiskLevel  // LOW, MEDIUM, HIGH
    Trend            string     // "increasing", "decreasing", "stable"
    Patterns         []string   // detected patterns
    LastChecked      time.Time
}

type RiskLevel string
const (
    RiskLow    RiskLevel = "LOW"
    RiskMedium RiskLevel = "MEDIUM"
    RiskHigh   RiskLevel = "HIGH"
)
```

**Updated Todo struct** - add risk-related fields:
```go
type Todo struct {
    // ... existing fields ...
    OnRiskHigh       string        // hook: shell command when risk is HIGH
}
```

### Storage

- Risk assessments cached in memory in daemon
- Optionally persisted to `.anvil/tasks/<task-id>/risk.json`
- Recalculated on each `predict` command or daemon startup

### Analysis Algorithm

1. **Load run records** - Read last 30 runs from `.anvil/runs/<task-id>/`
2. **Calculate metrics**:
   - Success rate = success_count / total_runs
   - Recent success rate = successes in last 10 runs / 10
3. **Detect trends**:
   - Compare first 10 runs success rate to last 10 runs
   - If recent < earlier by > 20%, trend = "decreasing"
4. **Risk scoring**:
   - If total_runs < 5: LOW (insufficient data)
   - If success_rate >= 0.9: LOW
   - If success_rate >= 0.7: MEDIUM
   - If success_rate < 0.7 OR trend == "decreasing": HIGH
5. **Pattern detection**:
   - Correlation with high token count (>100K input tokens)
   - Correlation with time of day (optional, v2)
   - Correlation with day of week (optional, v2)

### CLI Commands

**`anvil task predict <name>`**:
- Location: `cmd/anvil/task_predict.go` (new file)
- Loads run records, calculates risk, outputs analysis
- Output format:
```
Analysis of last N runs:
- Success rate: X% (N/N)
- Trend: increasing/decreasing/stable
- Risk: LOW/MEDIUM/HIGH
Patterns detected:
- Failures correlate with high token count (>100K)
```

**`anvil ps` - Add risk column**:
- Modify existing `cmd/anvil/ps.go`
- Calculate risk for running/idle tasks
- Show [LOW]/[MEDIUM]/[HIGH] in STATUS column

### Hook Integration

**`on_risk_high` hook**:
- Runs when risk assessment changes to HIGH
- Or runs periodically (each predict check)
- Environment variables:
  - `ANVIL_TASK_NAME`
  - `ANVIL_RISK_LEVEL`
  - `ANVIL_SUCCESS_RATE`
  - `ANVIL_TREND`

## Acceptance Criteria

1. `anvil task predict <name>` shows failure prediction analysis
2. Risk scores (LOW/MEDIUM/HIGH) based on historical patterns
3. Show detected patterns (correlation with inputs, timing)
4. Risk shown in `anvil ps` output
5. `on_risk_high` hook fires when risk is high

## Files to Modify/Create

1. `internal/project/project.go` - Add RiskAssessment struct, risk calculation functions
2. `cmd/anvil/task_predict.go` - New CLI command
3. `cmd/anvil/ps.go` - Add risk column to output
4. `internal/daemon/daemon.go` - Add risk hook execution
5. Update `internal/project/loader.go` if needed for task field loading

## Edge Cases

- **No run history**: Show "Insufficient data (< 5 runs)" - return LOW risk
- **All success**: LOW risk
- **All failures**: HIGH risk with suggestion to fix
- **Task deleted**: Handle gracefully, no panic
- **Daemon restart**: Risk cache cleared, recalculated on next access
