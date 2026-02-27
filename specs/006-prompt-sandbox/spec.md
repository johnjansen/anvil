# Feature Specification: Task Execution Sandbox for Safe Prompt Testing

**Feature Branch**: `006-prompt-sandbox`
**Created**: 2026-02-27
**Status**: Draft
**Input**: GitHub Issue #276 — Add task execution sandbox for safe prompt testing

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run Prompt in Sandbox (Priority: P1)

A user developing a new task prompt wants to test it against the LLM without creating a run record, polluting history, or triggering hooks. They run `anvil prompt sandbox my-task` and see the raw LLM output along with token count, estimated cost, and execution time — giving them confidence before deploying the prompt into their scheduled task.

**Why this priority**: This is the core value of the feature. Without side-effect-free prompt execution, no other sandbox feature is useful. It enables safe iteration on prompts.

**Independent Test**: Can be fully tested by creating a task file, running `anvil prompt sandbox <task>`, and verifying the LLM responds, no run record is written, and no hooks fire.

**Acceptance Scenarios**:

1. **Given** a task file with frontmatter and content, **When** the user runs `anvil prompt sandbox <task>`, **Then** the LLM executes with the task's prompt content and outputs the response to stdout
2. **Given** a sandbox execution completes, **When** the output is displayed, **Then** it includes: raw LLM response text, input/output token counts, estimated cost in USD, and wall-clock execution time
3. **Given** a sandbox execution completes, **When** the user checks `.anvil/runs/`, **Then** no new run record file exists for this execution
4. **Given** a task with `on_success` or `on_failure` hooks, **When** the sandbox runs, **Then** no hooks are triggered
5. **Given** a task that does not exist, **When** the user runs `anvil prompt sandbox <nonexistent>`, **Then** a clear error message is shown

---

### User Story 2 - Compare Prompt Variations (Priority: P2)

A user wants to compare the output of different prompt variations side-by-side to choose the best one. They provide alternative prompt files via `--compare` and see a comparison summary showing each variation's output, token usage, and cost.

**Why this priority**: Comparison builds on top of basic sandbox execution and is a key workflow for iterating on prompt quality.

**Independent Test**: Can be tested by creating two variation files, running `anvil prompt sandbox my-task --compare v1.md v2.md`, and verifying both are executed and results are displayed together.

**Acceptance Scenarios**:

1. **Given** a task and two variation files, **When** the user runs `anvil prompt sandbox my-task --compare v1.md v2.md`, **Then** each variation is executed against the LLM and results are displayed sequentially with labels
2. **Given** a comparison run completes, **When** output is displayed, **Then** each variation shows: variation label/filename, response text (truncated if long), token count, cost estimate, and execution time
3. **Given** a variation file that does not exist, **When** the user runs `--compare` with it, **Then** a clear error message identifies the missing file

---

### User Story 3 - Watch Mode for Iterative Development (Priority: P3)

A user is actively editing a task prompt and wants instant feedback. They run `anvil prompt sandbox my-task --watch`, which monitors the task file for changes and automatically re-runs the sandbox on each save.

**Why this priority**: Watch mode is a convenience feature that accelerates the edit-test loop. It depends on the core sandbox being stable and is less critical for initial adoption.

**Independent Test**: Can be tested by starting watch mode, editing the task file, and verifying the sandbox re-runs automatically after save.

**Acceptance Scenarios**:

1. **Given** a user runs `anvil prompt sandbox my-task --watch`, **When** the task file is modified and saved, **Then** the sandbox automatically re-executes with the updated content within 2 seconds
2. **Given** watch mode is running, **When** the user presses Ctrl+C, **Then** watch mode exits cleanly
3. **Given** watch mode is running, **When** consecutive rapid saves occur, **Then** the system debounces to avoid overlapping executions

---

### Edge Cases

- What happens when the task has no content (empty prompt body)? Show error "task has no prompt content"
- How does the system handle runner failures (LLM not available, network errors)? Display the error from the runner and exit with non-zero code
- What happens when `--compare` is given zero variation files? Show usage help requiring at least 1 file
- What happens if the runner command is not installed? Display runner error message clearly
- What happens when a task uses `checkpoint: true` in sandbox mode? Checkpoint is ignored — sandbox doesn't persist any state
- How does sandbox handle tasks with `depends_on` dependencies? Dependencies are ignored — sandbox runs the prompt independently
- What happens if the task frontmatter specifies a custom runner? The configured runner is used, same as normal execution
- How does watch mode handle syntax errors in frontmatter after editing? Show parse error, continue watching for next valid save

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a `prompt sandbox` subcommand that executes a task's prompt content against the configured runner
- **FR-002**: Sandbox execution MUST NOT create any run record files in `.anvil/runs/`
- **FR-003**: Sandbox execution MUST NOT trigger `on_success`, `on_failure`, or `on_timeout_warning` hooks
- **FR-004**: Sandbox output MUST include: raw LLM response, input token count, output token count, estimated cost in USD, and wall-clock execution time
- **FR-005**: System MUST load the task by name from the current project, resolving frontmatter and content identically to normal execution
- **FR-006**: System MUST use the task's configured runner (or fall back to the default runner chain) for sandbox execution
- **FR-007**: System MUST support `--compare <file1> <file2> ...` to execute multiple prompt variations and display results for each
- **FR-008**: Each `--compare` variation file provides replacement prompt content (the file body replaces the task's content for that run)
- **FR-009**: System MUST support `--watch` flag that monitors the task file for changes and re-runs the sandbox automatically
- **FR-010**: Watch mode MUST debounce file change events to prevent overlapping executions (minimum 500ms between re-runs)
- **FR-011**: System MUST display a clear error when the specified task does not exist
- **FR-012**: System MUST support `--json` flag to output sandbox results in machine-readable format
- **FR-013**: Sandbox execution MUST NOT count against persistent task budgets
- **FR-014**: Sandbox execution MUST respect the task's `skip_permissions` and `allowed_tools` settings

### Key Entities

- **SandboxResult**: Represents the outcome of a single sandbox execution — includes response text, token counts (input/output), estimated cost, execution duration, and variation label (if applicable)
- **PromptVariation**: A file whose body content replaces the task's default prompt for a comparison run

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can execute a task prompt in sandbox mode and see results within the same timeframe as a normal run (no meaningful overhead)
- **SC-002**: Sandbox mode produces zero side effects — no run records, no hooks, no budget consumption
- **SC-003**: Users can compare at least 2 prompt variations in a single command and see output for each
- **SC-004**: Watch mode detects file changes and re-runs within 2 seconds of save
- **SC-005**: All sandbox output modes (text, JSON) include token counts and cost estimates
