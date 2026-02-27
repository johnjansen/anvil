# Feature Specification: Fix Raw Mode Line Break Output

**Feature Branch**: `001-fix-raw-mode-linebreaks`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Fix broken line breaks in daemon terminal output when running in raw terminal mode"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Daemon foreground output displays correctly (Priority: P1)

A developer runs `anvil watch` (or `anvil serve`) in foreground mode. The daemon enters raw terminal mode to listen for hot-daemonize keypress. All scheduler tick messages, worker status lines, and task completion messages display on separate lines starting at column 0, with no horizontal drift or concatenation.

**Why this priority**: This is the core bug. Every user running the daemon in foreground sees corrupted output, making it unreadable. This has been reported and "fixed" multiple times but never actually resolved.

**Independent Test**: Run the daemon in foreground mode with at least one recurring task. Observe that every log line begins at column 0 with no horizontal offset.

**Acceptance Scenarios**:

1. **Given** the daemon is running in foreground with raw terminal mode active, **When** a scheduler tick message is logged, **Then** it appears on a new line starting at column 0
2. **Given** the daemon is running in foreground with raw terminal mode active, **When** a worker picks up, completes, or fails a task, **Then** the status message appears on a new line starting at column 0
3. **Given** the daemon is running in foreground with raw terminal mode active, **When** multiple workers log messages concurrently, **Then** each message appears on its own line with no interleaving or horizontal drift

---

### User Story 2 - Non-raw-mode output remains unaffected (Priority: P1)

When the daemon runs daemonized (background mode) or when stdout is not a TTY (e.g., piped to a file), output continues to work exactly as before with standard `\n` line endings.

**Why this priority**: The fix must not regress normal (non-raw-mode) daemon operation, which is the more common deployment mode.

**Independent Test**: Run the daemon daemonized and verify log file output uses standard newlines with no `\r` characters injected.

**Acceptance Scenarios**:

1. **Given** the daemon is running in background (daemonized) mode, **When** log messages are written, **Then** they use standard `\n` line endings (no `\r\n`)
2. **Given** the daemon stdout is piped to a file, **When** log messages are written, **Then** the file contains standard `\n` line endings

---

### Edge Cases

- What happens if the terminal is restored to cooked mode mid-session (e.g., hot-daemonize triggers)? Output should revert to standard `\n`.
- What happens if multiple goroutines call the raw mode toggle concurrently? The flag must be protected by the existing mutex.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The daemon logger MUST produce `\r\n` line endings when raw terminal mode is active, so that output lines start at column 0
- **FR-002**: The daemon logger MUST produce standard `\n` line endings when raw terminal mode is NOT active (default behavior)
- **FR-003**: The raw mode flag MUST be toggled from the same code path that already handles `term.MakeRaw()` and `term.Restore()`, ensuring consistency with the existing `log.SetOutput()` wrapper
- **FR-004**: The raw mode toggle MUST be thread-safe, protected by the daemon logger's existing mutex

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All daemon log lines in foreground (raw terminal) mode begin at column 0 with no horizontal offset
- **SC-002**: Existing daemon log file output contains no spurious `\r` characters when running in background mode
- **SC-003**: Existing unit tests continue to pass with no modifications required
- **SC-004**: The fix touches only the daemon logger and the raw-mode setup code — no architectural changes to logging infrastructure
