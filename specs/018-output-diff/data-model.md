# Data Model: Task Output Diffing

## Existing Entities (No Changes)

### RunRecord (internal/project/project.go)

Already contains all needed fields:
- `RunID string` - UUID identifying the run
- `Started time.Time` - When run started
- `Finished time.Time` - When run ended
- `Success bool` - Whether run succeeded
- `OutputSummary string` - First/last N lines of output
- `Error string` - Error message if failed

## New Entities

### DiffHunk (cmd/anvil/outputdiff.go)

Represents a single hunk in the diff output:
- `OldStart int` - Starting line in old output
- `OldCount int` - Number of lines from old output
- `NewStart int` - Starting line in new output
- `NewCount int` - Number of lines from new output
- `Lines []DiffLine` - Individual line changes

### DiffLine (cmd/anvil/outputdiff.go)

Represents a single line in a diff hunk:
- `Type string` - "context", "added", or "removed"
- `Content string` - The line content

### RunMeta (cmd/anvil/outputdiff.go)

Run metadata for JSON output:
- `RunID string` - Run identifier
- `Started time.Time` - When run started
- `Success bool` - Whether run succeeded

### OutputDiffResult (cmd/anvil/outputdiff.go)

Top-level JSON output structure:
- `Run1 RunMeta` - Metadata for the first (older) run
- `Run2 RunMeta` - Metadata for the second (newer) run
- `Identical bool` - True if outputs are the same
- `Hunks []DiffHunk` - The diff hunks (empty if identical)

## Storage

No new storage. Uses existing RunRecord JSON files at `.anvil/runs/<task-id>/<run-id>.json`.
