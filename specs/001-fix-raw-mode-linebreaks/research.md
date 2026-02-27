# Research: Fix Raw Mode Line Break Output

**Date**: 2026-02-27
**Feature**: 001-fix-raw-mode-linebreaks

## Research Topic 1: Why bare `\n` fails in raw terminal mode

**Decision**: Use `\r\n` in daemon logger when raw mode is active.

**Rationale**: When `term.MakeRaw()` is called, it disables OPOST (output post-processing) in the terminal driver. OPOST is responsible for translating `\n` (line feed) to `\r\n` (carriage return + line feed) on output. Without OPOST:
- `\n` moves the cursor down one line but does NOT return to column 0
- Each successive line starts at whatever column the previous line ended at
- This creates the "horizontal drift" pattern visible in the bug report

The existing code already handles this for `log.Printf()` output by wrapping `log.SetOutput()` with a `rawLineWriter` that does `bytes.ReplaceAll(p, []byte("\n"), []byte("\r\n"))`. The daemon logger bypasses this entirely because it writes directly to `os.Stdout`.

**Alternatives considered**:
1. **Channel-based writer**: Overly complex for the actual problem. The mutex-based synchronization is correct — the issue is purely missing `\r`, not a race condition.
2. **Wrap os.Stdout globally**: Too broad — would affect all stdout writes including those that already handle `\r\n` (like `writeRaw()`), causing double `\r\r\n`.
3. **Re-enable OPOST selectively**: Not possible with `x/term` API — `MakeRaw` sets all raw mode flags together.

## Research Topic 2: Thread safety of the rawMode flag

**Decision**: Protect `rawMode` with the existing `daemonLogger.mu` mutex.

**Rationale**: The `rawMode` flag is read inside `println()` which already holds `l.mu.Lock()`. The setter (`SetRawMode`) should also acquire the mutex. Since `SetRawMode` is called exactly twice (enter raw mode, restore terminal) and `println` is the only reader, the existing mutex is sufficient. No additional synchronization needed.

**Alternatives considered**:
1. **atomic.Bool**: Slightly faster for reads but unnecessary — `println()` already holds the mutex, so checking a field under that lock adds zero overhead.
2. **Separate mutex**: Unnecessary complexity for a flag that changes twice in the process lifetime.

## Research Topic 3: Why previous fixes failed

**Decision**: Previous fixes only wrapped `log.SetOutput()` — they never touched `dlog.println()`.

**Rationale**: The codebase has two distinct output paths:
1. `log.Printf()` — used by the runner package and some CLI code. Goes through Go's standard `log` package, which was correctly wrapped with `rawLineWriter`.
2. `dlog.println()` — used by ALL daemon output (scheduler ticks, worker status, warnings, etc.). Writes directly to `os.Stdout` via `fmt.Fprintln`. This path was **never** wrapped.

Since ~95% of daemon output goes through `dlog`, the previous fix to `log.SetOutput` only fixed ~5% of the output. The core symptom persisted.

## Summary

No NEEDS CLARIFICATION items remain. The fix is well-understood:
1. Add `rawMode bool` field to `daemonLogger` struct
2. Add exported `SetRawMode(bool)` method on `dlog`
3. In `println()`, write `line + "\r\n"` when `rawMode` is true, otherwise `fmt.Fprintln` (unchanged)
4. Call `dlog.SetRawMode(true)` in `serveCmd()` after `term.MakeRaw()`
5. Call `dlog.SetRawMode(false)` in the defer that calls `term.Restore()`
