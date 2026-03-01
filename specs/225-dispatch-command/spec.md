# Feature Specification: Add `anvil dispatch` Command

**Feature Branch**: `225-dispatch-command`
**Created**: 2026-03-01
**Status**: Draft
**Input**: User description: "#343: Add `anvil dispatch` — synchronous add + wait + return result"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Synchronous task dispatch with result (Priority: P1)

An LLM agent or script wants to fire off an anvil task and get the result back in a single command without race conditions.

**Why this priority**: This is the primary use case - LLM-to-LLM delegation is the most common pattern for Claude Code sessions dispatching work to anvil workers.

**Independent Test**: Can be fully tested by running `anvil dispatch "task prompt"` and verifying it returns the output_summary within a reasonable time.

**Acceptance Scenarios**:

1. **Given** a running anvil daemon, **When** user runs `anvil dispatch "Review PR #42"`, **Then** the command creates a one-shot task, waits for completion, and prints the output_summary to stdout
2. **Given** the task completes successfully, **When** dispatch completes, **Then** exit code is 0
3. **Given** the task fails, **When** dispatch completes, **Then** exit code is 1

---

### User Story 2 - Fire and forget async dispatch (Priority: P2)

A user wants to dispatch a task without waiting for completion, getting only the task ID for later tracking.

**Why this priority**: Enables parallel task dispatching and background processing patterns.

**Independent Test**: Can be tested with `anvil dispatch --no-wait "long task"` and verifying it prints UUID immediately without waiting.

**Acceptance Scenarios**:

1. **Given** a running anvil daemon, **When** user runs `anvil dispatch --no-wait "long task"`, **Then** command prints task UUID immediately and exits with code 0
2. **Given** a dispatched task ID, **When** user runs `anvil task wait <id>`, **Then** they can track the task separately

---

### User Story 3 - Programmatic JSON output (Priority: P2)

A script needs structured data from dispatch for further processing.

**Why this priority**: Enables integration with external tools and CI/CD pipelines.

**Independent Test**: Can be tested with `anvil dispatch --json "task"` and parsing the JSON output.

**Acceptance Scenarios**:

1. **Given** a running anvil daemon, **When** user runs `anvil dispatch --json "task"`, **Then** output is valid JSON containing at least task_id, run_id, success, and output_summary fields

---

### User Story 4 - Configurable timeout (Priority: P3)

A user wants to set a maximum wait time before the dispatch gives up.

**Why this priority**: Prevents indefinite hanging for long-running or stuck tasks.

**Independent Test**: Can be tested by dispatching a long task with `--timeout 1s` and verifying exit code 2.

**Acceptance Scenarios**:

1. **Given** a task that takes longer than the timeout, **When** dispatch reaches the timeout, **Then** exit code is 2 and an appropriate timeout message is shown

---

### User Story 5 - Piped input support (Priority: P3)

A user wants to provide the task prompt from a file or stdin.

**Why this priority**: Supports complex prompts and integration with other command outputs.

**Independent Test**: Can be tested with `echo "prompt" | anvil dispatch -` or `anvil dispatch -f file.md`.

**Acceptance Scenarios**:

1. **Given** a prompt in a file, **When** user runs `anvil dispatch -f prompt.md`, **Then** the file content is used as the task prompt
2. **Given** stdin input, **When** user pipes content to `anvil dispatch -`, **Then** stdin content is used as the task prompt

---

### Edge Cases

- What happens when the anvil daemon is not running?
- How does the system handle dispatching to a non-existent priority?
- What happens when the task is cancelled mid-execution?
- How does dispatch behave when the project has no .anvil directory?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a new `dispatch` command accessible as `anvil dispatch`
- **FR-002**: The dispatch command MUST accept a task prompt as a positional argument
- **FR-003**: The dispatch command MUST create a one-shot task (equivalent to `anvil add --once`)
- **FR-004**: The dispatch command MUST capture the task UUID internally without requiring file parsing
- **FR-005**: The dispatch command MUST poll for task completion and return when finished
- **FR-006**: On success, dispatch MUST print output_summary to stdout and exit with code 0
- **FR-007**: On failure, dispatch MUST print error information to stderr and exit with code 1
- **FR-008**: The dispatch command MUST support a `--timeout` flag with a duration value (default: 30 minutes)
- **FR-009**: On timeout, dispatch MUST exit with code 2
- **FR-010**: The dispatch command MUST support a `--json` flag to output full RunRecord as JSON
- **FR-011**: The dispatch command MUST support a `--no-wait` flag to create task and return UUID immediately
- **FR-012**: The dispatch command MUST support a `--quiet` flag to suppress progress/status output
- **FR-013**: The dispatch command MUST inherit all flags from `anvil add` (e.g., `--skip-permissions`, `-f`, priority flags)
- **FR-014**: The dispatch command MUST support reading prompt from stdin via `-` argument
- **FR-015**: The dispatch command MUST support reading prompt from file via `-f <file>` argument

### Key Entities *(include if feature involves data)*

- **Task**: A unit of work dispatched to anvil, with a unique UUID and prompt content
- **RunRecord**: Execution record containing run_id, task_id, started time, finished time, success status, output_summary, and error information

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can complete a task dispatch and receive results in a single command with no race conditions
- **SC-002**: The dispatch command completes and returns output within 100ms overhead beyond actual task execution time
- **SC-003**: 100% of dispatched tasks return their correct output_summary to the caller
- **SC-004**: Timeout functionality correctly terminates waiting after specified duration
- **SC-005**: JSON output is valid and parseable by standard JSON parsers
- **SC-006**: The `--no-wait` flag returns the task UUID within 500ms of invocation
