# Research: Task Output Diffing

## Decision 1: Command Name and Routing

**Decision**: Extend existing `anvil task diff` command to handle both file version diffs and run output diffs.

**Rationale**: The issue spec explicitly requests `anvil task diff my-task`. The existing `taskDiffCmd` requires 2+ args (`<name> <v1> [v2]`), so when called with just a task name (1 arg) or with `--run1`/`--run2` flags, it routes to output diffing. This avoids creating a confusing separate subcommand.

**Alternatives considered**:
- `anvil task output-diff`: More explicit but doesn't match issue spec and adds unnecessary verbosity
- Separate `taskOutputDiffCmd`: Would duplicate routing logic

## Decision 2: Diff Algorithm

**Decision**: Reuse existing `project.UnifiedDiff()` from `internal/project/diff.go`.

**Rationale**: The function already implements LCS-based unified diff with 3-line context and standard hunk headers. No need to implement a new algorithm.

**Alternatives considered**:
- External diff library: Adds dependency for no benefit
- Custom algorithm: project.UnifiedDiff already works well

## Decision 3: Output Data Source

**Decision**: Use `RunRecord.OutputSummary` field for diff comparison.

**Rationale**: OutputSummary is already captured by the daemon (first 3 + last 3 lines, or full output if <= 6 lines). This is the only output data persisted in run records.

**Alternatives considered**:
- Full session logs: Too verbose and contain internal Claude session data
- Checkpoint data: Different purpose, not output comparison

## Decision 4: Run Selection

**Decision**: Default to comparing the two most recent runs. Support `--run1` and `--run2` for specific runs with prefix matching.

**Rationale**: `ReadAllRunRecords()` returns records sorted newest-first, making it easy to pick the last two. Prefix matching on UUIDs is user-friendly.

## Decision 5: Ignore Whitespace

**Decision**: Implement by trimming whitespace from each line before comparison when `--ignore-whitespace` is set.

**Rationale**: Simple and effective. Applied by pre-processing lines before passing to UnifiedDiff.

## Decision 6: JSON Output Format

**Decision**: JSON output includes run metadata for both runs plus the diff as an array of hunks with line changes.

**Rationale**: Structured format enables programmatic consumption. Hunk-based structure mirrors the visual unified diff.

## Decision 7: Implementation Location

**Decision**: Create new file `cmd/anvil/outputdiff.go` for the output diff logic, modify `taskDiffCmd` in main.go to route appropriately.

**Rationale**: Keeps the new logic self-contained. Pattern follows activity.go and impact.go.
