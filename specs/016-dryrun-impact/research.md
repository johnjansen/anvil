# Research: Dry-Run Impact Analysis

## Decision 1: Reuse Existing Overlap Detection
- **Decision**: Refactor existing overlap detection code (main.go:2165-2226) into a reusable function, then call it from both the normal add flow and the enhanced dry-run flow.
- **Rationale**: The overlap detection algorithm already works correctly (30 future occurrences, 1-minute tolerance). Duplicating it would create maintenance burden.
- **Alternatives considered**: (a) Writing new overlap code from scratch — rejected because existing algorithm is proven. (b) Moving to a separate package — rejected as over-engineering for a CLI command.

## Decision 2: Time Window for Conflict Analysis
- **Decision**: Use a 24-hour window for enumerating firing times to detect conflicts and compute worker load.
- **Rationale**: 24 hours covers all daily patterns. Weekly/monthly patterns would need longer windows but 24h is sufficient for the most common scheduling scenarios and keeps computation fast.
- **Alternatives considered**: (a) 7-day window — rejected because it's slower and most schedules repeat daily. (b) 30 occurrences (current approach) — reused as-is for per-task overlap check.

## Decision 3: Worker Load Calculation
- **Decision**: Enumerate all active tasks' next-24h firing times into a time-slot map (minute resolution), then count concurrent tasks per slot.
- **Rationale**: Gives accurate concurrency picture. Minute resolution matches cron granularity.
- **Alternatives considered**: (a) Hour-resolution buckets — too coarse, misses minute-level conflicts. (b) Per-second resolution — unnecessary given cron's minute granularity.

## Decision 4: Alternative Schedule Suggestions
- **Decision**: Generate alternatives by shifting the proposed schedule by +/- 1-3 hours and checking which shifts have fewer conflicts. Only suggest alternatives with strictly fewer conflicts.
- **Rationale**: Simple heuristic that covers the most common fix (shift time). More complex approaches (genetic algorithms, constraint solvers) are over-engineering.
- **Alternatives considered**: (a) Random schedule generation — unpredictable. (b) Spread-across-day suggestions — too opinionated about user intent.

## Decision 5: JSON Output Format
- **Decision**: Add --json flag to anvil add --dry-run that outputs a structured JSON object with schedule info, conflicts, worker load, and suggestions.
- **Rationale**: Consistent with existing --json patterns in dryrun.go and other commands.
- **Alternatives considered**: None — JSON output is standard practice in this codebase.

## Decision 6: Implementation Location
- **Decision**: Add a new file cmd/anvil/impact.go for the impact analysis logic, keeping dryrun.go for existing task dry-run and main.go cleaner.
- **Rationale**: Separates concerns. The impact analysis is a distinct feature from task config validation (dryrun.go).
- **Alternatives considered**: (a) Add to dryrun.go — rejected because dryrun.go handles existing task validation, not new-task impact. (b) Add inline in main.go — rejected because main.go is already very large.

## Decision 7: Cost Estimation
- **Decision**: Exclude cost estimation from scope. The issue mentions "Monthly Cost: +$15.00" but there is no cost model in the codebase.
- **Rationale**: No pricing data, no cost-per-run metric, no billing integration exists. Adding a cost model is a separate feature.
- **Alternatives considered**: (a) Add placeholder cost model — rejected as speculative and misleading.
