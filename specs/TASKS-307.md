# Tasks: Task Failure Prediction (#307)

## Phase 1: Data Structures

### Task 1.1: Add risk fields to Todo struct
- **File:** `internal/project/project.go`
- **Description:** Add `OnRiskHigh` string field and `RiskThreshold` struct to Todo
- **Dependencies:** None
- **Estimate:** 0.5h

### Task 1.2: Add risk fields to TaskState
- **File:** `internal/project/project.go`
- **Description:** Add `CurrentRisk`, `RiskScore`, `RiskFactors`, `RiskLastUpdated`, `HistoricalStats` fields
- **Dependencies:** 1.1
- **Estimate:** 0.5h

### Task 1.3: Add risk fields to RunRecord
- **File:** `internal/project/project.go`
- **Description:** Add `RiskFactors` slice to capture detected factors per run
- **Dependencies:** 1.1
- **Estimate:** 0.25h

### Task 1.4: Create risk persistence functions
- **File:** `internal/project/project.go`
- **Description:** Add functions to read/write risk state to `.anvil/tasks/<task-id>/risk.json`
- **Dependencies:** 1.2
- **Estimate:** 1h

## Phase 2: Risk Analysis Engine

### Task 2.1: Create RiskAnalyzer type and core methods
- **File:** `internal/project/risk.go` (new)
- **Description:** Create `RiskAnalyzer` struct with `AnalyzeTask()`, `CalculateRiskScore()`, `DetectPatterns()`, `CalculateTrend()`
- **Dependencies:** 1.3, 1.4
- **Estimate:** 3h

### Task 2.2: Implement pattern detection - tokens
- **File:** `internal/project/risk.go`
- **Description:** Detect correlation between high token count and failures
- **Dependencies:** 2.1
- **Estimate:** 1h

### Task 2.3: Implement pattern detection - time
- **File:** `internal/project/risk.go`
- **Description:** Detect correlation between time of day/day of week and failures
- **Dependencies:** 2.1
- **Estimate:** 1h

### Task 2.4: Implement pattern detection - runtime
- **File:** `internal/project/risk.go`
- **Description:** Detect correlation between runtime duration and failures
- **Dependencies:** 2.1
- **Estimate:** 1h

### Task 2.5: Implement pattern detection - errors
- **File:** `internal/project/risk.go`
- **Description:** Detect correlation between specific error patterns and failures
- **Dependencies:** 2.1
- **Estimate:** 1h

## Phase 3: Daemon Integration

### Task 3.1: Add post-run risk calculation
- **File:** `internal/daemon/daemon.go`
- **Description:** After each run completes, call risk analyzer and update risk state
- **Dependencies:** 2.5
- **Estimate:** 2h

### Task 3.2: Implement risk hook execution
- **File:** `internal/daemon/daemon.go`
- **Description:** Add `runRiskHighHook()` function, trigger when risk transitions to HIGH
- **Dependencies:** 3.1
- **Estimate:** 1.5h

### Task 3.3: Add periodic risk check
- **File:** `internal/daemon/daemon.go`
- **Description:** Every 6 hours, recalculate risk for active tasks, trigger hooks on changes
- **Dependencies:** 3.2
- **Estimate:** 1h

## Phase 4: CLI Commands

### Task 4.1: Add `anvil task predict` command
- **File:** `cmd/anvil/main.go`
- **Description:** Implement `task predict <task-name>` subcommand with full analysis output
- **Dependencies:** 2.5
- **Estimate:** 2h

### Task 4.2: Modify `anvil ps` for risk display
- **File:** `cmd/anvil/main.go`
- **Description:** Add RISK column with [LOW]/[MEDIUM]/[HIGH] indicators
- **Dependencies:** 1.4
- **Estimate:** 1h

### Task 4.3: Modify `anvil task get` for risk display
- **File:** `cmd/anvil/main.go`
- **Description:** Add Risk section showing level, score, and factors
- **Dependencies:** 1.4
- **Estimate:** 1h

### Task 4.4: Add risk config parsing
- **File:** `internal/project/project.go`
- **Description:** Parse `risk_threshold` and `on_risk_high` from task YAML
- **Dependencies:** 1.1
- **Estimate:** 0.5h

## Phase 5: Testing & Polish

### Task 5.1: Unit tests for RiskAnalyzer
- **File:** `internal/project/risk_test.go`
- **Description:** Test success rate, trend detection, pattern correlation
- **Dependencies:** 2.5
- **Estimate:** 2h

### Task 5.2: Integration test for predict command
- **File:** Manual testing
- **Description:** Test predict command with various run record scenarios
- **Dependencies:** 4.1
- **Estimate:** 0.5h

### Task 5.3: Edge case handling
- **Files:** Multiple
- **Description:** Handle insufficient data, no failures, daemon restart
- **Dependencies:** All above
- **Estimate:** 1h

---

**Total Estimate:** ~21.25 hours

**Parallelization Opportunities:**
- Tasks 1.1-1.4 can be done in parallel (data structures)
- Tasks 2.2-2.5 can be done in parallel (pattern detection)
- Tasks 4.2-4.3 can be done in parallel (CLI output)
