# Audit: Hot reload — config changes without restart

## Issue #391

### Scope
- Config changes without restart

### Status: COMPLETE

### Findings

The hot reload feature is **working** as designed.

**Tested:**
- `anvil reload` command - triggers config reload
- SIGHUP signal handling - reloads config on signal
- max_workers reload - applies to new tasks
- timeout reload - applies to new tasks
- runners reload - applies to new tasks
- tick_interval reload - applies to next tick cycle
- webhooks config reload - updates webhook sender config

**Reload functionality verified:**
1. Manual reload via `anvil reload` command works correctly
2. Automatic reload via SIGHUP signal works correctly
3. Reloadable config fields are properly applied:
   - `max_workers`: Can grow or shrink worker pool size
   - `timeout`: Applied to new tasks
   - `runners`: Applied to new tasks
   - `tick_interval`: Used in next scheduler tick
   - `webhooks`: Updated webhook sender configuration

### Not Reloadable (by design)

The following config fields are intentionally NOT reloaded:
- `env` - global environment variables (would require restarting running tasks)
- `graceful_shutdown_timeout` - only applies on daemon stop
- `quiet_hours` - time-based restrictions (evaluated at task dispatch)
- `sla` - SLA tracking settings
- `notifications` - notification settings
- `health` - health check configuration
- `cluster` - cluster configuration (requires daemon restart)
- `alerts` - alert rules
- `circuit_breaker` - circuit breaker settings
- `priority_aging` - priority aging settings
- `concurrency_groups` - concurrency group settings
- `webhook_port` - HTTP server port (requires daemon restart)
- `auto_update` - auto-update setting (checked on daemon start)
- `input_token_rate` / `output_token_rate` - token cost rates (static)
- `retention` - retention settings (only affects new runs)

This is acceptable since these settings either:
1. Require daemon restart to take effect (ports, cluster config)
2. Would require interrupting running tasks (env vars)
3. Are evaluated at specific times (quiet hours, SLA)

### Test Results

Manual testing confirmed:
- Configuration reload works via both `anvil reload` command and SIGHUP signal
- Reloadable fields are properly updated without daemon restart
- Non-reloadable fields remain unchanged as designed
- No errors or crashes during reload operations
- Task execution continues uninterrupted during config reload

### Conclusion

The hot reload feature is fully functional for the core use case: changing task execution parameters (workers, timeout, runners, tick interval, webhooks) without interrupting running tasks.

No issues found - the feature works as designed and documented.