# Research: Task Execution Sandbox

## Decision 1: CLI Command Structure

**Decision**: Use `anvil prompt sandbox <task>` as a new top-level `prompt` subcommand with `sandbox` sub-subcommand.

**Rationale**: The issue explicitly uses `anvil prompt sandbox` syntax. Using a new `prompt` command group allows future prompt-related commands (e.g., `anvil prompt lint`, `anvil prompt format`) without cluttering the `task` namespace.

**Alternatives considered**:
- `anvil task sandbox <task>` — would add to the already-large `task` subcommand set (25+ subcommands)
- `anvil sandbox <task>` — too generic, doesn't convey the prompt-testing purpose

## Decision 2: Execution Approach (CLI-direct vs Daemon)

**Decision**: Execute the runner directly from the CLI process, bypassing the daemon entirely.

**Rationale**: The sandbox must NOT create run records, trigger hooks, or consume budgets. The daemon's `runTask` method is tightly coupled to all of these side effects. Calling `Runner.Run()` directly from the CLI avoids this entirely and is simpler to implement. The daemon does not need to be running for sandbox mode.

**Alternatives considered**:
- Adding a `sandbox: true` flag to daemon's RunRequest — would require threading sandbox awareness through the entire dispatch pipeline
- Daemon `/sandbox` endpoint — unnecessary complexity for a single-user CLI tool

## Decision 3: Token/Cost Extraction

**Decision**: Use the existing `runner.ParseTokenUsage()` function on stderr output from the runner, then calculate cost using config rates (default $3/1M input, $15/1M output).

**Rationale**: This is exactly how the daemon already computes cost. Reusing the same code path ensures consistent numbers.

**Alternatives considered**: None — the existing approach is clean and well-tested.

## Decision 4: Compare Mode Implementation

**Decision**: Run variations sequentially (one at a time), collecting results, then display all results together at the end.

**Rationale**: Sequential execution is simpler and avoids concurrent runner processes. LLM calls are the bottleneck anyway, and parallel execution would complicate output capture.

**Alternatives considered**:
- Parallel execution with goroutines — adds complexity for minimal benefit since LLM calls are rate-limited
- Side-by-side terminal output — too complex for initial implementation, hard to read with long responses

## Decision 5: Watch Mode Implementation

**Decision**: Use `os.Stat()` polling with 1-second interval and mtime comparison. Debounce with 500ms minimum gap.

**Rationale**: Polling is simple, portable, and sufficient for single-file watching. The fsnotify library would add an external dependency for minimal benefit.

**Alternatives considered**:
- `fsnotify` library — external dependency, adds complexity
- `inotify` directly — Linux-only, not portable to macOS

## Decision 6: Output Format

**Decision**: Default to human-readable text with clear sections (response, stats). Support `--json` for machine-readable output.

**Rationale**: Matches existing anvil patterns (e.g., `anvil ps --json`, `anvil task get --json`).

**Alternatives considered**: None — this is the established project pattern.

## Decision 7: Session ID and Log Handling

**Decision**: Use a temporary session ID (prefixed `sandbox-`) and write logs to a temp directory that is cleaned up after execution. Do not persist session data.

**Rationale**: The runner requires a session ID and log directory. Using temp paths ensures no artifacts remain after sandbox completes. This aligns with the "zero side effects" requirement.

**Alternatives considered**:
- No logging at all — runner.Run requires a log directory parameter
- Keep sandbox logs in `.anvil/sandbox/` — contradicts zero side effects goal
