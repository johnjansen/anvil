# Feature Specification: Task Runbook Linking

**Feature Branch**: `[012-task-runbook]`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Add task runbook linking for self-healing failed tasks"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Inline Runbook Storage (Priority: P1)

User wants to store troubleshooting instructions directly in the task configuration file so they don't need to maintain separate documentation.

**Why this priority**: Core feature - users need a way to embed runbook content in task files.

**Independent Test**: Can be tested by creating a task with inline runbook content and verifying it renders correctly.

**Acceptance Scenarios**:

1. **Given** a task file with `runbook:` field containing markdown, **When** `anvil task get <name>` is run, **Then** the runbook content is displayed in the output
2. **Given** a task file with empty `runbook:` field, **When** task is queried, **Then** no runbook section is shown

---

### User Story 2 - URL Runbook Reference (Priority: P1)

User wants to link to external documentation (wiki, Notion, GitHub) for troubleshooting.

**Why this priority**: Common use case - teams have existing documentation they want to reference.

**Independent Test**: Can be tested by adding a URL runbook to a task and verifying the link is clickable/displayed.

**Acceptance Scenarios**:

1. **Given** a task with `runbook: "https://wiki.example.com/runbooks/data-fetch"`, **When** task fails, **Then** the runbook URL is shown in the error output
2. **Given** a task with an invalid URL runbook, **When** user attempts to open it, **Then** appropriate error is shown

---

### User Story 3 - Dedicated Runbook CLI Command (Priority: P1)

User wants a dedicated command to view task runbooks.

**Why this priority**: Provides clear UX for accessing runbook content.

**Independent Test**: Can be tested by running `anvil task runbook <name>` and verifying output.

**Acceptance Scenarios**:

1. **Given** a task with runbook defined, **When** `anvil task runbook <name>` is run, **Then** the runbook content is displayed
2. **Given** a task without runbook, **When** `anvil task runbook <name>` is run, **Then** helpful message is shown

---

### User Story 4 - Auto-Suggest on Failure (Priority: P2)

When a task fails, automatically display the runbook link in the error output.

**Why this priority**: Reduces friction - users don't need to remember to check runbooks manually.

**Independent Test**: Can be tested by running a failing task and verifying runbook is displayed.

**Acceptance Scenarios**:

1. **Given** a task with runbook that fails, **When** task completes with failure status, **Then** runbook link/content is included in output
2. **Given** a task without runbook that fails, **When** task completes with failure status, **Then** no runbook section is shown

---

### User Story 5 - Open Runbook in Browser (Priority: P3)

User wants to open the runbook URL directly from CLI.

**Why this priority**: Convenience - one-click access to full documentation.

**Independent Test**: Can be tested with a URL-based runbook.

**Acceptance Scenarios**:

1. **Given** a task with URL runbook, **When** `anvil task runbook <name> --open` is run, **Then** URL opens in default browser
2. **Given** a task with inline runbook, **When** `anvil task runbook <name> --open` is run, **Then** error message explains inline runbooks can't be opened

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Tasks MUST support `runbook` field in frontmatter (string - either URL or inline markdown)
- **FR-002**: `anvil task get <name>` MUST display runbook content if defined
- **FR-003**: `anvil task runbook <name>` command MUST show runbook content
- **FR-004**: Failed task output MUST include runbook link/content
- **FR-005**: Inline runbooks MUST be rendered as markdown in terminal
- **FR-006**: URL runbooks MUST be clickable in terminal output (if terminal supports it)
- **FR-007**: `anvil task runbook <name> --open` MUST open URL in browser (URL runbooks only)

### Key Entities

- **Runbook**: Either a URL string or inline markdown content stored in task frontmatter
- **TaskConfig**: Existing config struct needs `Runbook` field added

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can define runbook in task frontmatter and view it via CLI
- **SC-002**: Failed task output automatically includes runbook information
- **SC-003**: Both URL and inline runbook types are supported
