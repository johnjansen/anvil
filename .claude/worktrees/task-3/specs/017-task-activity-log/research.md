# Research: Task Activity Log

## Decision 1: Storage Format
- **Decision**: Use JSONL (JSON Lines) format — one JSON object per line in a single file per task.
- **Rationale**: JSONL is append-friendly (no need to read/parse the whole file to add an entry), supports filtering by reading line-by-line, and is compact. Follows the project's existing JSON-based storage pattern.
- **Alternatives considered**: (a) Individual JSON files per event (like RunRecord) — rejected because tasks could generate thousands of events, creating too many files. (b) SQLite — rejected as over-engineering and adding a dependency.

## Decision 2: Storage Location
- **Decision**: Store activity logs at .anvil/activities/<task-id>.jsonl
- **Rationale**: Flat file per task-id (not nested directories) keeps it simple. Uses task-id (UUID) as the file name to be consistent with runs/ directory pattern.
- **Alternatives considered**: (a) .anvil/activities/<task-id>/<timestamp>.json — rejected for too many files. (b) Single activities.jsonl for all tasks — rejected because filtering by task would require reading everything.

## Decision 3: Where to Add Logging Calls
- **Decision**: Add a WriteActivity() function in project.go and call it from the existing code locations identified in research. Logging calls go in:
  - project.go AddTodo() — task created
  - daemon.go runTask() — run started/completed
  - main.go taskPauseCmd()/taskResumeCmd() — paused/resumed
  - main.go taskEditCmd() — edited (with field changes)
  - daemon.go handleKill() — killed
  - main.go taskUnlockCmd() — unlocked
  - daemon.go handleRun() — force-run
- **Rationale**: Minimal changes, each logging call is at the exact point where the action occurs.
- **Alternatives considered**: (a) Event bus/middleware pattern — rejected as over-engineering for a CLI tool. (b) Filesystem watcher — rejected as unreliable and complex.

## Decision 4: Activity Entry Fields
- **Decision**: Each entry has: Timestamp, Action, TaskID, TaskName, Details (map[string]string for flexible key-value pairs).
- **Rationale**: map[string]string for Details keeps it simple and extensible. Each action type can put whatever info is relevant.
- **Alternatives considered**: (a) Strongly typed detail structs per action — rejected as too many types for simple data. (b) Free-text details — rejected because structured data is needed for filtering and export.

## Decision 5: CLI Command Structure
- **Decision**: Add "activity" case to the existing taskCmd() dispatcher. Implement taskActivityCmd() in a new cmd/anvil/activity.go file.
- **Rationale**: Follows existing pattern (history, diff, restore, blame, dry-run are all under "task" subcommand).
- **Alternatives considered**: None — this is the obvious choice given the codebase pattern.

## Decision 6: Display Limit
- **Decision**: Default limit of 100 entries, adjustable with --limit flag.
- **Rationale**: Prevents overwhelming terminal output for tasks with long histories while still showing recent activity by default.
- **Alternatives considered**: (a) No limit — could produce thousands of lines. (b) Pagination — over-engineering for CLI.

## Decision 7: Edit Change Tracking
- **Decision**: For edit events, capture old and new values of changed fields by reading the task file before and after the edit operation.
- **Rationale**: This gives the most accurate diff of what changed. The edit command already reads the file, so capturing "before" values is cheap.
- **Alternatives considered**: (a) Only log which fields changed (not values) — rejected because seeing old/new values is needed for auditing.
