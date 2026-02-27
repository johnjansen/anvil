# Implementation Plan: Task Failure Prediction (#307)

## Overview

Add failure prediction capabilities to anvil that analyze historical run patterns to identify tasks at risk of failure. The implementation adds a new `predict` command, risk display in `ps` and `task get`, and a new `on_risk_high` hook.

## Implementation Steps

### Phase 1: Data Structures (Core)

**1.1 Add risk fields to Todo struct**
- File: `internal/project/project.go`
- Add `OnRiskHigh` string field for hook
- Add `RiskThreshold` struct with configurable thresholds

**1.2 Add risk fields to TaskState**
- File: `internal/project/project.go`
- Add `CurrentRisk`, `RiskScore`, `RiskFactors`, `RiskLastUpdated`, `HistoricalStats` fields

**1.3 Add risk fields to RunRecord**
- File: `internal/project/project.go`
- Add `RiskFactors` slice to capture detected factors per run

**1.4 Create risk storage file**
- File: `.anvil/tasks/<task-id>/risk.json`
- Functions to read/write risk state

### Phase 2: Risk Analysis Engine

**2.1 Create RiskAnalyzer in project package**
- File: `internal/project/risk.go` (new)
- `AnalyzeTask(taskID string) (*RiskAnalysis, error)`
- `CalculateRiskScore(records []RunRecord) float64`
- `DetectPatterns(records []RunRecord) []RiskFactor`
- `CalculateTrend(records []RunRecord) string`

**2.2 Implement pattern detection**
- Token count correlation (high input/output = failure)
- Time of day / day of week correlation
- Runtime duration correlation
- Error pattern correlation

**2.3 Implement risk state persistence**
- Save risk state after each analysis
- Load on daemon startup
- Update only when risk level changes

### Phase 3: Daemon Integration

**3.1 Add post-run risk calculation**
- File: `internal/daemon/daemon.go`
- After each run completes, calculate new risk
- If risk transitions to HIGH, trigger `on_risk_high` hook

**3.2 Implement risk hook execution**
- Add `runRiskHighHook()` function (similar to `runSLAViolationHook`)
- Pass risk-specific environment variables

**3.3 Add periodic risk check**
- Every 6 hours, recalculate risk for active tasks
- Trigger hooks on risk level changes

### Phase 4: CLI Commands

**4.1 Add `anvil task predict` command**
- File: `cmd/anvil/main.go`
- Subcommand: `task predict <task-name>`
- Output: success rate, trend, risk factors, risk score, prediction

**4.2 Modify `anvil ps` output**
- Add [LOW], [MEDIUM], [HIGH] risk indicator column
- Only show if task has enough runs for analysis

**4.3 Modify `anvil task get` output**
- Add Risk section showing current risk level and score
- List detected risk factors

**4.4 Add risk configuration to task YAML**
```yaml
---
schedule: "0 9 * * *"
risk_threshold:
  high: 0.7
  medium: 0.4
  min_runs: 5
on_risk_high: "echo 'Warning!'"
```

### Phase 5: Testing & Polish

**5.1 Unit tests for RiskAnalyzer**
- Test success rate calculation
- Test trend detection
- Test pattern correlation

**5.2 Integration tests**
- Test predict command with mock data
- Test hook execution

**5.3 Edge cases**
- Tasks with < 5 runs (show "insufficient data")
- Tasks with no failures (low risk by default)
- Daemon restart preserves risk state

## Dependencies

- No new external dependencies
- Uses existing run record system
- Uses existing hook infrastructure

## Risk Assessment

**Low risk changes:**
- Data structure additions (backward compatible)
- New command (isolated)
- Hook addition (uses existing pattern)

**Files touched:**
- `internal/project/project.go` - Add fields
- `internal/project/risk.go` - New file
- `internal/daemon/daemon.go` - Post-run hooks
- `cmd/anvil/main.go` - New command, output changes
