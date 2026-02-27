# PLAN.md - Task Graceful Cancellation with Partial Results

## Implementation Approach

This feature modifies the existing kill flow to support graceful cancellation with partial result capture. The implementation follows these phases:

### Phase 1: Data Model Extensions

1. **Update Todo struct** - Add `OnKill` and `GracefulTimeout` fields
2. **Update RunRecord struct** - Add `PartialResults`, `KillReason`, `ExitCode` fields
3. **Add KillRequest struct** - Track kill requests with graceful/force flags
4. **Add ParsePartialResults function** - Extract `##anvil:partial` from output

### Phase 2: CLI Updates

1. **Modify task_kill.go** - Add --graceful, --force, --timeout flags
2. **Create task_partial.go** - New command to show partial results
3. **Modify task_run.go** - Add --resume flag to pass partial data

### Phase 3: Daemon Integration

1. **Add kill request handling** - Queue and process kill requests
2. **Add hook execution** - Run on_kill hook before termination
3. **Add partial capture** - Extract and store partial results

### Phase 4: Runner Integration

1. **Add SIGTERM handling** - Graceful shutdown on signal
2. **Add ANVIL_IS_KILLED** - Environment variable for subprocess
3. **Add partial output parsing** - Detect and emit partial results

## Critical Files

### Modified Files
- `internal/project/project.go` - Data models
- `cmd/anvil/task_kill.go` - CLI kill command
- `cmd/anvil/task_run.go` - CLI run command (add resume)
- `internal/daemon/daemon.go` - Kill handling, hook execution
- `internal/runner/runner.go` - Signal handling, env vars

### New Files
- `cmd/anvil/task_partial.go` - Partial results viewer

## Dependencies

- Uses existing `internal/cron` for timeout handling
- Uses existing project config loading in `internal/project/loader.go`
- Uses existing daemon IPC mechanism

## Backward Compatibility

- Default behavior remains immediate kill (--force)
- Adding --graceful makes it opt-in
- No changes to existing run record format (optional fields added)

## Testing Strategy

1. Unit tests for ParsePartialResults
2. Integration tests for kill flow
3. Manual testing with sample task that emits partial results
