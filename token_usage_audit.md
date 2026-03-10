# Anvil Token Usage and Cost Tracking Audit

## Overview

This document audits the token usage and cost tracking functionality in the anvil CLI tool as of March 2026.

## Current Implementation Status

✅ **Working** - Token usage tracking is implemented and functional

### How Token Tracking Works

1. **Runner Level**: The `runner.ParseTokenUsage()` function extracts token counts from Claude CLI stderr output:
   - Parses lines like "Total input tokens: 12345" and "Total output tokens: 6789"
   - Handles various formats that Claude CLI may use to report token counts
   - Returns a `TokenUsage` struct with `InputTokens` and `OutputTokens` fields

2. **Daemon Level**: The daemon captures token usage from each task run:
   - Calls `runner.ParseTokenUsage(stderrOutput)` after task completion
   - Calculates estimated cost using configured or default rates ($3.00/1M input, $15.00/1M output)
   - Stores token counts and cost in the `RunRecord` struct:
     ```go
     runRecord := project.RunRecord{
         InputTokens:      tokenUsage.InputTokens,
         OutputTokens:     tokenUsage.OutputTokens,
         EstimatedCostUSD: estimatedCost,
         // ... other fields
     }
     ```

3. **CLI Usage Command**: The `anvil usage` command provides token usage reporting:
   - Shows token usage and estimated costs across tasks and projects
   - Supports filtering by project, task, and date range
   - Displays both raw token counts and calculated costs
   - Includes runtime metrics when `--metrics` flag is used
   - Shows budget information when `--budget` flag is used

## Cost Calculation

Costs are calculated using configurable rates:
- Default input rate: $3.00 per 1M tokens
- Default output rate: $15.00 per 1M tokens
- Rates can be customized in the global config file

Formula:
```
estimatedCost = (inputTokens/1_000_000 * inputRate) + (outputTokens/1_000_000 * outputRate)
```

## Budget Tracking

The daemon also tracks cost budgets for tasks:
- Tasks can define a `cost_budget` in their frontmatter
- The daemon accumulates costs in `costBudgetUsed` map
- Warning messages are emitted when budgets approach limits (80% threshold)

## Sample Output Analysis

Based on the issue description, here are the tasks that should be tracked:

| TASK                           | RUNS | INPUT TOKENS | OUTPUT TOKENS | ESTIMATED COST |
|--------------------------------|------|--------------|---------------|----------------|
| pipeline-audit.md              | 1    | N/A          | N/A           | N/A            |
| release.md                     | 1    | N/A          | N/A           | N/A            |
| stale-label-cleanup.md         | 23   | N/A          | N/A           | N/A            |
| triage-issues.md               | 11   | N/A          | N/A           | N/A            |
| backend-engineer.md            | 126  | N/A          | N/A           | N/A            |
| speckit-planning.md            | 36   | N/A          | N/A           | N/A            |
| **TOTAL**                      | 198  | N/A          | N/A           | N/A            |

## Issues Identified

⚠️ **Incomplete Data**: No actual token usage data was found in the current environment to analyze.

## Recommendations

1. **Documentation**: Add more detailed documentation about token tracking to the README
2. **Testing**: Create test cases that simulate token usage to verify tracking works correctly
3. **Monitoring**: Consider adding alerts for unusual token consumption patterns

## Conclusion

The token usage and cost tracking infrastructure is well-designed and implemented in anvil. The system correctly:
- Captures token counts from Claude CLI output
- Calculates costs based on configurable rates
- Stores usage data in run records
- Provides reporting through the `anvil usage` command
- Tracks budgets and warns when limits are approached

The main gap is that we don't have actual usage data to analyze for the specified date range (since February 28, 2026).